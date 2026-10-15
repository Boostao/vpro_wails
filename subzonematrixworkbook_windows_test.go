//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSubzoneMatrixWorkbookWindowsSpillCleanupFailure(t *testing.T) {
	source := subzoneMatrixMillionCellFixture(t)
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-%t", cancel), func(t *testing.T) {
			parent := t.TempDir()
			handle := windows.InvalidHandle
			t.Cleanup(func() {
				if handle != windows.InvalidHandle {
					if err := windows.CloseHandle(handle); err != nil {
						t.Error(err)
					}
				}
			})
			ctx := &subzoneMatrixSpillContext{Context: context.Background(), parent: parent}
			ctx.onSpill = func(path string) error {
				name, err := windows.UTF16PtrFromString(path)
				if err != nil {
					return err
				}
				handle, err = windows.CreateFile(name, windows.GENERIC_READ,
					windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
				if err != nil {
					return err
				}
				if cancel {
					return context.Canceled
				}
				return nil
			}
			result, err := prepareSubzoneMatrixWorkbookInDirectory(ctx, source, parent)
			if !ctx.seen || handle == windows.InvalidHandle || err == nil ||
				errors.Is(err, context.Canceled) != cancel ||
				!strings.Contains(err.Error(), "clean up Subzone Matrix worksheet staging") ||
				!reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
				t.Fatal("real denied-delete stream cleanup failure was hidden", err)
			}
			if err := windows.CloseHandle(handle); err != nil {
				t.Fatal(err)
			}
			handle = windows.InvalidHandle
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 1 {
				t.Fatal("failed owned staging not retained for explicit cleanup", entries, err)
			}
			directory := filepath.Join(parent, entries[0].Name())
			if err := os.RemoveAll(directory); err != nil {
				t.Fatal(err)
			}
			remaining, err := os.ReadDir(parent)
			if err != nil || len(remaining) != 0 {
				t.Fatal("released stream cleanup did not succeed", remaining, err)
			}
		})
	}
}
