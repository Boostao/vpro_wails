package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func siteUnitSummaryWorkbookFixture(t *testing.T, method int) SiteUnitSummaryPreview {
	t.Helper()
	env, admin, su, master := summaryReportFixture()
	report, err := planSiteUnitSummary(context.Background(), "Sample", "Selected", method, 9, env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	return SiteUnitSummaryPreview{"owned-context", `C:\owned\project.sqlite`, `C:\owned\su.sqlite`, report}
}

func openSiteUnitSummaryWorkbook(t *testing.T, result siteUnitSummaryWorkbook) *excelize.File {
	t.Helper()
	if len(result.Bytes) == 0 {
		t.Fatal("no workbook bytes")
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	})
	return book
}

func siteUnitSummaryWorkbookCell(t *testing.T, book *excelize.File, sheet, cell, want string) {
	t.Helper()
	got, err := book.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
	if err != nil || got != want {
		t.Fatalf("%s!%s = %q, want %q (%v)", sheet, cell, got, want, err)
	}
	formula, err := book.GetCellFormula(sheet, cell)
	if err != nil || formula != "" {
		t.Fatalf("%s!%s unexpected formula %q (%v)", sheet, cell, formula, err)
	}
	if want != "" {
		kind, err := book.GetCellType(sheet, cell)
		if err != nil || kind != excelize.CellTypeSharedString && kind != excelize.CellTypeInlineString {
			t.Fatalf("%s!%s is not literal text: %v (%v)", sheet, cell, kind, err)
		}
	}
}

func TestSiteUnitSummaryWorkbookSourceCellsStylesAndBothMethods(t *testing.T) {
	for _, method := range []int{1, 2} {
		t.Run(strconv.Itoa(method), func(t *testing.T) {
			preview := siteUnitSummaryWorkbookFixture(t, method)
			before, err := json.Marshal(preview)
			if err != nil {
				t.Fatal(err)
			}
			result, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
			if err != nil {
				t.Fatal(err)
			}
			book := openSiteUnitSummaryWorkbook(t, result)
			if !reflect.DeepEqual(book.GetSheetList(), []string{"U", "NoName1", "_VPRO_Source"}) ||
				len(result.Sheets) != 2 || !reflect.DeepEqual(result.Sheets[0].Unit, metadataText("U")) ||
				!reflect.DeepEqual(result.Sheets[1].Unit, metadataText("")) {
				t.Fatal("unit order, typed identity or extra Summary sheet", result.Sheets, book.GetSheetList())
			}
			methodText := "Quantitative values summarized as: Min---Mean---Max"
			if method == 2 {
				methodText = "Quantitative values summarized as: Interquartile 25% - 50% - 75%"
			}
			rows := []int{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27,
				30, 31, 32, 33, 34, 35, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49}
			for i, sheet := range result.Sheets {
				unit := preview.Report.Units[i]
				for j, value := range []string{unit.Code, "", "Plots in unit: " + strconv.Itoa(len(unit.Plots)), methodText} {
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("A%d", j+1), value)
				}
				for j, row := range []int{6, 29, 37} {
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("A%d", row), []string{"SITE", "VEGETATION", "SOILS"}[j])
				}
				for j, row := range rows {
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("A%d", row), preview.Report.Fields[j].Label)
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("B%d", row), unit.Values[j])
				}
				for _, row := range []int{5, 28, 36, 50} {
					for _, column := range []string{"A", "B"} {
						siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("%s%d", column, row), "")
					}
				}
				for _, cell := range []string{"A1", "A2", "A3", "A6", "A29", "A37", "A4", "A7", "B9"} {
					id, err := book.GetCellStyle(sheet.Name, cell)
					if err != nil {
						t.Fatal(err)
					}
					style, err := book.GetStyle(id)
					bold := cell != "A4" && cell != "A7" && cell != "B9"
					if err != nil || (style.Font != nil && style.Font.Bold) != bold ||
						style.NumFmt != 0 || style.CustomNumFmt != nil {
						t.Fatal("source bold/text style differs", cell, style, err)
					}
				}
				data, err := book.GetRows(sheet.Name)
				if err != nil || len(data) != 49 {
					t.Fatal("source row shape differs", len(data), err)
				}
				for _, row := range data {
					if len(row) > 2 {
						t.Fatal("invented species or additional output columns", row)
					}
				}
			}
			siteUnitSummaryWorkbookCell(t, book, "U", "A3", "Plots in unit: 8")
			siteUnitSummaryWorkbookCell(t, book, "U", "B9", "100---200---300")
			siteUnitSummaryWorkbookCell(t, book, "U", "A15", "Site Disturbance 1")
			siteUnitSummaryWorkbookCell(t, book, "U", "A16", "Site Disturbance 2")
			siteUnitSummaryWorkbookCell(t, book, "U", "B15", "Fire(8)  ")
			siteUnitSummaryWorkbookCell(t, book, "U", "B16", "Fire(8)  ")
			siteUnitSummaryWorkbookCell(t, book, "NoName1", "A3", "Plots in unit: 1")
			after, _ := json.Marshal(preview)
			if !bytes.Equal(before, after) {
				t.Fatal("preparation mutated accepted preview")
			}
		})
	}
}

