package main

import (
	"errors"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func closeFixture(t *testing.T) (*CloseService, *atomic.Int64, *atomic.Int64) {
	t.Helper()
	emitted, closed := new(atomic.Int64), new(atomic.Int64)
	s, err := NewCloseService(
		func(string) bool { emitted.Add(1); return false },
		func() { closed.Add(1) },
	)
	if err != nil {
		t.Fatal(err)
	}
	return s, emitted, closed
}

func TestCloseService_MissingOrWrongApprovalNeverCloses(t *testing.T) {
	s, emitted, closed := closeFixture(t)
	for _, id := range []string{"", "arbitrary", " stale "} {
		if err := s.ConfirmClose(id); !errors.Is(err, errNoPendingClose) {
			t.Fatalf("missing confirm: %v", err)
		}
		if err := s.CancelClose(id); !errors.Is(err, errNoPendingClose) {
			t.Fatalf("missing cancel: %v", err)
		}
	}
	if s.GetPendingCloseRequest() != "" || emitted.Load() != 0 || closed.Load() != 0 {
		t.Fatal("missing request changed state or closed")
	}
	if err := s.requestClose(); err != nil {
		t.Fatal(err)
	}
	nonce := s.GetPendingCloseRequest()
	for _, id := range []string{"", "arbitrary", nonce + " ", strings.ToUpper(nonce)} {
		// The random prefix might happen to contain no letters.
		if id == nonce {
			continue
		}
		if err := s.ConfirmClose(id); !errors.Is(err, errInvalidCloseRequest) {
			t.Fatalf("wrong confirm: %v", err)
		}
		if err := s.CancelClose(id); !errors.Is(err, errInvalidCloseRequest) {
			t.Fatalf("wrong cancel: %v", err)
		}
	}
	if s.GetPendingCloseRequest() != nonce || emitted.Load() != 1 || closed.Load() != 0 {
		t.Fatal("wrong approval changed pending request or closed")
	}
}

func TestCloseService_NativeHookCancelsAndRetainsMissedEvent(t *testing.T) {
	var s *CloseService
	var emitted []string
	var closed int
	var err error
	s, err = NewCloseService(func(nonce string) bool {
		// Event callbacks can reenter without deadlocking.
		if s.GetPendingCloseRequest() != nonce {
			t.Fatal("event did not carry the exact stored string nonce")
		}
		emitted = append(emitted, nonce)
		return false
	}, func() { closed++ })
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		event := application.NewWindowEvent()
		s.handleWindowClosing(event)
		if !event.IsCancelled() {
			t.Fatal("native WindowClosing was allowed")
		}
	}
	if len(emitted) != 1 || emitted[0] == "" || s.GetPendingCloseRequest() != emitted[0] || closed != 0 {
		t.Fatal("repeated native close changed nonce, re-emitted competing request or exited")
	}
	for i := 0; i < 5; i++ {
		if s.shouldQuit() {
			t.Fatal("application quit bypassed missing frontend approval")
		}
	}
	if len(emitted) != 1 || closed != 0 {
		t.Fatal("application quit emitted competing requests or exited")
	}
}

func TestCloseService_CancelRetryStaleAndDuplicate(t *testing.T) {
	s, emitted, closed := closeFixture(t)
	if err := s.requestClose(); err != nil {
		t.Fatal(err)
	}
	first := s.GetPendingCloseRequest()
	if err := s.CancelClose(first); err != nil {
		t.Fatal(err)
	}
	if err := s.CancelClose(first); !errors.Is(err, errNoPendingClose) {
		t.Fatalf("duplicate cancellation accepted: %v", err)
	}
	if s.GetPendingCloseRequest() != "" || closed.Load() != 0 {
		t.Fatal("cancel did not clear request or accidentally closed")
	}
	if err := s.requestClose(); err != nil {
		t.Fatal(err)
	}
	second := s.GetPendingCloseRequest()
	if second == "" || second == first || emitted.Load() != 2 {
		t.Fatal("retry reused cancelled identity")
	}
	if err := s.ConfirmClose(first); !errors.Is(err, errInvalidCloseRequest) {
		t.Fatalf("stale confirm accepted: %v", err)
	}
	if err := s.CancelClose(first); !errors.Is(err, errInvalidCloseRequest) {
		t.Fatalf("stale cancel accepted: %v", err)
	}
	if s.GetPendingCloseRequest() != second || closed.Load() != 0 {
		t.Fatal("stale request affected current request")
	}
	if err := s.ConfirmClose(second); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmClose(second); !errors.Is(err, errCloseAlreadyApproved) {
		t.Fatalf("duplicate confirmation accepted: %v", err)
	}
	if err := s.CancelClose(second); !errors.Is(err, errCloseAlreadyApproved) {
		t.Fatalf("cancel after approval accepted: %v", err)
	}
	if err := s.requestClose(); err != nil {
		t.Fatal(err)
	}
	event := application.NewWindowEvent()
	s.handleWindowClosing(event)
	if !event.IsCancelled() || !s.shouldQuit() || s.GetPendingCloseRequest() != "" ||
		closed.Load() != 1 || emitted.Load() != 2 {
		t.Fatal("approval did not consume exactly one request and quit once")
	}
}

