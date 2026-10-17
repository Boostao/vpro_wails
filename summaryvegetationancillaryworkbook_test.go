package main

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryAncillaryWorkbookFixture(t *testing.T, includeEnv bool) summaryVegetationAncillaryWorkbookInput {
	t.Helper()
	service, state := summaryEnvironmentalLifeformServiceFixture(t, true)
	contexts := service.environment.report.contexts
	owner := contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `
		DROP TABLE IF EXISTS MasterSiteUnitList;
		CREATE TABLE MasterSiteUnitList(SiteSeries TEXT, SiteSeriesLongName TEXT);
		INSERT INTO MasterSiteUnitList VALUES('A','=Alpha_x0041_'),('A','=Alpha_x0041_'),('B',NULL),('unused','Retain');
		DROP TABLE IF EXISTS _table_metadata;
		CREATE TABLE _table_metadata(table_name TEXT, description TEXT);
		INSERT INTO _table_metadata VALUES('USysAllSpecs','fixture version');`)
	before := databaseBytes(t, owner.attachments)
	config, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	snapshots := 0
	input, err := withContextPlotRequest(t.Context(), contexts, state.ContextID, func(plots *PlotService) (summaryVegetationAncillaryWorkbookInput, error) {
		return withOwnedPreferenceSnapshot(t.Context(), plots, publicationReadSnapshotHooks{
			commitRead: func(*sql.Tx) error { snapshots++; return nil },
		}, func(owned *sqliteContext, tx *sql.Tx) (summaryVegetationAncillaryWorkbookInput, error) {
			options := summaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: includeEnv, Title: "=literal title"}
			source, err := readSummaryVegetationWorkbookSource(t.Context(), service, plots, owned, tx, state.ContextID,
				options)
			if err != nil {
				return summaryVegetationAncillaryWorkbookInput{}, err
			}
			tables, err := readSummaryVegetationAncillaryTables(t.Context(), owned, tx)
			if err != nil {
				return summaryVegetationAncillaryWorkbookInput{}, err
			}
			return summaryVegetationAncillaryWorkbookInput{
				Source: source, Options: options, Tables: tables, CreatedAt: "2026-10-17T02:00:00Z", CreateSummary: true,
			}, nil
		})
	})
	if err != nil || snapshots != 1 {
		t.Fatal("complete ancillary/body input did not share one owned snapshot", snapshots, err)
	}
	afterConfig, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(config, afterConfig) || !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("ancillary/body snapshot changed data/audits/configuration", err)
	}
	return input
}

