package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readWorkingUnitPreference(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "env", nil
	}
	if err != nil {
		return "", fmt.Errorf("read Working Unit preference: %w", err)
	}
	return decodeWorkingUnitPreference(data)
}

func decodeWorkingUnitPreference(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", errors.New("Working Unit preference must be an object containing exactly one mode")
	}
	var mode string
	found := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", err
		}
		if key != "mode" || found {
			return "", errors.New("Working Unit preference has unknown/duplicate properties")
		}
		found = true
		if err := decoder.Decode(&mode); err != nil {
			return "", err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("Working Unit preference mode is required")
	}
	if err := validateWorkingUnitMode(mode); err != nil {
		return "", err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", errors.New("Working Unit preference contains trailing data")
	}
	return mode, nil
}

func writeWorkingUnitPreference(path, mode string, replace func(string, string) error) (err error) {
	data, err := json.Marshal(struct {
		Mode string `json:"mode"`
	}{mode})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".working-unit-settings-*.json")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		if name != "" {
			if removeErr := os.Remove(name); !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
		}
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	closeErr := file.Close()
	file = nil
	if closeErr != nil {
		return closeErr
	}
	if err := replace(name, path); err != nil {
		return fmt.Errorf("commit Working Unit preference: %w", err)
	}
	name = ""
	return nil
}

func (s *WorkingUnitService) workingUnitMode(requested *string) (WorkingUnitModeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return WorkingUnitModeState{}, errors.New("Working Unit catalogue is closed")
	}
	if err := s.db.Ping(); err != nil {
		return WorkingUnitModeState{}, fmt.Errorf("Working Unit catalogue is unavailable: %w", err)
	}
	var mode string
	var err error
	if s.projects.preferences != nil {
		mode, err = s.projects.preferences.workingUnitMode()
	} else {
		mode, err = readWorkingUnitPreference(s.settingsPath)
	}
	if err != nil {
		return WorkingUnitModeState{}, err
	}
	old := mode
	if requested != nil {
		mode = *requested
	}
	s.projects.mu.RLock()
	defer s.projects.mu.RUnlock()
	_, su, err := s.context()
	if err != nil {
		return WorkingUnitModeState{}, err
	}
	state := WorkingUnitModeState{Mode: mode}
	if su == "None" && (requested == nil || mode == "su") {
		state.Mode = "master"
		warning := "No active SU table is selected; Working Unit source initializes to Master."
		if requested != nil {
			warning = "No active SU table is selected; SU choices are unavailable, so Working Unit source is Master."
		}
		state.Warning = &warning
	}
	if state.Mode != old || requested != nil {
		var err error
		if s.projects.preferences != nil {
			err = s.projects.preferences.update("Current", map[string]any{"AssignedSuSource": workingUnitPreferenceNumber(state.Mode)})
		} else {
			err = writeWorkingUnitPreference(s.settingsPath, state.Mode, s.replaceFile)
		}
		if err != nil {
			return WorkingUnitModeState{}, err
		}
	}
	return state, nil
}

func (s *WorkingUnitService) GetWorkingUnitMode() (WorkingUnitModeState, error) {
	return s.workingUnitMode(nil)
}

func (s *WorkingUnitService) SetWorkingUnitMode(mode string) (WorkingUnitModeState, error) {
	if err := validateWorkingUnitMode(mode); err != nil {
		return WorkingUnitModeState{}, err
	}
	return s.workingUnitMode(&mode)
}
