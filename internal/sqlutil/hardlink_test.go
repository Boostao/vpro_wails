package sqlutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLinkOrCreateExclusiveFallbackAndNoOverwriteGuarantee(t *testing.T) {
	for _, injectedErr := range []error{
		errors.ErrUnsupported,
		syscall.EPERM,
		syscall.EOPNOTSUPP,
		syscall.ENOTSUP,
		syscall.ENOSYS,
		syscall.EXDEV,
	} {
		dir := t.TempDir()
		stage := filepath.Join(dir, ".stage-test.db")
		dest := filepath.Join(dir, "published.db")
		payload := []byte("SQLite format 3\x00portable-payload")
		if err := os.WriteFile(stage, payload, 0o600); err != nil {
			t.Fatal(err)
		}

		copied, err := LinkOrCreateExclusiveWith(stage, dest, func(string, string) error {
			return &os.LinkError{Op: "link", Old: stage, New: dest, Err: injectedErr}
		})
		if err != nil || !copied {
			t.Fatalf("expected exclusive copy fallback on %v, got copied=%v err=%v", injectedErr, copied, err)
		}
		got, err := os.ReadFile(dest)
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("published bytes mismatch on %v: %v", injectedErr, err)
		}
		// Stage file must remain present so caller cleanup succeeds without os.ErrNotExist.
		if _, err := os.Stat(stage); err != nil {
			t.Fatalf("stage file unexpectedly removed during fallback: %v", err)
		}

		// Second attempt against existing target must fail closed with os.ErrExist and never overwrite.
		otherStage := filepath.Join(dir, ".stage-other.db")
		if err := os.WriteFile(otherStage, []byte("competitor-overwrite"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = LinkOrCreateExclusiveWith(otherStage, dest, func(string, string) error {
			return &os.LinkError{Op: "link", Old: otherStage, New: dest, Err: injectedErr}
		})
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("expected os.ErrExist on existing destination, got %v", err)
		}
		still, err := os.ReadFile(dest)
		if err != nil || !bytes.Equal(still, payload) {
			t.Fatal("existing destination was overwritten during fallback collision")
		}
	}

	// Non-filesystem generic faults must NOT silently fall back.
	dir := t.TempDir()
	stage := filepath.Join(dir, "stage")
	dest := filepath.Join(dir, "dest")
	if err := os.WriteFile(stage, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("injected generic fault")
	if _, err := LinkOrCreateExclusiveWith(stage, dest, func(string, string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("generic fault bypassed: %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destination created on generic fault")
	}
}
