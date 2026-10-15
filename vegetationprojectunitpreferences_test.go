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
	"testing"
	"time"
)

func projectUnitJSON(t *testing.T, scope *LongVegetationReportScope, expected, proposed int) string {
	t.Helper()
	raw, err := json.Marshal(vegetationProjectUnitRequest{scope.Revision, scope.Build.ApprovalHash,
		LongVegetationUnitPreferenceValues{expected}, LongVegetationUnitPreferenceValues{proposed}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func projectUnitCASFixture(t *testing.T, external, all bool) (*LongVegetationReportScopeService, ProjectState, *LongVegetationReportScope) {
	t.Helper()
	service, state, scope := projectPreviewFixture(t, external, all, all)
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": 1}); err != nil {
		t.Fatal(err)
	}
	return service, state, scope
}

func TestProjectUnitSaveAtomicReviewedModeAndHistoricalNoop(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, all := range []bool{false, true} {
			service, state, scope := projectUnitCASFixture(t, external, all)
			desired := 2
			if all {
				desired = 3
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			saved := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, desired))
			if !saved.Outcome.Changed || !saved.Outcome.Committed || saved.Outcome.ErrorMessage != "" ||
				saved.SourceKind != scope.SourceKind || saved.Revision != scope.Revision {
				t.Fatal("project Save lost scope-owned mutation receipt", saved)
			}
			if _, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, projectReadJSON(t, scope)); err != nil {
				t.Fatal("committed normalized-mode Save invalidated original Build authority", err)
			}
			config, err := os.ReadFile(service.contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			noop := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, desired, desired))
			after, err := os.ReadFile(service.contexts.projects.preferences.path)
			if noop.Outcome.Changed || noop.Outcome.Committed || noop.Outcome.ErrorMessage != "" ||
				err != nil || !bytes.Equal(config, after) {
				t.Fatal("unchanged historical project mode rewrote YAML or fabricated commit", noop, err)
			}
			assertProfileSUFiles(t, service.contexts, before)
		}
	}
}

func TestProjectUnitSaveStrictIntentCASAndSnapshotFailuresNeverWrite(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, false, false)
	before, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`, projectUnitJSON(t, scope, 1, 1), projectUnitJSON(t, scope, 1, 3), projectUnitJSON(t, scope, 3, 2),
		`{"revision":"01","approvalHash":"` + scope.Build.ApprovalHash + `","expected":{"unitGroups":1},"proposed":{"unitGroups":2}}`,
		`{"revision":"` + scope.Revision + `","approvalHash":"` + scope.Build.ApprovalHash + `","expected":{"unitGroups":1},"proposed":{"unitGroups":2},"proposed":{"unitGroups":3}}`,
	} {
		result := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, raw)
		if result.Outcome.Changed || result.Outcome.Committed || result.Outcome.ErrorMessage == "" {
			t.Fatal("invalid intent/request/CAS wrote project preference", raw, result)
		}
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("project unit initial cleanup failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("project unit rollback cleanup failure") }},
	} {
		service.snapshot = hooks
		result := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, 2))
		if result.Outcome.Changed || result.Outcome.Committed || result.Outcome.ErrorMessage == "" {
			t.Fatal("failed observation cleanup authorized project unit CAS", result)
		}
	}
	reads := 0
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error {
		reads++
		if reads == 2 {
			return errors.New("project unit final cleanup failure")
		}
		return nil
	}}
	result := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, 2))
	if result.Outcome.Changed || result.Outcome.Committed || result.Outcome.ErrorMessage == "" {
		t.Fatal("failed final cleanup authorized project unit CAS", result)
	}
	after, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refused project Save changed YAML", err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if _, err := service.GetLongVegetationReportScope(context.Background(), state.ContextID); err != nil {
		t.Fatal("failed cleanup leaked project lease/attachments", err)
	}
}

func TestProjectUnitSaveFinalConfigGuardAndScopeInstallOwnership(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, true, false)
	ctx := context.Background()
	result := service.saveProjectUnits(ctx, state.ContextID, projectUnitJSON(t, scope, 1, 2), func() {
		if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVReportTitle": "drift"}); err != nil {
			t.Fatal(err)
		}
	})
	if result.Outcome.Changed || result.Outcome.Committed || result.Outcome.ErrorMessage == "" {
		t.Fatal("final complete-config guard accepted unrelated preference drift", result)
	}
	scope, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, scope.Revision)
	if err != nil {
		t.Fatal(err)
	}
	result = service.saveProjectUnits(ctx, state.ContextID, projectUnitJSON(t, scope, 1, 2), func() {
		short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()
		if _, err := service.SetLongVegetationReportScope(short, state.ContextID,
			projectScopeRequestJSON(t, scope.Revision, vegetationSelectedListScope)); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("project scope install interleaved with owned final CAS: %v", err)
		}
	})
	if !result.Outcome.Changed || !result.Outcome.Committed || result.Outcome.ErrorMessage != "" {
		t.Fatal("failed competing scope attempt erased legitimate project commit", result)
	}
}

func TestProjectUnitSavePersistenceCollisionRetryAndCommittedCancellation(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, true, false)
	store := service.contexts.projects.preferences
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("project unit collision") }
	failed := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, 2))
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if failed.Outcome.Committed || failed.Outcome.Changed || failed.Outcome.ErrorMessage == "" ||
		err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed effective YAML or leaked staging", failed, err, stageErr, stages)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	committed := service.SaveLongVegetationProjectUnits(ctx, state.ContextID, projectUnitJSON(t, scope, 1, 2))
	store.replace = replace
	if !committed.Outcome.Committed || !committed.Outcome.Changed || committed.SourceKind != scope.SourceKind {
		t.Fatal("postcommit cancellation discarded irreversible scoped receipt", committed)
	}
	value, err := store.readLongVegetationUnitPreferences(context.Background())
	if err != nil || !reflect.DeepEqual(value, LongVegetationUnitPreferenceValues{2}) {
		t.Fatal("committed scoped unit choice missing after cancellation", value, err)
	}
}
