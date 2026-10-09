package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationThresholdPreferencesInvalidateWorkbookApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationThresholdPreferencesService(private.contexts, true)
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for field := 0; field < 2; field++ {
				old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil {
					t.Fatal(err)
				}
				current, err := preferences.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				next := current.Values
				if field == 0 {
					next.PresenceGreaterThan = 101
				} else {
					next.MeanCoverGreaterThan = 100.5
				}
				result := preferences.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID,
					vegetationThresholdRequest(t, current.Values, next))
				if !result.Committed || result.ErrorMessage != "" {
					t.Fatal(result)
				}
				destination := filepath.Join(t.TempDir(), "thresholds.xlsx")
				stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
				if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
					t.Fatal("changed threshold published stale approval", field, stale, err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale threshold approval wrote output", err)
				}
				fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil || fresh.ApprovalHash == old.ApprovalHash ||
					fresh.Preview.Settings.PresenceGreaterThan != next.PresenceGreaterThan ||
					fresh.Preview.Settings.MeanCoverGreaterThan != next.MeanCoverGreaterThan {
					t.Fatal("saved thresholds did not propagate", fresh, err)
				}
				settings := fresh.Preview.Settings
				settings.PresenceGreaterThan = old.Preview.Settings.PresenceGreaterThan
				settings.MeanCoverGreaterThan = old.Preview.Settings.MeanCoverGreaterThan
				if !reflect.DeepEqual(settings, old.Preview.Settings) {
					t.Fatal("threshold Save changed unrelated settings")
				}
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, private.contexts.projects.sqlite.attachments)) {
				t.Fatal("threshold Save changed data/audits")
			}
		})
	}
}
