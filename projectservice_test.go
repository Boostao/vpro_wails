package main

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSampleProjectStartupAndPlots(t *testing.T) {
	root := t.TempDir()
	service, err := NewProjectService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveProject != "Sample" || len(state.Projects) != 1 || !state.Projects[0].Compatible || state.Projects[0].Version != "VP08" {
		t.Fatalf("unexpected startup state: %+v", state)
	}
	page, err := service.ListPlots(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 52 || len(page.Plots) != 10 {
		t.Fatalf("unexpected sample plot page: %+v", page)
	}
	if _, err := service.SelectProject("../Sample"); err == nil {
		t.Fatal("path-like project name was accepted")
	}
	if _, err := service.SelectProject("Sample"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProjectService(root); err != nil {
		t.Fatalf("selection was not recoverable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "Sample.db"), []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProjectService(root); err == nil {
		t.Fatal("existing data was overwritten or invalid SQLite was accepted")
	}
}

func TestPlotPageBounds(t *testing.T) {
	service, err := NewProjectService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, bounds := range [][2]int{{-1, 10}, {0, 0}, {0, 201}} {
		if _, err := service.ListPlots(bounds[0], bounds[1]); err == nil {
			t.Fatalf("accepted invalid page: %v", bounds)
		}
	}
}

func TestStartupWithUnrelatedCorruptProject(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "projects")
	if err := os.MkdirAll(projects, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projects, "Broken.db"), []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := NewProjectService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveProject != "Sample" || len(state.Projects) != 1 || len(state.Diagnostics) != 1 {
		t.Fatalf("unexpected recovery state: %+v", state)
	}
	if state.Diagnostics[0].File != "Broken.db" || state.Diagnostics[0].Message == "" {
		t.Fatalf("missing failed project diagnostic: %+v", state.Diagnostics)
	}
}

func TestSampleSUSelectionAndRecovery(t *testing.T) {
	root := t.TempDir()
	config := t.TempDir()
	service, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.SelectSU("Sample")
	if err != nil || state.ActiveSU != "Sample" {
		t.Fatalf("could not select Sample SU: %+v: %v", state, err)
	}
	page, err := service.ListPlots(0, 100)
	if err != nil || page.Total != 51 || len(page.Plots) != 51 {
		t.Fatalf("SU did not filter the Env-SU-Admin intersection: %+v: %v", page, err)
	}
	reopened, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	state, err = reopened.GetState()
	if err != nil || state.ActiveSU != "Sample" {
		t.Fatalf("SU selection did not survive restart: %+v: %v", state, err)
	}
	state, err = reopened.SelectSU("None")
	if err != nil || state.ActiveSU != "None" {
		t.Fatalf("could not clear SU: %+v: %v", state, err)
	}
	page, err = reopened.ListPlots(0, 100)
	if err != nil || page.Total != 52 {
		t.Fatalf("clearing SU did not restore project plots: %+v: %v", page, err)
	}
	if _, err := reopened.SelectSU("../Sample"); err == nil {
		t.Fatal("path-like SU name was accepted")
	}
}

func TestProjectPlotsExcludeEnvRowsWithoutAdmin(t *testing.T) {
	root := t.TempDir()
	service, err := NewProjectService(root)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite3", filepath.Join(root, "projects", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO Sample_Env (PlotNumber) VALUES ('Orphan')`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListPlots(0, 100)
	if err != nil || page.Total != 52 || len(page.Plots) != 52 {
		t.Fatalf("unmatched Env row appeared in project-wide USysEnv: %+v: %v", page, err)
	}
}

func TestMasterSUCannotBeSelectedWithoutAuthorization(t *testing.T) {
	root := t.TempDir()
	service, err := NewProjectService(root)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite3", filepath.Join(root, "projects", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE _vpro_su_policy (table_name TEXT PRIMARY KEY, kind TEXT); INSERT INTO _vpro_su_policy VALUES ('Sample_SU', 'master')`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SelectSU("Sample"); err == nil {
		t.Fatal("selected a master SU without authorization")
	}
	state, err := service.GetState()
	if err != nil || state.ActiveSU != "None" || len(state.SUs) != 1 || state.SUs[0].Compatible {
		t.Fatalf("master SU was not disabled: %+v: %v", state, err)
	}
}

func TestHierarchySelectionSurvivesRestartAndProjectSwitch(t *testing.T) {
	root := t.TempDir()
	config := t.TempDir()
	service, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.SelectHierarchy("Sample", "Sample.db")
	if err != nil || state.ActiveHierarchy != "Sample" || state.HierarchyFile != "Sample.db" {
		t.Fatalf("could not select bundled hierarchy: %+v: %v", state, err)
	}
	nodes, err := service.GetHierarchyNodes()
	if err != nil || len(nodes) != 43 {
		t.Fatalf("bundled hierarchy nodes unavailable: %d: %v", len(nodes), err)
	}
	reopened, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	state, err = reopened.SelectProject("Sample")
	if err != nil || state.ActiveHierarchy != "Sample" || state.HierarchyFile != "Sample.db" {
		t.Fatalf("project switch cleared independent hierarchy: %+v: %v", state, err)
	}
	if _, err := reopened.SelectHierarchy("../Sample", "Sample.db"); err == nil {
		t.Fatal("accepted path-like hierarchy name")
	}
	if _, err := reopened.SelectHierarchy("Sample", "../Sample.db"); err == nil {
		t.Fatal("accepted hierarchy file outside the project directory")
	}
	state, err = reopened.SelectHierarchy("None", "")
	if err != nil || state.ActiveHierarchy != "None" {
		t.Fatalf("could not clear hierarchy: %+v: %v", state, err)
	}
}

func TestHierarchyFileIdentityAndMissingRecovery(t *testing.T) {
	root := t.TempDir()
	config := t.TempDir()
	service, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "projects", "Trees.db")
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE Sample_Hierarchy (ID INTEGER, Name TEXT, Parent INTEGER, Level INTEGER); INSERT INTO Sample_Hierarchy VALUES (91, 'Other source', NULL, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	state, err := service.SelectHierarchy("Sample", "Trees.db")
	if err != nil || state.HierarchyFile != "Trees.db" || len(state.Hierarchies) != 2 {
		t.Fatalf("hierarchy identity lost: %+v: %v", state, err)
	}
	nodes, err := service.GetHierarchyNodes()
	if err != nil || len(nodes) != 1 || nodes[0].ID != 91 {
		t.Fatalf("wrong hierarchy source selected: %+v: %v", nodes, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	state, err = reopened.GetState()
	if err != nil || state.ActiveHierarchy != "None" || state.HierarchyFile != "" {
		t.Fatalf("missing hierarchy did not deactivate: %+v: %v", state, err)
	}
}

func TestUserDataOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VPRO_DATA_DIR", root)
	directory, err := userDataDir()
	if err != nil || directory != root {
		t.Fatalf("unexpected data directory: %q: %v", directory, err)
	}
}

func TestUserConfigOverride(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("VPRO_CONFIG_DIR", directory)
	config, err := userConfigDir()
	if err != nil || config != directory {
		t.Fatalf("unexpected config directory: %q: %v", config, err)
	}
}

func TestSelectionConfigDirectoryAndLegacyFallback(t *testing.T) {
	root := t.TempDir()
	config := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "desktop-selection.json"), []byte("invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProjectServiceWithConfig(root, config); err == nil {
		t.Fatal("legacy selection was not read when config selection is absent")
	}
	legacy := []byte(`{"activeProject":"Sample"}`)
	if err := os.WriteFile(filepath.Join(root, "desktop-selection.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	service, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SelectProject("Sample"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(config, "desktop-selection.json")); err != nil {
		t.Fatalf("selection not saved in config directory: %v", err)
	}
	settings, err := os.ReadFile(filepath.Join(root, "desktop-selection.json"))
	if err != nil || string(settings) != string(legacy) {
		t.Fatalf("legacy selection was modified: %q: %v", settings, err)
	}
	if _, err := NewProjectServiceWithConfig(root, config); err != nil {
		t.Fatalf("selection was not recoverable: %v", err)
	}
}

func TestMissingSelectedProjectPersistsSampleFallback(t *testing.T) {
	root := t.TempDir()
	config := t.TempDir()
	if err := os.WriteFile(filepath.Join(config, "desktop-selection.json"), []byte(`{"activeProject":"Missing"}`), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.GetState()
	if err != nil || state.ActiveProject != "Sample" {
		t.Fatalf("missing project was not replaced with Sample: %+v: %v", state, err)
	}
	selection, err := os.ReadFile(filepath.Join(config, "desktop-selection.json"))
	if err != nil || string(selection) != `{"activeProject":"Sample"}` {
		t.Fatalf("Sample fallback was not persisted: %q: %v", selection, err)
	}
}

func TestProjectSwitchSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	service, err := NewProjectService(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SelectSU("Sample"); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(filepath.Join(root, "projects", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	path := filepath.Join(root, "projects", "Other.db")
	target, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range coreTables {
		if _, err := database.Exec(`ALTER TABLE "Sample_` + suffix + `" RENAME TO "Other_` + suffix + `"`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`UPDATE _table_metadata SET table_name = 'Other_Env' WHERE table_name = 'Sample_Env'`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	state, err := service.SelectProject("Other")
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveProject != "Other" || state.ActiveSU != "None" || len(state.Projects) != 2 {
		t.Fatalf("unexpected project switch state: %+v", state)
	}
	reopened, err := NewProjectService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err = reopened.GetState()
	if err != nil || state.ActiveProject != "Other" || state.ActiveSU != "None" {
		t.Fatalf("selection was not restored: %+v: %v", state, err)
	}
	page, err := reopened.ListPlots(50, 25)
	if err != nil || page.Total != 52 || len(page.Plots) != 2 {
		t.Fatalf("second project pagination failed: %+v: %v", page, err)
	}
}
