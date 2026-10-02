package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlotProfileSelectionPreservesProjectAndAllStoredData(t *testing.T) {
	service, state, input := profileRunFixture(t)
	ctx := context.Background()
	owner := service.projects.sqlite
	originalFiles := databaseBytes(t, owner.attachments)
	originalConfig, err := service.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, present := originalConfig["Current"].(map[string]any)["ProfilePath"]; present {
		t.Fatal("fixture implicitly activated previously unavailable profile configuration")
	}
	path := filepath.Join(t.TempDir(), "profile O'Brien #.db")
	if err := os.WriteFile(path, originalFiles["project"], 0600); err != nil {
		t.Fatal(err)
	}
	choices, err := service.ListPlotProfileSources(ctx, state.ContextID, path)
	if err != nil || len(choices) != 1 || !choices[0].Available || choices[0].Writable {
		t.Fatal("external readonly original profile unavailable", choices, err)
	}
	if _, err := service.SelectPlotProfile("stale", choices[0].Source); err == nil {
		t.Fatal("foreign context selected an external profile")
	}
	next, err := service.SelectPlotProfile(state.ContextID, choices[0].Source)
	if err != nil || next.ContextID == state.ContextID || next.ProjectPath != state.ProjectPath ||
		next.ActiveProject != state.ActiveProject || next.ActiveSU != state.ActiveSU || next.PlotProfile.Writable {
		t.Fatal("profile selection changed project/SU or inferred external write authority", next, err)
	}
	if _, err := service.ReviewProjectPlotProfile(ctx, state.ContextID); err == nil {
		t.Fatal("old editor identity remained authorized")
	}
	review, err := service.ReviewProjectPlotProfile(ctx, next.ContextID)
	if err != nil || !reflect.DeepEqual(review.Rules, input.OriginalRules) {
		t.Fatal("selected rules lost physical storage/snapshot identity", review, err)
	}
	result, err := service.RunProjectPlotProfile(ctx, next.ContextID, input)
	if err != nil || len(result.PlotNumbers) != 11 || result.Project != state.ActiveProject {
		t.Fatal("external rules did not execute against the same selected project/SU", result, err)
	}
	if _, err := service.ResolveProjectPlotProfileNavigation(ctx, next.ContextID, ProjectPlotProfileFilterRequest{input, result}); err != nil {
		t.Fatal("external readonly result navigation failed", err)
	}
	if err := service.SaveProjectPlotProfile(ctx, next.ContextID, ProjectPlotProfileEdit{OriginalRules: review.Rules}); err == nil ||
		!strings.Contains(err.Error(), "read-only") {
		t.Fatal("selected external profile acquired writer authority", err)
	}
	for role, original := range originalFiles {
		now, err := os.ReadFile(owner.attachments[role])
		if err != nil || !bytes.Equal(original, now) {
			t.Fatal("source discovery/selection/run changed canonical bytes", role, err)
		}
	}
	external, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(external, originalFiles["project"]) {
		t.Fatal("external profile discovery/selection/run wrote storage", err)
	}
	configured, err := service.projects.preferences.plotProfileSelection()
	if err != nil || configured == nil || *configured != choices[0].Source {
		t.Fatal("explicit existing vocabulary/path was not retained in YAML", configured, err)
	}
	reopened, err := newSQLiteProjectService(service.projects.root, service.projects.config, service.projects.preferences)
	if err != nil {
		t.Fatal("persistent selection cannot reopen", err)
	}
	defer reopened.closeSQLiteContext()
	reopenedInfo, err := reopened.sqlite.profileInfo(ctx)
	if err != nil || reopenedInfo.Source != *configured {
		t.Fatal("restart silently replaced selected profile")
	}
	none, err := service.SelectPlotProfile(next.ContextID, PlotProfileSource{"None", ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewProjectPlotProfile(ctx, none.ContextID); err == nil {
		t.Fatal("None silently repaired to project-local profile")
	}
	if _, err := service.RunProjectPlotProfile(ctx, none.ContextID, input); err == nil {
		t.Fatal("None executed project-local rules")
	}
}

func TestPlotProfileSelectionFailedPublicationRetainsContextAndYAML(t *testing.T) {
	service, state, _ := profileRunFixture(t)
	before, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	originalOwner := service.projects.sqlite
	for _, source := range []PlotProfileSource{
		{"None", state.ProjectPath}, {"Sample", ""}, {"Sample", "relative.db"},
		{"Missing", state.ProjectPath}, {"Sample", filepath.Join(t.TempDir(), "missing.db")},
	} {
		if _, err := service.SelectPlotProfile(state.ContextID, source); err == nil {
			t.Fatal("malformed/unavailable selection published", source)
		}
	}
	service.projects.preferences.replace = func(string, string) error { return errors.New("configuration publication rejected") }
	_, err = service.SelectPlotProfile(state.ContextID, PlotProfileSource{state.ActiveProject, state.ProjectPath})
	if err == nil {
		t.Fatal("failed YAML publication accepted")
	}
	after, readErr := os.ReadFile(service.projects.preferences.path)
	if readErr != nil || !bytes.Equal(before, after) || service.projects.contextID != state.ContextID ||
		service.projects.sqlite != originalOwner || originalOwner.conn == nil {
		t.Fatal("failed profile selection changed context/config or closed original owner", err, readErr)
	}
	service.projects.preferences.replace = os.Rename
	if _, err := service.SelectPlotProfile(state.ContextID, PlotProfileSource{state.ActiveProject, state.ProjectPath}); err != nil {
		t.Fatal("unchanged retained selection could not retry", err)
	}
}

func TestPlotProfileSelectionRejectsMalformedTransport(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"name":"Sample","path":null}`, `{"name":null,"path":""}`,
		`{"name":"Sample","path":"C:\\x.db","unknown":true}`,
		`{"name":"\ud800","path":"C:\\x.db"}`, `{"name":"None","path":"C:\\x.db"}`,
	} {
		var source PlotProfileSource
		if err := json.Unmarshal([]byte(raw), &source); err == nil {
			t.Fatal("malformed/repaired source transport accepted", raw)
		}
	}
}

func TestPlotProfileLiteralTableMetadataAndParentWriterRemainIndependent(t *testing.T) {
	service, state, _ := profileRunFixture(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "literal external profile.db")
	data, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	name := " Odd ' # "
	table := name + "_Profile"
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`ALTER TABLE Sample_Profile RENAME TO ` + quoteHeaderIdentifier(table)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE _table_metadata RENAME TO ProfileMetadataOriginal;
		CREATE TABLE _table_metadata AS SELECT * FROM ProfileMetadataOriginal;
		DROP TABLE ProfileMetadataOriginal`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE _table_metadata SET table_name=? WHERE table_name='Sample_Profile';
				INSERT INTO _table_metadata(table_name,description) VALUES(?,NULL),(?,'');
				DELETE FROM Sample_Env WHERE PlotNumber='108050'`, table, table, table); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next, err := service.SelectPlotProfile(state.ContextID, PlotProfileSource{name, path})
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectPlotProfile(ctx, next.ContextID)
	if err != nil || review.Table != table || review.Source.Source.Name != name || review.Source.Writable {
		t.Fatal("literal source name/readonly authority normalized", review, err)
	}
	nulls, empty := 0, 0
	for _, row := range review.Descriptions.Rows {
		for index, column := range review.Descriptions.Columns {
			if column.Name == "description" {
				cell := row.Cells[index]
				if cell.Storage == "null" {
					nulls++
				}
				if cell.Storage == "text" && cell.Text != nil && *cell.Text == "" {
					empty++
				}
			}
		}
	}
	if nulls != 1 || empty != 1 {
		t.Fatal("duplicate NULL/empty descriptions were merged/repaired", review.Descriptions)
	}
	header, err := service.GetPlot(ctx, next.ContextID, "108050")
	if err != nil || header == nil || header.PlotNumber != "108050" {
		t.Fatal("parent read followed the external profile's unrelated Env table", header, err)
	}
	writer, err := service.projects.sqlite.projectDatabase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := writer.QueryRowContext(ctx, `SELECT COUNT(*) FROM Sample_Env WHERE PlotNumber='108050'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("parent writer followed external profile ownership", count, err)
	}
	if _, err := service.CreateProjectPlotProfileRule(ctx, next.ContextID, blankProfileCreation(review.Rules)); err == nil ||
		!strings.Contains(err.Error(), "read-only") {
		t.Fatal("external creation acquired implicit writer authority", err)
	}
	if err := service.DeleteProjectPlotProfileRule(ctx, next.ContextID, ProjectPlotProfileDeletion{
		OriginalRules: review.Rules, RowID: review.Rules.Rows[0].RowID, Confirmed: true,
	}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatal("external deletion acquired implicit writer authority", err)
	}
	changed, err := service.SwitchContext(next.ContextID, ContextSelection{Project: next.ActiveProject,
		ProjectPath: next.ProjectPath, SU: next.ActiveSU, SUPath: next.SUPath,
		Hierarchy: next.ActiveHierarchy, HierarchyPath: next.HierarchyPath})
	if err != nil || changed.PlotProfile.Source != next.PlotProfile.Source {
		t.Fatal("project/SU context operation silently reset independently selected profile", changed, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("literal profile selection wrote original fields/description metadata/history", err)
	}
}

func TestPlotProfileDiscoveryCancellationAndUnavailableSchemaAreExplicit(t *testing.T) {
	service, state, _ := profileRunFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ListPlotProfileSources(ctx, state.ContextID, state.ProjectPath); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled profile discovery published results", err)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE Old_Profile("Order" INTEGER);
				CREATE VIEW View_Profile AS SELECT * FROM Sample_Profile`); err != nil {
		t.Fatal(err)
	}
	release()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	sources, err := service.ListPlotProfileSources(context.Background(), state.ContextID, state.ProjectPath)
	if err != nil || len(sources) != 2 {
		t.Fatal("discovery hid a physical unavailable table or offered a view as storage", sources, err)
	}
	old := sources[0]
	if old.Source.Name != "Old" || old.Available || old.Writable || old.Reason == "" {
		t.Fatal("physical availability was inferred or failure silently discarded", sources)
	}
	if _, err := service.SelectPlotProfile(state.ContextID, old.Source); err == nil {
		t.Fatal("incomplete profile was repaired/selected")
	}
	for role, original := range before {
		now, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(original, now) {
			t.Fatal("unavailable profile was changed/repaired", role, err)
		}
	}
}
