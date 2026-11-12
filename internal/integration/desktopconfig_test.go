package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeConfigFixture(t *testing.T, dir, name, value string) []byte {
	t.Helper()
	data := []byte(value)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDesktopConfigDefaultsAndExistingRValues(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := vprocore.DecodeConfig(vprocore.DesktopDefaults)
	if err != nil {
		t.Fatal(err)
	}
	for key, expected := range defaults {
		if !reflect.DeepEqual(values[key], expected) {
			t.Fatalf("default %s changed", key)
		}
	}
	if mode, err := store.CoordinateMode(); err != nil || mode != vprocore.CoordinateModeDD {
		t.Fatalf("coordinate default: %s %v", mode, err)
	}
	if mode, err := store.WorkingUnitMode(); err != nil || mode != "env" {
		t.Fatalf("Working Unit default: %s %v", mode, err)
	}
	if channel, err := store.ReleaseChannel(); err != nil || channel != vprocore.ReleaseChannelStable {
		t.Fatalf("release_channel default: %q %v", channel, err)
	}
	expData, expConfig := vprotest.ConfigFixture(t)
	writeConfigFixture(t, expConfig, "config.yml", "release_channel: \"experimental\"\n")
	expStore, err := vprocore.OpenDesktopConfig(expData, expConfig)
	if err != nil {
		t.Fatal(err)
	}
	if channel, err := expStore.ReleaseChannel(); err != nil || channel != vprocore.ReleaseChannelExperimental {
		t.Fatalf("explicit experimental release_channel lost: %q %v", channel, err)
	}
	data, config = vprotest.ConfigFixture(t)
	writeConfigFixture(t, config, "config.yml", "Current:\n  CoordMethod: 3\n  User: \" R User \"\nReportOptions:\n  DataQualityFilterVegNull: \"True\"\nCustom:\n  Values: [false, \"False\", null, 0]\n")
	store, err = vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update("Current", map[string]any{"AssignedSuSource": 3}); err != nil {
		t.Fatal(err)
	}
	after, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before["Custom"], after["Custom"]) || !reflect.DeepEqual(before["ReportOptions"], after["ReportOptions"]) {
		t.Fatal("unknown/inactive settings or scalar types changed")
	}
	if user, err := vprocore.ConfigString(after, "Current", "User"); err != nil || user != " R User " {
		t.Fatalf("user identity was trimmed/replaced: %q %v", user, err)
	}
}

func TestDesktopConfigThreeFileMigrationFallbackAndIdempotence(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "config-directory", true: "legacy-data-root"}[fallback], func(t *testing.T) {
			data, config := vprotest.ConfigFixture(t)
			selectionDir := config
			if fallback {
				selectionDir = data
			}
			legacy := map[string][]byte{
				"desktop-selection.json": writeConfigFixture(t, selectionDir, "desktop-selection.json",
					`{"activeProject":"Sample","activeSU":"Choice","activeHierarchy":"Tree","hierarchyFile":"Tree data.db"}`),
				"coordinate-settings.json":   writeConfigFixture(t, config, "coordinate-settings.json", `{"mode":"dms"}`),
				"working-unit-settings.json": writeConfigFixture(t, config, "working-unit-settings.json", `{"mode":"su"}`),
			}
			store, err := vprocore.OpenDesktopConfig(data, config)
			if err != nil {
				t.Fatal(err)
			}
			selection, err := store.Selection()
			if err != nil || selection.Project != "Sample" || selection.SU != "Choice" || selection.Hierarchy != "Tree" ||
				selection.HierarchyPath != filepath.Join(data, "projects", "Tree data.db") {
				t.Fatalf("selection migration: %#v %v", selection, err)
			}
			if mode, err := store.CoordinateMode(); err != nil || mode != vprocore.CoordinateModeDMS {
				t.Fatalf("coordinate migration: %s %v", mode, err)
			}
			if mode, err := store.WorkingUnitMode(); err != nil || mode != "su" {
				t.Fatalf("Working Unit migration: %s %v", mode, err)
			}
			values, _ := store.Snapshot()
			if user, _ := vprocore.ConfigString(values, "Current", "User"); user != "User" {
				t.Fatal("legacy Go audit identity silently changed to R's new-install Admin")
			}
			if err := store.Update("Current", map[string]any{"CoordMethod": 2}); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(store.Path)
			reopened, err := vprocore.OpenDesktopConfig(data, config)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(store.Path)
			if !bytes.Equal(before, after) {
				t.Fatal("reopen rewrote YAML or replayed migration")
			}
			if mode, err := reopened.CoordinateMode(); err != nil || mode != vprocore.CoordinateModeDM {
				t.Fatal("retired JSON replayed over runtime YAML")
			}
			for name, expected := range legacy {
				dir := config
				if name == "desktop-selection.json" {
					dir = selectionDir
				}
				actual, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || !bytes.Equal(actual, expected) {
					t.Fatalf("legacy source changed/deleted: %s", name)
				}
			}
		})
	}
}

