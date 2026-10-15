package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestSelectedUnitAuthorityHistorical2CannotInferSessionFields(t *testing.T) {
	workbook, state := vegetationWorkbookServiceFixture(t, false)
	contexts := workbook.contexts
	values, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": 2}); err != nil {
		t.Fatal(err)
	}
	contexts.longVegetationSelectedUnitsEnabled = true
	ctx := context.Background()
	if _, err := contexts.GetLongVegetationOptions(ctx, state.ContextID); err == nil {
		t.Fatal("historical2 inferred source fields without session Build")
	}
	if _, err := contexts.PreviewLongVegetation(ctx, state.ContextID); err == nil {
		t.Fatal("historical2 enabled report without session Build")
	}
	if _, err := workbook.GetReview(ctx, state.ContextID, `{"scope":"unlumped"}`); err == nil {
		t.Fatal("historical2 enabled workbook without session Build")
	}
	preferences := NewLongVegetationUnitPreferencesService(contexts, true)
	review, err := preferences.GetLongVegetationUnitPreferences(ctx, state.ContextID)
	if err != nil || review.Values.UnitGroups != 2 || review.Availability.SelectedFields {
		t.Fatal("historical2 was reset or advertised as available", review, err)
	}
	nop := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, `{"expected":{"unitGroups":2},"proposed":{"unitGroups":2}}`)
	if nop.Changed || nop.Committed || nop.ErrorMessage != "" {
		t.Fatal("historical2 exact no-op failed", nop)
	}
	original := values["ReportOptions"].(map[string]any)["LVUnitGroups"]
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": original}); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedUnitAuthorityNew2CommitSurvivesReadCleanupFailureWithoutReplay(t *testing.T) {
	workbook, state := vegetationWorkbookServiceFixture(t, false)
	contexts := workbook.contexts
	contexts.longVegetationSelectedUnitsEnabled = true
	selection := NewLongVegetationSelectedUnitSelectionService(contexts, true)
	ctx := context.Background()
	draft, err := selection.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selection.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision); err != nil {
		t.Fatal(err)
	}
	preferences := NewLongVegetationUnitPreferencesService(contexts, true)
	preferences.snapshot.commitRead = func(*sql.Tx) error { return errors.New("injected post-CAS read cleanup failure") }
	request := `{"expected":{"unitGroups":1},"proposed":{"unitGroups":2}}`
	committed := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request)
	if !committed.Changed || !committed.Committed || committed.ErrorMessage == "" {
		t.Fatal("completed unit2 persistence was disguised as an uncommitted read failure", committed)
	}
	preferences.snapshot = publicationReadSnapshotHooks{}
	observed, err := preferences.GetLongVegetationUnitPreferences(ctx, state.ContextID)
	if err != nil || observed.Values.UnitGroups != 2 || !observed.Availability.SelectedFields {
		t.Fatal("physical owned configuration lost committed mode2 or its planned Build", observed, err)
	}
	repeated := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request)
	if repeated.Changed || repeated.Committed || repeated.ErrorMessage == "" {
		t.Fatal("stale expected1 replayed committed selected-unit Save", repeated)
	}
	recovery := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, `{"expected":{"unitGroups":2},"proposed":{"unitGroups":1}}`)
	if !recovery.Committed || recovery.ErrorMessage != "" {
		t.Fatal("explicit original1 correction failed", recovery)
	}
}
