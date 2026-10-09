package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func profileSUProjectFixture(t *testing.T) (*ContextService, ProjectState, ProfileSUProjectCreation) {
	t.Helper()
	service, state, file := profileSUFixture(t)
	review, err := service.ReviewProjectPlotProfileSUInProject(context.Background(), state.ContextID, file.Review.Filter)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, ProfileSUProjectCreation{Review: review, Name: "InProject", Confirmed: true}
}

func TestProfileSUProjectPreservesExistingDataDescriptionsAndIndependentSelection(t *testing.T) {
	service, state, request := profileSUProjectFixture(t)
	c, ctx := service.projects.sqlite, context.Background()
	files := databaseBytes(t, c.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := readSQLiteStorageRows(ctx, c.conn, "project", "sqlite_master", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	before := map[string]ProjectMetadataTable{}
	for _, row := range schema.Rows {
		if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" {
			name := *row.Cells[1].Text
			before[name], err = readSQLiteStorageRows(ctx, c.conn, "project", name, "", nil, "")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	result, err := service.SaveProjectPlotProfileSUInProject(ctx, state.ContextID, request)
	if err != nil || result.Name != request.Name || result.Path != state.ProjectPath || result.PlotCount != 11 {
		t.Fatal("reviewed existing-project SU not created", result, err)
	}
	if service.projects.sqlite != c || service.projects.contextID != state.ContextID {
		t.Fatal("SU creation implicitly selected/attached the new table")
	}
	afterConfig, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, afterConfig) {
		t.Fatal("SU creation wrote YAML", err)
	}
	afterSchema, err := readSQLiteStorageRows(ctx, c.conn, "project", "sqlite_master", "", nil, "")
	if err != nil || len(afterSchema.Rows) != len(schema.Rows)+4 || !reflect.DeepEqual(schema.Rows, afterSchema.Rows[:len(schema.Rows)]) {
		t.Fatal("existing schema/indexes/views/triggers changed", err)
	}
	for name, original := range before {
		current, err := readSQLiteStorageRows(ctx, c.conn, "project", name, "", nil, "")
		if err != nil || !reflect.DeepEqual(current, original) {
			t.Fatal("existing data/descriptions changed", name, err)
		}
	}
	for role, original := range files {
		if sameDesktopPath(c.attachments[role], state.ProjectPath) {
			continue
		}
		current, err := os.ReadFile(c.attachments[role])
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("independent support/source file changed", role, err)
		}
	}
	var raw string
	if err := c.conn.QueryRowContext(ctx, `SELECT Proposal FROM project.__VPRO_ProfileSUHistory WHERE ID=1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved ProfileSUProjectCreation
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || !reflect.DeepEqual(saved, request) {
		t.Fatal("creation history lost the complete destination/description/source proposal", err)
	}
	next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: result.Name, SUPath: result.Path, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
	if err != nil || next.ActiveSU != result.Name {
		t.Fatal("created existing-file SU cannot be selected independently", next, err)
	}
	plots, err := service.projects.ListPlots(ctx, 0, 25)
	if err != nil || plots.Total != 11 {
		t.Fatal("created existing-file SU cannot filter the selected project", plots, err)
	}
}

func TestProfileSUProjectRejectsStaleTransportDestinationAndCollision(t *testing.T) {
	service, state, request := profileSUProjectFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, alter := range []func(*ProfileSUProjectCreation){
		func(value *ProfileSUProjectCreation) { value.Confirmed = false },
		func(value *ProfileSUProjectCreation) { value.Name = "None" },
		func(value *ProfileSUProjectCreation) { value.Name = "Sample" },
		func(value *ProfileSUProjectCreation) { value.Review.Path = "C:\\foreign.db" },
		func(value *ProfileSUProjectCreation) { value.Review.Project = "Foreign" },
		func(value *ProfileSUProjectCreation) { value.Review.Metadata.Rows = nil },
	} {
		invalid := request
		alter(&invalid)
		if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), state.ContextID, invalid); err == nil {
			t.Fatal("invalid/stale destination accepted")
		}
	}
	for _, raw := range []string{`{}`, `{"review":null,"name":"Good","confirmed":true}`,
		`{"review":{},"name":"\ud800","confirmed":true}`,
		`{"review":{},"name":"Good","confirmed":true,"overwrite":true}`,
		`{"review":{"su":{},"project":"Sample","path":"C:\\p.db","metadata":{}},"name":"Good","confirmed":true}`} {
		var decoded ProfileSUProjectCreation
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("partial/repaired/unsupported transport accepted", raw)
		}
	}
	if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), "stale", request); err == nil {
		t.Fatal("stale context created an SU")
	}
	assertProfileSUFiles(t, service, before)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE VIEW inproject_su AS SELECT 'Protected' AS PlotNumber`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("case-insensitive view collision replaced an existing object")
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DROP VIEW inproject_su; CREATE TABLE __VPRO_ProfileSUHistory(Protected TEXT)`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("incompatible history committed staged SU rows/indexes")
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DROP TABLE __VPRO_ProfileSUHistory`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("retained proposal cannot retry", err)
	}
}

func TestProfileSUProjectAppendsIndependentCreationProposals(t *testing.T) {
	service, state, request := profileSUProjectFixture(t)
	ctx := context.Background()
	for _, name := range []string{"First", "Second"} {
		request.Name = name
		if _, err := service.SaveProjectPlotProfileSUInProject(ctx, state.ContextID, request); err != nil {
			t.Fatal("independent creation could not append to retained provenance", name, err)
		}
	}
	rows, err := service.projects.sqlite.conn.QueryContext(ctx, `SELECT ID,Proposal FROM project.__VPRO_ProfileSUHistory ORDER BY ID`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for i, name := range []string{"First", "Second"} {
		if !rows.Next() {
			t.Fatal("missing creation event", name, rows.Err())
		}
		var id int
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		var saved ProfileSUProjectCreation
		if err := json.Unmarshal([]byte(raw), &saved); err != nil {
			t.Fatal(err)
		}
		request.Name = name
		if id != i+1 || !reflect.DeepEqual(saved, request) {
			t.Fatal("creation event lost its independent identity/proposal", id, name)
		}
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatal("unexpected creation history", rows.Err())
	}
}

func TestProfileSUProjectLateCancellationAndSourceDriftRollBackEverything(t *testing.T) {
	for _, cancelLate := range []bool{true, false} {
		t.Run(fmt.Sprint(cancelLate), func(t *testing.T) {
			service, state, request := profileSUProjectFixture(t)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := service.saveProfileSUInProject(ctx, state.ContextID, request, func(tx *sql.Tx) error {
				var count int
				if err := tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM InProject_SU`).Scan(&count); err != nil || count != 11 {
					t.Fatal("late boundary did not observe staged rows", count, err)
				}

				if cancelLate {
					cancel()
					return nil
				}
				_, err := tx.ExecContext(ctx, `UPDATE Sample_Env SET SiteSurveyor='late source change' WHERE PlotNumber='108050'`)
				return err
			})
			if err == nil || cancelLate && !errors.Is(err, context.Canceled) {
				t.Fatal("late cancellation/source drift committed or lost cause", err)
			}
			assertProfileSUFiles(t, service, before)
			if _, err := service.SaveProjectPlotProfileSUInProject(context.Background(), state.ContextID, request); err != nil {
				t.Fatal("retained rolled-back proposal cannot retry", err)
			}
		})
	}
}
