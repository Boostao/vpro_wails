package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"bytes"
	"context"
	"crypto/sha256"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/resources"
	"gopkg.in/yaml.v3"
)

var DesktopDefaults = resources.ConfigInitYML

type ConfigValues = map[string]any

const (
	ReleaseChannelStable       = "stable"
	ReleaseChannelExperimental = "experimental"
)

type DesktopConfig struct {
	Mu      sync.Mutex                 `json:"-" yaml:"-"`
	Path    string                     `json:"-" yaml:"-"`
	Replace func(string, string) error `json:"-" yaml:"-"`
}

type DesktopSelection struct {
	Project, SU, Hierarchy, ProjectPath, SUPath, HierarchyPath string
}

func DecodeConfig(data []byte) (ConfigValues, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("configuration contains malformed UTF-8")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var values ConfigValues
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("decode YAML configuration: %w", err)
	}
	if values == nil {
		return nil, errors.New("configuration must be a YAML mapping")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("configuration must contain exactly one YAML document")
	}
	return values, nil
}

func ConfigSection(values ConfigValues, name string) (map[string]any, error) {
	section, ok := values[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configuration section %s must be a mapping", name)
	}
	return section, nil
}

func ConfigString(values ConfigValues, section, key string) (string, error) {
	fields, err := ConfigSection(values, section)
	if err != nil {
		return "", err
	}
	value, ok := fields[key].(string)
	if !ok {
		return "", fmt.Errorf("%s.%s must be a string, not NULL or an implicit default", section, key)
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s.%s contains malformed Unicode", section, key)
	}
	return value, nil
}

func ConfigInt(values ConfigValues, section, key string, minimum, maximum int) (int, error) {
	fields, err := ConfigSection(values, section)
	if err != nil {
		return 0, err
	}
	value, ok := fields[key].(int)
	if !ok || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s.%s must be an integer from %d to %d", section, key, minimum, maximum)
	}
	return value, nil
}

func ValidateDesktopConfig(values ConfigValues) error {
	if rawChannel, present := values["release_channel"]; present {
		channel, ok := rawChannel.(string)
		if !ok || (channel != ReleaseChannelStable && channel != ReleaseChannelExperimental) {
			return errors.New(`release_channel must be "stable" or "experimental"`)
		}
	}
	for _, field := range []struct {
		section, key string
		min, max     int
	}{{"Desktop", "SchemaVersion", 1, 1}, {"Current", "CoordMethod", 1, 3},
		{"Current", "AssignedSuSource", 1, 3}, {"Audit", "AuditStrength", 0, 3}} {
		if _, err := ConfigInt(values, field.section, field.key, field.min, field.max); err != nil {
			return err
		}
	}
	for _, key := range []string{"CurrProject", "CurrPlotlist", "CurrHierarchy", "ProjectPath", "SUPath", "HierarchyPath", "User"} {
		value, err := ConfigString(values, "Current", key)
		if err != nil {
			return err
		}
		switch key {
		case "CurrProject", "CurrPlotlist", "CurrHierarchy":
			if !ProjectNamePattern.MatchString(value) {
				return fmt.Errorf("Current.%s is not a valid context name", key)
			}
		case "User":
			if value == "" {
				return errors.New("Current.User must not be empty")
			}
		}
	}
	return nil
}

func mergeConfigDefaults(values ConfigValues) error {
	defaults, err := DecodeConfig(DesktopDefaults)
	if err != nil {
		return err
	}
	for section, raw := range defaults {
		if _, present := values[section]; !present {
			values[section] = raw
			continue
		}
		defaultMap, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		fields, err := ConfigSection(values, section)
		if err != nil {
			return err
		}
		for key, value := range defaultMap {
			if _, present := fields[key]; !present {
				fields[key] = value
			}
		}
	}
	return nil
}

func setConfigValue(values ConfigValues, section, key string, value any, conflict bool) error {
	if _, present := values[section]; !present {
		values[section] = map[string]any{}
	}
	fields, err := ConfigSection(values, section)
	if err != nil {
		return err
	}
	if previous, present := fields[key]; conflict && present && !reflect.DeepEqual(previous, value) {
		return fmt.Errorf("legacy JSON conflicts with explicit YAML %s.%s; both files were retained", section, key)
	}
	fields[key] = value
	return nil
}

