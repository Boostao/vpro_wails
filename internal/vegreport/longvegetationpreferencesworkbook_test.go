package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationPreferencesInvalidateWorkbookApproval(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			workbooks := &VegetationWorkbookService{Private: private}
			titles := NewLongVegetationPreferencesService(private.contexts)
			beforeDB := vprotest.DatabaseBytes(t, private.contexts.Projects.Sqlite.Attachments)
			before, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil {
				t.Fatal(err)
			}
			title, err := titles.GetLongVegetationPreferences(context.Background(), state.ContextID)
			if err != nil || title.Values.Title != before.Preview.Settings.Title {
				t.Fatal(title, err)
			}
			next := " Literal changed workbook title "
			outcome := titles.SaveLongVegetationPreferences(context.Background(), state.ContextID,
				longVegetationTitleRequest(t, title.Values.Title, next))
			if !outcome.Committed || outcome.ErrorMessage != "" {
				t.Fatal(outcome)
			}
			destination := filepath.Join(t.TempDir(), "title.xlsx")
			oldRequest := vprotest.WorkbookRequest(t, vegetationWorkbookExportRequest{"unlumped", before.CreatedDate, before.ApprovalHash, destination})
			result, err := workbooks.ExportReviewed(context.Background(), state.ContextID, oldRequest)
			if err == nil && (result == nil || result.Status != "not-published") {
				t.Fatal("changed title published stale approval", result, err)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("stale title wrote an output file", err)
			}
			after, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil {
				t.Fatal(err)
			}
			if after.Preview.Settings.Title != next || after.Preview.Report.Title != next ||
				after.ApprovalHash == before.ApprovalHash || after.WorkbookSHA256 == before.WorkbookSHA256 {
				t.Fatal("new title did not propagate into report/approval/workbook")
			}
			after.Preview.Settings.Title = before.Preview.Settings.Title
			after.Preview.Report.Title = before.Preview.Report.Title
			if !reflect.DeepEqual(after.Preview, before.Preview) {
				t.Fatal("title changed report calculations/provenance")
			}
			freshRequest := vprotest.WorkbookRequest(t, vegetationWorkbookExportRequest{"unlumped", after.CreatedDate, after.ApprovalHash, destination})
			result, err = workbooks.ExportReviewed(context.Background(), state.ContextID, freshRequest)
			if err != nil || result == nil || result.Status != "published" || result.SHA256 != after.WorkbookSHA256 {
				t.Fatal(result, err)
			}
			if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, private.contexts.Projects.Sqlite.Attachments)) {
				t.Fatal("title or publication changed data/audits")
			}
		})
	}
}
