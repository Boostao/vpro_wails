package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSummaryVegetationWorkbookNullableLiteralOrdering(t *testing.T) {
	input := summaryVegetationWorkbookFixture()
	template := input.Vegetation.Rows[0]
	input.Vegetation.Rows = nil
	for _, name := range []ProjectMetadataCell{{Storage: "null"}, metadataText(""), metadataText("\n"), metadataText(" "), metadataText("\"")} {
		row := template
		row.Layer, row.Species = ProjectMetadataCell{Storage: "null"}, name
		input.Vegetation.Rows = append(input.Vegetation.Rows, row)
	}
	result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
	if err != nil {
		t.Fatal("nullable/raw text order must not use JSON-escaped sort keys", err)
	}
	book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
	for i, want := range []string{"", "", "\n", " ", "\""} {
		address := fmt.Sprintf("B%d", i+6)
		value, err := book.GetCellValue(result.Sheet, address, excelize.Options{RawCellValue: true})
		if err != nil || value != want {
			t.Fatal("literal name order changed", address, value, err)
		}
		if i == 1 {
			kind, err := book.GetCellType(result.Sheet, address)
			if err != nil || kind != excelize.CellTypeSharedString {
				t.Fatal("empty scientific name became NULL", kind, err)
			}
		}
	}
}

func TestSummaryVegetationWorkbookExistingOptionTransport(t *testing.T) {
	for _, average := range []int{1, 2} {
		for _, display := range []int{1, 2, 3, 4, 5, 6} {
			classes := []int{0}
			if display == 4 || display == 5 {
				classes = []int{1, 10, 20}
			}
			for _, class := range classes {
				t.Run(fmt.Sprintf("%d/%d/%d", average, display, class), func(t *testing.T) {
					input := summaryVegetationWorkbookFixture()
					input.Vegetation.Average, input.Vegetation.Display, input.Vegetation.ShowClass = average, display, class
					result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
					if err != nil {
						t.Fatal("qualified option shape cannot transport existing literal outputs", err)
					}
					book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
					source, _ := json.Marshal(input)
					if !bytes.Equal(source, summaryEnvironmentalWorkbookSource(t, book)) {
						t.Fatal("explicit calculation options were replaced")
					}
				})
			}
		}
	}
}

func TestSummaryVegetationWorkbookEnvironmentalMismatchAndRowOverflow(t *testing.T) {
	for _, edit := range []func(*summaryEnvironmentalWorkbookInput){
		func(p *summaryEnvironmentalWorkbookInput) { p.FirstRow = 4 },
		func(p *summaryEnvironmentalWorkbookInput) { p.English = true },
		func(p *summaryEnvironmentalWorkbookInput) { p.Units[0].NumPlots++ },
		func(p *summaryEnvironmentalWorkbookInput) { p.Rows = p.Rows[:1] },
	} {
		input := summaryVegetationWorkbookFixture()
		environment := summaryEnvironmentalWorkbookFixture(t)
		environment.Units = summaryEnvironmentalPreviewUnits(input.Vegetation)
		edit(&environment)
		input.Environment = &environment
		result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
		if err == nil || !reflect.DeepEqual(result, summaryVegetationWorkbook{}) {
			t.Fatal("mismatched environmental scope returned partial output", err)
		}
	}
	input := summaryVegetationWorkbookFixture()
	input.Vegetation.Rows = make([]SummaryVegetationRow, 32763)
	if result, err := prepareSummaryVegetationWorkbook(t.Context(), input); err == nil || len(result.Bytes) != 0 {
		t.Fatal("source Integer row overflow returned output", err)
	}
}

func TestSummaryVegetationWorkbookPostSerializationCancellation(t *testing.T) {
	input := summaryVegetationWorkbookFixture()
	observed := &summaryHeaderCancellingContext{Context: context.Background(), remaining: 1000}
	if result, err := prepareSummaryVegetationWorkbook(observed, input); err != nil || len(result.Bytes) == 0 {
		t.Fatal("uncancelled checkpoint calibration failed", err)
	}
	checks := 1000 - observed.remaining
	for _, at := range []int{checks - 1, checks} {
		ctx := &summaryHeaderCancellingContext{Context: context.Background(), remaining: at}
		result, err := prepareSummaryVegetationWorkbook(ctx, input)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationWorkbook{}) {
			t.Fatal("post-serialization/cleanup cancellation leaked a successful result", at, err)
		}
	}
}

func TestSummaryWorkbookRendererRejectsInvalidPlacement(t *testing.T) {
	text := metadataText("literal")
	for _, test := range []struct {
		name string
		edit func(*summaryWorkbookLayout)
	}{
		{"duplicate", func(p *summaryWorkbookLayout) { p.Cells = append(p.Cells, p.Cells[0]) }},
		{"outside", func(p *summaryWorkbookLayout) { p.Cells[0].Row = 4 }},
		{"unknown-tag", func(p *summaryWorkbookLayout) { p.Cells[0].Value.Kind = "other" }},
		{"missing-cell", func(p *summaryWorkbookLayout) { p.Cells[0].Value.Cell = nil }},
		{"empty-payload", func(p *summaryWorkbookLayout) { p.Cells[0].Value.Kind = "empty" }},
		{"visible-real", func(p *summaryWorkbookLayout) {
			value := 1.5
			p.Cells[0].Value.Cell = &ProjectMetadataCell{Storage: "real", Real: &value}
		}},
		{"row-overflow", func(p *summaryWorkbookLayout) { p.LastRow = 32768 }},
		{"column-overflow", func(p *summaryWorkbookLayout) { p.LastColumn = 255 }},
		{"separator", func(p *summaryWorkbookLayout) { p.SeparatorRow = 4 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout := summaryWorkbookLayout{Sheet: "Summary", LastRow: 3, LastColumn: 4,
				Cells: []summaryEnvironmentalHeaderCell{{Row: 1, Column: 1, Value: SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &text}}}}
			test.edit(&layout)
			result, err := renderSummaryWorkbook(t.Context(), "source", layout)
			if err == nil || result != nil {
				t.Fatal("invalid internal placement returned partial bytes", err)
			}
		})
	}
}
