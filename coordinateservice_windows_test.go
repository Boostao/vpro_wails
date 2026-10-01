//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCoordinateService_WindowsLockedTargetRollback(t *testing.T) {
	root := coordinateConfigFixture(t)
	service, err := NewCoordinateService(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetCoordinateMode(CoordinateModeDM); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(service.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(service.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Keep reads possible but deny deletion/replacement of the committed file.
	handle, err := windows.CreateFile(path, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if handle != windows.InvalidHandle {
			if err := windows.CloseHandle(handle); err != nil {
				t.Error(err)
			}
		}
	})
	if err := service.SetCoordinateMode(CoordinateModeDMS); err == nil {
		t.Fatal("locked target replacement unexpectedly succeeded")
	}
	after, err := os.ReadFile(service.settingsPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("actual Windows replacement failure corrupted committed settings")
	}
	if mode, err := service.GetCoordinateMode(); err != nil || mode != CoordinateModeDM {
		t.Fatalf("locked-target failure changed mode: %q %v", mode, err)
	}
	files, err := filepath.Glob(filepath.Join(root, ".coordinate-settings-*.json"))
	if err != nil || len(files) != 0 {
		t.Fatal("locked-target failure left staging files")
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	handle = windows.InvalidHandle
	if err := service.SetCoordinateMode(CoordinateModeDMS); err != nil {
		t.Fatalf("retry after releasing target lock failed: %v", err)
	}
}
