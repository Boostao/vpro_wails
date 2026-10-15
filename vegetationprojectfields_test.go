package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReportUnitFieldDefinitionsIndependentSchemaPolicies(t *testing.T) {
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{"PlotNumber", "TEXT"}, {"LongValue", "LONG"}, {"Flag", "BOOLEAN"}, {"Floating", "REAL"},
		{"Duplicate", "TEXT"}, {"bad-name", "TEXT"},
	}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{"Plot", "VARCHAR"}, {"CountValue", "INTEGER"}, {"BitValue", "BIT"}, {"duplicate", "TEXT"},
		{"MemoValue", "MEMO"}, {"Decimal", "NUMERIC"},
	}}
	definitions := reportUnitFieldDefinitions(env, admin)
	expected := []bool{true, true, true, false, false, false, true, true, true, false, false, false}
	if len(definitions) != len(expected) {
		t.Fatal("physical duplicate definitions omitted", definitions)
	}
	for i, field := range definitions {
		table := "Env"
		column := env.Columns[i%len(env.Columns)]
		if i >= len(env.Columns) {
			table, column = "Admin", admin.Columns[i-len(env.Columns)]
		}
		if field.Table != table || field.Name != column.Name || field.DeclaredType != column.DeclaredType ||
			field.Available != expected[i] || (field.Reason == "") != expected[i] {
			t.Fatal("schema availability lost literal physical type/name or operation-specific refusal", i, field)
		}
	}
	for _, i := range []int{4, 9} {
		if !strings.Contains(definitions[i].Reason, "ambiguous") {
			t.Fatal("cross-table case-equivalent field ambiguity hidden", definitions[i])
		}
	}
}

func TestProjectFieldCatalogueOwnedAttachedExternalAndNoneSchemasZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, none := range []bool{false, true} {
			contexts, state, _ := projectOwnedSourceFixture(t, external, none)
			service := NewLongVegetationReportScopeService(contexts, true)
			ctx := context.Background()
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			initial, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
			if err != nil || initial.ContextID != state.ContextID || initial.Project != "Sample" ||
				initial.ProjectPath != state.ProjectPath || initial.SU != state.ActiveSU || initial.SUPath != state.SUPath ||
				initial.SourceKind != vegetationSelectedListScope || initial.Revision != "0" || len(initial.Definitions) == 0 {
				t.Fatal("schema catalogue invented scope or lost physical owner", initial, err)
			}
			if !none {
				selected := NewLongVegetationSelectedUnitSelectionService(contexts, true)
				ordinary, err := selected.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
				if err != nil || !reflect.DeepEqual(initial.Definitions, ordinary.Definitions) {
					t.Fatal("shared catalogue changed legacy selected-list definitions", err)
				}
			} else if initial.SU != "None" || initial.SUPath != "" {
				t.Fatal("None catalogue invented an SU source", initial)
			}
			active, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
				projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope))
			if err != nil {
				t.Fatal(err)
			}
			draft, err := service.SetLongVegetationProjectFields(ctx, state.ContextID,
				selectedUnitDraftJSON(t, active.Revision, []string{"MissingField"}))
			if err != nil {
				t.Fatal(err)
			}
			ledger := contexts.projectReportScopes.read(state.ContextID)
			current, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
			if err != nil || current.SourceKind != vegetationProjectFieldsScope || current.Revision != draft.Revision ||
				!reflect.DeepEqual(initial.Definitions, current.Definitions) ||
				!reflect.DeepEqual(ledger, contexts.projectReportScopes.read(state.ContextID)) {
				t.Fatal("catalogue changed retained invalid draft or stale scope identity", current, err)
			}
			current.Definitions[0].Name = "mutated"
			retry, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
			if err != nil || !reflect.DeepEqual(initial.Definitions, retry.Definitions) {
				t.Fatal("catalogue DTO aliases retained state", err)
			}
			assertProfileSUFiles(t, contexts, before)
			after, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("field read persisted report preferences", err)
			}
		}
	}
}

