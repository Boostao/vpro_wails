package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingUnitExplicitEnvWithoutSUAndInitializationLifecycle(t *testing.T) {
	service, projects, _ := workingUnitServiceFixture(t)
	projectPath := filepath.Join(projects.root, "projects", "Sample.db")
	before, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"env", "master", "env"} {
		state, err := service.SetWorkingUnitMode(mode)
		if err != nil || state.Mode != mode || state.Warning != nil {
			t.Fatalf("explicit %s without SU must succeed without fallback: %#v %v", mode, state, err)
		}
		if saved, err := readWorkingUnitPreference(service.settingsPath); err != nil || saved != mode {
			t.Fatalf("explicit %s not persisted: %s %v", mode, saved, err)
		}
		if _, err := service.GetWorkingUnitChoices(mode); err != nil {
			t.Fatalf("refresh explicit %s without SU: %v", mode, err)
		}
		if saved, err := readWorkingUnitPreference(service.settingsPath); err != nil || saved != mode {
			t.Fatalf("choice refresh reinitialized mode: %s %v", saved, err)
		}
	}
	state, err := service.GetWorkingUnitMode()
	if err != nil || state.Mode != "master" || state.Warning == nil || !strings.Contains(*state.Warning, "initializes") {
		t.Fatalf("new editor initialization must force Master without SU: %#v %v", state, err)
	}
	if saved, err := readWorkingUnitPreference(service.settingsPath); err != nil || saved != "master" {
		t.Fatalf("initialization force not persisted: %s %v", saved, err)
	}
	if _, err := service.SetWorkingUnitMode("env"); err != nil {
		t.Fatal(err)
	}
	state, err = service.SetWorkingUnitMode("su")
	if err != nil || state.Mode != "master" || state.Warning == nil || !strings.Contains(*state.Warning, "SU choices are unavailable") {
		t.Fatalf("explicit SU request without SU must warn and fallback: %#v %v", state, err)
	}
	after, err := os.ReadFile(projectPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("mode lifecycle mutated project/history: %v", err)
	}
}

func TestWorkingUnitNoSUEditorInitializationAfterRestart(t *testing.T) {
	service, projects, _ := workingUnitServiceFixture(t)
	if _, err := service.SetWorkingUnitMode("env"); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewWorkingUnitService(projects, filepath.Dir(service.settingsPath))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if saved, err := readWorkingUnitPreference(service.settingsPath); err != nil || saved != "master" {
		t.Fatalf("fresh service initialization did not force Master: %s %v", saved, err)
	}
	if state, err := restarted.SetWorkingUnitMode("env"); err != nil || state.Mode != "env" || state.Warning != nil {
		t.Fatalf("explicit Env after restart blocked without SU: %#v %v", state, err)
	}
}
