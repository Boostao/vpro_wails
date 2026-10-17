package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

type summaryGroupedWorkbookSourceCase struct {
	Grouping       int             `json:"grouping"`
	Average        int             `json:"average"`
	Display        int             `json:"display"`
	ShowClass      int             `json:"showClass"`
	AverageIgnored bool            `json:"averageIgnored"`
	SourceError    json.RawMessage `json:"sourceError"`
	Fields         []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		Size int    `json:"size"`
	} `json:"expectedFields"`
	Rows [][]any `json:"expectedRows"`
}

func summaryGroupedWorkbookSourceInput(t *testing.T, source summaryGroupedWorkbookSourceCase) summaryVegetationWorkbookInput {
	t.Helper()
	input := summaryVegetationWorkbookFixture()
	preview := &input.Vegetation
	preview.Average, preview.Display, preview.ShowClass = source.Average, source.Display, source.ShowClass
	preview.Source, preview.Ordering = "literal-normal-su-grouped", "literal-null-first-group-species"
	preview.GroupAndOrder = source.Grouping
	preview.GroupField = &SummaryVegetationField{source.Fields[0].Name, source.Fields[0].Type, source.Fields[0].Size}
	preview.OrderField = &SummaryVegetationField{source.Fields[2].Name, source.Fields[2].Type, source.Fields[2].Size}
	preview.Rows = nil
	for _, values := range source.Rows {
		cells := make([]ProjectMetadataCell, len(values))
		for i, value := range values {
			switch value := value.(type) {
			case nil:
				cells[i] = ProjectMetadataCell{Storage: "null"}
			case string:
				cells[i] = metadataText(value)
			case float64:
				cells[i] = strataReal(value)
			default:
				t.Fatal("independent source scalar unavailable", value)
			}
		}
		preview.Rows = append(preview.Rows, SummaryVegetationRow{
			Layer: cells[0], Species: cells[1], Order: cells[2], Covers: cells[3:],
			Witnesses: []SummaryVegetationWitness{{StageRowID: "1", MembershipID: "1", Unit: "A",
				Cover: ProjectMetadataCell{Storage: "null"}}},
		})
	}
	return input
}

func assertSummaryGroupedWorkbookCells(t *testing.T, input summaryVegetationWorkbookInput, result summaryVegetationWorkbook) {
	t.Helper()
	book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
	archive, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
	if err != nil {
		t.Fatal(err)
	}
	var worksheet struct {
		Cells []struct {
			Address string `xml:"r,attr"`
			Type    string `xml:"t,attr"`
			Value   string `xml:"v"`
			Formula string `xml:"f"`
		} `xml:"sheetData>row>c"`
	}
	found := false
	for _, file := range archive.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		if err := xml.Unmarshal(data, &worksheet); err != nil {
			t.Fatal(err)
		}
		found = true
	}
	if !found {
		t.Fatal("required worksheet XML absent")
	}
	rawTypes := map[string]string{}
	rawValues := map[string]string{}
	for _, cell := range worksheet.Cells {
		if cell.Formula != "" {
			t.Fatal("typed inspection produced a formula", cell.Address)
		}
		rawTypes[cell.Address], rawValues[cell.Address] = cell.Type, cell.Value
	}
	preview := input.Vegetation
	if input.Environment == nil {
		label, err := book.GetCellValue(result.Sheet, "A5")
		if err != nil || label != preview.GroupField.Name {
			t.Fatal("original group field caption lost", label, err)
		}
	} else {
		value, err := book.GetCellValue(result.Sheet, fmt.Sprintf("A%d", result.FirstVegetationRow-1))
		if err != nil || value != "StartVeg" {
			t.Fatal("combined separator moved", value, err)
		}
	}
	for i, row := range preview.Rows {
		for j, cell := range append([]ProjectMetadataCell{row.Layer, row.Species, row.Order}, row.Covers...) {
			address, err := excelize.CoordinatesToCellName(j+1, result.FirstVegetationRow+i)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := book.GetCellValue(result.Sheet, address, excelize.Options{RawCellValue: true})
			if err != nil {
				t.Fatal(err)
			}
			switch cell.Storage {
			case "real":
				number, err := strconv.ParseFloat(rawValues[address], 64)
				if err != nil || math.Float64bits(number) != math.Float64bits(*cell.Real) ||
					rawTypes[address] != "" && rawTypes[address] != "n" {
					t.Fatal("original DOUBLE Order changed or became text", address, rawValues[address], rawTypes[address], err)
				}
			case "text":
				if actual != *cell.Text || rawTypes[address] != "s" && rawTypes[address] != "inlineStr" {
					t.Fatal("original TEXT changed or became numeric", address, actual, *cell.Text, rawTypes[address])
				}
			case "null":
				if actual != "" {
					t.Fatal("NULL became a visible scalar", address, actual)
				}
			default:
				t.Fatal("unexpected source output", cell)
			}
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil || !bytes.Equal(encoded, summaryEnvironmentalWorkbookSource(t, book)) {
		t.Fatal("complete grouped input/provenance differs", err)
	}
}

func TestSummaryGroupedWorkbookIndependentSourceRows(t *testing.T) {
	accepted, refused := 0, 0
	for _, filename := range []string{"summaryVegetationGroupSource.json", "summaryVegetationGroupAdditionalSource.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", filename))
		if err != nil {
			t.Fatal(err)
		}
		var fixture struct {
			Base       []summaryGroupedWorkbookSourceCase `json:"stagedQueryCases"`
			Additional []summaryGroupedWorkbookSourceCase `json:"cases"`
		}
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatal(err)
		}
		for index, source := range append(fixture.Base, fixture.Additional...) {
			t.Run(fmt.Sprintf("%s/%d", filename, index), func(t *testing.T) {
				if len(source.SourceError) != 0 && string(source.SourceError) != "null" {
					input := summaryVegetationWorkbookFixture()
					input.Vegetation.Source, input.Vegetation.GroupAndOrder = "literal-normal-su-grouped", source.Grouping
					input.Vegetation.Display = source.Display
					result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
					if err == nil || !strings.Contains(err.Error(), "CDbl composite criteria type mismatch") ||
						!reflect.DeepEqual(result, summaryVegetationWorkbook{}) {
						t.Fatal("original grouped criteria failure silently repaired", err)
					}
					refused++
					return
				}
				averages := []int{source.Average}
				if source.AverageIgnored {
					averages = []int{1, 2}
				}
				for _, average := range averages {
					input := summaryGroupedWorkbookSourceInput(t, source)
					input.Vegetation.Average = average
					for _, combined := range []bool{false, true} {
						if combined {
							env := summaryEnvironmentalWorkbookFixture(t)
							env.Units = summaryEnvironmentalPreviewUnits(input.Vegetation)
							input.Environment = &env
						}
						result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
						if err != nil {
							t.Fatal("independent original rows cannot render", average, combined, err)
						}
						assertSummaryGroupedWorkbookCells(t, input, result)
						accepted++
					}
				}
			})
		}
	}
	if accepted != 112 || refused != 4 {
		t.Fatal("source matrix coverage changed", accepted, refused)
	}
}