func TestProjectFieldCatalogueSchemaDriftCorrectionAndBuildDataValidityRemainSeparate(t *testing.T) {
	contexts, state, _ := projectOwnedSourceFixture(t, true, true)
	service := NewLongVegetationReportScopeService(contexts, true)
	ctx := context.Background()
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(query string) {
		t.Helper()
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	exec(`ALTER TABLE Sample_Env ADD COLUMN FixtureDuplicate TEXT;
		ALTER TABLE Sample_Admin ADD COLUMN fixtureduplicate TEXT;
		ALTER TABLE Sample_Env ADD COLUMN FixtureFlag BOOLEAN;
		ALTER TABLE Sample_Admin ADD COLUMN FixtureReal REAL;
		UPDATE Sample_Env SET PlotNumber=CAST(X'FF' AS TEXT) WHERE rowid=1`)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	catalogue, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
	if err != nil {
		t.Fatal("schema correction must remain readable when physical row values are invalid", err)
	}
	found := map[string]LongVegetationSelectedUnitField{}
	for _, field := range catalogue.Definitions {
		found[field.Table+"."+field.Name] = field
	}
	for name, available := range map[string]bool{
		"Env.FixtureDuplicate": false, "Admin.fixtureduplicate": false, "Env.FixtureFlag": true, "Admin.FixtureReal": false,
	} {
		field, ok := found[name]
		if !ok || field.Available != available || (field.Reason == "") != available {
			t.Fatal("schema drift was hidden or treated as data validation", name, field)
		}
	}
	assertProfileSUFiles(t, contexts, before)
	active, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope))
	if err != nil {
		t.Fatal(err)
	}
	active, err = service.SetLongVegetationProjectFields(ctx, state.ContextID,
		selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	ledger := contexts.projectReportScopes.read(state.ContextID)
	if output, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision); err == nil || output != nil {
		t.Fatal("schema availability authorized malformed row Build", output, err)
	}
	if !reflect.DeepEqual(ledger, contexts.projectReportScopes.read(state.ContextID)) {
		t.Fatal("failed data read mutated Build/revision")
	}
	exec(`UPDATE Sample_Env SET PlotNumber='108050' WHERE rowid=1;
		ALTER TABLE Sample_Admin DROP COLUMN fixtureduplicate`)
	catalogue, err = service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	found = map[string]LongVegetationSelectedUnitField{}
	for _, field := range catalogue.Definitions {
		found[field.Table+"."+field.Name] = field
	}
	if !found["Env.FixtureDuplicate"].Available || found["Env.FixtureDuplicate"].Reason != "" {
		t.Fatal("schema correction did not restore explicit availability", found)
	}
	if output, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision); err != nil || !output.BuildCurrent {
		t.Fatal("corrected physical rows could not retry original Build", output, err)
	}
}

func TestProjectFieldCatalogueRefusesViewsMissingTablesAndRecovers(t *testing.T) {
	contexts, state, _ := projectOwnedSourceFixture(t, false, true)
	service := NewLongVegetationReportScopeService(contexts, true)
	ctx := context.Background()
	initial, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE FixtureAdmin AS SELECT * FROM Sample_Admin;
		DROP TABLE Sample_Admin; CREATE VIEW Sample_Admin AS SELECT * FROM FixtureAdmin`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "DROP VIEW Sample_Admin"} {
		if query != "" {
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		output, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
		if output != nil || err == nil || !strings.Contains(err.Error(), "original physical table") {
			t.Fatal("project schema read substituted view or absent physical Admin", output, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	if _, err := db.Exec(`CREATE TABLE Sample_Admin AS SELECT * FROM FixtureAdmin; DROP TABLE FixtureAdmin`); err != nil {
		t.Fatal(err)
	}
	retry, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID)
	if err != nil || !reflect.DeepEqual(initial, retry) {
		t.Fatal("physical schema restoration lost catalogue correction/retry", retry, err)
	}
}

func TestProjectFieldCatalogueDisabledStaleCancelledCleanupAndOwnerLease(t *testing.T) {
	contexts, state, _ := projectOwnedSourceFixture(t, true, true)
	service := NewLongVegetationReportScopeService(contexts, true)
	ctx := context.Background()
	for _, disabled := range []*LongVegetationReportScopeService{nil, NewLongVegetationReportScopeService(contexts, false)} {
		if output, err := disabled.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID); err == nil || output != nil {
			t.Fatal("disabled field catalogue returned success", output, err)
		}
	}
	if output, err := service.GetLongVegetationProjectFieldDefinitions(ctx, "stale-context"); err == nil || output != nil {
		t.Fatal("stale catalogue owner accepted", output, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if output, err := service.GetLongVegetationProjectFieldDefinitions(cancelled, state.ContextID); !errors.Is(err, context.Canceled) || output != nil {
		t.Fatal("cancelled catalogue returned success", output, err)
	}
	before := contexts.projectReportScopes.read(state.ContextID)
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("field snapshot failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("field cleanup failure") }},
	} {
		service.snapshot = hooks
		if output, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID); err == nil || output != nil ||
			!reflect.DeepEqual(before, contexts.projectReportScopes.read(state.ContextID)) {
			t.Fatal("fallible read cleanup leaked successful catalogue or changed authority", output, err)
		}
	}
	service.snapshot = publicationReadSnapshotHooks{}
	inflight, stopInflight := context.WithCancel(ctx)
	service.snapshot.commitRead = func(*sql.Tx) error {
		stopInflight()
		return nil
	}
	if output, err := service.GetLongVegetationProjectFieldDefinitions(inflight, state.ContextID); output != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight snapshot cancellation leaked successful schema output", output, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	short, stop := context.WithTimeout(ctx, 20*time.Millisecond)
	output, readErr := service.GetLongVegetationProjectFieldDefinitions(short, state.ContextID)
	stop()
	owner.mu.Unlock()
	if output != nil || !errors.Is(readErr, context.DeadlineExceeded) {
		t.Fatal("field read bypassed cancellable owner lease", output, readErr)
	}
	if output, err := service.GetLongVegetationProjectFieldDefinitions(ctx, state.ContextID); err != nil || len(output.Definitions) == 0 {
		t.Fatal("cleanup/cancellation lost attachments or retry ownership", output, err)
	}
}
