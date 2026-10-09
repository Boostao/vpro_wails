package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func environmentWorkbookFixture(t *testing.T) EnvironmentReport {
	t.Helper()
	env, admin, su, master := environmentReportFixture()
	master.Rows = master.Rows[:1]
	report, err := planLongEnvironment(context.Background(), "Project", "Selected", "  Title 😀  ", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestLongEnvironmentWorkbookTransposedLiteralCellsAndLosslessSource(t *testing.T) {
	report := environmentWorkbookFixture(t)
	report.Units[0].Plots[0].Values[1] = metadataText("=1+1")
	report.Units[0].Plots[0].Values[2] = metadataText("  exact\r\ntext 😀  ")
	report.Units[0].Plots[0].Values[14] = metadataInteger("-1")
	report.Units[0].Plots[0].Values[15] = siviReal(0.125)
	first, err := prepareLongEnvironmentWorkbook(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareLongEnvironmentWorkbook(context.Background(), report)
	if err != nil || !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatal("workbook bytes depend on clock/order or changed between identical reads", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(first.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for sheet, unit := range report.Units {
		name := first.Sheets[sheet].Name
		if first.Sheets[sheet].Unit != unit.Code {
			t.Fatal("worksheet identity lost", first.Sheets)
		}
		for row, expected := range []string{report.Title, "Environment Table", "Site Unit - " + unit.Code} {
			address, _ := excelize.CoordinatesToCellName(1, row+1)
			actual, err := book.GetCellValue(name, address)
			if err != nil || actual != expected {
				t.Fatal("original title/group header changed", name, address, actual, expected, err)
			}
		}
		for row, field := range report.Fields {
			address, _ := excelize.CoordinatesToCellName(1, row+5)
			label, err := book.GetCellValue(name, address)
			if err != nil || label != field.Label {
				t.Fatal("72-field transposition/labels changed", name, address, label, err)
			}
		}
	}
	for address, expected := range map[string]string{"B6": "=1+1", "B7": "  exact\r\ntext 😀  ", "B19": "-1", "B20": "0.125"} {
		actual, err := book.GetCellValue(first.Sheets[0].Name, address, excelize.Options{RawCellValue: true})
		if err != nil || actual != expected {
			t.Fatal("typed/literal spreadsheet value changed", address, actual, expected, err)
		}
	}
	formula, err := book.GetCellFormula(first.Sheets[0].Name, "B6")
	if err != nil || formula != "" {
		t.Fatal("literal text became a formula", formula, err)
	}
	var chunks strings.Builder
	for row := 1; ; row++ {
		address, _ := excelize.CoordinatesToCellName(1, row)
		value, err := book.GetCellValue("_VPRO_Source", address)
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			break
		}
		chunks.WriteString(value)
	}
	source, err := hex.DecodeString(chunks.String())
	if err != nil {
		t.Fatal(err)
	}
	var original EnvironmentReport
	if err := json.Unmarshal(source, &original); err != nil || !reflect.DeepEqual(original, report) {
		t.Fatal("hidden original typed values/provenance/NULL diagnostics lost", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(first.Bytes), int64(len(first.Bytes)))
	if err != nil {
		t.Fatal(err)
	}
	var worksheet, workbook string
	for _, file := range archive.File {
		if file.Name != "xl/worksheets/sheet1.xml" && file.Name != "xl/workbook.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(reader)
		if err := errors.Join(readErr, reader.Close()); err != nil {
			t.Fatal(err)
		}
		if file.Name == "xl/workbook.xml" {
			workbook = string(data)
		} else {
			worksheet = string(data)
		}
	}
	if strings.Contains(worksheet, "<f>") || !strings.Contains(workbook, `name="_VPRO_Source"`) ||
		!strings.Contains(workbook, `state="veryHidden"`) || !strings.Contains(worksheet, `state="frozen"`) {
		t.Fatal("independent OpenXML formulas/provenance/freeze contract changed")
	}
}

func TestLongEnvironmentWorkbookSourceSheetNamesAndExplicitRefusals(t *testing.T) {
	for _, candidate := range []struct {
		code string
		want string
	}{
		{"", "NoName2"}, {`A:/\[]*?B`, "A-------B"},
		{strings.Repeat("x", 32), strings.Repeat("x", 31)},
		{" exact ", " exact "}, {strings.Repeat("x", 29) + "😀", strings.Repeat("x", 29) + "😀"},
	} {
		actual, err := environmentWorkbookSheetName(candidate.code, 2)
		if err != nil || actual != candidate.want {
			t.Fatal("source name substitution/truncation changed", candidate, actual, err)
		}
	}
	for _, code := range []string{"'edge", "edge'", "_vpro_source", "bad\x00", strings.Repeat("x", 30) + "😀"} {
		if _, err := environmentWorkbookSheetName(code, 0); err == nil {
			t.Fatal("invalid source worksheet silently repaired", code)
		}
	}
	for _, mutate := range []func(*EnvironmentReport){
		func(r *EnvironmentReport) { r.Fields = r.Fields[:71] },
		func(r *EnvironmentReport) { r.Units = nil },
		func(r *EnvironmentReport) { r.Title = "\xff" },
		func(r *EnvironmentReport) { r.Units[1].Code = strings.ToLower(r.Units[0].Code) },
		func(r *EnvironmentReport) { r.Units[0].Plots[0].Values = nil },
		func(r *EnvironmentReport) { r.Units[0].Plots[0].Values[0] = metadataText(strings.Repeat("x", 32768)) },
		func(r *EnvironmentReport) { r.Units[0].Plots[0].Values[0] = metadataInteger("1000000000000000") },
		func(r *EnvironmentReport) {
			r.Units[0].LongName = nil
			r.Units[0].NameStatus = "conflicting"
			r.Units[0].NameCandidates = []EnvironmentReportName{{RowID: "1", Value: metadataText("one")}, {RowID: "2", Value: metadataText("two")}}
		},
	} {
		report := environmentWorkbookFixture(t)
		mutate(&report)
		result, err := prepareLongEnvironmentWorkbook(context.Background(), report)
		if err == nil || !reflect.DeepEqual(result, environmentWorkbook{}) {
			t.Fatal("incomplete/invalid workbook published", result.Sheets, err)
		}
	}
}

func TestLongEnvironmentWorkbookPrintSettingsOrderingAndDuplicateNullNames(t *testing.T) {
	report := environmentWorkbookFixture(t)
	report.Units[0].Code, report.Units[1].Code = "z:unit", "A:unit"
	report.Units[0].LongName, report.Units[0].NameStatus = nil, "missing"
	report.Units[0].NameCandidates = []EnvironmentReportName{
		{RowID: "1", Value: ProjectMetadataCell{Storage: "null"}}, {RowID: "2", Value: ProjectMetadataCell{Storage: "null"}},
	}
	result, err := prepareLongEnvironmentWorkbook(context.Background(), report)
	if err != nil {
		t.Fatal("identical original NULL candidates became ambiguous", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"A-unit", "V", "z-unit", "_VPRO_Source"}) {
		t.Fatal("source uppercase worksheet sorting lost", got)
	}
	if result.Sheets[0].Unit != "z:unit" || result.Sheets[0].Name != "z-unit" {
		t.Fatal("physical sorting changed original unit mapping order", result.Sheets)
	}
	for _, sheet := range result.Sheets {
		layout, err := book.GetPageLayout(sheet.Name)
		if err != nil || layout.Orientation == nil || *layout.Orientation != "portrait" ||
			layout.FitToHeight == nil || *layout.FitToHeight != 1 || layout.FitToWidth == nil || *layout.FitToWidth != 0 {
			t.Fatal("source page orientation/fitting changed", layout, err)
		}
		props, err := book.GetSheetProps(sheet.Name)
		if err != nil || props.FitToPage == nil || !*props.FitToPage {
			t.Fatal("source Zoom=False fit mode lost", props, err)
		}
		margins, err := book.GetPageMargins(sheet.Name)
		if err != nil || margins.Left == nil || *margins.Left != .55 || margins.Right == nil || *margins.Right != .55 ||
			margins.Top == nil || *margins.Top != .5 || margins.Bottom == nil || *margins.Bottom != .5 ||
			margins.Header == nil || *margins.Header != .5 || margins.Footer == nil || *margins.Footer != .5 {
			t.Fatal("source inch margins changed", margins, err)
		}
		header, err := book.GetHeaderFooter(sheet.Name)
		if err != nil || header.OddHeader != "&RPage &P of &N" || header.OddFooter != "&R&D" {
			t.Fatal("source page-count/date header/footer changed", header, err)
		}
	}
}

func TestLongEnvironmentWorkbookCancelledRequestHasNoBytes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, request := range []context.Context{nil, ctx} {
		result, err := prepareLongEnvironmentWorkbook(request, environmentWorkbookFixture(t))
		if err == nil || request != nil && !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, environmentWorkbook{}) {
			t.Fatal("invalid request published", result.Sheets, err)
		}
	}
}
