package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLongVegetationNoneUnitsQualifiedSummaryRetainsOriginalUnitsAndIndices(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	contexts, owner := private.contexts, private.contexts.projects.sqlite
	project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer project.Close()
	if _, err := project.Exec(`INSERT INTO Sample_Env(PlotNumber) SELECT '108050x'
		WHERE NOT EXISTS(SELECT 1 FROM Sample_Env WHERE PlotNumber='108050x');
		DELETE FROM Sample_Admin WHERE Plot IN ('108050','108050x');
		INSERT INTO Sample_Admin(Plot,SitePlotQuality,VegPlotQuality,SoilPlotQuality)
		VALUES('108050','Good','Good','Good'),('108050x','Fair','Fair','Fair')`); err != nil {
		t.Fatal(err)
	}
	lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer lists.Close()
	if _, err := lists.Exec(`DELETE FROM USysTableOfLists WHERE ListName='DataQuality';
		INSERT INTO USysTableOfLists(ListName,Item,ItemOrder) VALUES
		('DataQuality','Poor',1),('DataQuality','Fair',2),('DataQuality','Good',3),('DataQuality','Good',3)`); err != nil {
		t.Fatal(err)
	}
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{
		"LVUnitGroups": 3, "DataQualityFilterEnforceLV": -1,
		"DataQualityFilterSiteLV": "Poor", "DataQualityFilterVegLV": "Poor", "DataQualityFilterSoilLV": "Poor",
	}); err != nil {
		t.Fatal(err)
	}
	contexts.longVegetationNoneUnitsEnabled = true
	before := databaseBytes(t, owner.attachments)
	review, err := private.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil {
		t.Fatal(err)
	}
	report := review.Preview.Report
	if report.Quality == nil || len(report.Quality.Occurrences) != 9 || len(report.NoneUnits) != 9 ||
		len(report.Units) != 1 || report.Units[0].NumPlots != 9 ||
		!reflect.DeepEqual(report.Units[0].MembershipIDs, []string{"1", "3"}) ||
		review.Layout.Summary.SelectedPlotRows != 9 || len(review.Layout.Summary.Memberships) != 9 {
		t.Fatal("QCLV8+1 followed by unique outer Env must yield9, not81/deduplicated2", review)
	}
	for i, occurrence := range report.NoneUnits {
		if occurrence.QualityOccurrence == nil || *occurrence.QualityOccurrence != i ||
			*occurrence.SiteUnit.Text != "All plots" ||
			report.Quality.Occurrences[i].MembershipID != occurrence.MembershipID {
			t.Fatal("None current-list quality index or derived identity lost", occurrence)
		}
		originalUnit := report.Quality.Occurrences[i].SiteUnit
		if i < 8 && (originalUnit.Text == nil || *originalUnit.Text != "  Unit 'quoted'  ") ||
			i == 8 && originalUnit.Storage != "null" {
			t.Fatal("None derivation overwrote original quality SiteUnit witness", originalUnit)
		}
		member := review.Layout.Summary.Memberships[i]
		if member.NoneUnit == nil || *member.NoneUnit.QualityOccurrence != i || member.Quality != nil ||
			*member.SiteUnit.Text != "All plots" {
			t.Fatal("derived summary lost quality linkage/original physical witness", member)
		}
	}
	diagnostics := map[string]int{}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Identity == "1" {
			diagnostics[diagnostic.Code] = diagnostic.Count
		}
	}
	if diagnostics["quality_reference_multiplicity"] != 8 || diagnostics["none_unit_join_multiplicity"] != 8 {
		t.Fatal("original quality versus final derived multiplicity must remain explicit", diagnostics)
	}
	destination := filepath.Join(t.TempDir(), "none-qualified.xlsx")
	request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
	if result, err := private.ExportReviewed(context.Background(), state.ContextID, request); err != nil ||
		result == nil || result.Status != "published" || result.SHA256 != review.WorkbookSHA256 {
		t.Fatal("qualified None workbook failed reviewed publication", result, err)
	}
	book, err := excelize.OpenFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	for address, expected := range map[string]string{"B5": "USysSuTableDynamic_SU", "B12": "9", "D13": "108050x", "E13": "All plots"} {
		if value, err := book.GetCellValue("ReportSummary", address); err != nil || value != expected {
			t.Fatal("reopened qualified None summary differs", address, value, err)
		}
	}
	if err := book.Close(); err != nil {
		t.Fatal(err)
	}
	contexts.longVegetationNoneUnitsEnabled = false
	refused := filepath.Join(t.TempDir(), "disabled.xlsx")
	if result, err := private.ExportReviewed(context.Background(), state.ContextID,
		workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, refused})); err == nil &&
		(result == nil || result.Status != "not-published" || result.ErrorMessage == "") {
		t.Fatal("disabled None consumer published reviewed active approval", result)
	}
	if _, err := os.Stat(refused); !os.IsNotExist(err) {
		t.Fatal("disabled None consumer emitted output", err)
	}
	assertProfileSUFiles(t, contexts, before)
}
