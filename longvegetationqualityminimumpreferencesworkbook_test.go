package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationQualityMinimumPreferencesIndependentQualification(t *testing.T) {
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
	beforeDB := databaseBytes(t, owner.attachments)
	preferences := NewLongVegetationQualityMinimumPreferencesService(contexts, true)
	for field := 0; field < 3; field++ {
		if err := contexts.projects.preferences.update("ReportOptions", map[string]any{
			"DataQualityFilterEnforceLV": -1, "DataQualityFilterSiteLV": "Poor",
			"DataQualityFilterVegLV": "Poor", "DataQualityFilterSoilLV": "Poor",
		}); err != nil {
			t.Fatal(err)
		}
		before, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID)
		if err != nil || before.Report.Quality == nil ||
			!reflect.DeepEqual(before.Report.Quality.ExcludedMembershipIDs, []string{"2"}) {
			t.Fatal("independent Poor threshold must retain Fair and Good memberships, not the orphan", field, before, err)
		}
		current, err := preferences.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		next := current.Values
		switch field {
		case 0:
			next.SiteMinimum = "Good"
		case 1:
			next.VegMinimum = "Good"
		case 2:
			next.SoilMinimum = "Good"
		}
		result := preferences.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
			vegetationQualityMinimumRequest(t, current.Values, next))
		if !result.Committed || result.ErrorMessage != "" {
			t.Fatal(result)
		}
		after, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID)
		if err != nil || after.Report.Quality == nil ||
			!reflect.DeepEqual(after.Report.Quality.ExcludedMembershipIDs, []string{"1", "2"}) ||
			len(after.Report.Quality.Occurrences) != 1 ||
			after.Report.Quality.Occurrences[0].MembershipID != "3" ||
			after.Report.Quality.Occurrences[0].PlotNumber.Text == nil ||
			*after.Report.Quality.Occurrences[0].PlotNumber.Text != "108050x" {
			t.Fatal("independent Good threshold must retain only Good, using DataQuality not contradictory suggestions", field, after, err)
		}
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, owner.attachments)) {
		t.Fatal("minimum Save or qualification changed fixture data/audits")
	}
}

func TestLongVegetationQualityMinimumPreferencesWorkbookActiveInactiveAndStaleApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationQualityMinimumPreferencesService(private.contexts, true)
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for _, active := range []bool{false, true} {
				if err := private.contexts.projects.preferences.update("ReportOptions", map[string]any{
					"DataQualityFilterEnforceLV": active, "DataQualityFilterSiteLV": "Poor",
					"DataQualityFilterVegLV": "Poor", "DataQualityFilterSoilLV": "Poor",
				}); err != nil {
					t.Fatal(err)
				}
				for field := 0; field < 3; field++ {
					old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
					if err != nil {
						t.Fatal(active, field, err)
					}
					current, err := preferences.GetLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID)
					if err != nil {
						t.Fatal(err)
					}
					next := current.Values
					switch field {
					case 0:
						next.SiteMinimum = "Good"
					case 1:
						next.VegMinimum = "Good"
					case 2:
						next.SoilMinimum = "Good"
					}
					result := preferences.SaveLongVegetationQualityMinimumPreferences(context.Background(), state.ContextID,
						vegetationQualityMinimumRequest(t, current.Values, next))
					if !result.Committed || !result.Changed || result.ErrorMessage != "" {
						t.Fatal(result)
					}
					destination := filepath.Join(t.TempDir(), "quality-minima.xlsx")
					stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
						workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
					if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
						t.Fatal("changed minima published stale approval", active, field, stale, err)
					}
					if _, err := os.Stat(destination); !os.IsNotExist(err) {
						t.Fatal("stale approval wrote output", active, field, err)
					}
					fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
					if err != nil || fresh.ApprovalHash == old.ApprovalHash {
						t.Fatal("fresh approval did not observe owned minimum strings", active, field, err)
					}
					settings := fresh.Preview.Settings
					if active {
						if settings.Quality == nil || settings.Quality.Site.Minimum != next.SiteMinimum ||
							settings.Quality.Veg.Minimum != next.VegMinimum || settings.Quality.Soil.Minimum != next.SoilMinimum {
							t.Fatal("active minimum Save did not propagate", field, settings)
						}
						settings.Quality = old.Preview.Settings.Quality
						if !reflect.DeepEqual(settings, old.Preview.Settings) {
							t.Fatal("minimum Save changed unrelated report settings")
						}
					} else if fresh.Preview.Settings.Quality != nil || !reflect.DeepEqual(fresh.Preview, old.Preview) ||
						fresh.WorkbookSHA256 != old.WorkbookSHA256 {
						t.Fatal("inactive minimum Save enabled enforcement or changed output/arithmetic", field)
					}
					if field == 2 {
						published, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
							workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", fresh.CreatedDate, fresh.ApprovalHash, destination}))
						if err != nil || published == nil || published.Status != "published" || published.SHA256 != fresh.WorkbookSHA256 {
							t.Fatal(active, published, err)
						}
					}
				}
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, private.contexts.projects.sqlite.attachments)) {
				t.Fatal("minimum Save or workbook publication changed data/audits")
			}
		})
	}
}
