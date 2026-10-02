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

func profileTableFixture(t *testing.T, external bool) (*ContextService, ProjectState, PlotProfileTableCreation) {
	t.Helper()
	var service *ContextService
	var state ProjectState
	if external {
		service, state, _ = independentProfileFixture(t, false)
		state = authorizeProfileFixture(t, service, state, true)
	} else {
		service, state, _ = profileRunFixture(t)
	}
	review, err := service.ReviewPlotProfileTableCreation(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, PlotProfileTableCreation{Review: review, Name: "Additional", Confirmed: true}
}

func TestProfileTableCreationPreservesExistingTablesDescriptionsAndIndependentSelection(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project-file", true: "authorized-external-file"}[external], func(t *testing.T) {
			service, state, request := profileTableFixture(t, external)
			ctx, c := context.Background(), service.projects.sqlite
			role, _, err := c.profileLocation()
			if err != nil {
				t.Fatal(err)
			}
			files := databaseBytes(t, c.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := readSQLiteStorageRows(ctx, c.conn, role, "sqlite_master", "", nil, "")
			if err != nil {
				t.Fatal(err)
			}
			tables := map[string]ProjectMetadataTable{}
			for _, row := range schema.Rows {
				if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" {
					name := *row.Cells[1].Text
					tables[name], err = readSQLiteStorageRows(ctx, c.conn, role, name, "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			created, err := service.CreatePlotProfileTable(ctx, state.ContextID, request)
			if err != nil || created.Source.Path != state.PlotProfile.Source.Path || created.Source.Name != request.Name ||
				created.Table != request.Name+"_Profile" || created.RuleCount != 0 {
				t.Fatal("owned existing-file profile not created", created, err)
			}
			if service.projects.sqlite != c || service.projects.contextID != state.ContextID {
				t.Fatal("creation implicitly switched selection or granted new-table ownership")
			}
			afterConfig, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, afterConfig) {
				t.Fatal("creation changed YAML", err)
			}
			afterSchema, err := readSQLiteStorageRows(ctx, c.conn, role, "sqlite_master", "", nil, "")
			if err != nil || len(afterSchema.Rows) != len(schema.Rows)+2 ||
				!reflect.DeepEqual(afterSchema.Rows[:len(schema.Rows)], schema.Rows) {
				t.Fatal("old tables/indexes/views/triggers/schema changed", err)
			}
			for name, original := range tables {
				observed, err := readSQLiteStorageRows(ctx, c.conn, role, name, "", nil, "")
				if err != nil || !reflect.DeepEqual(observed, original) {
					t.Fatal("original data or descriptions changed", name, err)
				}
			}
			for otherRole, original := range files {
				if sameDesktopPath(c.attachments[otherRole], created.Source.Path) {
					continue
				}
				current, err := os.ReadFile(c.attachments[otherRole])
				if err != nil || !bytes.Equal(original, current) {
					t.Fatal("independent project/support file changed", otherRole, err)
				}
			}
			var raw string
			if err := c.conn.QueryRowContext(ctx, `SELECT Proposal FROM `+quoteHeaderIdentifier(role)+
				`.__VPRO_ProfileCreationHistory WHERE ID=1`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var saved PlotProfileTableCreation
			if err := json.Unmarshal([]byte(raw), &saved); err != nil || !reflect.DeepEqual(saved, request) {
				t.Fatal("complete template/ownership/proposal provenance omitted", err)
			}
			request.Name = "Second"
			if _, err := service.CreatePlotProfileTable(ctx, state.ContextID, request); err != nil {
				t.Fatal("valid shared history cannot append another independent table", err)
			}
			selected, err := service.SelectPlotProfile(state.ContextID, created.Source)
			if err != nil || selected.PlotProfile.Writable && external {
				t.Fatal("new table cannot be explicitly selected without an inherited external grant", err)
			}
			selected = authorizeProfileFixture(t, service, selected, true)
			review, err := service.ReviewProjectPlotProfile(ctx, selected.ContextID)
			if err != nil || !reflect.DeepEqual(review.Rules, request.Review.Template.Template) || len(review.Descriptions.Rows) != 0 {
				t.Fatal("new profile is not usable or inherited phantom descriptions", err)
			}
			if _, err := service.CreateProjectPlotProfileRule(ctx, selected.ContextID, blankProfileCreation(review.Rules)); err != nil {
				t.Fatal("new-table independent rule workflow failed", err)
			}
		})
	}
}

func TestProfileTableCreationRejectsReadOnlyStaleOwnershipTransportAndTemplate(t *testing.T) {
	service, state, _ := independentProfileFixture(t, false)
	if _, err := service.ReviewPlotProfileTableCreation(context.Background(), state.ContextID); err == nil {
		t.Fatal("readonly external selection can review write creation")
	}
	state = authorizeProfileFixture(t, service, state, true)
	review, err := service.ReviewPlotProfileTableCreation(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request := PlotProfileTableCreation{Review: review, Name: "Additional", Confirmed: true}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, alter := range []func(*PlotProfileTableCreation){
		func(value *PlotProfileTableCreation) { value.Confirmed = false },
		func(value *PlotProfileTableCreation) { value.Name = "Sample" },
		func(value *PlotProfileTableCreation) { value.Name = "None" },
		func(value *PlotProfileTableCreation) { value.Name = strings.Repeat("A", 32) },
		func(value *PlotProfileTableCreation) { value.Review.Template.MetadataAbsent = false },
		func(value *PlotProfileTableCreation) { value.Review.Profile.Source.Writable = false },
		func(value *PlotProfileTableCreation) { value.Review.Profile.Source.Source.Path = state.ProjectPath },
	} {
		invalid := request
		alter(&invalid)
		if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, invalid); err == nil {
			t.Fatal("invalid creation accepted")
		}
	}
	for _, raw := range []string{`{}`, `{"review":null,"name":"Additional","confirmed":true}`,
		`{"review":{"template":null,"profile":null},"name":"Additional","confirmed":true}`,
		`{"review":{},"name":"\ud800","confirmed":true}`, `{"review":{},"name":"Additional","confirmed":null}`,
		`{"review":{},"name":"Additional","confirmed":true,"overwrite":true}`} {
		var decoded PlotProfileTableCreation
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("incomplete or repaired transport accepted", raw)
		}
	}
	if _, err := service.CreatePlotProfileTable(context.Background(), "stale", request); err == nil {
		t.Fatal("stale editor created a table")
	}
	assertProfileSUFiles(t, service, before)
	state = authorizeProfileFixture(t, service, state, false)
	if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("revoked authorization accepted old review")
	}
	assertProfileSUFiles(t, service, before)
}