func TestDesktopConfigMigrationFailuresLeaveAllInputsUntouched(t *testing.T) {
	for _, malformed := range []struct{ name, content string }{
		{"coordinate-settings.json", `{"mode":"dm","mode":"dd"}`},
		{"working-unit-settings.json", `{"mode":null}`},
		{"desktop-selection.json", `{"activeProject":"Sample","unknown":"lost"}`},
		{"desktop-selection.json", `{"activeProject":"Sample","activeProject":"Other"}`},
		{"desktop-selection.json", `{"activeProject":null}`},
		{"desktop-selection.json", `{"hierarchyFile":"broken\uD800.db"}`},
		{"desktop-selection.json", `{"hierarchyFile":"..\\external.db"}`},
		{"desktop-selection.json", `{"activeProject":"Sample"} {}`},
	} {
		t.Run(malformed.content, func(t *testing.T) {
			data, config := vprotest.ConfigFixture(t)
			valid := writeConfigFixture(t, config, "coordinate-settings.json", `{"mode":"dm"}`)
			bad := writeConfigFixture(t, config, malformed.name, malformed.content)
			if _, err := vprocore.OpenDesktopConfig(data, config); err == nil {
				t.Fatal("malformed migration reported success")
			}
			if _, err := os.Stat(filepath.Join(config, "config.yml")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed migration committed partial YAML")
			}
			actual, _ := os.ReadFile(filepath.Join(config, malformed.name))
			if !bytes.Equal(actual, bad) {
				t.Fatal("failed migration changed source")
			}
			if malformed.name != "coordinate-settings.json" {
				actual, _ = os.ReadFile(filepath.Join(config, "coordinate-settings.json"))
				if !bytes.Equal(actual, valid) {
					t.Fatal("another preference was consumed before migration failed")
				}
			}
		})
	}
	data, config := vprotest.ConfigFixture(t)
	before := writeConfigFixture(t, config, "config.yml", "Current:\n  CoordMethod: 3\n")
	writeConfigFixture(t, config, "coordinate-settings.json", `{"mode":"dm"}`)
	if _, err := vprocore.OpenDesktopConfig(data, config); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("explicit YAML/JSON conflict was hidden: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(config, "config.yml"))
	if !bytes.Equal(before, after) {
		t.Fatal("conflict replaced YAML")
	}
}

func TestDesktopConfigMalformedYAMLAndChangedLegacyNeverDefault(t *testing.T) {
	for _, bad := range []string{
		"", "null", "[]", "Current: null", "Current:\n  CoordMethod: null",
		"Current:\n  CoordMethod: 2\n  CoordMethod: 3", "Current:\n  CoordMethod: false",
		"Current:\n  CoordMethod: \"2\"", "Audit:\n  AuditStrength: 4",
		"Desktop:\n  SchemaVersion: 2", "{}\n---\n{}", "Current:\n  User: \"\"",
		"release_channel: null", "release_channel: \"beta\"", "release_channel: 1",
	} {
		t.Run(bad, func(t *testing.T) {
			data, config := vprotest.ConfigFixture(t)
			before := writeConfigFixture(t, config, "config.yml", bad)
			if _, err := vprocore.OpenDesktopConfig(data, config); err == nil {
				t.Fatal("invalid YAML silently defaulted")
			}
			after, _ := os.ReadFile(filepath.Join(config, "config.yml"))
			if !bytes.Equal(before, after) {
				t.Fatal("invalid YAML was rewritten")
			}
		})
	}
	data, config := vprotest.ConfigFixture(t)
	writeConfigFixture(t, config, "coordinate-settings.json", `{"mode":"dm"}`)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.Path)
	writeConfigFixture(t, config, "coordinate-settings.json", `{"mode":"dms"}`)
	if _, err := vprocore.OpenDesktopConfig(data, config); err == nil || !strings.Contains(err.Error(), "changed after migration") {
		t.Fatalf("changed legacy preference silently ignored/reapplied: %v", err)
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("changed legacy file replaced YAML")
	}
}

