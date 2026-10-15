package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestSelectedUnitAuthorityBuildCASSaveOptionsPreviewAndWorkbook(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			workbook, state := vegetationWorkbookServiceFixture(t, external)
			contexts := workbook.contexts
			ctx := context.Background()
			su, err := sql.Open("sqlite3", sqliteFileURI(state.SUPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, removeErr := su.Exec(`DELETE FROM ` + quoteHeaderIdentifier(state.ActiveSU+"_SU") + ` WHERE PlotNumber='missing'`)
			if err := errors.Join(removeErr, su.Close()); err != nil {
				t.Fatal(err)
			}
			contexts.longVegetationSelectedUnitsEnabled = true
			selection := NewLongVegetationSelectedUnitSelectionService(contexts, true)
			preferences := NewLongVegetationUnitPreferencesService(contexts, true)
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			initial, err := preferences.GetLongVegetationUnitPreferences(ctx, state.ContextID)
			if err != nil || initial.Availability.SelectedFields || initial.Values.UnitGroups != 1 {
				t.Fatal("unreviewed session enabled selected units", initial, err)
			}
			save := `{"expected":{"unitGroups":1},"proposed":{"unitGroups":2}}`
			if refused := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, save); refused.Committed ||
				refused.Changed || refused.ErrorMessage == "" {
				t.Fatal("new2 bypassed explicit Build", refused)
			}
			draft, err := selection.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
			if err != nil {
				t.Fatal(err)
			}
			built, err := selection.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
			if err != nil || !built.BuildCurrent {
				t.Fatal("explicit Build", built, err)
			}
			current, err := preferences.GetLongVegetationUnitPreferences(ctx, state.ContextID)
			if err != nil || !current.Availability.SelectedFields || current.Values.UnitGroups != 1 {
				t.Fatal("Build changed saved unit mode or failed to advertise reviewed choice", current, err)
			}
			afterBuild, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil || string(config) != string(afterBuild) {
				t.Fatal("Build persisted LVUnitGroups", err)
			}
			contexts.longVegetationSelectedUnitsEnabled = false
			if disabled := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, save); disabled.Committed ||
				disabled.ErrorMessage == "" {
				t.Fatal("consumer gate bypassed by selection Build", disabled)
			}
			contexts.longVegetationSelectedUnitsEnabled = true
			saved := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, save)
			if !saved.Changed || !saved.Committed || saved.ErrorMessage != "" {
				t.Fatal("reviewed selected unit CAS failed", saved)
			}
			values, err := contexts.projects.preferences.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeLongVegetationOptions(values); err == nil {
				t.Fatal("legacy decoder inferred saved2 field selection")
			}
			retained, err := selection.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
			if err != nil || !retained.BuildCurrent || retained.Build.ApprovalHash != built.Build.ApprovalHash {
				t.Fatal("explicit planned mode2 CAS invalidated its own Build", retained, err)
			}
			options, err := contexts.GetLongVegetationOptions(ctx, state.ContextID)
			if err != nil || options.Settings.UnitGrouping != "selected-fields" ||
				!reflect.DeepEqual(options.Settings.UnitFields, []string{"PlotNumber"}) {
				t.Fatal("owned options lost explicit session fields", options, err)
			}
			preview, err := contexts.PreviewLongVegetation(ctx, state.ContextID)
			if err != nil || len(preview.Report.SelectedUnits) != 2 || len(preview.Report.Units) != 2 {
				t.Fatal("owned Preview failed source2 routing", preview, err)
			}
			core, err := contexts.previewLongVegetationLayers(ctx, state.ContextID)
			if err != nil || len(core.SelectedUnits) != 2 {
				t.Fatal("private preview route differs from transport", core, err)
			}
			review, err := workbook.GetReview(ctx, state.ContextID, `{"scope":"unlumped"}`)
			if err != nil || review.Layout.Summary.SelectedPlotRows != 2 || len(review.Preview.Report.SelectedUnits) != 2 ||
				review.Preview.Settings.UnitGrouping != "selected-fields" {
				t.Fatal("owned workbook did not use approved source2 summary", review, err)
			}
			contexts.longVegetationSelectedUnitsEnabled = false
			if _, err := contexts.GetLongVegetationOptions(ctx, state.ContextID); err == nil {
				t.Fatal("saved2 options ignored backend gate")
			}
			if _, err := contexts.PreviewLongVegetation(ctx, state.ContextID); err == nil {
				t.Fatal("saved2 report ignored backend gate")
			}
			if _, err := workbook.GetReview(ctx, state.ContextID, `{"scope":"unlumped"}`); err == nil {
				t.Fatal("saved2 workbook ignored backend gate")
			}
			contexts.longVegetationSelectedUnitsEnabled = true
			changed, err := selection.SetLongVegetationSelectedUnitFields(ctx, state.ContextID,
				selectedUnitDraftJSON(t, retained.Revision, []string{"Plot"}))
			if err != nil || changed.Build != nil {
				t.Fatal("new field draft did not invalidate approved output", changed, err)
			}
			if _, err := contexts.PreviewLongVegetation(ctx, state.ContextID); err == nil {
				t.Fatal("saved2 report reused invalidated field approval")
			}
			if _, err := workbook.GetReview(ctx, state.ContextID, `{"scope":"unlumped"}`); err == nil {
				t.Fatal("saved2 workbook reused invalidated field approval")
			}
			nop := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, `{"expected":{"unitGroups":2},"proposed":{"unitGroups":2}}`)
			if nop.Changed || nop.Committed || nop.ErrorMessage != "" {
				t.Fatal("unchanged historical2 no-op requires unavailable authority", nop)
			}
			recovery := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, `{"expected":{"unitGroups":2},"proposed":{"unitGroups":1}}`)
			if !recovery.Committed || recovery.ErrorMessage != "" {
				t.Fatal("explicit original1 recovery failed", recovery)
			}
			ordinary, err := contexts.PreviewLongVegetation(ctx, state.ContextID)
			if err != nil || ordinary.Settings.UnitGrouping != "" || ordinary.Report.SelectedUnits != nil {
				t.Fatal("source1 recovery inherited selected fields", ordinary, err)
			}
			assertProfileSUFiles(t, contexts, before)
		})
	}
}

func TestSelectedUnitAuthorityModeOverlayPreservesOriginalAndOtherConfig(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	before, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	for _, choice := range []int{1, 2, 3} {
		planned, err := selectedUnitConfigForChoice(values, choice)
		if err != nil || planned["ReportOptions"].(map[string]any)["LVUnitGroups"] != choice {
			t.Fatal("explicit mode overlay", choice, err)
		}
		planned["ReportOptions"].(map[string]any)["LVReportTitle"] = "mutated"
		after, err := json.Marshal(values)
		if err != nil || string(before) != string(after) {
			t.Fatal("approval material mutated effective settings", err)
		}
	}
	if _, err := selectedUnitConfigForChoice(values, 4); err == nil {
		t.Fatal("invalid planned unit mode accepted")
	}
}
