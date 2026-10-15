package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func enforcementFixtureObserver(context.Context) (PrinterObservation, error) {
	return PrinterObservation{Names: []string{}, Source: printerObservationSource}, nil
}

func TestLongVegetationEnforcementIndependentQualification(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	owner := contexts.projects.sqlite
	project, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["project"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = project.Exec(`INSERT INTO Sample_Env(PlotNumber) SELECT '108050x'
		WHERE NOT EXISTS(SELECT 1 FROM Sample_Env WHERE PlotNumber='108050x');
		DELETE FROM Sample_Admin WHERE Plot IN ('108050','108050x');
		INSERT INTO Sample_Admin(Plot,SitePlotQuality,VegPlotQuality,SoilPlotQuality)
		VALUES('108050','Fair','Fair','Fair'),('108050x','Good','Good','Good')`)
	closeErr := project.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = lists.Exec(`DELETE FROM USysTableOfLists WHERE ListName IN ('DataQuality','PlotQualitySite');
		INSERT INTO USysTableOfLists(ListName,Item,ItemOrder) VALUES
		('DataQuality','Poor',1),('DataQuality','Fair',2),('DataQuality','Good',3),
		('PlotQualitySite','Fair',99),('PlotQualitySite','Good',-99)`)
	closeErr = lists.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{
		"DataQualityFilterEnforceLV": 0, "LVUnitGroups": 1, "DataQualityFilterSiteLV": "Good",
		"DataQualityFilterVegLV": "Poor", "DataQualityFilterSoilLV": "Poor",
	}); err != nil {
		t.Fatal(err)
	}
	beforeDB := databaseBytes(t, owner.attachments)
	preferences := NewLongVegetationEnforcementPreferencesService(contexts, true)
	preferences.printers = enforcementFixtureObserver
	for _, active := range []bool{true, false} {
		before, err := preferences.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		next := LongVegetationEnforcementPreferenceValues{active, 1}
		result := preferences.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID,
			enforcementRequest(t, before.Values, next))
		if !result.Committed || !result.Changed || result.ErrorMessage != "" {
			t.Fatal(result)
		}
		preview, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID)
		if err != nil || (preview.Report.Quality != nil) != active || (preview.Settings.Quality != nil) != active {
			t.Fatal("enforcement failed to propagate to source qualification", active, preview, err)
		}
		if active && (!reflect.DeepEqual(preview.Report.Quality.ExcludedMembershipIDs, []string{"1", "2"}) ||
			len(preview.Report.Quality.Occurrences) != 1 || preview.Report.Quality.Occurrences[0].MembershipID != "3") {
			t.Fatal("DataQuality Good must retain only Good membership, independent of contrary suggestions", preview)
		}
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, owner.attachments)) {
		t.Fatal("enforcement preference save/report changed source data or audits")
	}
}

func TestLongVegetationEnforcementWorkbookStaleApprovalAndReset(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationEnforcementPreferencesService(private.contexts, true)
			preferences.printers = enforcementFixtureObserver
			store := private.contexts.projects.preferences
			if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": false, "LVUnitGroups": 1}); err != nil {
				t.Fatal(err)
			}
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for _, active := range []bool{true, false} {
				old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil {
					t.Fatal(err)
				}
				current, err := preferences.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				next := LongVegetationEnforcementPreferenceValues{active, 1}
				result := preferences.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID,
					enforcementRequest(t, current.Values, next))
				if !result.Committed || result.ErrorMessage != "" {
					t.Fatal(result)
				}
				destination := filepath.Join(t.TempDir(), "enforcement.xlsx")
				stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
				if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
					t.Fatal("enforcement drift published stale approval", stale)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale enforcement approval emitted a destination", err)
				}
				fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil || fresh.ApprovalHash == old.ApprovalHash || (fresh.Preview.Settings.Quality != nil) != active {
					t.Fatal("fresh approval did not observe enforcement", fresh, err)
				}
				published, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", fresh.CreatedDate, fresh.ApprovalHash, destination}))
				if err != nil || published == nil || published.Status != "published" || published.SHA256 != fresh.WorkbookSHA256 {
					t.Fatal("fresh enforced/unenforced workbook unavailable", published, err)
				}
			}
			if err := store.update("ReportOptions", map[string]any{"LVUnitGroups": 3}); err != nil {
				t.Fatal(err)
			}
			if review, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`); review != nil || err == nil {
				t.Fatal("unimplemented source unit-group3 became available", review, err)
			}
			result := preferences.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID,
				enforcementRequest(t, LongVegetationEnforcementPreferenceValues{false, 3}, LongVegetationEnforcementPreferenceValues{false, 1}))
			if !result.Committed || result.ErrorMessage != "" {
				t.Fatal("changed-away/back pair failed unit-group reset", result)
			}
			if review, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`); review == nil || err != nil {
				t.Fatal("source callback unit-group1 recovery unavailable", review, err)
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, private.contexts.projects.sqlite.attachments)) {
				t.Fatal("enforcement/workbook operations changed fixture data or audits")
			}
		})
	}
}