func cloneSummaryAncillaryWorkbook(t *testing.T, input summaryVegetationAncillaryWorkbookInput) summaryVegetationAncillaryWorkbookInput {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var result summaryVegetationAncillaryWorkbookInput
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSummaryAncillaryWorkbookOwnedPlacementsProvenanceAndModes(t *testing.T) {
	for _, includeEnv := range []bool{false, true} {
		input := summaryAncillaryWorkbookFixture(t, includeEnv)
		for _, summary := range []bool{false, true} {
			for _, average := range []int{1, 2} {
				input := cloneSummaryAncillaryWorkbook(t, input)
				input.CreateSummary = summary
				input.Options.Average = average
				input.Options.ShowClass = 20
				input.Source.Input.Vegetation.Average = average
				input.Source.Raw.Preview.Average = average
				before, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				result, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input)
				if err != nil {
					t.Fatal("private ancillary composition unavailable", includeEnv, summary, average, err)
				}
				book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
				wantSheets := []string{input.Source.Input.Vegetation.Project, "SU Table", "_VPRO_Source"}
				if summary {
					wantSheets = []string{input.Source.Input.Vegetation.Project, "SU Table", "ReportSummary", "_VPRO_Source"}
				}
				if !reflect.DeepEqual(book.GetSheetList(), wantSheets) {
					t.Fatal("sheet identity/order changed", book.GetSheetList(), wantSheets)
				}
				assertCell := func(sheet, address, want string) {
					t.Helper()
					actual, err := book.GetCellValue(sheet, address, excelize.Options{RawCellValue: true})
					formula, formulaErr := book.GetCellFormula(sheet, address)
					if err != nil || formulaErr != nil || actual != want || formula != "" {
						t.Fatal("independent literal placement differs", sheet, address, actual, want, formula, err, formulaErr)
					}
				}
				for address, want := range map[string]string{"A1": "Plotnumber", "B1": "Site Unit", "A2": "P1", "B2": "A",
					"A3": "P1", "B3": "A", "A5": "P5", "A7": "", "B7": "A", "A8": "P9", "B8": "", "A9": "P6", "B9": "Orphan"} {
					assertCell("SU Table", address, want)
				}
				for _, column := range []string{"D", "E", "F", "G"} {
					if visible, err := book.GetColVisible("SU Table", column); err != nil || visible {
						t.Fatal("source-hidden ancillary columns visible", column, err)
					}
				}
				assertCell(result.Sheet, "A1", "=literal title")
				if summary {
					averageText := "n Plots"
					if average == 2 {
						averageText = "Characteristic"
					}
					for address, want := range map[string]string{
						"A1": "=literal title", "A2": "Date created: 2026-10-17T02:00:00Z",
						"A4": "Vegetation table:", "B4": input.Source.Input.Vegetation.Project, "B5": "Report_SU",
						"B6": "None_Lump", "B7": "Layer", "B8": "Species", "B9": "0",
						"B10": "3", "B11": "7", "B12": averageText, "B13": "fixture version",
						"D4": "Short name", "E4": "Units", "F4": "Long name", "D5": "A", "E5": "8",
						"F5": "=Alpha_x0041_", "D6": "B", "E6": "1", "F6": "", "D7": "",
						"H4": "Plotnumber", "I4": "Site Unit", "H5": "P1", "H6": "P1", "H10": "", "I10": "A",
						"H11": "P9", "I11": "", "H12": "P6", "I12": "Orphan",
					} {
						assertCell("ReportSummary", address, want)
					}
					for _, address := range []string{"B9", "B10", "B11", "E5", "E6"} {
						if kind, err := book.GetCellType("ReportSummary", address); err != nil ||
							kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset {
							t.Fatal("ancillary count was not numeric", address, kind, err)
						}
					}
					archive, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
					if err != nil {
						t.Fatal(err)
					}
					part, err := archive.Open("xl/worksheets/sheet3.xml")
					if err != nil {
						t.Fatal(err)
					}
					raw, readErr := io.ReadAll(part)
					closeErr := part.Close()
					if readErr != nil || closeErr != nil {
						t.Fatal(readErr, closeErr)
					}
					var worksheet struct {
						Rows []struct {
							Cells []struct {
								Address string  `xml:"r,attr"`
								Type    string  `xml:"t,attr"`
								Value   string  `xml:"v"`
								Formula *string `xml:"f"`
							} `xml:"c"`
						} `xml:"sheetData>row"`
					}
					if err := xml.Unmarshal(raw, &worksheet); err != nil {
						t.Fatal(err)
					}
					numbers := map[string]string{"B9": "0", "B10": "3", "B11": "7", "E5": "8", "E6": "1"}
					for _, row := range worksheet.Rows {
						for _, cell := range row.Cells {
							if expected, ok := numbers[cell.Address]; ok {
								if cell.Value != expected || cell.Type != "" && cell.Type != "n" || cell.Formula != nil {
									t.Fatal("physical numeric/formula XML differs", cell)
								}
								delete(numbers, cell.Address)
							}
						}
					}
					if len(numbers) != 0 {
						t.Fatal("required physical numeric cells absent", numbers)
					}
				}
				var provenance struct {
					Scope string
					Input summaryVegetationAncillaryWorkbookInput
				}
				if err := json.Unmarshal(summaryEnvironmentalWorkbookSource(t, book), &provenance); err != nil ||
					provenance.Scope == "" || !reflect.DeepEqual(provenance.Input, input) {
					t.Fatal("complete raw/body/options provenance unavailable", err)
				}
				after, _ := json.Marshal(input)
				again, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input)
				if err != nil || !bytes.Equal(before, after) || !bytes.Equal(result.Bytes, again.Bytes) {
					t.Fatal("source mutated or multi-sheet output nondeterministic", includeEnv, summary, average, err)
				}
			}
		}
	}
}

