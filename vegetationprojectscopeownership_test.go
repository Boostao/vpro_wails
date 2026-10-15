package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestProjectScopeSnapshotFailuresNeverInstallDraftsOrBuilds(t *testing.T) {
	for _, operation := range []string{"scope", "fields", "build"} {
		contexts, state, _ := projectOwnedSourceFixture(t, false, true)
		service := NewLongVegetationReportScopeService(contexts, true)
		ctx := context.Background()
		if operation != "scope" {
			active, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
				projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope))
			if err != nil {
				t.Fatal(err)
			}
			if operation == "build" {
				if _, err := service.SetLongVegetationProjectFields(ctx, state.ContextID,
					selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"})); err != nil {
					t.Fatal(err)
				}
			}
		}
		before := contexts.projectReportScopes.read(state.ContextID)
		selectedBefore := contexts.selectedUnitSelections.read(state.ContextID)
		for _, hooks := range []publicationReadSnapshotHooks{
			{commitRead: func(*sql.Tx) error { return errors.New("scope read commit failure") }},
			{rollbackRead: func(*sql.Tx) error { return errors.New("scope cleanup failure") }},
		} {
			service.snapshot = hooks
			var err error
			switch operation {
			case "scope":
				_, err = service.SetLongVegetationReportScope(ctx, state.ContextID,
					projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope))
			case "fields":
				_, err = service.SetLongVegetationProjectFields(ctx, state.ContextID,
					selectedUnitDraftJSON(t, "1", []string{"PlotNumber"}))
			case "build":
				_, err = service.BuildLongVegetationProjectScope(ctx, state.ContextID, "2")
			}
			if err == nil || !reflect.DeepEqual(before, contexts.projectReportScopes.read(state.ContextID)) ||
				!reflect.DeepEqual(selectedBefore, contexts.selectedUnitSelections.read(state.ContextID)) {
				t.Fatal("fallible read cleanup authorized scope/draft/Build mutation", operation, err)
			}
		}
		service.snapshot = publicationReadSnapshotHooks{}
		if _, err := service.GetLongVegetationReportScope(ctx, state.ContextID); err != nil {
			t.Fatal("failed snapshot cleanup lost owned attachments/lease", err)
		}
	}
}

func TestProjectScopeSwitchCannotInterleaveWithLegacyUnitCAS(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, true)
	contexts := private.contexts
	contexts.longVegetationNoneUnitsEnabled = true
	scope := NewLongVegetationReportScopeService(contexts, true)
	preferences := NewLongVegetationUnitPreferencesService(contexts, true)
	store := contexts.projects.preferences
	replace := store.replace
	store.replace = func(from, to string) error {
		short, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := scope.SetLongVegetationReportScope(short, state.ContextID,
			projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("scope switch passed between current source authority and config CAS: %v", err)
		}
		return replace(from, to)
	}
	outcome := preferences.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID,
		vegetationUnitRequest(t, 1, 3))
	store.replace = replace
	if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" ||
		contexts.projectReportScopes.read(state.ContextID).sourceKind != vegetationSelectedListScope {
		t.Fatal("owned legacy CAS was split by scope switch or lost committed receipt", outcome)
	}
}

func TestProjectScopeContextRetirementClearsAuthorityOnlyAfterSuccessfulSwitch(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	scope := NewLongVegetationReportScopeService(private.contexts, true)
	ctx := context.Background()
	if _, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope)); err != nil {
		t.Fatal(err)
	}
	before := private.contexts.projectReportScopes.read(state.ContextID)
	if _, err := private.contexts.SwitchContext("stale", contextSelection(state)); err == nil ||
		!reflect.DeepEqual(before, private.contexts.projectReportScopes.read(state.ContextID)) {
		t.Fatal("failed owner switch erased current scope authority", err)
	}
	next, err := private.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scope.GetLongVegetationReportScope(ctx, state.ContextID); err == nil {
		t.Fatal("retired context remained selectable")
	}
	current, err := scope.GetLongVegetationReportScope(ctx, next.ContextID)
	if err != nil || current.SourceKind != vegetationSelectedListScope || current.Revision != "0" ||
		current.Build != nil || len(private.contexts.projectReportScopes.entries) != 0 {
		t.Fatal("new owner inherited project intent/draft/approval", current, err)
	}
}
