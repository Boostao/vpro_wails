package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestProjectScopeBuildFinalRevalidationRejectsGapDriftWithoutReplacingReceipt(t *testing.T) {
	for _, change := range []string{"project", "su", "config", "final-read-cleanup"} {
		private, state := vegetationWorkbookServiceFixture(t, true)
		contexts := private.contexts
		scope := NewLongVegetationReportScopeService(contexts, true)
		ctx := context.Background()
		active, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
			projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope))
		if err != nil {
			t.Fatal(err)
		}
		active, err = scope.SetLongVegetationProjectFields(ctx, state.ContextID,
			selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"}))
		if err != nil {
			t.Fatal(err)
		}
		built, err := scope.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision)
		if err != nil {
			t.Fatal(err)
		}
		before := contexts.projectReportScopes.read(state.ContextID)
		switch change {
		case "project", "su":
			role, table, column := "project", "Sample_Admin", "OfficeNotes"
			if change == "su" {
				role, table, column = "su", "Report_SU", "SiteUnit"
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments[role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			scope.beforeInstall = func() {
				if _, err := db.Exec(`UPDATE ` + quoteHeaderIdentifier(table) + ` SET ` + quoteHeaderIdentifier(column) + `='gap drift' WHERE rowid=1`); err != nil {
					t.Fatal(err)
				}
			}
		case "config":
			scope.beforeInstall = func() {
				if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVReportTitle": "gap drift"}); err != nil {
					t.Fatal(err)
				}
			}
		case "final-read-cleanup":
			reads := 0
			scope.snapshot.commitRead = func(*sql.Tx) error {
				reads++
				if reads == 2 {
					return errors.New("final reviewed source cleanup failure")
				}
				return nil
			}
		}
		if result, err := scope.BuildLongVegetationProjectScope(ctx, state.ContextID, built.Revision); err == nil || result != nil || !reflect.DeepEqual(before, contexts.projectReportScopes.read(state.ContextID)) {
			t.Fatal("gap drift/final-read failure installed stale current Build or replaced prior receipt", change, result, err)
		}
	}
}

func TestProjectScopeSelectedGetterRechecksIntentInsideOwnedSnapshot(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, true)
	scope := NewLongVegetationReportScopeService(private.contexts, true)
	selected := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
	ctx := context.Background()
	selected.beforeRead = func() {
		if _, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
			projectScopeRequestJSON(t, "0", vegetationProjectFieldsScope)); err != nil {
			t.Fatal(err)
		}
	}
	if result, err := selected.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID); err == nil || result != nil {
		t.Fatal("selected getter returned obsolete intent after owned project switch", result, err)
	}
}