func TestSiteUnitSummaryWorkbookLiteralStringsNamesProvenanceAndDeterminism(t *testing.T) {
	preview := siteUnitSummaryWorkbookFixture(t, 1)
	old := preview.Report.Units[0].Code
	preview.Report.Units[0].Code = "Z:/raw"
	for i := range preview.Report.Memberships {
		member := &preview.Report.Memberships[i]
		if member.SiteUnit.Text != nil && *member.SiteUnit.Text == old {
			member.SiteUnit = metadataText("Z:/raw")
		}
	}
	longName := "=literal name 😃"
	preview.Report.Units[0].LongName = &longName
	preview.Report.Units[0].NameCandidates[0].Value = metadataText(longName)
	for i, value := range []string{"=SUM(A1:A2)", "+1", "-2", "@name", "001", "1E20", "0.5", "1/2", "'literal",
		" x \r\n\t😃 ", "_x0001_", "<>&"} {
		preview.Report.Units[0].Values[i] = value
	}
	preview.Report.Units[0].Values[9] = preview.Report.Units[0].Values[8]
	preview.Report.Units[0].Values[12] = "_x005F_x0001_ _x0001__x0001_ _xFFFF_"
	preview.Report.Units[0].Values[13] = " x \r\n\t😃 "
	// Multiple source chunks preserve valid astral Unicode and exact typed NULL/empty names.
	preview.Report.Units[0].Values[38] = strings.Repeat("😃", 8000)
	before, _ := json.Marshal(preview)
	result, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
	if err != nil {
		t.Fatal(err)
	}
	book := openSiteUnitSummaryWorkbook(t, result)
	if result.Sheets[0].Name != "Z--raw" {
		t.Fatal("shared checked sheet sanitization differs", result.Sheets)
	}
	siteUnitSummaryWorkbookCell(t, book, "Z--raw", "A1", "Z:/raw")
	siteUnitSummaryWorkbookCell(t, book, "Z--raw", "A2", longName)
	rows := []int{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	for i, row := range rows {
		siteUnitSummaryWorkbookCell(t, book, "Z--raw", fmt.Sprintf("B%d", row), preview.Report.Units[0].Values[i])
	}
	siteUnitSummaryWorkbookCell(t, book, "Z--raw", "B49", preview.Report.Units[0].Values[38])
	var encoded strings.Builder
	chunks := 0
	for row := 1; ; row++ {
		value, err := book.GetCellValue("_VPRO_Source", "A"+strconv.Itoa(row))
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			break
		}
		if len(value) > 30000 || len(value)%2 != 0 {
			t.Fatal("lossless hex chunk boundary differs")
		}
		chunks++
		encoded.WriteString(value)
	}
	raw, err := hex.DecodeString(encoded.String())
	if err != nil || chunks < 2 || !bytes.Equal(raw, before) {
		t.Fatal("complete typed preview provenance differs", chunks, err)
	}
	var restored SiteUnitSummaryPreview
	if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored, preview) {
		t.Fatal("typed provenance roundtrip differs", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
	if err != nil {
		t.Fatal(err)
	}
	hidden := false
	for _, file := range archive.File {
		if file.Name == "xl/workbook.xml" {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			xml, err := io.ReadAll(reader)
			closeErr := reader.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			hidden = strings.Contains(string(xml), `name="_VPRO_Source"`) && strings.Contains(string(xml), `state="veryHidden"`)
		}
	}
	if !hidden {
		t.Fatal("lossless source sheet is not very-hidden")
	}
	for i := 0; i < 3; i++ {
		again, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
		if err != nil || !bytes.Equal(result.Bytes, again.Bytes) || !reflect.DeepEqual(result.Sheets, again.Sheets) {
			t.Fatal("ordered exact XLSX bytes are nondeterministic", err)
		}
	}
	*result.Sheets[0].Unit.Text = "changed"
	result.Bytes[0] = 0
	after, _ := json.Marshal(preview)
	if !bytes.Equal(before, after) {
		t.Fatal("workbook output aliases preview")
	}
}

func TestSiteUnitSummaryWorkbookRejectsMalformedSourceAndShape(t *testing.T) {
	tests := []struct {
		name   string
		change func(*SiteUnitSummaryPreview)
	}{
		{"context-owner", func(p *SiteUnitSummaryPreview) { p.ContextID = "" }},
		{"project-path", func(p *SiteUnitSummaryPreview) { p.ProjectPath = "" }},
		{"su-path", func(p *SiteUnitSummaryPreview) { p.SUPath = "" }},
		{"project", func(p *SiteUnitSummaryPreview) { p.Report.Project = "" }},
		{"su", func(p *SiteUnitSummaryPreview) { p.Report.SU = "" }},
		{"none", func(p *SiteUnitSummaryPreview) { p.Report.SU = "None" }},
		{"dynamic", func(p *SiteUnitSummaryPreview) { p.Report.SU = "USysSuTableDynamic" }},
		{"method-zero", func(p *SiteUnitSummaryPreview) { p.Report.Method = 0 }},
		{"method-three", func(p *SiteUnitSummaryPreview) { p.Report.Method = 3 }},
		{"query", func(p *SiteUnitSummaryPreview) { p.Report.QuerySource = string(siteUnitDetailProjectJoin) }},
		{"fields-count", func(p *SiteUnitSummaryPreview) { p.Report.Fields = p.Report.Fields[:38] }},
		{"fields-label", func(p *SiteUnitSummaryPreview) { p.Report.Fields[0].Label = "changed" }},
		{"fields-source", func(p *SiteUnitSummaryPreview) { p.Report.Fields[29].Source = "Env" }},
		{"fields-order", func(p *SiteUnitSummaryPreview) {
			p.Report.Fields[0], p.Report.Fields[1] = p.Report.Fields[1], p.Report.Fields[0]
		}},
		{"units-empty", func(p *SiteUnitSummaryPreview) { p.Report.Units = nil }},
		{"units-omitted", func(p *SiteUnitSummaryPreview) { p.Report.Units = p.Report.Units[:1] }},
		{"units-duplicate", func(p *SiteUnitSummaryPreview) { p.Report.Units[1].Code = "U" }},
		{"units-order", func(p *SiteUnitSummaryPreview) {
			p.Report.Units[0], p.Report.Units[1] = p.Report.Units[1], p.Report.Units[0]
		}},
		{"values-short", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values = p.Report.Units[0].Values[:38] }},
		{"values-long", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values = append(p.Report.Units[0].Values, "") }},
		{"disturbance-replacement", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values[9] = "invented" }},
		{"plots-empty", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots = nil }},
		{"plot-order", func(p *SiteUnitSummaryPreview) {
			u := &p.Report.Units[0]
			u.Plots[0], u.Plots[1] = u.Plots[1], u.Plots[0]
		}},
		{"plot-duplicate", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots[1] = p.Report.Units[0].Plots[0] }},
		{"plot-owner", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots[0].SURowID = "missing" }},
		{"plot-wrong-unit", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots[0].SURowID = "7" }},
		{"plot-identity", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots[0].PlotNumber = "OTHER" }},
		{"env-id-empty", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots[0].EnvRowID = "" }},
		{"admin-id-empty", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots[0].AdminRowID = "" }},
		{"env-owner", func(p *SiteUnitSummaryPreview) { p.Report.Units[1].Plots[0].EnvRowID = "1" }},
		{"admin-owner", func(p *SiteUnitSummaryPreview) { p.Report.Units[1].Plots[0].AdminRowID = "1" }},
		{"plots-omitted", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Plots = p.Report.Units[0].Plots[:7] }},
		{"join-incomplete", func(p *SiteUnitSummaryPreview) {
			p.Report.Units[0].Plots = p.Report.Units[0].Plots[1:]
			p.Report.Memberships[0].JoinedRows--
		}},
		{"members-empty", func(p *SiteUnitSummaryPreview) { p.Report.Memberships = nil }},
		{"member-id-empty", func(p *SiteUnitSummaryPreview) { p.Report.Memberships[0].RowID = "" }},
		{"member-duplicate", func(p *SiteUnitSummaryPreview) { p.Report.Memberships[1].RowID = "1" }},
		{"member-order", func(p *SiteUnitSummaryPreview) {
			p.Report.Memberships[0], p.Report.Memberships[1] = p.Report.Memberships[1], p.Report.Memberships[0]
		}},
		{"member-negative", func(p *SiteUnitSummaryPreview) { p.Report.Memberships[0].JoinedRows = -1 }},
		{"member-status", func(p *SiteUnitSummaryPreview) { p.Report.Memberships[0].Status = "unknown" }},
		{"member-null", func(p *SiteUnitSummaryPreview) {
			p.Report.Memberships[0].PlotNumber = ProjectMetadataCell{Storage: "null"}
		}},
		{"member-extra-tag", func(p *SiteUnitSummaryPreview) {
			p.Report.Memberships[5].SiteUnit.Text = p.Report.Memberships[0].SiteUnit.Text
		}},
		{"member-storage", func(p *SiteUnitSummaryPreview) {
			s := "1"
			p.Report.Memberships[0].SiteUnit = ProjectMetadataCell{Storage: "integer", Integer: &s}
		}},
		{"false-exclusion", func(p *SiteUnitSummaryPreview) { p.Report.Memberships[2].PlotNumber = metadataText("P1") }},
		{"names-nil", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].NameCandidates = nil }},
		{"name-id-empty", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].NameCandidates[0].RowID = "" }},
		{"name-duplicate", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].NameCandidates[1].RowID = "1" }},
		{"name-order", func(p *SiteUnitSummaryPreview) {
			u := &p.Report.Units[0]
			u.NameCandidates[0], u.NameCandidates[1] = u.NameCandidates[1], u.NameCandidates[0]
		}},
		{"name-cross-owner", func(p *SiteUnitSummaryPreview) {
			p.Report.Units[1].NameCandidates = []EnvironmentReportName{{"1", ProjectMetadataCell{Storage: "null"}}}
		}},
		{"name-status", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].NameStatus = "missing" }},
		{"name-unsubstantiated", func(p *SiteUnitSummaryPreview) { s := "invented"; p.Report.Units[0].LongName = &s }},
		{"name-null-mismatch", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].LongName = nil }},
		{"name-conflicting", func(p *SiteUnitSummaryPreview) {
			p.Report.Units[0].NameCandidates[1].Value = metadataText("conflict")
			p.Report.Units[0].LongName = nil
			p.Report.Units[0].NameStatus = "conflicting"
		}},
		{"name-malformed", func(p *SiteUnitSummaryPreview) {
			p.Report.Units[0].NameCandidates[0].Value = ProjectMetadataCell{Storage: "text"}
		}},
		{"invalid-utf8-value", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values[0] = string([]byte{0xff}) }},
		{"xml-control-value", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values[0] = "\x01" }},
		{"xml-noncharacter", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values[0] = "\uffff" }},
		{"overlength-value", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values[0] = strings.Repeat("😃", 16384) }},
		{"invalid-owner-unicode", func(p *SiteUnitSummaryPreview) { p.ProjectPath = "\xff" }},
		{"invalid-member-unicode", func(p *SiteUnitSummaryPreview) { p.Report.Memberships[0].RowID = "\xff" }},
		{"invalid-name-unicode", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].NameCandidates[0].Value = metadataText("\xff") }},
		{"unrepresentable-literal-escape", func(p *SiteUnitSummaryPreview) { p.Report.Units[0].Values[0] = "_xD800_" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preview := siteUnitSummaryWorkbookFixture(t, 1)
			test.change(&preview)
			result, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
			if err == nil || !reflect.DeepEqual(result, siteUnitSummaryWorkbook{}) {
				t.Fatal("malformed source returned partial workbook", err, len(result.Bytes), result.Sheets)
			}
		})
	}
}

