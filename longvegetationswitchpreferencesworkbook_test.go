package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationSwitchPreferencesInvalidateWorkbookApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationSwitchPreferencesService(private.contexts, true)
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for field := 0; field < 4; field++ {
				old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil {
					t.Fatal(err)
				}
				current, err := preferences.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				next := current.Values
				switch field {
				case 0:
					next.QuickReport = false
				case 1:
					next.SpaceBetweenGroups = true
				case 2:
					next.ReportSummary = false
				case 3:
					next.ConstantSpeciesList = false
				}
				result := preferences.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
					vegetationSwitchRequest(t, current.Values, next))
				if !result.Committed || result.ErrorMessage != "" {
					t.Fatal(result)
				}
				destination := filepath.Join(t.TempDir(), "switches.xlsx")
				stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
				if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
					t.Fatal("changed switch published stale approval", field, stale, err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale approval wrote output", field, err)
				}
				fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil || fresh.ApprovalHash == old.ApprovalHash ||
					fresh.Options.QuickReport != next.QuickReport || fresh.Options.SpaceBetweenGroups != next.SpaceBetweenGroups ||
					fresh.Options.ReportSummary != next.ReportSummary || (fresh.Summary != nil) != next.ReportSummary ||
					fresh.Preview.Settings.ConstantSpeciesList != next.ConstantSpeciesList {
					t.Fatal("saved source switches did not propagate", field, fresh, err)
				}
				settings := fresh.Preview.Settings
				settings.ConstantSpeciesList = old.Preview.Settings.ConstantSpeciesList
				if !reflect.DeepEqual(settings, old.Preview.Settings) {
					t.Fatal("switch Save changed unrelated settings")
				}
				if field < 3 && !reflect.DeepEqual(fresh.Preview, old.Preview) {
					t.Fatal("layout/summary switch changed calculation/provenance")
				}
				if field == 3 {
					published, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
						workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", fresh.CreatedDate, fresh.ApprovalHash, destination}))
					if err != nil || published == nil || published.Status != "published" || published.SHA256 != fresh.WorkbookSHA256 {
						t.Fatal(published, err)
					}
				}
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, private.contexts.projects.sqlite.attachments)) {
				t.Fatal("switch Save or publication changed data/audits")
			}
		})
	}
}
