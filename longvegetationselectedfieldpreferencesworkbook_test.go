package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLongVegetationSelectedScalarOwnedPublicationStaleSourceAndConfigurationRetry(t *testing.T) {
	for _, test := range []struct {
		declaration, kind string
		value, changed    any
	}{
		{"TEXT", "text", "literal native", "changed native"},
		{"INTEGER", "integer", int64(2147483647), int64(1)},
		{"BOOLEAN", "boolean", int64(-1), int64(0)},
		{"REAL", "real", 1.005, 0.15},
	} {
		t.Run(test.kind, func(t *testing.T) {
			service, state := vegetationWorkbookServiceFixture(t, true)
			contexts := service.contexts
			lists, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := lists.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := lists.Exec(`ALTER TABLE USysAllSpecs ADD COLUMN Literal ` + test.declaration); err != nil {
				t.Fatal(err)
			}
			if _, err := lists.Exec(`UPDATE USysAllSpecs SET Literal=?`, test.value); err != nil {
				t.Fatal(err)
			}
			contexts.longVegetationSelectedFieldEnabled = true
			if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVShowEnglishName": 4, "LVShowField": "Literal"}); err != nil {
				t.Fatal(err)
			}
			review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil || review == nil || review.Preview.Report.SelectedFieldType != test.kind {
				t.Fatal("selected scalar workbook review failed", review, err)
			}
			destination := filepath.Join(t.TempDir(), "selected.xlsx")
			request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
			for _, drift := range []string{"field", "source"} {
				if drift == "field" {
					if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVShowField": "EnglishName"}); err != nil {
						t.Fatal(err)
					}
				} else if _, err := lists.Exec(`UPDATE USysAllSpecs SET Literal=?`, test.changed); err != nil {
					t.Fatal(err)
				}
				if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil &&
					(outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "") {
					t.Fatal("changed selected scalar plan published", drift, outcome, err)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("stale selected scalar emitted output", drift, err)
				}
				if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVShowField": "Literal"}); err != nil {
					t.Fatal(err)
				}
				if _, err := lists.Exec(`UPDATE USysAllSpecs SET Literal=?`, test.value); err != nil {
					t.Fatal(err)
				}
			}
			fresh, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
			if err != nil || fresh == nil || fresh.ApprovalHash != review.ApprovalHash || fresh.WorkbookSHA256 != review.WorkbookSHA256 {
				t.Fatal("exact selected scalar source recovery changed original approval", fresh, err)
			}
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request)
			if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
				t.Fatal("exact source recovery could not publish original selected scalar approval", outcome, err)
			}
			data, err := os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 || len(data) != review.Bytes {
				t.Fatal("selected scalar publication differs from approved bytes")
			}
			book, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			for _, sheet := range review.Sheets {
				header, err := book.GetCellValue(sheet.Name, "C4")
				if err != nil || header != "Literal" {
					t.Error("published selected scalar lost literal header", header, err)
				}
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
			if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err != nil ||
				outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
				t.Fatal("selected scalar output collision replaced original publication", outcome, err)
			}
			after, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatal("collision changed selected scalar output", err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("selected scalar publication changed source databases")
			}
		})
	}
}

