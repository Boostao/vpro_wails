package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sqliteServiceFixture(t *testing.T) (*ProjectService, *desktopConfig, string, string) {
	t.Helper()
	data, config := configFixture(t)
	preferences, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}

	service, err := newSQLiteProjectService(data, config, preferences)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.closeSQLiteContext(); err != nil {
			t.Error(err)
		}
	})
	return service, preferences, data, config
}

func switchSQLiteTestSU(t *testing.T, service *ProjectService, name string) (ProjectState, error) {
	t.Helper()
	coordinator, err := NewContextService(service, NewPlotService(service))
	if err != nil {
		t.Fatal(err)
	}
	selection := service.sqlite.selection
	path := ""
	if name != "None" {
		path = selection.ProjectPath
	}
	return coordinator.SwitchContext(service.contextID, ContextSelection{
		Project: selection.Project, ProjectPath: selection.ProjectPath, SU: name, SUPath: path,
		Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath,
	})
}

func TestSQLiteProjectServiceStartupRestoresOwnedContextAndPreservesYAML(t *testing.T) {
	service, preferences, data, config := sqliteServiceFixture(t)
	state, err := service.GetState(context.Background())
	if err != nil || state.ContextID == "" || state.ProjectPath == "" || state.HierarchyPath == "" ||
		state.ActiveProject != "Sample" || state.ActiveHierarchy != "Sample" {
		t.Fatalf("incomplete restored context: %+v %v", state, err)
	}
	page, err := service.ListPlots(context.Background(), 0, 25)
	if err != nil || page.Total == 0 || len(page.Plots) == 0 {
		t.Fatalf("pinned project views did not load plots: %+v %v", page, err)
	}
	nodes, err := service.GetHierarchyNodes(context.Background())
	if err != nil || len(nodes) == 0 {
		t.Fatalf("pinned hierarchy did not load: %v %v", nodes, err)
	}
	before, err := os.ReadFile(preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.closeSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	reopened, err := newSQLiteProjectService(data, config, preferences)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.closeSQLiteContext()
	after, err := os.ReadFile(preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("restoration rewrote unchanged configuration")
	}
	restored, err := reopened.GetState(context.Background())
	if err != nil || restored.ProjectPath != state.ProjectPath || restored.ContextID == state.ContextID {
		t.Fatalf("restoration lost file identity or reused old context identity: %+v %v", restored, err)
	}
}

func TestSQLiteProjectServiceFailedPublishKeepsHandlesSelectionAndPreferences(t *testing.T) {
	service, preferences, _, _ := sqliteServiceFixture(t)
	before, err := service.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := service.sqlite
	content, err := os.ReadFile(preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	preferences.replace = func(string, string) error { return errors.New("injected context persistence failure") }
	if _, err := switchSQLiteTestSU(t, service, "Sample"); err == nil || !strings.Contains(err.Error(), "persistence failure") {
		t.Fatalf("candidate published despite failed configuration: %v", err)
	}
	if service.sqlite != original {
		t.Fatal("failed publication replaced the old owner")
	}
	after, err := service.GetState(context.Background())
	if err != nil || after.ContextID != before.ContextID || after.ActiveSU != "None" || service.activeSU != "None" {
		t.Fatalf("failed publication changed effective selection: %+v %v", after, err)
	}
	actual, err := os.ReadFile(preferences.path)
	if err != nil || !bytes.Equal(content, actual) {
		t.Fatal("failed publication changed previous YAML bytes")
	}
	if _, err := service.ListPlots(context.Background(), 0, 25); err != nil {
		t.Fatalf("failed publication closed the previous context: %v", err)
	}
	preferences.replace = os.Rename
	published, err := switchSQLiteTestSU(t, service, "Sample")
	if err != nil || published.ActiveSU != "Sample" || published.ContextID == before.ContextID {
		t.Fatalf("candidate retry failed: %+v %v", published, err)
	}
	if original.conn != nil {
		t.Fatal("publication leaked the previous owned coordinator")
	}
	if _, err := switchSQLiteTestSU(t, service, "Missing"); err == nil {
		t.Fatal("unavailable candidate SU was accepted")
	}
	still, err := service.GetState(context.Background())
	if err != nil || still.ContextID != published.ContextID || still.ActiveSU != "Sample" {
		t.Fatal("failed validation changed a published context")
	}
}

func TestSQLiteProjectServiceExternalPathsDriveExistingReadsAndWrites(t *testing.T) {
	data, config := configFixture(t)
	selection, _ := sqliteContextFixture(t)
	preferences, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := preferences.update("Current", map[string]any{
		"ProjectPath": selection.ProjectPath, "CurrHierarchy": "None",
	}); err != nil {
		t.Fatal(err)
	}
	service, err := newSQLiteProjectService(data, config, preferences)
	if err != nil {
		t.Fatal(err)
	}
	defer service.closeSQLiteContext()
	state, err := service.GetState(context.Background())
	if err != nil || len(state.Projects) != 2 || state.HierarchyFile != "" {
		t.Fatalf("explicit external and managed identities not represented: %+v %v", state, err)
	}
	if _, err := service.SelectProject("Sample"); err == nil || !strings.Contains(err.Error(), "identity-bound") {
		t.Fatalf("same-name external identity was implicitly substituted: %v", err)
	}
	local := filepath.Join(data, "projects", "Sample.db")
	localBefore, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	plots, err := newPlotServiceWithPreferences(service)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListPlots(context.Background(), 0, 1)
	if err != nil || len(page.Plots) != 1 {
		t.Fatal("external coordinator did not supply plot rows")
	}
	header, err := plots.GetPlot(page.Plots[0].PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	if err := plots.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	value := "external context"
	header.PlotRepresenting = &value
	coordinator, err := NewContextService(service, plots)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.UpdatePlot(state.ContextID, *header); err != nil {
		t.Fatal(err)
	}
	actual, err := plots.GetPlot(header.PlotNumber)
	if err != nil || actual.PlotRepresenting == nil || *actual.PlotRepresenting != value {
		t.Fatalf("verified writer did not use resolved external path: %+v %v", actual, err)
	}
	localAfter, err := os.ReadFile(local)
	if err != nil || !bytes.Equal(localBefore, localAfter) {
		t.Fatal("external write was redirected into the same-name managed file")
	}
	db, err := openReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var descriptions int
	if err := db.QueryRow("SELECT COUNT(*) FROM _table_metadata WHERE table_name='Sample_Env' AND description='VP08'").Scan(&descriptions); err != nil || descriptions != 1 {
		t.Fatal("external writer changed native table descriptions")
	}
	var identity string
	if err := db.QueryRow(`SELECT User FROM Sample_Audit WHERE PlotNumber=? AND EditField='PlotRepresenting' ORDER BY rowid DESC LIMIT 1`, header.PlotNumber).Scan(&identity); err != nil || identity != "Admin" {
		t.Fatalf("external writer lost configured audit identity: %q %v", identity, err)
	}
}

func TestSQLiteProjectServiceMissingExternalAndInvalidSupportKeepPreferences(t *testing.T) {
	for _, invalidSupport := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing project", true: "unknown support role"}[invalidSupport], func(t *testing.T) {
			data, config := configFixture(t)
			preferences, err := openDesktopConfig(data, config)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "missing.db")
			if invalidSupport {
				err = preferences.update("Desktop", map[string]any{"DatabasePaths": map[string]any{"Unknown": path}})
			} else {
				err = preferences.update("Current", map[string]any{"ProjectPath": path})
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			if service, err := newSQLiteProjectService(data, config, preferences); err == nil {
				service.closeSQLiteContext()
				t.Fatal("invalid persisted context silently fell back to Sample")
			}
			after, err := os.ReadFile(preferences.path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("unavailable persisted context was silently reset")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("missing external file was created")
			}
		})
	}
}

func TestSQLiteProjectServiceClosedContextRejectsReads(t *testing.T) {
	service, _, _, _ := sqliteServiceFixture(t)
	conn := service.sqlite.conn
	if err := service.closeSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetState(context.Background()); err == nil {
		t.Fatal("closed context returned success-shaped state")
	}
	if _, err := service.ListPlots(context.Background(), 0, 25); err == nil {
		t.Fatal("closed context returned successful plot data")
	}
	if _, err := conn.ExecContext(t.Context(), "SELECT 1"); !errors.Is(err, sql.ErrConnDone) {
		t.Fatal("closed owner retained a usable connection")
	}
}
