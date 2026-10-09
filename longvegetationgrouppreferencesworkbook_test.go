package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationGroupPreferencesInvalidateWorkbookApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			private.contexts.longVegetationStrataEnabled = true
			private.contexts.longVegetationLifeformEnabled = true
			private.contexts.longVegetationNoneEnabled = true
			workbooks := &VegetationWorkbookService{private: private}
			preferences := NewLongVegetationGroupPreferencesService(private.contexts, true)
			beforeDB := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			for _, mode := range []struct {
				value int
				label string
			}{{2, "strata"}, {3, "lifeform"}, {4, "none"}, {1, "layer"}} {
				old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil {
					t.Fatal(err)
				}
				current, err := preferences.GetLongVegetationGroupPreferences(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				result := preferences.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID,
					vegetationGroupRequest(t, current.Values.GroupBy, mode.value))
				if !result.Committed || result.ErrorMessage != "" {
					t.Fatal(result)
				}
				destination := filepath.Join(t.TempDir(), "grouping.xlsx")
				stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
				if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
					t.Fatal("changed grouping published stale approval", mode, stale, err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale grouping approval wrote output", err)
				}
				fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil || fresh.ApprovalHash == old.ApprovalHash || fresh.Preview.Settings.Grouping != mode.label {
					t.Fatal("saved grouping did not propagate", fresh, err)
				}
				settings := fresh.Preview.Settings
				settings.Grouping = old.Preview.Settings.Grouping
				if !reflect.DeepEqual(settings, old.Preview.Settings) {
					t.Fatal("grouping Save changed unrelated settings")
				}
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, private.contexts.projects.sqlite.attachments)) {
				t.Fatal("grouping Save changed data/audits")
			}
		})
	}
}
