package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLongVegetationNoneUnitsOwnedPreviewWorkbookAndStaleEnv(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			contexts := private.contexts
			store, owner := contexts.projects.preferences, contexts.projects.sqlite
			if err := store.update("ReportOptions", map[string]any{"LVUnitGroups": 3}); err != nil {
				t.Fatal(err)
			}

			for _, read := range []func() error{
				func() error {
					_, err := contexts.GetLongVegetationOptions(context.Background(), state.ContextID)
					return err
				},
				func() error {
					_, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID)
					return err
				},
				func() error {
					_, err := private.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
					return err
				},
			} {
				if err := read(); err == nil || !strings.Contains(err.Error(), "None report units are disabled") {
					t.Fatal("source3 bypassed its independent default-off gate", err)
				}
			}
			project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer project.Close()
			if _, err := project.Exec(`CREATE TABLE NoneUnitEnv AS SELECT * FROM Sample_Env WHERE 0;
				DROP TABLE Sample_Env;
				CREATE TABLE Sample_Env AS SELECT * FROM NoneUnitEnv;
				DROP TABLE NoneUnitEnv;
				INSERT INTO Sample_Env(PlotNumber) VALUES('108050'),('108050'),(NULL)`); err != nil {
				t.Fatal(err)
			}
			su, err := sql.Open("sqlite3", sqliteFileURI(state.SUPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer su.Close()
			if _, err := su.Exec(`INSERT INTO Report_SU(PlotNumber,SiteUnit) VALUES('108050',NULL)`); err != nil {
				t.Fatal(err)
			}
			contexts.longVegetationNoneUnitsEnabled = true
			beforeDB := databaseBytes(t, owner.attachments)
			beforeConfig, err := os.ReadFile(store.path)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || preview.Settings.UnitGrouping != "all-plots" || len(preview.Report.NoneUnits) != 4 ||
				preview.Report.Quality != nil || len(preview.Report.Units) != 1 ||
				preview.Report.Units[0].NumPlots != 4 || *preview.Report.Units[0].Code.Text != "All plots" ||
				!reflect.DeepEqual(preview.Report.Units[0].MembershipIDs, []string{"1", "4"}) {
				t.Fatal("Env/current-list join lost independent count, NULL source unit or original physical IDs", preview, err)
			}
			for i, occurrence := range preview.Report.NoneUnits {
				if occurrence.QualityOccurrence != nil || occurrence.EnvRowID != []string{"1", "1", "2", "2"}[i] ||
					occurrence.MembershipID != []string{"1", "4", "1", "4"}[i] ||
					*occurrence.SiteUnit.Text != "All plots" || *occurrence.PlotNumber.Text != "108050" {
					t.Fatal("derived source witnesses differ from independently enumerated join", occurrence)
				}
			}
			raw, err := json.Marshal(preview)
			var reopened LongVegetationPreview
			if err != nil || json.Unmarshal(raw, &reopened) != nil || !reflect.DeepEqual(preview, reopened) ||
				!bytes.Contains(raw, []byte(`"qualityOccurrence":null`)) {
				t.Fatal("None source transport roundtrip lost derived witness", string(raw), err)
			}
			review, err := private.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil || review.Layout.Summary == nil || review.Layout.Summary.SelectedPlotRows != 4 ||
				len(review.Layout.Summary.Memberships) != 4 {
				t.Fatal("None summary counted original SU instead of derived Env/current-list occurrences", review, err)
			}
			for _, member := range review.Layout.Summary.Memberships {
				if member.NoneUnit == nil || member.NoneUnit.MembershipID != member.RowID ||
					member.Quality != nil || *member.SiteUnit.Text != "All plots" {
					t.Fatal("summary lost distinct physical/derived witness", member)
				}
			}
			assertProfileSUFiles(t, contexts, beforeDB)
			destination := filepath.Join(t.TempDir(), "all-plots.xlsx")
			request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
			if _, err := project.Exec(`UPDATE Sample_Env SET PlotNumber='changed' WHERE rowid=2`); err != nil {
				t.Fatal(err)
			}
			stale, err := private.ExportReviewed(context.Background(), state.ContextID, request)
			if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
				t.Fatal("changed original Env published stale None workbook", stale)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("stale None approval emitted output", err)
			}
			if _, err := project.Exec(`UPDATE Sample_Env SET PlotNumber='108050' WHERE rowid=2`); err != nil {
				t.Fatal(err)
			}
			beforeDB = databaseBytes(t, owner.attachments)
			outcome, err := private.ExportReviewed(context.Background(), state.ContextID, request)
			if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
				t.Fatal("restored original None join failed reviewed retry", outcome, err)
			}
			book, err := excelize.OpenFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			for address, expected := range map[string]string{
				"B5": "USysSuTableDynamic_SU", "B11": "3", "B12": "4",
				"D5": "108050", "E5": "All plots", "D8": "108050", "E8": "All plots",
			} {
				if value, err := book.GetCellValue("ReportSummary", address); err != nil || value != expected {
					t.Fatal("reopened None summary differs", address, value, err)
				}
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
			beforeCancel, cancel := context.WithCancel(context.Background())
			cancel()
			if value, err := contexts.PreviewLongVegetation(beforeCancel, state.ContextID); !errors.Is(err, context.Canceled) ||
				!reflect.DeepEqual(value, LongVegetationPreview{}) {
				t.Fatal("cancelled None preview returned partial witnesses", value, err)
			}
			if _, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID); err != nil {
				t.Fatal("None cancellation leaked owned lease", err)
			}
			assertProfileSUFiles(t, contexts, beforeDB)
			afterConfig, err := os.ReadFile(store.path)
			if err != nil || !bytes.Equal(beforeConfig, afterConfig) {
				t.Fatal("None read/workbook rewrote retained source3 preference", err)
			}
		})
	}
}
