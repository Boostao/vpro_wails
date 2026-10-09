package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const closeRequestEvent = "vpro:close-request"

func init() {
	application.RegisterEvent[string]("vpro:close-request")
}

var (
	errNoPendingClose       = errors.New("no close request is pending")
	errInvalidCloseRequest  = errors.New("close request ID does not match the current pending request")
	errCloseAlreadyApproved = errors.New("application close has already been approved")
)

// CloseService owns one pending native-close handshake for this app instance.
// Event delivery is advisory: only an exact pending nonce can authorize exit.
type CloseService struct {
	mu       sync.Mutex
	prefix   string
	sequence uint64
	pending  string
	approved bool
	emit     func(string) bool
	quit     func()
}

func NewCloseService(emit func(string) bool, quit func()) (*CloseService, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return nil, fmt.Errorf("initialize close request identity: %w", err)
	}
	return newCloseService(hex.EncodeToString(entropy[:]), emit, quit)
}

func newCloseService(prefix string, emit func(string) bool, quit func()) (*CloseService, error) {
	if prefix == "" || emit == nil || quit == nil {
		return nil, errors.New("close service requires a nonce prefix, event emitter and quit action")
	}
	return &CloseService{prefix: prefix, emit: emit, quit: quit}, nil
}

// GetPendingCloseRequest permits the frontend to recover an event emitted
// before its listener mounted. Empty means no pending approval request.
func (s *CloseService) GetPendingCloseRequest() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending
}

func (s *CloseService) validateLocked(requestID string) error {
	if s.approved {
		return errCloseAlreadyApproved
	}
	if s.pending == "" {
		return errNoPendingClose
	}
	if requestID == "" || requestID != s.pending {
		return errInvalidCloseRequest
	}
	return nil
}

func (s *CloseService) ConfirmClose(requestID string) error {
	s.mu.Lock()
	if err := s.validateLocked(requestID); err != nil {
		s.mu.Unlock()
		return err
	}
	s.pending = ""
	s.approved = true
	s.mu.Unlock()
	// Wails Quit synchronously consults ShouldQuit; no service lock may be
	// held here. Approval is consumed before invoking the native quit action.
	s.quit()
	return nil
}

func (s *CloseService) CancelClose(requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateLocked(requestID); err != nil {
		return err
	}
	s.pending = ""
	return nil
}

func (s *CloseService) requestClose() error {
	s.mu.Lock()
	if s.approved || s.pending != "" {
		s.mu.Unlock()
		return nil
	}
	if s.sequence == math.MaxUint64 {
		s.mu.Unlock()
		return errors.New("close request identity sequence exhausted; close remains blocked")
	}
	s.sequence++
	nonce := fmt.Sprintf("%s:%016x", s.prefix, s.sequence)
	s.pending = nonce
	s.mu.Unlock()
	if s.emit(nonce) {
		return errors.New("close-request event was cancelled; pending request remains available for retrieval")
	}
	return nil
}

// RegisterHook runs synchronously before Wails' closing listeners. Always
// cancel this window event: ConfirmClose exits through the separate App.Quit.
func (s *CloseService) handleWindowClosing(event *application.WindowEvent) {
	event.Cancel()
	if err := s.requestClose(); err != nil {
		log.Printf("Native close remains blocked: %v", err)
	}
}

// Also guard application-level quit routes, not just the window's close button.
func (s *CloseService) shouldQuit() bool {
	s.mu.Lock()
	approved := s.approved
	s.mu.Unlock()
	if approved {
		return true
	}
	if err := s.requestClose(); err != nil {
		log.Printf("Application quit remains blocked: %v", err)
	}
	return false
}
