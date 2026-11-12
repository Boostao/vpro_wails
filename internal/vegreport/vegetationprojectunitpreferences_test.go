package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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

func projectUnitCASFixture(t *testing.T, external, all bool) (*LongVegetationReportScopeService, vprocore.ProjectState, *LongVegetationReportScope) {
	t.Helper()
	service, state, scope := projectPreviewFixture(t, external, all, all)
	if err := service.contexts.Projects.Preferences.Update("ReportOptions", map[string]any{"LVUnitGroups": 1}); err != nil {
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
			before := vprotest.DatabaseBytes(t, service.contexts.Projects.Sqlite.Attachments)
			saved := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, desired))
			if !saved.Outcome.Changed || !saved.Outcome.Committed || saved.Outcome.ErrorMessage != "" ||
				saved.SourceKind != scope.SourceKind || saved.Revision != scope.Revision {
				t.Fatal("project Save lost scope-owned mutation receipt", saved)
			}
			if _, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, projectReadJSON(t, scope)); err != nil {
				t.Fatal("committed normalized-mode Save invalidated original Build authority", err)
			}
			config, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			noop := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, desired, desired))
			after, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
			if noop.Outcome.Changed || noop.Outcome.Committed || noop.Outcome.ErrorMessage != "" ||
				err != nil || !bytes.Equal(config, after) {
				t.Fatal("unchanged historical project mode rewrote YAML or fabricated commit", noop, err)
			}
			vprotest.AssertProfileSUFiles(t, service.contexts, before)
		}
	}
}

func TestProjectUnitSaveStrictIntentCASAndSnapshotFailuresNeverWrite(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, false, false)
	before, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
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
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("project unit initial cleanup failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("project unit rollback cleanup failure") }},
	} {
		service.snapshot = hooks
		result := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, 2))
		if result.Outcome.Changed || result.Outcome.Committed || result.Outcome.ErrorMessage == "" {
			t.Fatal("failed observation cleanup authorized project unit CAS", result)
		}
	}
	reads := 0
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error {
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
	after, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refused project Save changed YAML", err)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	if _, err := service.GetLongVegetationReportScope(context.Background(), state.ContextID); err != nil {
		t.Fatal("failed cleanup leaked project lease/attachments", err)
	}
}

func TestProjectUnitSaveFinalConfigGuardAndScopeInstallOwnership(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, true, false)
	ctx := context.Background()
	result := service.saveProjectUnits(ctx, state.ContextID, projectUnitJSON(t, scope, 1, 2), func() {
		if err := service.contexts.Projects.Preferences.Update("ReportOptions", map[string]any{"LVReportTitle": "drift"}); err != nil {
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
			projectScopeRequestJSON(t, scope.Revision, vprocore.VegetationSelectedListScope)); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("project scope install interleaved with owned final CAS: %v", err)
		}
	})
	if !result.Outcome.Changed || !result.Outcome.Committed || result.Outcome.ErrorMessage != "" {
		t.Fatal("failed competing scope attempt erased legitimate project commit", result)
	}
}

func TestProjectUnitSavePersistenceCollisionRetryAndCommittedCancellation(t *testing.T) {
	service, state, scope := projectUnitCASFixture(t, true, false)
	store := service.contexts.Projects.Preferences
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("project unit collision") }
	failed := service.SaveLongVegetationProjectUnits(context.Background(), state.ContextID, projectUnitJSON(t, scope, 1, 2))
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if failed.Outcome.Committed || failed.Outcome.Changed || failed.Outcome.ErrorMessage == "" ||
		err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed effective YAML or leaked staging", failed, err, stageErr, stages)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.Replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	committed := service.SaveLongVegetationProjectUnits(ctx, state.ContextID, projectUnitJSON(t, scope, 1, 2))
	store.Replace = replace
	if !committed.Outcome.Committed || !committed.Outcome.Changed || committed.SourceKind != scope.SourceKind {
		t.Fatal("postcommit cancellation discarded irreversible scoped receipt", committed)
	}
	value, err := ReadLongVegetationUnitPreferences(store, context.Background())
	if err != nil || !reflect.DeepEqual(value, LongVegetationUnitPreferenceValues{2}) {
		t.Fatal("committed scoped unit choice missing after cancellation", value, err)
	}
}