func TestSiteUnitSummaryWorkbookCheckedSheetNamesAndCollisions(t *testing.T) {
	for _, codes := range [][2]string{
		{"Z?", "Z*"}, {"z", "Z"}, {strings.Repeat("Z", 31) + "b", strings.Repeat("Z", 31) + "a"},
		{"NoName1", ""}, {"'Z", ""}, {"Z'", ""}, {"_VPRO_Source", ""},
		{strings.Repeat("Z", 30) + "😃", ""}, {"\xff", ""},
	} {
		t.Run(fmt.Sprintf("%q", codes), func(t *testing.T) {
			preview := siteUnitSummaryWorkbookFixture(t, 1)
			for i, code := range codes {
				old := preview.Report.Units[i].Code
				preview.Report.Units[i].Code = code
				for j := range preview.Report.Memberships {
					member := &preview.Report.Memberships[j]
					if member.SiteUnit.Text != nil && *member.SiteUnit.Text == old {
						member.SiteUnit = metadataText(code)
					}
				}
			}
			result, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
			if err == nil || !reflect.DeepEqual(result, siteUnitSummaryWorkbook{}) {
				t.Fatal("unsafe worksheet identity returned output", err, result.Sheets)
			}
		})
	}
	preview := siteUnitSummaryWorkbookFixture(t, 1)
	preview.Report.Units[0].NameCandidates = []EnvironmentReportName{
		{"1", ProjectMetadataCell{Storage: "null"}}, {"2", ProjectMetadataCell{Storage: "null"}},
	}
	preview.Report.Units[0].LongName, preview.Report.Units[0].NameStatus = nil, "missing"
	result, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
	if err != nil {
		t.Fatal("explicit NULL name candidates should display blank", err)
	}
	book := openSiteUnitSummaryWorkbook(t, result)
	siteUnitSummaryWorkbookCell(t, book, "U", "A2", "")
}