func TestCloseService_ApprovalBeforeReentrantNativeQuit(t *testing.T) {
	var s *CloseService
	var closed int
	var err error
	s, err = NewCloseService(func(string) bool { return false }, func() {
		if !s.shouldQuit() || s.GetPendingCloseRequest() != "" {
			t.Fatal("native Quit cannot observe consumed approval")
		}
		closed++
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.requestClose(); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmClose(s.GetPendingCloseRequest()); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatal("approved action was not executed exactly once")
	}
}

func TestCloseService_EventCancellationAndIdentityExhaustionFailClosed(t *testing.T) {
	s, err := newCloseService("fixture", func(string) bool { return true }, func() {
		t.Fatal("unapproved exit")
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.requestClose(); err == nil || !strings.Contains(err.Error(), "event was cancelled") {
		t.Fatalf("event cancellation was silent: %v", err)
	}
	if s.GetPendingCloseRequest() == "" || s.shouldQuit() {
		t.Fatal("event cancellation lost request or allowed quit")
	}
	if err := s.CancelClose(s.GetPendingCloseRequest()); err != nil {
		t.Fatal(err)
	}
	s.sequence = math.MaxUint64
	if err := s.requestClose(); err == nil || !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("identity exhaustion was silent: %v", err)
	}
	event := application.NewWindowEvent()
	s.handleWindowClosing(event)
	if !event.IsCancelled() || s.GetPendingCloseRequest() != "" || s.shouldQuit() {
		t.Fatal("identity failure permitted automatic close")
	}
}

func TestCloseService_ConcurrentRequestsAndConfirmation(t *testing.T) {
	s, emitted, closed := closeFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.requestClose(); err != nil {
				t.Error(err)
			}
			_ = s.GetPendingCloseRequest()
		}()
	}
	wg.Wait()
	nonce := s.GetPendingCloseRequest()
	if nonce == "" || emitted.Load() != 1 || closed.Load() != 0 {
		t.Fatal("concurrent requests did not coalesce")
	}
	var approvals atomic.Int64
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.ConfirmClose(nonce); err == nil {
				approvals.Add(1)
			} else if !errors.Is(err, errCloseAlreadyApproved) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if approvals.Load() != 1 || closed.Load() != 1 || s.GetPendingCloseRequest() != "" {
		t.Fatal("concurrent confirmation permitted duplicate native exit")
	}
}

func TestCloseService_CancelConfirmationRace(t *testing.T) {
	for i := 0; i < 32; i++ {
		s, _, closed := closeFixture(t)
		if err := s.requestClose(); err != nil {
			t.Fatal(err)
		}
		nonce := s.GetPendingCloseRequest()
		results := make(chan error, 2)
		go func() { results <- s.CancelClose(nonce) }()
		go func() { results <- s.ConfirmClose(nonce) }()
		first, second := <-results, <-results
		if (first == nil) == (second == nil) || closed.Load() > 1 || s.GetPendingCloseRequest() != "" {
			t.Fatalf("cancel/confirm race did not choose one winner: %v %v", first, second)
		}
		if closed.Load() == 0 {
			if err := s.requestClose(); err != nil {
				t.Fatal(err)
			}
			if s.GetPendingCloseRequest() == nonce {
				t.Fatal("cancel race reused retired nonce")
			}
		}
	}
}

func TestCloseService_WailsHookSignatureAndDependencyValidation(t *testing.T) {
	// Compile against beta.26's real hook and callback types without launching.
	var register func(events.WindowEventType, func(*application.WindowEvent)) func() = (*application.WebviewWindow)(nil).RegisterHook
	if register == nil {
		t.Fatal("missing native hook")
	}
	for _, test := range []struct {
		prefix string
		emit   func(string) bool
		quit   func()
	}{
		{"", func(string) bool { return false }, func() {}},
		{"fixture", nil, func() {}},
		{"fixture", func(string) bool { return false }, nil},
	} {
		if _, err := newCloseService(test.prefix, test.emit, test.quit); err == nil {
			t.Fatal("missing close dependency accepted")
		}
	}
}