func TestSummaryAncillaryWorkbookConditionalSummaryAndNullEmpty(t *testing.T) {
	base := summaryAncillaryWorkbookFixture(t, false)
	for _, state := range []string{"missing", "null", "empty"} {
		input := cloneSummaryAncillaryWorkbook(t, base)
		input.Tables.Env.Rows = []ProjectMetadataRow{}
		want := "Unknown"
		switch state {
		case "missing":
			input.Tables.Descriptions.Rows = []ProjectMetadataRow{}
		case "null":
			input.Tables.Descriptions.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "null"}
		case "empty":
			input.Tables.Descriptions.Rows[0].Cells[1] = metadataText("")
			want = ""
		}
		input.Source.Raw.SU.Rows[0].Cells[0] = metadataText("")
		input.Source.Input.Vegetation.Memberships[0].PlotNumber = metadataText("")
		input.Source.Raw.Preview.Memberships[0].PlotNumber = metadataText("")
		got, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input)
		if err != nil {
			t.Fatal(state, err)
		}
		book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: got.Bytes})
		for address, expected := range map[string]string{"B10": "N/A", "B13": want} {
			actual, err := book.GetCellValue("ReportSummary", address)
			if err != nil || actual != expected {
				t.Fatal("empty Env/metadata state changed", state, address, actual, expected, err)
			}
		}
		emptyKind, err := book.GetCellType("SU Table", "A2")
		nullKind, nullErr := book.GetCellType("SU Table", "A7")
		if err != nil || nullErr != nil || emptyKind == nullKind {
			t.Fatal("empty and NULL membership cells collapsed", emptyKind, nullKind, err, nullErr)
		}
	}
	for _, change := range []func(*summaryVegetationAncillaryWorkbookInput){
		func(p *summaryVegetationAncillaryWorkbookInput) { p.Tables = summaryVegetationAncillaryTables{} },
		func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Tables.UnitNames.Rows[1].Cells[1] = metadataText("conflicting First")
		},
	} {
		input := cloneSummaryAncillaryWorkbook(t, base)
		change(&input)
		if _, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input); err == nil {
			t.Fatal("summary-only missing/ambiguous inputs accepted")
		}
		input.CreateSummary = false
		if _, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input); err != nil {
			t.Fatal("disabled summary inherited a summary-only constraint", err)
		}
	}
	input := cloneSummaryAncillaryWorkbook(t, base)
	input.CreateSummary = false
	input.Source.Input.Vegetation.Project = "ReportSummary"
	input.Source.Raw.Preview.Project = "ReportSummary"
	if _, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input); err != nil {
		t.Fatal("unallocated summary sheet name was incorrectly reserved", err)
	}
}

func TestSummaryAncillaryWorksheetExactRowFrames(t *testing.T) {
	for _, test := range []struct {
		name               string
		memberships, units int
		summary, valid     bool
	}{
		{"SU-last-row", 1048575, 0, false, true},
		{"SU-overflow", 1048576, 0, false, false},
		{"summary-last-membership", 1048572, 0, true, true},
		{"summary-membership-overflow", 1048573, 0, true, false},
		{"summary-final-counter", 0, 32762, true, true},
		{"summary-counter-overflow-after-write", 0, 32763, true, false},
		{"negative-membership", -1, 0, false, false},
		{"negative-unit-count", 0, -1, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateSummaryAncillaryWorksheetRows(test.memberships, test.units, test.summary)
			if (err == nil) != test.valid {
				t.Fatal("worksheet boundary differs", test, err)
			}
		})
	}
	input := summaryAncillaryWorkbookFixture(t, false)
	input.Source.Raw.SU.Rows = make([]ProjectMetadataRow, 1048576)
	got, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input)
	if err == nil || err.Error() != "Summary ancillary SU worksheet exceeds its row frame" ||
		!reflect.DeepEqual(got, summaryVegetationWorkbook{}) {
		t.Fatal("oversized physical SU was not refused before row projection", err)
	}
}