func TestSiteUnitSummaryWorkbookUTF16BoundaryAndCheckedTruncation(t *testing.T) {
	preview := siteUnitSummaryWorkbookFixture(t, 2)
	code := strings.Repeat("Z", 29) + "😃" + "trailing"
	preview.Report.Units[0].Code = code
	for i := range preview.Report.Memberships {
		member := &preview.Report.Memberships[i]
		if member.SiteUnit.Text != nil && *member.SiteUnit.Text == "U" {
			member.SiteUnit = metadataText(code)
		}
	}
	value := strings.Repeat("😃", 16383) + "a"
	preview.Report.Units[0].Values[38] = value
	preview.Report.Units[0].NameCandidates = append(preview.Report.Units[0].NameCandidates,
		EnvironmentReportName{"3", metadataText("")})
	result, err := prepareSiteUnitSummaryWorkbook(context.Background(), preview)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("Z", 29) + "😃"
	if result.Sheets[0].Name != name {
		t.Fatal("worksheet name split/changed complete UTF16 pair", result.Sheets)
	}
	book := openSiteUnitSummaryWorkbook(t, result)
	siteUnitSummaryWorkbookCell(t, book, name, "A1", code)
	siteUnitSummaryWorkbookCell(t, book, name, "B49", value)
}

func TestSiteUnitSummaryWorkbookOwnedReaderKernelIntegration(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external-%t", external), func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []int{1, 2} {
				input, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
				if err != nil || len(input.Tables) != 4 || input.Preview.ContextID != state.ContextID ||
					input.Preview.ProjectPath != state.ProjectPath || input.Preview.SUPath != state.SUPath {
					t.Fatal("complete owned workbook input unavailable", err)
				}
				raw, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				result, err := prepareSiteUnitSummaryWorkbook(context.Background(), input.Preview)
				if err != nil {
					t.Fatal("owned preview rejected by kernel", err)
				}
				book := openSiteUnitSummaryWorkbook(t, result)
				if len(result.Sheets) != len(input.Preview.Report.Units) || len(book.GetSheetList()) != len(result.Sheets)+1 {
					t.Fatal("owned units omitted or invented", result.Sheets)
				}
				for i, sheet := range result.Sheets {
					unit := input.Preview.Report.Units[i]
					if !reflect.DeepEqual(sheet.Unit, metadataText(unit.Code)) {
						t.Fatal("owned typed identity changed", sheet)
					}
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A1", unit.Code)
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A3", "Plots in unit: "+strconv.Itoa(len(unit.Plots)))
					for j, field := range input.Preview.Report.Fields {
						row := j + 7
						if j >= 21 {
							row += 2
						}
						if j >= 27 {
							row += 2
						}
						siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("A%d", row), field.Label)
						siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("B%d", row), unit.Values[j])
					}
				}
				after, err := json.Marshal(input)
				if err != nil || !bytes.Equal(raw, after) {
					t.Fatal("kernel modified captured preview/tables", err)
				}
				again, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
				if err != nil || !reflect.DeepEqual(input, again) {
					t.Fatal("owned source changed after kernel preparation", err)
				}
				repeated, err := prepareSiteUnitSummaryWorkbook(context.Background(), again.Preview)
				if err != nil || !bytes.Equal(result.Bytes, repeated.Bytes) {
					t.Fatal("owned input produced nondeterministic workbook bytes", err)
				}
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("owned kernel integration wrote configuration", err)
			}
		})
	}
}

