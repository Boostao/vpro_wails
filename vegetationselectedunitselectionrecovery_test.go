package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSelectedUnitSessionInvalidProjectionDriftRetainsBuildReceiptAndCorrection(t *testing.T) {
	for _, change := range []string{"empty-label", "overlength-label", "empty-memberships"} {
		t.Run(change, func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, false)
			service := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
			db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`ALTER TABLE Sample_Env ADD COLUMN Label TEXT; UPDATE Sample_Env SET Label='Alpha'`); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			draft, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"Label"}))
			if err != nil {
				t.Fatal(err)
			}
			built, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "empty-label", "overlength-label":
				label := ""
				if change == "overlength-label" {
					label = strings.Repeat("x", 256)
				}
				if _, err := db.Exec(`UPDATE Sample_Env SET Label=?`, label); err != nil {
					t.Fatal(err)
				}
			case "empty-memberships":
				su, err := sql.Open("sqlite3", sqliteFileURI(state.SUPath, "rw"))
				if err != nil {
					t.Fatal(err)
				}
				_, deleteErr := su.Exec(`DELETE FROM ` + quoteHeaderIdentifier(state.ActiveSU+"_SU"))
				closeErr := su.Close()
				if err := errors.Join(deleteErr, closeErr); err != nil {
					t.Fatal(err)
				}
			}
			current, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
			if err != nil || current.BuildCurrent || current.Revision != built.Revision ||
				!reflect.DeepEqual(current.Build, built.Build) || !reflect.DeepEqual(current.Fields, []string{"Label"}) {
				t.Fatal("invalid current projection erased reviewed receipt/revision/draft", current, err)
			}
			if _, err := selectedUnitAuthority(t, service, state.ContextID); err == nil {
				t.Fatal("invalid projection accepted for output")
			}
			if _, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, current.Revision); err == nil {
				t.Fatal("invalid destination source Build succeeded")
			}
			corrected, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID,
				selectedUnitDraftJSON(t, current.Revision, []string{"PlotNumber"}))
			if err != nil || corrected.Build != nil {
				t.Fatal("source drift blocked correction/approval invalidation", corrected, err)
			}
		})
	}
}

func TestSelectedUnitSessionRetiredContextBuildReceiptsAreReleased(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	contexts := private.contexts
	service := NewLongVegetationSelectedUnitSelectionService(contexts, true)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		draft, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
		if err != nil {
			t.Fatal(err)
		}
		built, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
		if err != nil || !built.BuildCurrent {
			t.Fatal("current context Build", built, err)
		}
		if _, err := contexts.SwitchContext("stale", ContextSelection{}); err == nil {
			t.Fatal("stale switch accepted")
		}
		if len(contexts.selectedUnitSelections.entries) != 1 {
			t.Fatal("failed switch erased current receipt")
		}
		selection := contexts.projects.sqlite.selection
		next, err := contexts.SwitchContext(state.ContextID, ContextSelection{
			Project: selection.Project, ProjectPath: selection.ProjectPath, SU: selection.SU, SUPath: selection.SUPath,
			Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(contexts.selectedUnitSelections.entries) != 0 {
			t.Fatal("retired context retained unreachable large Build receipts")
		}
		if _, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, built.Revision); err == nil {
			t.Fatal("retired context permitted late Build installation")
		}
		state = next
		current, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
		if err != nil || current.Revision != "0" || len(current.Fields) != 0 || current.Build != nil {
			t.Fatal("replacement context inherited retired approval/draft", current, err)
		}
	}
}