func TestSummaryGroupedWorkbookOwnedModesNoSourceWrites(t *testing.T) {
	count := 0
	for _, external := range []bool{false, true} {
		service, state := summaryVegetationGroupedServiceFixture(t, external)
		contexts := service.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, grouping := range []int{12, 13, 14} {
			for _, average := range []int{1, 2} {
				for _, display := range []int{1, 2, 3, 4, 5, 6} {
					if display == 2 && grouping != 14 {
						continue
					}
					shows := []int{0}
					if display == 4 || display == 5 {
						shows = []int{1, 10, 20}
					}
					for _, show := range shows {
						preview, err := service.PreviewGroupedDisplay(t.Context(), state.ContextID, average, grouping, display, show)
						if err != nil {
							t.Fatal(external, grouping, average, display, show, err)
						}
						for _, combined := range []bool{false, true} {
							input := summaryVegetationWorkbookInput{Title: "=literal _x0041_", Vegetation: preview}
							if combined {
								env := summaryEnvironmentalWorkbookFixture(t)
								env.Units = summaryEnvironmentalPreviewUnits(preview)
								input.Environment = &env
							}
							original, err := json.Marshal(input)
							if err != nil {
								t.Fatal(err)
							}
							result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
							if err != nil {
								t.Fatal(external, grouping, average, display, show, combined, err)
							}
							assertSummaryGroupedWorkbookCells(t, input, result)
							after, _ := json.Marshal(input)
							if !bytes.Equal(original, after) {
								t.Fatal("owned input mutated")
							}
							count++
						}
					}
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("workbook changed saved preferences", err)
		}
	}
	if count != 224 {
		t.Fatal("owned grouped modes coverage changed", count)
	}
}

func summaryGroupedWorkbookNumericInput(t *testing.T) summaryVegetationWorkbookInput {
	t.Helper()
	input := summaryVegetationWorkbookFixture()
	input.Vegetation.Source, input.Vegetation.Ordering = "literal-normal-su-grouped", "literal-null-first-group-species"
	input.Vegetation.GroupAndOrder = 13
	input.Vegetation.GroupField = &SummaryVegetationField{Name: "LifeForm", DAOType: 10, Size: 3}
	input.Vegetation.OrderField = &SummaryVegetationField{Name: "Order", DAOType: 7, Size: 8}
	for i := range input.Vegetation.Rows {
		input.Vegetation.Rows[i].Order = strataReal(float64(i) + 0.01)
	}
	return input
}

func TestSummaryGroupedWorkbookRefusalsAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*summaryVegetationWorkbookInput)
	}{
		{"group-metadata", func(p *summaryVegetationWorkbookInput) { p.Vegetation.GroupField.Size = 255 }},
		{"order-metadata", func(p *summaryVegetationWorkbookInput) { p.Vegetation.OrderField.DAOType = 10 }},
		{"missing-group", func(p *summaryVegetationWorkbookInput) { p.Vegetation.GroupField = nil }},
		{"missing-order", func(p *summaryVegetationWorkbookInput) { p.Vegetation.OrderField = nil }},
		{"unknown-group", func(p *summaryVegetationWorkbookInput) { p.Vegetation.GroupAndOrder = 15 }},
		{"ordinary-source", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Source = "literal-normal-su-layer" }},
		{"unknown-ordering", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Ordering = "invented" }},
		{"effective-references", func(p *summaryVegetationWorkbookInput) { p.Vegetation.References = &SummaryVegetationReferenceReview{} }},
		{"text-order", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[0].Order = metadataText("0.01") }},
		{"null-order", func(p *summaryVegetationWorkbookInput) {
			p.Vegetation.Rows[0].Order = ProjectMetadataCell{Storage: "null"}
		}},
		{"nonfinite-order", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[0].Order = strataReal(math.Inf(1)) }},
		{"real-cover", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[0].Covers[0] = strataReal(0.01) }},
		{"wide-group", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[0].Layer = metadataText("1234") }},
		{"wide-strata", func(p *summaryVegetationWorkbookInput) {
			p.Vegetation.GroupAndOrder = 12
			p.Vegetation.GroupField = &SummaryVegetationField{Name: "Layer", DAOType: 10, Size: 1}
			p.Vegetation.Rows[0].Layer = metadataText("\U0001f603")
		}},
		{"nonnull-none", func(p *summaryVegetationWorkbookInput) {
			p.Vegetation.GroupAndOrder = 14
			p.Vegetation.GroupField = &SummaryVegetationField{Name: "Group", DAOType: 9}
			p.Vegetation.Rows[0].Layer = metadataText("")
		}},
		{"class-order-real", func(p *summaryVegetationWorkbookInput) {
			p.Vegetation.Display, p.Vegetation.ShowClass = 4, 10
			p.Vegetation.OrderField = &SummaryVegetationField{Name: "Order", DAOType: 10}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := summaryGroupedWorkbookNumericInput(t)
			test.edit(&input)
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err == nil || !reflect.DeepEqual(result, summaryVegetationWorkbook{}) {
				t.Fatal("invalid grouped preview returned bytes", err)
			}
		})
	}
	input := summaryGroupedWorkbookNumericInput(t)
	probe := &projectUnitCancellationContext{Context: context.Background()}
	first, err := prepareSummaryVegetationWorkbook(probe, input)
	again, repeatErr := prepareSummaryVegetationWorkbook(t.Context(), input)
	if err != nil || repeatErr != nil || !bytes.Equal(first.Bytes, again.Bytes) {
		t.Fatal("grouped output nondeterministic", err, repeatErr)
	}
	for _, point := range []int{1, 5, 20, probe.calls / 2, probe.calls - 2, probe.calls - 1, probe.calls} {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		result, err := prepareSummaryVegetationWorkbook(ctx, input)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationWorkbook{}) {
			t.Fatal("cancelled grouped workbook leaked bytes", point, probe.calls, err)
		}
	}
}