func TestDesktopConfigConcurrentSettersAndRollback(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for section, changes := range map[string]map[string]any{
		"Audit":   {"AuditStrength": 3},
		"Current": {"CoordMethod": 2, "AssignedSuSource": 3, "User": "Keep Me"},
		"Custom":  {"Values": []any{false, "True", nil}},
	} {
		wg.Add(1)
		go func(section string, changes map[string]any) {
			defer wg.Done()
			if err := store.Update(section, changes); err != nil {
				t.Error(err)
			}
		}(section, changes)
	}
	wg.Wait()
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if strength, _ := vprocore.ConfigInt(values, "Audit", "AuditStrength", 0, 3); strength != 3 {
		t.Fatal("concurrent update lost audit setting")
	}
	if user, _ := vprocore.ConfigString(values, "Current", "User"); user != "Keep Me" || values["Custom"] == nil {
		t.Fatal("concurrent update lost unrelated keys")
	}
	before, _ := os.ReadFile(store.Path)
	injected := errors.New("configuration commit rejected")
	store.Replace = func(from, to string) error {
		content, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		var complete vprocore.ConfigValues
		if err := yaml.Unmarshal(content, &complete); err != nil || vprocore.ValidateDesktopConfig(complete) != nil {
			t.Fatal("replacement was not a complete validated YAML file")
		}
		if to != store.Path || filepath.Dir(from) != filepath.Dir(to) {
			t.Fatal("replacement left the configuration directory")
		}
		return injected
	}
	if err := store.Update("Current", map[string]any{"CoordMethod": 3}); !errors.Is(err, injected) {
		t.Fatalf("failed commit reported success: %v", err)
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed commit damaged previous YAML")
	}
	if mode, err := store.CoordinateMode(); err != nil || mode != vprocore.CoordinateModeDM {
		t.Fatal("failed commit changed effective preferences")
	}
	leftovers, err := filepath.Glob(filepath.Join(config, ".config-*.yml"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("failed commit leaked staging files")
	}
	store.Replace = os.Rename
	if err := store.Update("Current", map[string]any{"CoordMethod": 3}); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopConfigServicesSharePreferencesAndFailureState(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	writeConfigFixture(t, config, "desktop-selection.json", `{"activeProject":"Sample"}`)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}

	projects, err := vprocore.NewProjectServiceWithPreferences(data, config, store)
	if err != nil {
		t.Fatal(err)
	}
	coordinates, err := vprocore.NewCoordinateServiceInternal(config, store)
	if err != nil {
		t.Fatal(err)
	}
	plots, err := vprocore.NewPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	working, err := vprocore.NewWorkingUnitService(projects, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { working.Close() })
	if err := coordinates.SetCoordinateMode(vprocore.CoordinateModeDMS); err != nil {
		t.Fatal(err)
	}
	if _, err := working.SetWorkingUnitMode("env"); err != nil {
		t.Fatal(err)
	}
	if err := plots.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	if err := plots.SetCurrentUser("Migrated User"); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectProject("Sample"); err != nil {
		t.Fatal(err)
	}
	reopened, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if mode, _ := reopened.CoordinateMode(); mode != vprocore.CoordinateModeDMS {
		t.Fatal("project selection overwrote coordinate preference")
	}
	before, _ := os.ReadFile(store.Path)
	for _, invalid := range []int{-1, 4, 999} {
		if err := plots.SetAuditStrength(invalid); err == nil || plots.AuditStrength != 3 {
			t.Fatal("invalid audit strength changed effective settings")
		}
	}
	if err := plots.SetCurrentUser(""); err == nil || plots.CurrentUser != "Migrated User" {
		t.Fatal("empty audit user changed effective identity")
	}
	store.Replace = func(string, string) error { return errors.New("config disk failure") }
	if err := plots.SetAuditStrength(0); err == nil || plots.AuditStrength != 3 {
		t.Fatal("failed preference commit changed audit strength")
	}
	if err := plots.SetCurrentUser("Lost User"); err == nil || plots.CurrentUser != "Migrated User" {
		t.Fatal("failed preference commit changed user")
	}
	if _, err := projects.SelectHierarchy("None", ""); err == nil {
		t.Fatal("failed selection commit reported success")
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("service failure damaged shared configuration")
	}
	for _, name := range []string{"coordinate-settings.json", "working-unit-settings.json"} {
		if _, err := os.Stat(filepath.Join(config, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("YAML service still writes retired JSON")
		}
	}
}

func TestDesktopConfigDefaultApplicationStartupAndUnavailableSelection(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	projects, err := vprocore.NewProjectServiceWithPreferences(data, config, store)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := store.Selection()
	if err != nil || selected.ProjectPath != filepath.Join(data, "projects", "Sample.db") {
		t.Fatalf("selected backend path was not established: %#v %v", selected, err)
	}
	state, err := projects.GetState(context.Background())
	if err != nil || state.ActiveProject != "Sample" || state.ActiveHierarchy != "Sample" {
		t.Fatalf("R startup defaults failed: %#v %v", state, err)
	}
	if err := store.Update("Current", map[string]any{"ProjectPath": filepath.Join(data, "outside.db")}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.Path)
	if _, err := vprocore.NewProjectServiceWithPreferences(data, config, store); err == nil ||
		!strings.Contains(err.Error(), "external project/SU path") {
		t.Fatalf("unsupported external preference was ignored: %v", err)
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("unsupported external preference was rewritten")
	}
}

func TestDesktopConfigLegacyGoIdentityWithoutSavedJSONAndInvalidLiveYAML(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	writeConfigFixture(t, data, "working-unit.db", "legacy installation marker")
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if user, _ := vprocore.ConfigString(values, "Current", "User"); user != "User" {
		t.Fatal("prior Go installation's default identity changed")
	}
	before := writeConfigFixture(t, config, "config.yml", "Current:\n  CoordMethod: null")
	if err := store.Update("Current", map[string]any{"CoordMethod": 2}); err == nil {
		t.Fatal("setter silently repaired invalid live YAML")
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid live YAML was overwritten")
	}
}

func TestDesktopConfigInitialInstallNeverOverwritesConcurrentTarget(t *testing.T) {
	_, config := vprotest.ConfigFixture(t)
	source := filepath.Join(config, "candidate.yml")
	target := filepath.Join(config, "config.yml")
	writeConfigFixture(t, config, "candidate.yml", "new validated file")
	original := writeConfigFixture(t, config, "config.yml", "another owner's configuration")
	if err := vprocore.InstallDesktopConfig(source, target); err == nil {
		t.Fatal("first installation overwrote a concurrently created target")
	}

	actual, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("concurrent target was modified")
	}
}

func TestDesktopConfigPartialDesktopOverlayFillsOnlyAbsentSchemaVersion(t *testing.T) {
	data, config := vprotest.ConfigFixture(t)
	writeConfigFixture(t, config, "config.yml", "Desktop:\n  DatabasePaths:\n    VUser: \"explicit support path\"\n")
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	desktop, err := vprocore.ConfigSection(values, "Desktop")
	if err != nil || desktop["SchemaVersion"] != 1 ||
		desktop["DatabasePaths"].(map[string]any)["VUser"] != "explicit support path" {
		t.Fatal("partial desktop overlay lost its explicit settings or missing default")
	}
	data, config = vprotest.ConfigFixture(t)
	before := writeConfigFixture(t, config, "config.yml", "Desktop:\n  SchemaVersion: null\n")
	if _, err := vprocore.OpenDesktopConfig(data, config); err == nil {
		t.Fatal("explicit NULL schema version was silently defaulted")
	}
	after, err := os.ReadFile(filepath.Join(config, "config.yml"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("explicit invalid schema version was rewritten")
	}
}
