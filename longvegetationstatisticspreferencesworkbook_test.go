package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationStatisticsPreferencesInvalidateWorkbookApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationStatisticsPreferencesService(private.contexts, true)
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for _, next := range []struct {
				values  LongVegetationStatisticsPreferenceValues
				average string
				order   string
			}{
				{LongVegetationStatisticsPreferenceValues{10, 10}, "all-plots", "species"},
				{LongVegetationStatisticsPreferenceValues{20, 10}, "observations", "species"},
				{LongVegetationStatisticsPreferenceValues{20, 20}, "observations", "presence"},
				{LongVegetationStatisticsPreferenceValues{10, 20}, "all-plots", "presence"},
			} {
				before, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil {
					t.Fatal(err)
				}
				current, err := preferences.GetLongVegetationStatisticsPreferences(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				outcome := preferences.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
					vegetationStatisticsRequest(t, current.Values, next.values))
				if !outcome.Committed || outcome.ErrorMessage != "" {
					t.Fatal(outcome)
				}
				destination := filepath.Join(t.TempDir(), "statistics.xlsx")
				stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", before.CreatedDate, before.ApprovalHash, destination}))
				if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
					t.Fatal("saved statistics published a stale approval", stale, err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale approval wrote an output file", err)
				}
				fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil || fresh.Preview.Settings.Average != next.average || fresh.Preview.Settings.Order != next.order ||
					fresh.ApprovalHash == before.ApprovalHash || fresh.Preview.Settings.Title != before.Preview.Settings.Title {
					t.Fatal("saved source choices did not propagate independently", fresh, err)
				}
				settings := fresh.Preview.Settings
				settings.Average, settings.Order = before.Preview.Settings.Average, before.Preview.Settings.Order
				if !reflect.DeepEqual(settings, before.Preview.Settings) || !reflect.DeepEqual(fresh.Options, before.Options) {
					t.Fatal("statistics Save changed unrelated report settings")
				}
				published, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", fresh.CreatedDate, fresh.ApprovalHash, destination}))
				if err != nil || published == nil || published.Status != "published" || published.SHA256 != fresh.WorkbookSHA256 {
					t.Fatal(published, err)
				}
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, private.contexts.projects.sqlite.attachments)) {
				t.Fatal("saved statistics or publication changed data/audits")
			}
		})
	}
}