func TestSummaryGroupedWorkbookRealOrderRangeDoesNotWidenLegacyRendering(t *testing.T) {
	for _, test := range []struct {
		first, orderColumn, row, column int
		accepted                        bool
	}{
		{0, 0, 6, 3, false}, {-1, 3, 6, 3, false}, {7, 3, 6, 3, false},
		{6, 3, 5, 3, false}, {6, 3, 6, 2, false}, {6, 3, 6, 4, false}, {6, 3, 6, 3, true},
		{6, 4, 6, 4, true}, {6, 4, 6, 3, false}, {6, 5, 6, 4, false},
		{6, 0, 6, 3, false}, {0, 3, 6, 3, false}, {6, 2, 6, 2, false},
	} {
		cell := strataReal(math.Nextafter(0.15, 0))
		layout := summaryWorkbookLayout{Sheet: "Typed", LastRow: 6, LastColumn: 4,
			RealOrderFromRow: test.first, RealOrderColumn: test.orderColumn,
			Cells: []summaryEnvironmentalHeaderCell{{Row: test.row, Column: test.column,
				Value: SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}}}}
		result, err := renderSummaryWorkbook(t.Context(), cell, layout)
		if test.accepted {
			if err != nil || len(result) == 0 {
				t.Fatal("explicit finite real Order refused", err)
			}
		} else if err == nil || len(result) != 0 {
			t.Fatal("real Order permission escaped its explicit range", test, err)
		}
		for _, number := range []float64{math.Nextafter(0.15, 0), 0.1, math.Copysign(0, -1)} {
			input := summaryGroupedWorkbookNumericInput(t)
			input.Vegetation.Rows[0].Order = strataReal(number)
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err != nil {
				t.Fatal("finite binary64 transport refused", number, err)
			}
			assertSummaryGroupedWorkbookCells(t, input, result)
		}
	}
}
