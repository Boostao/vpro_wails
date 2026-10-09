package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWorkingUnitPreferencesNoSUForceRestartAndAtomicFailure(t *testing.T) {
	service, projects, db := workingUnitServiceFixture(t)
	state, err := service.GetWorkingUnitMode()
	if err != nil || state.Mode != "master" || state.Warning == nil {
		t.Fatalf("source no-SU force: %#v %v", state, err)
	}
	state, err = service.SetWorkingUnitMode("su")
	if err != nil || state.Mode != "master" || state.Warning == nil {
		t.Fatalf("no-SU requested mode: %#v %v", state, err)
	}
	if mode, err := readWorkingUnitPreference(service.settingsPath); err != nil || mode != "master" {
		t.Fatalf("forced source preference not persisted: %s %v", mode, err)
	}
	if _, err := db.Exec(`CREATE TABLE Choice_SU(PlotNumber TEXT,SiteUnit TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectSU("Choice"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"env", "master", "su"} {
		if state, err := service.SetWorkingUnitMode(mode); err != nil || state.Mode != mode {
			t.Fatalf("persist mode: %#v %v", state, err)
		}
		if err := service.Close(); err != nil {
			t.Fatal(err)
		}
		restarted, err := NewWorkingUnitService(projects, filepath.Dir(service.settingsPath))
		if err != nil {
			t.Fatal(err)
		}
		service = restarted
		t.Cleanup(func() { restarted.Close() })
		if state, err := service.GetWorkingUnitMode(); err != nil || state.Mode != mode || state.Warning != nil {
			t.Fatalf("restarted mode: %#v %v", state, err)
		}
	}
	before, err := os.ReadFile(service.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	service.replaceFile = func(string, string) error { return errors.New("controlled preference commit rejection") }
	if _, err := service.SetWorkingUnitMode("env"); err == nil {
		t.Fatal("failed preference commit reported success")
	}
	after, err := os.ReadFile(service.settingsPath)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("preference failure changed prior file: %v", err)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(service.settingsPath), ".working-unit-settings-*.json"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("failed preference commit left temporary files: %#v %v", leftovers, err)
	}
}

func TestWorkingUnitMalformedSavedPreferencesFailExplicitly(t *testing.T) {
	service, projects, _ := workingUnitServiceFixture(t)
	for _, malformed := range []string{
		`{}`, `null`, `[]`, `{"mode":null}`, `{"mode":1}`, `{"mode":"unknown"}`,
		`{"mode":"master","mode":"env"}`, `{"mode":"master","extra":true}`,
		`{"mode":"master"} {}`, `{"mode":`, "",
	} {
		if err := os.WriteFile(service.settingsPath, []byte(malformed), 0600); err != nil {
			t.Fatal(err)
		}

		if _, err := service.GetWorkingUnitMode(); err == nil {
			t.Fatalf("invalid saved preference accepted: %q", malformed)
		}
		if _, err := NewWorkingUnitService(projects, filepath.Dir(service.settingsPath)); err == nil {
			t.Fatalf("invalid saved preference accepted on startup: %q", malformed)
		}
	}
}
