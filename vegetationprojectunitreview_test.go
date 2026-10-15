package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestProjectUnitReviewActualSavedModeBeforeCASAndOwnedAvailability(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, all := range []bool{false, true} {
			service, state, scope := projectUnitCASFixture(t, external, all)
			ctx := context.Background()
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(service.contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			review, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope))
			expectedAvailability := LongVegetationUnitAvailability{SelectedFields: !all, None: all}
			if err != nil || !reflect.DeepEqual(review.Scope, *scope) ||
				review.Preferences.ContextID != state.ContextID || review.Preferences.ProjectPath != state.ProjectPath ||
				review.Preferences.Project != state.ActiveProject || review.Preferences.SU != state.ActiveSU ||
				review.Preferences.SUPath != state.SUPath || review.Preferences.Values.UnitGroups != 1 ||
				review.Preferences.Availability != expectedAvailability {
				t.Fatal("scoped review normalized saved source mode or lost owned exclusive availability", review, err)
			}
			if all && (review.Preferences.SU != "None" || review.Preferences.SUPath != "") {
				t.Fatal("project None review invented a physical SU owner", review)
			}
			raw, err := json.Marshal(review)
			if err != nil {
				t.Fatal(err)
			}
			var decoded LongVegetationProjectUnitPreferences
			if err := decodeProfileLifecycleJSON(raw, &decoded, "scope", "preferences"); err != nil {
				t.Fatal("scoped preference review wire decode", err)
			}
			if !reflect.DeepEqual(decoded, review) {
				t.Fatal("scoped preference review lost exact production wire", decoded, review)
			}
			if _, err := service.GetLongVegetationProjectOptions(ctx, state.ContextID, projectReadJSON(t, scope)); err == nil {
				t.Fatal("readable current preference was mistaken for saved matching output mode")
			}
			after, err := os.ReadFile(service.contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("readonly unit review persisted planned Build choice", err)
			}
			desired := 2
			if all {
				desired = 3
			}
			saved := service.SaveLongVegetationProjectUnits(ctx, state.ContextID,
				projectUnitJSON(t, &review.Scope, review.Preferences.Values.UnitGroups, desired))
			if !saved.Outcome.Committed || !saved.Outcome.Changed || saved.Outcome.ErrorMessage != "" {
				t.Fatal("actual same-snapshot preference read could not drive reviewed CAS", saved)
			}
			current, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope))
			if err != nil || current.Preferences.Values.UnitGroups != desired || current.Preferences.Availability != expectedAvailability {
				t.Fatal("committed mode invalidated normalized Build or exclusive scope review", current, err)
			}
			*current.Scope.Build.ProjectTuples[0].SiteUnit.Text = "mutated"
			retry, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope))
			if err != nil || reflect.DeepEqual(retry.Scope, current.Scope) || !reflect.DeepEqual(retry.Scope, *scope) {
				t.Fatal("preference review aliases internal Build", retry, err)
			}
			if _, err := service.PreviewLongVegetationProject(ctx, state.ContextID, projectReadJSON(t, scope)); err != nil {
				t.Fatal("read/CAS did not unlock matching reviewed output", err)
			}
			assertProfileSUFiles(t, service.contexts, before)
		}
	}
}

func TestProjectUnitReviewStrictRequestsCleanupCancellationAndRetry(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, false, false)
	ctx := context.Background()
	before, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	zero := LongVegetationProjectUnitPreferences{}
	for _, raw := range []string{
		`{}`, `{"revision":"01","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
		`{"revision":"` + scope.Revision + `","approvalHash":"` + strings.Repeat("0", 64) + `"}`,
		`{"revision":"99","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
		`{"revision":"` + scope.Revision + `","approvalHash":"` + scope.Build.ApprovalHash + `","extra":true}`,
		`{"revision":"` + scope.Revision + `","revision":"0","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
		`{"revision":"` + scope.Revision + `","\u0072evision":"0","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
	} {
		output, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, raw)
		if err == nil || !reflect.DeepEqual(output, zero) {
			t.Fatal("invalid requested approval produced successful unit review", raw, output, err)
		}
	}
	for _, disabled := range []*LongVegetationReportScopeService{nil, NewLongVegetationReportScopeService(service.contexts, false)} {
		if output, err := disabled.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope)); err == nil || !reflect.DeepEqual(output, zero) {
			t.Fatal("disabled project preference read succeeded", output, err)
		}
	}
	if output, err := service.GetLongVegetationProjectUnitPreferences(ctx, "stale", projectReadJSON(t, scope)); err == nil || !reflect.DeepEqual(output, zero) {
		t.Fatal("stale unit review owner accepted", output, err)
	}
	ledger := service.contexts.projectReportScopes.read(state.ContextID)
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("unit review commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("unit review cleanup failure") }},
	} {
		service.snapshot = hooks
		if output, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope)); err == nil ||
			!reflect.DeepEqual(output, zero) || !reflect.DeepEqual(ledger, service.contexts.projectReportScopes.read(state.ContextID)) {
			t.Fatal("fallible read cleanup leaked unit review or changed authority", output, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error {
		cancel()
		return nil
	}}
	if output, err := service.GetLongVegetationProjectUnitPreferences(cancelled, state.ContextID, projectReadJSON(t, scope)); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(output, zero) {
		t.Fatal("in-flight cancelled unit review returned successful output", output, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if _, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope)); err != nil {
		t.Fatal("read cleanup/cancel lost persistent owner attachments or retry", err)
	}
	after, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("strict failure/read cleanup wrote actual preferences", err)
	}
}

func TestProjectUnitReviewRejectsSourceConfigDriftAndMalformedSavedMode(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, true, false)
	ctx := context.Background()
	store := service.contexts.projects.preferences
	zero := LongVegetationProjectUnitPreferences{}
	for _, mode := range []any{0, 4, "wrong"} {
		if err := store.update("ReportOptions", map[string]any{"LVUnitGroups": mode}); err != nil {
			t.Fatal(err)
		}
		output, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope))
		if err == nil || !reflect.DeepEqual(output, zero) {
			t.Fatal("planned Build normalization repaired invalid saved unit value", mode, output, err)
		}
	}
	if err := store.update("ReportOptions", map[string]any{"LVUnitGroups": 1, "LVReportTitle": "changed"}); err != nil {
		t.Fatal(err)
	}
	if output, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, scope)); err == nil || !reflect.DeepEqual(output, zero) {
		t.Fatal("complete config drift returned current unit authority", output, err)
	}
	current, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, scope.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, current)); err != nil {
		t.Fatal("explicit config-drift rebuild could not recover preference review", err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	var original sql.NullString
	if err := db.QueryRow(`SELECT FieldNumber FROM Sample_Env WHERE rowid=1`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET FieldNumber='source drift' WHERE rowid=1`); err != nil {
		t.Fatal(err)
	}
	if output, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, current)); err == nil || !reflect.DeepEqual(output, zero) {
		t.Fatal("complete original physical source drift returned current unit authority", output, err)
	}
	_, restoreErr := db.Exec(`UPDATE Sample_Env SET FieldNumber=? WHERE rowid=1`, original)
	if err := errors.Join(restoreErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, current)); err != nil {
		t.Fatal("exact source restoration could not retry unit review", err)
	}
	if _, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, current.Revision, vegetationSelectedListScope)); err != nil {
		t.Fatal(err)
	}
	if output, err := service.GetLongVegetationProjectUnitPreferences(ctx, state.ContextID, projectReadJSON(t, current)); err == nil || !reflect.DeepEqual(output, zero) {
		t.Fatal("selected-list intent inherited old project review", output, err)
	}
}