func TestProfileTableCreationCollisionAndProvenanceRollbackRetainRetry(t *testing.T) {
	for _, setup := range []string{
		`CREATE VIEW additional_profile AS SELECT 1 AS Protected`,
		`CREATE TABLE ADDITIONAL_PROFILE(Protected TEXT); INSERT INTO ADDITIONAL_PROFILE VALUES('do not replace')`,
		`CREATE TABLE __VPRO_ProfileCreationHistory(Protected TEXT)`,
		profileCreationHistorySQL + `; CREATE TRIGGER RejectCreation AFTER INSERT ON __VPRO_ProfileCreationHistory BEGIN SELECT RAISE(ABORT,'history fault'); END`,
	} {
		t.Run(setup, func(t *testing.T) {
			service, state, request := profileTableFixture(t, false)
			path := state.ProjectPath
			db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(setup); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, request); err == nil {
				t.Fatal("collision/incompatible history committed")
			}
			assertProfileSUFiles(t, service, before)
			if strings.Contains(setup, "PROFILE(") {
				_, err = db.Exec(`DROP TABLE ADDITIONAL_PROFILE`)
			} else if strings.Contains(setup, "VIEW") {
				_, err = db.Exec(`DROP VIEW additional_profile`)
			} else {
				_, err = db.Exec(`DROP TABLE __VPRO_ProfileCreationHistory`)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, request); err != nil {
				t.Fatal("retained proposal cannot retry after independent fault correction", err)
			}
		})
	}
}

func TestProfileTableCreationLateCancellationRollsBackSchemaAndProvenance(t *testing.T) {
	service, state, request := profileTableFixture(t, false)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := service.projects.sqlite.createProfileTable(ctx, request, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM __VPRO_ProfileCreationHistory`).Scan(&count); err != nil || count != 1 {
			t.Fatal("late cancellation did not observe staged provenance", count, err)
		}

		if _, err := readSQLiteStorageRows(context.Background(), tx, "main", request.Name+"_Profile", "", nil, ""); err != nil {
			t.Fatal("late cancellation did not observe staged table", err)
		}
		cancel()
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatal("late transaction cancellation lost cause or committed", err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := os.Stat(state.ProjectPath + "-journal"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled transaction left a journal", err)
	}
	if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("retained cancelled proposal cannot retry", err)
	}
}

func TestProfileTableCreationRejectsChangedRulesDescriptionsAndTemplate(t *testing.T) {
	for _, change := range []struct{ role, sql string }{
		{"project", `UPDATE Sample_Profile SET Criteria='Changed after review' WHERE rowid=3`},
		{"project", `UPDATE _table_metadata SET description='Changed after review' WHERE table_name='Sample_Profile'`},
		{"VPro64", `CREATE INDEX ChangedTemplate ON USysProfileTable("Order")`},
	} {
		t.Run(change.role+change.sql, func(t *testing.T) {
			service, state, request := profileTableFixture(t, false)
			c := service.projects.sqlite
			db, err := sql.Open("sqlite3", sqliteFileURI(c.attachments[change.role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(change.sql); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, c.attachments)
			if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, request); err == nil {
				t.Fatal("stale rules/descriptions/template allowed creation")
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}

func TestProfileTableCreationLateOwnershipFailureRollsBackAndRetries(t *testing.T) {
	service, state, request := profileTableFixture(t, true)
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	path := c.attachments["profile"]
	replacement := filepath.Join(t.TempDir(), "replacement.db")
	if err := os.WriteFile(replacement, before["profile"], 0600); err != nil {
		t.Fatal(err)
	}
	_, err := c.createProfileTable(context.Background(), request, func(*sql.Tx) error {
		c.attachments["profile"] = replacement
		return nil
	})
	c.attachments["profile"] = path
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal("late file ownership failure was not rejected", err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := service.CreatePlotProfileTable(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("restored original owner cannot retry", err)
	}
}
