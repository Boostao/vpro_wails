package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTwoPageParentOwnedLocalExternalRawSnapshotsAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # two page.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				next, err := service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
				state = next
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
				got, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
				if err != nil || got == nil || len(got.Rows) != 1 || got.ContextID != state.ContextID || got.Form != form {
					t.Fatal("owned source variant unavailable", got, err)
				}
				retry, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
				if err != nil || !reflect.DeepEqual(retry, got) {
					t.Fatal("read/retry changed original provenance", err)
				}
			}
			if got, err := service.readTwoPageParent(context.Background(), "stale", "108050", "FS882-8x6XL"); err == nil || got != nil {
				t.Fatal("stale context published source rows", got, err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("two-page read changed owned configuration", err)
			}
		})
	}
}

func TestTwoPageParentOwnedCancellationAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	before, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	got, err := service.readTwoPageParent(ctx, state.ContextID, "108050", "FS882-8x6XL")
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked parent request ignored cancellation", got, err)
	}
	retry, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil || !reflect.DeepEqual(retry, before) {
		t.Fatal("cancelled request retired original attachments", retry, err)
	}
}
