package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLongVegetationNamingPreferencesIndependentPhysicalNamesAndWorkbook(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			contexts := private.contexts
			contexts.longVegetationCodeEnabled = true
			preferences := NewLongVegetationNamingPreferencesService(contexts, true)
			workbooks := &VegetationWorkbookService{private: private}
			lists, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "ro"))
			if err != nil {
				t.Fatal(err)
			}
			rows, err := lists.Query(`SELECT Code,EnglishName FROM USysAllSpecs`)
			if err != nil {
				t.Fatal(err)
			}
			physicalNames, physicalCodes := map[string]bool{}, map[string]bool{}
			for rows.Next() {
				var code, english sql.NullString
				if err := rows.Scan(&code, &english); err != nil {
					t.Fatal(err)
				}
				if code.Valid {
					physicalCodes[code.String] = true
				}
				if english.Valid {
					physicalNames[english.String] = true
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if err := lists.Close(); err != nil {
				t.Fatal(err)
			}
			if len(physicalCodes) == 0 || len(physicalNames) == 0 {
				t.Fatal("independent physical naming witness is empty")
			}
			if err := contexts.projects.preferences.update("ReportOptions", map[string]any{
				"LVShowEnglishName": 1, "LVShowField": " historical\u0000field ",
			}); err != nil {
				t.Fatal(err)
			}
			beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
			for _, next := range []int{2, 0, 1} {
				old, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil {
					t.Fatal(err)
				}
				current, err := preferences.GetLongVegetationNamingPreferences(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				proposed := LongVegetationNamingPreferenceValues{next, ""}
				saved := preferences.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
					vegetationNamingRequest(t, current.Values, proposed))
				if !saved.Changed || !saved.Committed || saved.ErrorMessage != "" {
					t.Fatal(saved)
				}
				destination := filepath.Join(t.TempDir(), "naming.xlsx")
				stale, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", old.CreatedDate, old.ApprovalHash, destination}))
				if err == nil && (stale == nil || stale.Status != "not-published" || stale.ErrorMessage == "") {
					t.Fatal("changed naming published stale approval", next, stale, err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale naming approval wrote output", next, err)
				}
				fresh, err := workbooks.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
				if err != nil || fresh.ApprovalHash == old.ApprovalHash ||
					fresh.Preview.Settings.ShowEnglishName != (next == 1) || fresh.Preview.Settings.ShowSpeciesCode != (next == 2) {
					t.Fatal("saved source mode did not propagate", next, fresh, err)
				}
				settings := fresh.Preview.Settings
				settings.ShowEnglishName, settings.ShowSpeciesCode = old.Preview.Settings.ShowEnglishName, old.Preview.Settings.ShowSpeciesCode
				if !reflect.DeepEqual(settings, old.Preview.Settings) {
					t.Fatal("naming changed unrelated options")
				}
				observed := 0
				for _, unit := range fresh.Preview.Report.Units {
					for _, row := range unit.Rows {
						if next == 0 {
							if row.EnglishName.Storage != "null" || row.MatchedName.Storage != "null" {
								t.Fatal("Show none invented a supplementary name", row)
							}
						} else {
							for _, cell := range []ProjectMetadataCell{row.EnglishName, row.MatchedName} {
								if cell.Text != nil {
									observed++
									if next == 1 && !physicalNames[*cell.Text] || next == 2 && !physicalCodes[*cell.Text] {
										t.Fatal("supplementary text was not observed in independent physical source", next, cell)
									}
								}
							}
						}
					}
				}
				if next != 0 && observed == 0 {
					t.Fatal("naming proof observed no named physical rows", next)
				}
				published, err := workbooks.ExportReviewed(context.Background(), state.ContextID,
					workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", fresh.CreatedDate, fresh.ApprovalHash, destination}))
				if err != nil || published == nil || published.Status != "published" || published.SHA256 != fresh.WorkbookSHA256 {
					t.Fatal(next, published, err)
				}
				raw, err := os.ReadFile(destination)
				if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != fresh.WorkbookSHA256 {
					t.Fatal("published bytes differ from reviewed SHA", next, err)
				}
				book, err := excelize.OpenFile(destination)
				if err != nil {
					t.Fatal(err)
				}
				if len(fresh.Sheets) == 0 {
					t.Fatal("no published data sheet")
				}
				headers := []string{"Layer", "Spp", "P", "MC"}
				if next == 1 {
					headers = []string{"Layer", "Spp", "English Name", "P", "MC"}
				} else if next == 2 {
					headers = []string{"Layer", "Spp", "Code", "P", "MC"}
				}
				for column, expected := range headers {
					address, err := excelize.CoordinatesToCellName(column+1, 4)
					if err != nil {
						t.Fatal(err)
					}
					actual, err := book.GetCellValue(fresh.Sheets[0].Name, address)
					if err != nil || actual != expected {
						t.Fatal("independently reopened source naming columns differ", next, address, actual, expected, err)
					}
				}
				if err := book.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("naming persistence/report/workbook changed physical data or source audits")
			}
		})
	}
}
