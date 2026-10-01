package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type CoordinateService struct {
	mu           sync.RWMutex
	settingsPath string
	replaceFile  func(string, string) error
	preferences  *desktopConfig
}

func NewCoordinateService(configDir string) (*CoordinateService, error) {
	return newCoordinateService(configDir, nil)
}

func newCoordinateService(configDir string, preferences *desktopConfig) (*CoordinateService, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, errors.New("coordinate configuration directory is required")
	}
	configDir, err := filepath.Abs(configDir)
	if err != nil {
		return nil, fmt.Errorf("resolve coordinate configuration directory: %w", err)
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create coordinate configuration directory: %w", err)
	}
	service := &CoordinateService{
		settingsPath: filepath.Join(configDir, "coordinate-settings.json"),
		replaceFile:  os.Rename,
		preferences:  preferences,
	}
	if _, err := service.GetCoordinateMode(); err != nil {
		return nil, err
	}
	return service, nil
}

func readCoordinateSettings(path string) (CoordinateMode, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CoordinateModeDD, nil
	}
	if err != nil {
		return "", fmt.Errorf("read coordinate settings: %w", err)
	}
	mode, err := decodeCoordinateSettings(data)
	if err != nil {
		return "", fmt.Errorf("invalid coordinate settings %q: %w", path, err)
	}
	return mode, nil
}

func decodeCoordinateSettings(data []byte) (CoordinateMode, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", errors.New("settings must be a JSON object containing exactly one mode")
	}
	var mode CoordinateMode
	var found bool
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", err
		}
		if key != "mode" {
			return "", fmt.Errorf("unknown settings property %q", key)
		}
		if found {
			return "", errors.New("duplicate coordinate mode property")
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
		return "", errors.New("coordinate mode property is required")
	}
	if err := validateCoordinateMode(mode); err != nil {
		return "", err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "", errors.New("unexpected trailing coordinate settings data")
	}
	return mode, nil
}

func (s *CoordinateService) GetCoordinateMode() (CoordinateMode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.preferences != nil {
		return s.preferences.coordinateMode()
	}
	return readCoordinateSettings(s.settingsPath)
}

func (s *CoordinateService) SetCoordinateMode(mode CoordinateMode) error {
	if err := validateCoordinateMode(mode); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.preferences != nil {
		return s.preferences.update("Current", map[string]any{"CoordMethod": coordinatePreferenceNumber(mode)})
	}
	return writeCoordinateSettings(s.settingsPath, mode, s.replaceFile)
}

func writeCoordinateSettings(path string, mode CoordinateMode, replace func(string, string) error) (err error) {
	data, err := json.Marshal(struct {
		Mode CoordinateMode `json:"mode"`
	}{mode})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".coordinate-settings-*.json")
	if err != nil {
		return fmt.Errorf("create coordinate settings replacement: %w", err)
	}
	name := file.Name()
	defer func() {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		if name != "" {
			removeErr := os.Remove(name)
			if !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
		}
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write coordinate settings replacement: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync coordinate settings replacement: %w", err)
	}
	closeErr := file.Close()
	file = nil
	if closeErr != nil {
		return fmt.Errorf("close coordinate settings replacement: %w", closeErr)
	}
	// Same-directory replacement keeps the previous complete file until commit.
	// Go's Windows implementation uses MoveFileExW(MOVEFILE_REPLACE_EXISTING).
	if err := replace(name, path); err != nil {
		return fmt.Errorf("commit coordinate settings: %w", err)
	}
	name = ""
	return nil
}

// Conversion is independent of preferences and never edits either plot axis.
func (s *CoordinateService) ConvertCoordinate(axis string, mode CoordinateMode, parts CoordinateParts) (*float64, error) {
	return ConvertCoordinate(axis, mode, parts)
}

func (s *CoordinateService) DecomposeCoordinate(value *float64, mode CoordinateMode) (CoordinateParts, error) {
	return DecomposeCoordinate(value, mode)
}