type siteUnitSummaryWorkbookCountingContext struct {
	context.Context
	checks int
	stop   int
}

func (ctx *siteUnitSummaryWorkbookCountingContext) Err() error {
	ctx.checks++
	if ctx.stop > 0 && ctx.checks >= ctx.stop {
		return context.Canceled
	}
	return nil
}

func TestSiteUnitSummaryWorkbookNilCancelledAndLateCancellation(t *testing.T) {
	preview := siteUnitSummaryWorkbookFixture(t, 1)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer expire()
	for _, ctx := range []context.Context{nil, cancelled, expired} {
		result, err := prepareSiteUnitSummaryWorkbook(ctx, preview)
		if err == nil || !reflect.DeepEqual(result, siteUnitSummaryWorkbook{}) ||
			ctx == cancelled && !errors.Is(err, context.Canceled) ||
			ctx == expired && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("nil/cancelled context returned output", err, result.Sheets)
		}
	}
	baseline := &siteUnitSummaryWorkbookCountingContext{Context: context.Background()}
	if _, err := prepareSiteUnitSummaryWorkbook(baseline, preview); err != nil {
		t.Fatal(err)
	}
	for _, stop := range []int{2, 10, baseline.checks - 3, baseline.checks - 1, baseline.checks} {
		ctx := &siteUnitSummaryWorkbookCountingContext{Context: context.Background(), stop: stop}
		result, err := prepareSiteUnitSummaryWorkbook(ctx, preview)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, siteUnitSummaryWorkbook{}) {
			t.Fatal("mid-validation/serialization/final-close cancellation retained output", stop, err, result.Sheets)
		}
	}
}
