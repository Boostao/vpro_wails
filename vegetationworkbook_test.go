package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func vegetationWorkbookFixture(t *testing.T) LongVegetationPreview {
	t.Helper()
	service, state := reportServiceFixture(t, false)
	preview, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return preview
}

func TestLongVegetationWorkbookSourceColumnsTypedValuesAndDeterminism(t *testing.T) {
	preview := vegetationWorkbookFixture(t)
	if len(preview.Report.Units[0].Rows) == 0 {
		t.Fatal("owned source fixture has no rows")
	}
	preview.Settings.ShowEnglishName, preview.Settings.ShowSpeciesCode = false, false
	preview.Report.Units = preview.Report.Units[:1]
	presence, mean, plotCover := .5, 1.25, 2.5
	row := preview.Report.Units[0].Rows[0]
	row.Layer, row.Species = metadataText("1"), metadataText("=1+1")
	row.Presence, row.MeanCover = &presence, &mean
	row.Plots = []LongVegetationPlot{{PlotNumber: "P2", Cover: &plotCover}, {PlotNumber: "P1"}}
	second := row
	second.Layer = metadataText("2")
	preview.Report.Units[0].Rows = []LongVegetationRow{row, second}
	layout := vegetationWorkbookLayout{SpaceBetweenGroups: true}
	first, err := prepareLongVegetationWorkbook(context.Background(), preview, layout)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		next, err := prepareLongVegetationWorkbook(context.Background(), preview, layout)
		if err != nil || !bytes.Equal(first.Bytes, next.Bytes) {
			t.Fatal("identical preview produces clock/order-dependent workbook bytes", err)
		}
	}
	book, err := excelize.OpenReader(bytes.NewReader(first.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	name := first.Sheets[0].Name
	for address, expected := range map[string]string{"A1": preview.Report.Title, "A3": " ", "A4": "Layer", "B4": "Spp", "B5": "=1+1"} {
		actual, err := book.GetCellValue(name, address)
		if err != nil || actual != expected {
			t.Fatal("source heading/row placement/literal species changed", address, actual, expected, err)
		}
	}
	formula, err := book.GetCellFormula(name, "B5")
	if err != nil || formula != "" {
		t.Fatal("formula-shaped species became executable", formula, err)
	}
	for address, expected := range map[string]string{
		"C4": "P", "D4": "MC", "E4": "P1", "F4": "P2",
		"C5": "0.5", "D5": "1.25", "E5": "", "F5": "2.5", "A6": "2", "A7": "",
	} {
		actual, err := book.GetCellValue(name, address, excelize.Options{RawCellValue: true})
		if err != nil || actual != expected {
			t.Fatal("numeric precision/NULL/source plot ordering/group gap changed", address, actual, expected, err)
		}
	}
	for _, sheet := range first.Sheets {
		page, err := book.GetPageLayout(sheet.Name)
		if err != nil || page.Orientation == nil || *page.Orientation != "landscape" {
			t.Fatal("source non-quick landscape print setup lost", page, err)
		}
		margins, err := book.GetPageMargins(sheet.Name)
		if err != nil || margins.Left == nil || *margins.Left != .75 || margins.Top == nil || *margins.Top != .5 ||
			margins.Right == nil || *margins.Right != .75 || margins.Bottom == nil || *margins.Bottom != .75 {
			t.Fatal("source non-quick print margins changed", margins, err)
		}
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
	data, err := hex.DecodeString(chunks.String())
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Preview LongVegetationPreview
		Layout  vegetationWorkbookLayout
	}
	if err := json.Unmarshal(data, &source); err != nil || !reflect.DeepEqual(source.Preview, preview) || !reflect.DeepEqual(source.Layout, layout) {
		t.Fatal("lossless typed owned report/settings/layout changed", err)
	}
}

func TestLongVegetationWorkbookSourceGapScanStartsAtRowSevenAndStopsAtBlank(t *testing.T) {
	for _, longScan := range []bool{false, true} {
		preview := vegetationWorkbookFixture(t)
		preview.Report.Units = preview.Report.Units[:1]
		base := preview.Report.Units[0].Rows[0]
		rows := []LongVegetationRow{}
		groups := []string{"1", "2", "3", "", "4"}
		if longScan {
			groups = make([]string, 1005)
			for i := range groups {
				groups[i] = strconv.Itoa(i)
			}
		}
		for _, group := range groups {
			row := base
			row.Layer = metadataText(group)
			rows = append(rows, row)
		}
		preview.Report.Units[0].Rows = rows
		result, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{SpaceBetweenGroups: true})
		if err != nil {
			t.Fatal(err)
		}
		book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
		if err != nil {
			t.Fatal(err)
		}
		expected := map[string]string{"A5": "1", "A6": "2", "A7": "", "A8": "3", "A9": "", "A10": "4"}
		if longScan {
			expected = map[string]string{"A5": "0", "A6": "1", "A7": "", "A8": "2", "A2006": "1001", "A2007": "1002"}
		}
		for address, value := range expected {
			actual, err := book.GetCellValue(result.Sheets[0].Name, address)
			if err != nil || actual != value {
				t.Fatal("source row7/blank/1000-iteration gap scan changed", address, actual, value, err)
			}
		}
		if err := book.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLongVegetationWorkbookOptionalColumnsAndOriginalUnitNaming(t *testing.T) {
	for _, grouping := range []string{"layer", "none", "lifeform", "strata"} {
		for _, mode := range []string{"none", "english", "code"} {
			preview := vegetationWorkbookFixture(t)
			preview.Settings.Grouping = grouping
			preview.Settings.ShowEnglishName, preview.Settings.ShowSpeciesCode = mode == "english", mode == "code"
			result, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{QuickReport: true})
			if err != nil {
				t.Fatal(grouping, mode, err)
			}
			book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
			if err != nil {
				t.Fatal(err)
			}
			expected := []string{"Layer", "Spp", "P", "MC"}
			if grouping == "lifeform" {
				expected[0] = "Lifeform"
			} else if grouping == "strata" {
				expected[0] = "Strata"
			}
			if mode != "none" {
				label := "English Name"
				if mode == "code" {
					label = "Code"
				}
				expected = append(expected[:2], append([]string{label}, expected[2:]...)...)
			}
			for column, label := range expected {
				address, _ := excelize.CoordinatesToCellName(column+1, 4)
				actual, err := book.GetCellValue(result.Sheets[0].Name, address)
				if err != nil || actual != label {
					t.Fatal("source grouping/optional field/P/MC columns changed", grouping, mode, address, actual, label, err)
				}
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	preview := vegetationWorkbookFixture(t)
	preview.Report.Units[0].Code = metadataText("")
	preview.Report.Units[1].Code = ProjectMetadataCell{Storage: "null"}
	result, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
	if err != nil || result.Sheets[0].Name != "NoName0" || result.Sheets[1].Name != "Unassigned" ||
		result.Sheets[0].Unit.Storage != "text" || result.Sheets[1].Unit.Storage != "null" {
		t.Fatal("empty and NULL unit identity/naming collapsed", result.Sheets, err)
	}
}

func TestLongVegetationWorkbookExplicitSourceLimitsRefusalsAndCancellation(t *testing.T) {
	for _, mutate := range []func(*LongVegetationPreview){
		func(p *LongVegetationPreview) { p.ContextID = "" },
		func(p *LongVegetationPreview) { p.Settings.Title += "changed" },
		func(p *LongVegetationPreview) { p.Settings.Grouping = "unknown" },
		func(p *LongVegetationPreview) { p.Settings.Average = "unknown" },
		func(p *LongVegetationPreview) { p.Settings.Order = "unknown" },
		func(p *LongVegetationPreview) { p.Settings.ShowEnglishName, p.Settings.ShowSpeciesCode = true, true },
		func(p *LongVegetationPreview) { p.Report.Units[0].NumPlots = 251 },
		func(p *LongVegetationPreview) { p.Report.Units[0].NumPlots = -1 },
		func(p *LongVegetationPreview) { p.Report.Units[0].Rows = nil },
		func(p *LongVegetationPreview) { p.Report.Units[1].Code = p.Report.Units[0].Code },
		func(p *LongVegetationPreview) {
			p.Report.Units[0].Code = ProjectMetadataCell{Storage: "null"}
			p.Report.Units[1].Code = metadataText("Unassigned")
		},
		func(p *LongVegetationPreview) { p.Report.Units[0].Code = metadataText("_VPRO_Source") },
		func(p *LongVegetationPreview) { p.Report.Units[0].NameStatus = "conflicting" },
		func(p *LongVegetationPreview) { p.Report.Units[0].Rows[0].Species = metadataText("bad\x00") },
		func(p *LongVegetationPreview) { p.Report.Units[0].Rows[0].Species = metadataInteger("1") },
		func(p *LongVegetationPreview) {
			value := math.Inf(1)
			p.Report.Units[0].Rows[0].Presence = &value
		},
	} {
		preview := vegetationWorkbookFixture(t)
		mutate(&preview)
		result, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
		if err == nil || !reflect.DeepEqual(result, vegetationWorkbook{}) {
			t.Fatal("invalid workbook returned successful bytes/mapping", err, result.Sheets)
		}
	}
	preview := vegetationWorkbookFixture(t)
	if _, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{ReportSummary: true}); err == nil {
		t.Fatal("requested unsupported summary silently omitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := prepareLongVegetationWorkbook(ctx, preview, vegetationWorkbookLayout{})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, vegetationWorkbook{}) {
		t.Fatal("cancelled workbook returned bytes", err)
	}
	preview.Report.Units[0].NumPlots = 0
	result, err = prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
	if err != nil || len(result.SkippedUnits) != 1 || len(result.Sheets) != 1 {
		t.Fatal("source zero-count unit omission loses explicit typed evidence", result.SkippedUnits, err)
	}
}