func TestSummaryAncillaryWorkbookRefusalsAndCancellation(t *testing.T) {
	base := summaryAncillaryWorkbookFixture(t, true)
	for _, test := range []struct {
		name string
		edit func(*summaryVegetationAncillaryWorkbookInput)
	}{
		{"SU-name", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Input.Vegetation.Project = "SU Table"
			p.Source.Raw.Preview.Project = "SU Table"
		}},
		{"Unicode-SU-name", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Input.Vegetation.Project = "\u017fU Table"
			p.Source.Raw.Preview.Project = "\u017fU Table"
		}},
		{"summary-name", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Input.Vegetation.Project = "reportsummary"
			p.Source.Raw.Preview.Project = "reportsummary"
		}},
		{"source-name", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Input.Vegetation.Project = "_VPRO_Source"
			p.Source.Raw.Preview.Project = "_VPRO_Source"
		}},
		{"different-preview", func(p *summaryVegetationAncillaryWorkbookInput) { p.Source.Input.Vegetation.ContextID = "different" }},
		{"different-options", func(p *summaryVegetationAncillaryWorkbookInput) { p.Options.Average = 2 }},
		{"invalid-retained-show", func(p *summaryVegetationAncillaryWorkbookInput) { p.Options.ShowClass = 2 }},
		{"different-calculation-show", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Input.Vegetation.ShowClass = 1
			p.Source.Raw.Preview.ShowClass = 1
		}},
		{"missing-membership", func(p *summaryVegetationAncillaryWorkbookInput) { p.Source.Raw.SU.Rows = p.Source.Raw.SU.Rows[1:] }},
		{"changed-membership", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Raw.SU.Rows[0].Cells[1] = metadataText("changed")
		}},
		{"different-Env", func(p *summaryVegetationAncillaryWorkbookInput) { p.Tables.Env.Rows = p.Tables.Env.Rows[1:] }},
		{"missing-Env", func(p *summaryVegetationAncillaryWorkbookInput) { p.Tables.Env = ProjectMetadataTable{} }},
		{"invalid-time", func(p *summaryVegetationAncillaryWorkbookInput) { p.CreatedAt = "2026-10-17" }},
		{"noncanonical-time", func(p *summaryVegetationAncillaryWorkbookInput) { p.CreatedAt = "2026-10-17T02:00:00+00:00" }},
		{"unrenderable-unused-master", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Tables.UnitNames.Rows[3].Cells[1] = metadataText("\x00")
		}},
		{"malformed-unused-raw", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Raw.Master.Rows[0].Cells[0] = metadataText(string([]byte{0xff}))
		}},
		{"malformed-membership", func(p *summaryVegetationAncillaryWorkbookInput) {
			p.Source.Raw.SU.Rows[0].Cells[0].Integer = func() *string { v := "1"; return &v }()
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := cloneSummaryAncillaryWorkbook(t, base)
			test.edit(&input)
			got, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), input)
			if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbook{}) {
				t.Fatal("unqualified input returned success/partial workbook", err)
			}

		})
	}
	for _, ctx := range []context.Context{nil, &summaryHeaderCancellingContext{Context: context.Background(), remaining: 30}} {
		got, err := prepareSummaryVegetationAncillaryWorkbook(ctx, base)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbook{}) || ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled composition returned success/partial workbook", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	data, err := renderSummaryWorkbookWithSheets(ctx, struct{}{}, summaryWorkbookLayout{Sheet: "Project", LastRow: 1, LastColumn: 3},
		func(book *excelize.File) error {
			if _, err := book.NewSheet("Added"); err != nil {
				return err
			}
			cancel()
			return nil
		})
	if len(data) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("post-append cancellation leaked success/partial bytes", err)
	}
	wantErr := errors.New("owned ancillary writer failed")
	data, err = renderSummaryWorkbookWithSheets(t.Context(), struct{}{}, summaryWorkbookLayout{Sheet: "Project", LastRow: 1, LastColumn: 3},
		func(book *excelize.File) error {
			if _, err := book.NewSheet("Added"); err != nil {
				return err
			}
			return wantErr
		})
	if len(data) != 0 || !errors.Is(err, wantErr) {
		t.Fatal("ancillary writer failure leaked success/partial bytes", err)
	}
}