func OpenDesktopConfig(dataDir, configDir string) (*DesktopConfig, error) {
	if dataDir == "" || configDir == "" {
		return nil, errors.New("data and configuration directories are required")
	}
	configDir, err := filepath.Abs(configDir)
	if err != nil {
		return nil, err
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}
	store := &DesktopConfig{Path: filepath.Join(configDir, "config.yml"), Replace: os.Rename}
	data, err := os.ReadFile(store.Path)
	creating := errors.Is(err, os.ErrNotExist)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	values := ConfigValues{}
	if err == nil {
		values, err = DecodeConfig(data)
		if err != nil {
			return nil, err
		}
	}
	if _, present := values["Desktop"]; !present {
		values["Desktop"] = map[string]any{}
	}
	desktop, err := ConfigSection(values, "Desktop")
	if err != nil {
		return nil, err
	}
	if _, present := desktop["SchemaVersion"]; !present {
		desktop["SchemaVersion"] = 1
	}
	if _, err := ConfigInt(values, "Desktop", "SchemaVersion", 1, 1); err != nil {
		return nil, err
	}
	if err := migrateDesktopJSON(values, dataDir, configDir); err != nil {
		return nil, err
	}
	if err := mergeConfigDefaults(values); err != nil {
		return nil, err
	}
	if err := ValidateDesktopConfig(values); err != nil {
		return nil, err
	}
	encoded, err := yaml.Marshal(values)
	if err != nil {
		return nil, err
	}
	// Do not rewrite existing YAML simply to change formatting or key order.
	if len(data) > 0 {
		before, err := DecodeConfig(data)
		if err != nil {
			return nil, err
		}
		if reflect.DeepEqual(before, values) {
			return store, nil
		}
	}
	if creating {
		store.Replace = InstallDesktopConfig
	}
	if err := store.Commit(encoded); err != nil {
		return nil, err
	}
	store.Replace = os.Rename
	return store, nil
}

func migrateDesktopJSON(values ConfigValues, dataDir, configDir string) error {
	desktop, err := ConfigSection(values, "Desktop")
	if err != nil {
		return err
	}
	imports := map[string]any{}
	if raw, present := desktop["LegacyImports"]; present {
		var ok bool
		imports, ok = raw.(map[string]any)
		if !ok {
			return errors.New("Desktop.LegacyImports must be a mapping")
		}
	}
	legacyFound := false
	for _, name := range []string{"desktop-selection.json", "coordinate-settings.json", "working-unit-settings.json"} {
		path := filepath.Join(configDir, name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) && name == "desktop-selection.json" && configDir != dataDir {
			path = filepath.Join(dataDir, name)
			data, err = os.ReadFile(path)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read legacy %s: %w", path, err)
		}
		hash := "absent"
		if err == nil {
			legacyFound = true
			hash = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		if previous, present := imports[name]; present {
			if previous != hash {
				return fmt.Errorf("legacy %s changed after migration; YAML and JSON were retained for explicit resolution", path)
			}
			continue
		}
		if err == nil {
			var changes map[string]any
			switch name {
			case "coordinate-settings.json":
				mode, err := decodeCoordinateSettings(data)
				if err != nil {
					return fmt.Errorf("migrate %s: %w", path, err)
				}
				changes = map[string]any{"CoordMethod": coordinatePreferenceNumber(mode)}
			case "working-unit-settings.json":
				mode, err := decodeWorkingUnitPreference(data)
				if err != nil {
					return fmt.Errorf("migrate %s: %w", path, err)
				}
				changes = map[string]any{"AssignedSuSource": workingUnitPreferenceNumber(mode)}
			default:
				changes, err = decodeLegacySelection(data, dataDir)
				if err != nil {
					return fmt.Errorf("migrate %s: %w", path, err)
				}
			}
			for key, value := range changes {
				if err := setConfigValue(values, "Current", key, value, true); err != nil {
					return err
				}
			}
		}
		imports[name] = hash
	}
	legacyCatalogue := filepath.Join(dataDir, "working-unit.db")
	info, statErr := os.Stat(legacyCatalogue)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect legacy desktop installation: %w", statErr)
	}
	if info != nil && !info.Mode().IsRegular() {
		return fmt.Errorf("legacy desktop catalogue %s is not a regular file", legacyCatalogue)
	}
	if legacyFound || info != nil {
		if _, present := values["Current"]; !present {
			values["Current"] = map[string]any{}
		}
		current, err := ConfigSection(values, "Current")
		if err != nil {
			return err
		}
		if _, present := current["User"]; !present {
			current["User"] = "User"
		}
	}
	desktop["LegacyImports"] = imports
	return nil
}

