package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func coordinateConfigFixture(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(".", ".coordinate-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	return root
}

func TestCoordinateService_DefaultPersistReopenAndSeparateSettings(t *testing.T) {
	root := coordinateConfigFixture(t)
	partner := []byte(`{"active":"DO_NOT_TOUCH"}`)
	partnerPath := filepath.Join(root, "desktop-selection.json")
	if err := os.WriteFile(partnerPath, partner, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := NewCoordinateService(root)
	if err != nil {
		t.Fatal(err)
	}
	if mode, err := service.GetCoordinateMode(); err != nil || mode != CoordinateModeDD {
		t.Fatalf("absent default = %q %v", mode, err)
	}
	if _, err := os.Stat(service.settingsPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reading absent settings unexpectedly created a file")
	}
	for _, expected := range []CoordinateMode{CoordinateModeDM, CoordinateModeDMS, CoordinateModeDD} {
		if err := service.SetCoordinateMode(expected); err != nil {
			t.Fatal(err)
		}
		reopened, err := NewCoordinateService(root)
		if err != nil {
			t.Fatal(err)
		}
		if mode, err := reopened.GetCoordinateMode(); err != nil || mode != expected {
			t.Fatalf("reopened mode = %q %v, want %q", mode, err, expected)
		}
		data, err := os.ReadFile(service.settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		if mode, err := decodeCoordinateSettings(data); err != nil || mode != expected {
			t.Fatal("persisted file was incomplete or malformed")
		}
	}
	data, err := os.ReadFile(partnerPath)
	if err != nil || !bytes.Equal(data, partner) {
		t.Fatal("coordinate settings changed unrelated project selection")
	}
	files, err := filepath.Glob(filepath.Join(root, ".coordinate-settings-*.json"))
	if err != nil || len(files) != 0 {
		t.Fatal("successful replacement left staging files")
	}
}

func TestCoordinateService_ConversionDoesNotChangePreferences(t *testing.T) {
	root := coordinateConfigFixture(t)
	service, err := NewCoordinateService(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetCoordinateMode(CoordinateModeDMS); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(service.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	value := -49.1234567890123
	parts, err := service.DecomposeCoordinate(&value, CoordinateModeDM)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConvertCoordinate("latitude", CoordinateModeDM, parts); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConvertCoordinate("invalid", CoordinateModeDD, CoordinateParts{}); err == nil {
		t.Fatal("invalid conversion unexpectedly succeeded")
	}
	after, err := os.ReadFile(service.settingsPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("pure conversion changed persisted preferences")
	}
}

func TestCoordinateService_InvalidSettingsNeverDefault(t *testing.T) {
	for _, data := range []string{
		"", " ", "null", "[]", `"dd"`, "{}", `{"mode":null}`, `{"mode":""}`,
		`{"mode":"unknown"}`, `{"mode":"DD"}`, `{"mode":1}`, `{"mode":true}`,
		`{"mode":"dm","unknown":1}`, `{"mode":"dm","mode":"dd"}`,
		`{"Mode":"dd"}`, `{"mode":"dd"} {}`, `{"mode":"dd"} junk`, `{"mode":`,
	} {
		t.Run(data, func(t *testing.T) {
			root := coordinateConfigFixture(t)
			service, err := NewCoordinateService(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(service.settingsPath, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if mode, err := service.GetCoordinateMode(); err == nil || mode != "" {
				t.Fatalf("bad persisted settings defaulted: %q %v", mode, err)
			}
			if reopened, err := NewCoordinateService(root); err == nil || reopened != nil {
				t.Fatalf("corrupt configuration allowed startup: %v %v", reopened, err)
			}
		})
	}
}

func TestCoordinateService_InvalidConfigAndMode(t *testing.T) {
	if _, err := NewCoordinateService(""); err == nil {
		t.Fatal("empty configuration directory accepted")
	}
	root := coordinateConfigFixture(t)
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCoordinateService(file); err == nil {
		t.Fatal("configuration file accepted as a directory")
	}
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
	for _, invalid := range []CoordinateMode{"", "bad", "DD"} {
		if err := service.SetCoordinateMode(invalid); err == nil {
			t.Fatal("invalid mode persisted")
		}
	}
	after, err := os.ReadFile(service.settingsPath)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatal("rejected mode changed settings")
	}
}

func TestCoordinateService_FailedCommitRollbackAndCleanup(t *testing.T) {
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
	injected := errors.New("injected atomic replacement failure")
	service.replaceFile = func(from, to string) error {
		staged, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if mode, err := decodeCoordinateSettings(staged); err != nil || mode != CoordinateModeDMS {
			t.Fatal("staging file was not complete before replacement")
		}
		if to != service.settingsPath || filepath.Dir(from) != filepath.Dir(to) {
			t.Fatal("replacement was not in the settings directory")
		}
		return injected
	}
	if err := service.SetCoordinateMode(CoordinateModeDMS); !errors.Is(err, injected) {
		t.Fatalf("failed write was hidden: %v", err)
	}
	after, err := os.ReadFile(service.settingsPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed replacement changed previously committed file")
	}
	if mode, err := service.GetCoordinateMode(); err != nil || mode != CoordinateModeDM {
		t.Fatalf("failed write changed selection: %q %v", mode, err)
	}
	reopened, err := NewCoordinateService(root)
	if err != nil {
		t.Fatal(err)
	}
	if mode, err := reopened.GetCoordinateMode(); err != nil || mode != CoordinateModeDM {
		t.Fatal("failed-write rollback did not survive reopening")
	}
	files, err := filepath.Glob(filepath.Join(root, ".coordinate-settings-*.json"))
	if err != nil || len(files) != 0 {
		t.Fatal("failed replacement leaked staging file")
	}
	service.replaceFile = os.Rename
	if err := service.SetCoordinateMode(CoordinateModeDMS); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinateService_RealWriteFailureAndConcurrentReaders(t *testing.T) {
	root := coordinateConfigFixture(t)
	service, err := NewCoordinateService(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(service.settingsPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := service.SetCoordinateMode(CoordinateModeDM); err == nil {
		t.Fatal("directory collision at settings path did not fail")
	}
	if mode, err := service.GetCoordinateMode(); err == nil || mode != "" {
		t.Fatal("unreadable settings directory silently defaulted")
	}
	if err := os.Remove(service.settingsPath); err != nil {
		t.Fatal(err)
	}
	if err := service.SetCoordinateMode(CoordinateModeDD); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mode := []CoordinateMode{CoordinateModeDD, CoordinateModeDM, CoordinateModeDMS}[i%3]
			if err := service.SetCoordinateMode(mode); err != nil {
				t.Error(err)
			}
			if mode, err := service.GetCoordinateMode(); err != nil || validateCoordinateMode(mode) != nil {
				t.Errorf("concurrent read saw incomplete settings: %q %v", mode, err)
			}
		}(i)
	}
	wg.Wait()
	if _, err := NewCoordinateService(root); err != nil {
		t.Fatal("concurrent writes left corrupt committed settings")
	}
	files, err := filepath.Glob(filepath.Join(root, ".coordinate-settings-*.json"))
	if err != nil || len(files) != 0 {
		t.Fatal("write failure/concurrency leaked staging files")
	}
}
