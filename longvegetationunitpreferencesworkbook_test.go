package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLongVegetationUnitPreferencesWorkbookStaleRefusalNoneAndSourceRecovery(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			contexts := private.contexts
			preferences := NewLongVegetationUnitPreferencesService(contexts, true)
			beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
			old, err := private.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil {
				t.Fatal(err)
			}
			disabled := preferences.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3))
			if disabled.Committed || disabled.Changed || disabled.ErrorMessage == "" {
				t.Fatal("preference gate borrowed None species grouping/consumer availability", disabled)
			}
			contexts.longVegetationNoneUnitsEnabled = true
			result := preferences.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3))
			if !result.Committed || !result.Changed || result.ErrorMessage != "" {
				t.Fatal("owned source3 preference save failed", result)
			}
			destination := filepath.Join(t.TempDir(), "unit-groups.xlsx")
			stale, err := private.ExportReviewed(context.Background(), state.ContextID,
				workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
			if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
				t.Fatal("unit preference drift published obsolete physical-unit workbook", stale)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("stale unit preference emitted output", err)
			}
			fresh, err := private.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil || fresh.ApprovalHash == old.ApprovalHash || fresh.Preview.Settings.UnitGrouping != "all-plots" ||
				len(fresh.Preview.Report.NoneUnits) != 2 || len(fresh.Preview.Report.Units) != 1 ||
				fresh.Preview.Report.Units[0].NumPlots != 2 ||
				!reflect.DeepEqual(fresh.Preview.Report.Units[0].MembershipIDs, []string{"1", "3"}) {
				t.Fatal("source3 must retain108050 and108050x, excluding orphan missing", err)
			}
			for i, occurrence := range fresh.Preview.Report.NoneUnits {
				if occurrence.PlotNumber.Text == nil || *occurrence.PlotNumber.Text != []string{"108050", "108050x"}[i] ||
					occurrence.MembershipID != []string{"1", "3"}[i] {
					t.Fatal("independently enumerated original selected/Env plot scope differs", occurrence)
				}
			}
			outcome, err := private.ExportReviewed(context.Background(), state.ContextID,
				workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", fresh.CreatedDate, fresh.ApprovalHash, destination}))
			if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != fresh.WorkbookSHA256 {
				t.Fatal("reviewed None unit workbook publication failed", outcome, err)
			}
			book, err := excelize.OpenFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			for address, expected := range map[string]string{
				"B5": "USysSuTableDynamic_SU", "B12": "2", "D5": "108050", "E5": "All plots", "D6": "108050x", "E6": "All plots",
			} {
				if actual, err := book.GetCellValue("ReportSummary", address); err != nil || actual != expected {
					t.Fatal("reopened saved unit-group summary differs", address, actual, err)
				}
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
			result = preferences.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 3, 1))
			if !result.Committed || result.ErrorMessage != "" {
				t.Fatal("source1 recovery failed", result)
			}
			recovered, err := private.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil || recovered.ApprovalHash != old.ApprovalHash || recovered.WorkbookSHA256 != old.WorkbookSHA256 ||
				recovered.Preview.Settings.UnitGrouping != "" || len(recovered.Preview.Report.NoneUnits) != 0 {
				t.Fatal("source1 recovery changed original source approval or invented derived scope", recovered, err)
			}
			assertProfileSUFiles(t, contexts, beforeDB)
		})
	}
}