func decodeLegacySelection(data []byte, dataDir string) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("selection must be a JSON object")
	}
	fields := map[string]string{}
	allowed := map[string]bool{"activeProject": true, "activeSU": true, "activeHierarchy": true, "hierarchyFile": true}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok || !allowed[key] {
			return nil, fmt.Errorf("unknown selection property %q", token)
		}
		if _, found := fields[key]; found {
			return nil, fmt.Errorf("duplicate selection property %s", key)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		if err := sqlutil.ValidateQualityJSONToken(raw); err != nil {
			return nil, fmt.Errorf("legacy selection %s: %w", key, err)
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("selection %s must be a string", key)
		}
		fields[key] = text
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("selection contains trailing data")
	}
	changes := map[string]any{}
	for jsonKey, yamlKey := range map[string]string{
		"activeProject": "CurrProject", "activeSU": "CurrPlotlist", "activeHierarchy": "CurrHierarchy",
	} {
		value := fields[jsonKey]
		if value == "" {
			value = "None"
			if jsonKey == "activeProject" {
				value = "Sample"
			}
		}
		if !ProjectNamePattern.MatchString(value) {
			return nil, fmt.Errorf("invalid legacy %s", jsonKey)
		}
		changes[yamlKey] = value
	}
	file := fields["hierarchyFile"]
	if file != "" && (filepath.Base(file) != file || strings.ContainsAny(file, `/\`) || file == "." || file == ".." || filepath.Ext(file) != ".db") {
		return nil, errors.New("legacy hierarchyFile must be a local database filename")
	}
	path := ""
	if file != "" {
		path = filepath.Join(dataDir, "projects", file)
	}
	changes["HierarchyPath"] = path
	return changes, nil
}

func (s *DesktopConfig) ReadLocked() (ConfigValues, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("read runtime YAML: %w", err)
	}
	values, err := DecodeConfig(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateDesktopConfig(values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *DesktopConfig) Snapshot() (ConfigValues, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.ReadLocked()
}

func (s *DesktopConfig) Update(section string, changes map[string]any) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return err
	}

	for key, value := range changes {
		if err := setConfigValue(values, section, key, value, false); err != nil {
			return err
		}
	}
	if err := ValidateDesktopConfig(values); err != nil {
		return err
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return err
	}
	return s.Commit(data)
}

func (s *DesktopConfig) CompareAndSetProjectIDSource(ctx context.Context, expected, source int) error {
	if source != 1 && source != 2 {
		return errors.New("ProjectID source must be Env (1) or Master (2)")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return err
	}
	current, err := ConfigInt(values, "Current", "ProjectIdSource", 1, 2)
	if err != nil {
		return err
	}
	if current != expected {
		return errors.New("ProjectID source changed; reload choices before switching")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if current == source {
		return nil
	}
	if err := setConfigValue(values, "Current", "ProjectIdSource", source, false); err != nil {
		return err
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Commit(data)
}
func (s *DesktopConfig) Commit(data []byte) error {
	_, err := s.commitWithContext(context.Background(), data)
	return err
}

func (s *DesktopConfig) commitWithContext(ctx context.Context, data []byte) (committed bool, err error) {
	if ctx == nil {
		return false, errors.New("configuration commit requires a context")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	file, err := os.CreateTemp(filepath.Dir(s.Path), ".config-*.yml")
	if err != nil {
		return false, err
	}
	name := file.Name()
	defer func() {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		removeErr := os.Remove(name)
		if !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err := file.Chmod(0600); err != nil {
		return false, err
	}
	if _, err := file.Write(data); err != nil {
		return false, err
	}
	if err := file.Sync(); err != nil {
		return false, err
	}
	closeErr := file.Close()
	file = nil
	if closeErr != nil {
		return false, closeErr
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := s.Replace(name, s.Path); err != nil {
		return false, fmt.Errorf("commit runtime YAML: %w", err)
	}
	return true, ctx.Err()
}

func coordinatePreferenceNumber(mode CoordinateMode) int {
	switch mode {
	case CoordinateModeDM:
		return 2
	case CoordinateModeDMS:
		return 3
	default:
		return 1
	}
}

func workingUnitPreferenceNumber(mode string) int {
	switch mode {
	case "master":
		return 2
	case "su":
		return 3
	default:
		return 1
	}
}

func (s *DesktopConfig) CoordinateMode() (CoordinateMode, error) {
	values, err := s.Snapshot()
	if err != nil {
		return "", err
	}
	number, err := ConfigInt(values, "Current", "CoordMethod", 1, 3)
	if err != nil {
		return "", err
	}
	return []CoordinateMode{CoordinateModeDD, CoordinateModeDM, CoordinateModeDMS}[number-1], nil
}

func (s *DesktopConfig) WorkingUnitMode() (string, error) {
	values, err := s.Snapshot()
	if err != nil {
		return "", err
	}
	number, err := ConfigInt(values, "Current", "AssignedSuSource", 1, 3)
	if err != nil {
		return "", err
	}
	return []string{"env", "master", "su"}[number-1], nil
}

func (s *DesktopConfig) ReleaseChannel() (string, error) {
	values, err := s.Snapshot()
	if err != nil {
		return "", err
	}
	raw, present := values["release_channel"]
	if !present {
		return ReleaseChannelStable, nil
	}
	channel, ok := raw.(string)
	if !ok || (channel != ReleaseChannelStable && channel != ReleaseChannelExperimental) {
		return "", errors.New(`release_channel must be "stable" or "experimental"`)
	}
	return channel, nil
}

func (s *DesktopConfig) Selection() (DesktopSelection, error) {
	values, err := s.Snapshot()
	if err != nil {
		return DesktopSelection{}, err
	}
	current, err := ConfigSection(values, "Current")
	if err != nil {
		return DesktopSelection{}, err
	}
	return DesktopSelection{current["CurrProject"].(string), current["CurrPlotlist"].(string),
		current["CurrHierarchy"].(string), current["ProjectPath"].(string),
		current["SUPath"].(string), current["HierarchyPath"].(string)}, nil
}