func TestLongVegetationSelectedScalarOwnedNullScientificFallbackRefusesUnverifiedTextWithoutOutput(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-fallback", true: "case-equivalent-mixed-fallback"}[mixed], func(t *testing.T) {
			service, state := vegetationWorkbookServiceFixture(t, true)
			contexts := service.contexts
			contexts.longVegetationSelectedFieldEnabled = true
			contexts.longVegetationNoneEnabled, contexts.longVegetationStrataEnabled, contexts.longVegetationLifeformEnabled = true, true, true
			project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "ro"))
			if err != nil {
				t.Fatal(err)
			}
			var species string
			err = project.QueryRow(`SELECT Species FROM ` + quoteHeaderIdentifier(contexts.projects.sqlite.selection.Project+"_Veg") +
				` WHERE PlotNumber='108050' AND typeof(Species)='text' ORDER BY rowid LIMIT 1`).Scan(&species)
			err = errors.Join(err, project.Close())
			if err != nil {
				t.Fatal(err)
			}
			alternate := strings.ToLower(species)
			if alternate == species {
				alternate = strings.ToUpper(species)
			}
			if alternate == species {
				t.Fatal("planned owned species has no ASCII case witness", species)
			}
			lists, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := lists.Exec(`DELETE FROM USysAllSpecs`); err != nil {
				lists.Close()
				t.Fatal(err)
			}
			if mixed {
				_, err = lists.Exec(`INSERT INTO USysAllSpecs (Code,ScientificName,EnglishName,Codetype,Lifeform)
					VALUES (?,NULL,'A','U',1),(?,?,'a','U',1)`, species, species, alternate)
			} else {
				_, err = lists.Exec(`INSERT INTO USysAllSpecs (Code,ScientificName,EnglishName,Codetype,Lifeform)
					VALUES (?,NULL,'A','U',1),(?,NULL,'a','U',1)`, species, species)
			}
			if err != nil {
				lists.Close()
				t.Fatal(err)
			}
			if err := lists.Close(); err != nil {
				t.Fatal(err)
			}
			if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVShowEnglishName": 4, "LVShowField": "EnglishName"}); err != nil {
				t.Fatal(err)
			}
			message := "unverified native collation representatives"
			if mixed {
				message = "unverified native effective species representatives"
			}
			for _, grouping := range []int{1, 2, 3, 4} {
				if err := contexts.projects.preferences.update("ReportOptions", map[string]any{
					"LVGroupBy": grouping, "LVConstantSppList": false,
					"LVPresenceGreaterThan": 1000000, "LVCoverGreaterThan": 1000000,
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID); err == nil ||
					!strings.Contains(err.Error(), message) {
					t.Fatal("thresholds hid the joined NULL scientific fallback refusal", mixed, grouping, err)
				}
				if review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`); err == nil || review != nil {
					t.Fatal("filtered NULL scientific fallback produced an approved workbook", grouping, review, err)
				}
			}
			destination := filepath.Join(t.TempDir(), "fallback.xlsx")
			request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", "2026-10-06", strings.Repeat("a", 64), destination})
			if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil &&
				(outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "") {
				t.Fatal("NULL scientific fallback emitted an unproven workbook", outcome, err)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("NULL scientific fallback left output", err)
			}
			lists, err = sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = lists.Exec(`UPDATE USysAllSpecs SET ScientificName=NULL, EnglishName='A'`)
			err = errors.Join(err, lists.Close())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID); err != nil {
				t.Fatal("corrected joined representatives could not retry the filtered report", err)
			}
		})
	}
}

func TestLongVegetationSelectedFieldMatchingUnsupportedSourceIdentifiersRemainStoredButUnavailable(t *testing.T) {
	service, state := vegetationWorkbookServiceFixture(t, true)
	contexts := service.contexts
	contexts.longVegetationSelectedFieldEnabled = true
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVShowField": ""}); err != nil {
		t.Fatal(err)
	}
	original, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil || original == nil {
		t.Fatal(original, err)
	}
	lists, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lists.Close(); err != nil {
			t.Error(err)
		}
	})
	editor := NewLongVegetationSelectedFieldPreferencesService(contexts, true)
	for _, field := range []string{"Bad Name", "Select", `Quoted"name`, "Foo.Bar", "[Code]", "UnverifiedName"} {
		if _, err := lists.Exec(`ALTER TABLE USysAllSpecs ADD COLUMN ` + quoteHeaderIdentifier(field) + ` TEXT`); err != nil {
			t.Fatal(err)
		}
		if _, err := lists.Exec(`UPDATE USysAllSpecs SET `+quoteHeaderIdentifier(field)+`=?`, "literal"); err != nil {
			t.Fatal(err)
		}
		review, err := editor.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID)
		if err != nil || review == nil {
			t.Fatal(review, err)
		}
		outcome := editor.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID,
			vegetationSelectedFieldRequest(t, review.Values, LongVegetationSelectedFieldPreferenceValues{4, field}))
		if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" {
			t.Fatal("report identifier restriction changed source-reachable preference storage", field, outcome)
		}
		stored, err := editor.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID)
		if err != nil || stored == nil || stored.Values.SelectedField != field {
			t.Fatal("unsupported report identifier was silently repaired", field, stored, err)
		}
		if _, err := contexts.GetLongVegetationOptions(context.Background(), state.ContextID); err == nil {
			t.Fatal("matching unsupported raw source identifier produced options", field)
		}
		if _, err := contexts.PreviewLongVegetation(context.Background(), state.ContextID); err == nil {
			t.Fatal("matching unsupported raw source identifier produced preview", field)
		}
		if review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`); err == nil || review != nil {
			t.Fatal("matching unsupported raw source identifier produced workbook review", field, review, err)
		}
		destination := filepath.Join(t.TempDir(), "refused.xlsx")
		request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", original.CreatedDate, original.ApprovalHash, destination})
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil &&
			(outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "") {
			t.Fatal("unsupported raw source identifier published old approved output", field, outcome, err)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("unsupported source identifier emitted output", field, err)
		}
	}
}

func TestLongVegetationSelectedScalarWorkbookRefusesUnprovenProvenanceAndStorageWithoutOutput(t *testing.T) {
	integer := func(value string) ProjectMetadataCell {
		return ProjectMetadataCell{Storage: "integer", Integer: &value}
	}
	original := vegetationWorkbookFixture(t)
	for _, test := range []struct {
		kind string
		cell ProjectMetadataCell
	}{
		{"", ProjectMetadataCell{Storage: "null"}},
		{"unknown", ProjectMetadataCell{Storage: "null"}},
		{"text", metadataText("bad\x00text")},
		{"text", metadataText(string([]byte{0xff}))},
		{"integer", metadataText("1")},
		{"integer", integer("2147483648")},
		{"boolean", integer("1")},
		{"real", integer("1")},
	} {
		t.Run(test.kind+test.cell.Storage, func(t *testing.T) {
			preview := original
			preview.Settings.ShowEnglishName, preview.Settings.ShowSpeciesCode = false, false
			preview.Settings.SelectedField, preview.Report.SelectedFieldType = "Literal", test.kind
			unit, row := original.Report.Units[0], original.Report.Units[0].Rows[0]
			row.EnglishName, row.MatchedName = cloneSiteUnitCell(test.cell), cloneSiteUnitCell(test.cell)
			unit.Rows = []LongVegetationRow{row}
			preview.Report.Units = []LongVegetationUnit{unit}
			workbook, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
			if err == nil || len(workbook.Bytes) != 0 || len(workbook.Sheets) != 0 || len(workbook.SkippedUnits) != 0 {
				t.Fatal("unproven selected scalar produced a partial/success-shaped workbook", workbook, err)
			}
		})
	}
}

func TestLongVegetationSelectedScalarWorkbookLiteralHeaderAndPrimitiveCellTypes(t *testing.T) {
	integer := func(value string) ProjectMetadataCell {
		return ProjectMetadataCell{Storage: "integer", Integer: &value}
	}

	real := 1.005
	original := vegetationWorkbookFixture(t)
	for _, test := range []struct {
		kind     string
		cell     ProjectMetadataCell
		wantType excelize.CellType
		wantText string
	}{
		{"text", metadataText("=literal & native"), excelize.CellTypeSharedString, "=literal & native"},
		{"integer", integer("2147483647"), excelize.CellTypeUnset, "2147483647.0"},
		{"boolean", integer("-1"), excelize.CellTypeBool, "TRUE"},
		{"boolean", integer("0"), excelize.CellTypeBool, "FALSE"},
		{"real", ProjectMetadataCell{Storage: "real", Real: &real}, excelize.CellTypeUnset, "1.0"},
	} {
		t.Run(test.kind+test.wantText, func(t *testing.T) {
			preview := original
			preview.Settings.ShowEnglishName, preview.Settings.ShowSpeciesCode = false, false
			preview.Settings.SelectedField = ` Literal & "field" `
			preview.Report.SelectedFieldType = test.kind
			unit, row := original.Report.Units[0], original.Report.Units[0].Rows[0]
			row.EnglishName, row.MatchedName = cloneSiteUnitCell(test.cell), cloneSiteUnitCell(test.cell)
			unit.Rows = []LongVegetationRow{row}
			preview.Report.Units = []LongVegetationUnit{unit}
			workbook, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
			if err != nil || len(workbook.Bytes) == 0 || len(workbook.Sheets) != 1 {
				t.Fatal(workbook, err)
			}
			book, err := excelize.OpenReader(bytes.NewReader(workbook.Bytes))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := book.Close(); err != nil {
					t.Error(err)
				}
			}()
			name := workbook.Sheets[0].Name
			header, err := book.GetCellValue(name, "C4")
			if err != nil || header != preview.Settings.SelectedField {
				t.Fatal("literal source header changed", header, err)
			}
			kind, err := book.GetCellType(name, "C5")
			if err != nil || kind != test.wantType {
				t.Fatal("selected scalar became a string or numeric Boolean", kind, test.wantType, err)
			}
			value, err := book.GetCellValue(name, "C5")
			if err != nil || value != test.wantText {
				t.Fatal("source scalar display changed", value, test.wantText, err)
			}
			if test.kind == "integer" || test.kind == "real" {
				raw, err := book.GetCellValue(name, "C5", excelize.Options{RawCellValue: true})
				if err != nil {
					t.Fatal(err)
				}
				if test.kind == "integer" {
					number, err := strconv.ParseInt(raw, 10, 64)
					if err != nil || number != 2147483647 {
						t.Fatal("reopened native Long changed", raw, err)
					}
				} else {
					number, err := strconv.ParseFloat(raw, 64)
					if err != nil || math.Float64bits(number) != math.Float64bits(real) {
						t.Fatal("reopened DOUBLE bits changed", raw, err)
					}
				}
			}
			formula, err := book.GetCellFormula(name, "C5")
			if err != nil || formula != "" {
				t.Fatal("literal source text became executable worksheet syntax", formula, err)
			}
			if !reflect.DeepEqual(preview.Report.Units[0].Rows[0].EnglishName, test.cell) {
				t.Fatal("workbook formatting changed typed source")
			}
		})
	}
}
