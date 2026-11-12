//go:build windows

package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDesktopConfigWindowsLockedTargetRollback(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(store.Path)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := store.Update("Current", map[string]any{"CoordMethod": 3}); err == nil {
		t.Fatal("actual Windows locked-file replacement unexpectedly succeeded")
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("locked-file failure corrupted runtime YAML")
	}
	if mode, err := store.CoordinateMode(); err != nil || mode != vprocore.CoordinateModeDD {
		t.Fatal("locked-file failure changed effective mode")
	}
	leftovers, err := filepath.Glob(filepath.Join(config, ".config-*.yml"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("locked-file failure leaked staging files")
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	handle = windows.InvalidHandle
	if err := store.Update("Current", map[string]any{"CoordMethod": 3}); err != nil {
		t.Fatal(err)
	}
}
