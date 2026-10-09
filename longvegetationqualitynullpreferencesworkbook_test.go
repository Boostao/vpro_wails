package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationQualityNullPreferencesWorkbookActiveInactiveAndStaleApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationQualityNullPreferencesService(private.contexts, true)
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for _, active := range []bool{false, true} {
				if err := private.contexts.projects.preferences.update("ReportOptions", map[string]any{
					"DataQualityFilterEnforceLV": active, "DataQualityFilterSiteNullLV": true,
					"DataQualityFilterVegNullLV": true, "DataQualityFilterSoilNullLV": true,
				}); err != nil {
					t.Fatal(err)
				}
				for field := 0; field < 3; field++ {
					old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
					if err != nil {
						t.Fatal(active, field, err)
					}
					current, err := preferences.GetLongVegetationQualityNullPreferences(context.Background(), state.ContextID)
					if err != nil {
						t.Fatal(err)
					}
					next := current.Values
					switch field {
					case 0:
						next.SiteIncludeNulls = false
					case 1:
						next.VegIncludeNulls = false
					case 2:
						next.SoilIncludeNulls = false
					}
					result := preferences.SaveLongVegetationQualityNullPreferences(context.Background(), state.ContextID,
						vegetationQualityNullRequest(t, current.Values, next))
					if !result.Committed || result.ErrorMessage != "" {
						t.Fatal(result)
					}
					destination := filepath.Join(t.TempDir(), "quality-nulls.xlsx")
					stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
						workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
					if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
						t.Fatal("changed flags published stale approval", active, field, stale, err)
					}
					if _, err := os.Stat(destination); !os.IsNotExist(err) {
						t.Fatal("stale approval wrote output", active, field, err)
					}
					fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
					if err != nil || fresh.ApprovalHash == old.ApprovalHash {
						t.Fatal("fresh approval did not observe changed owned flags", active, field, err)
					}
					settings := fresh.Preview.Settings
					if active {
						if settings.Quality == nil || settings.Quality.Site.IncludeNull != next.SiteIncludeNulls ||
							settings.Quality.Veg.IncludeNull != next.VegIncludeNulls || settings.Quality.Soil.IncludeNull != next.SoilIncludeNulls {
							t.Fatal("active saved flags did not propagate", field, settings)
						}
						settings.Quality = old.Preview.Settings.Quality
						if !reflect.DeepEqual(settings, old.Preview.Settings) {
							t.Fatal("flag Save changed unrelated report settings")
						}
					} else if fresh.Preview.Settings.Quality != nil || !reflect.DeepEqual(fresh.Preview, old.Preview) ||
						fresh.WorkbookSHA256 != old.WorkbookSHA256 {
						t.Fatal("inactive flag Save enabled enforcement or changed output/arithmetic", field)
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
				t.Fatal("flag Save or workbook publication changed data/audits")
			}
		})
	}
}
