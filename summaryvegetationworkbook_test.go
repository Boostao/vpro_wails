package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryVegetationWorkbookFixture() summaryVegetationWorkbookInput {
	null := ProjectMetadataCell{Storage: "null"}
	preview := SummaryVegetationPreview{
		ContextID: "owned", Project: "Project", ProjectPath: `C:\fixture\project.sqlite`, SU: "Unit",
		SUPath: `C:\fixture\units.sqlite`, Average: 1, Display: 6,
		Source: "literal-normal-su-layer", Ordering: "literal-null-first-layer-species",
		Memberships: []VegetationReportMembership{{RowID: "1", PlotNumber: metadataText("P1"), SiteUnit: metadataText("A")}},
		Units: []SummaryVegetationUnit{
			{Code: "A", NumPlots: 4, MembershipIDs: []string{"1", "2", "3", "4"}, RawVegRowIDs: []string{"1"}},
			{Code: "B", NumPlots: 1, MembershipIDs: []string{"5"}, RawVegRowIDs: []string{"2"}},
		},
		Columns: []string{"A", "B"},
		Rows: []SummaryVegetationRow{
			{Layer: metadataText("1"), Species: metadataText("Alpha"), Order: metadataText("0.01"),
				Covers:    []ProjectMetadataCell{metadataText("0.01"), null},
				Witnesses: []SummaryVegetationWitness{{StageRowID: "1", MembershipID: "1", Unit: "A", Cover: null}}},
			{Layer: metadataText("2"), Species: metadataText("Beta"), Order: metadataText("1 - 2 - 3"),
				Covers:    []ProjectMetadataCell{metadataText("1 - 2 - 3"), metadataText("")},
				Witnesses: []SummaryVegetationWitness{{StageRowID: "2", MembershipID: "5", Unit: "B", Cover: null}}},
		},
	}
	return summaryVegetationWorkbookInput{Title: "=literal title", Vegetation: preview}
}

func TestSummaryVegetationWorkbookOriginalPlacementsAndLiteralTypes(t *testing.T) {
	for _, include := range []bool{false, true} {
		t.Run(map[bool]string{false: "vegetation", true: "combined"}[include], func(t *testing.T) {
			input := summaryVegetationWorkbookFixture()
			first, caption := 6, "Vegetation Table"
			if include {
				environment := summaryEnvironmentalWorkbookFixture(t)
				environment.Units = summaryEnvironmentalPreviewUnits(input.Vegetation)
				input.Environment = &environment
				first, caption = 10, "Site Unit Summary Table"
			}
			before, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err != nil || result.FirstVegetationRow != first || result.Sheet != "Project" {
				t.Fatal("source body placement unavailable", result.FirstVegetationRow, err)
			}
			book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
			expected := map[string]string{"A1": "=literal title", "A2": caption}
			if include {
				for cell, value := range map[string]string{"D4": "A", "E4": "B", "B5": "Null order", "D6": "0007",
					"C7": "n Plots", "D7": "0", "A9": "StartVeg", "B10": "Alpha", "D10": "0.01", "D11": "1 - 2 - 3"} {
					expected[cell] = value
				}
			} else {
				for cell, value := range map[string]string{"C4": "n Plots", "D4": "4", "E4": "1", "A5": "Layer",
					"B5": "Spp", "C5": "Order", "D5": "A", "E5": "B", "B6": "Alpha", "D6": "0.01", "D7": "1 - 2 - 3"} {
					expected[cell] = value
				}
			}
			for address, want := range expected {
				actual, err := book.GetCellValue(result.Sheet, address, excelize.Options{RawCellValue: true})
				formula, formulaErr := book.GetCellFormula(result.Sheet, address)
				if err != nil || formulaErr != nil || actual != want || formula != "" {
					t.Fatal("independent original placement/literal value differs", address, actual, want, err, formulaErr)
				}
			}
			raw := summaryEnvironmentalWorkbookSource(t, book)
			if !bytes.Equal(raw, before) {
				t.Fatal("complete combined typed provenance differs")
			}
			after, _ := json.Marshal(input)
			again, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err != nil || !bytes.Equal(before, after) || !bytes.Equal(again.Bytes, result.Bytes) {
				t.Fatal("input mutated or output nondeterministic", err)
			}
		})
	}
}

func TestSummaryVegetationWorkbookMixedOwnedWire(t *testing.T) {
	var preview SummaryVegetationEnvironmentalLifeformPreview
	if err := json.Unmarshal([]byte(summaryEnvironmentalHeaderMixedWire), &preview); err != nil {
		t.Fatal(err)
	}
	input := summaryVegetationWorkbookInput{
		Title: "Summary", Vegetation: preview.Vegetation,
		Environment: &summaryEnvironmentalWorkbookInput{FirstRow: 5, FieldList: preview.FieldList,
			Units: summaryEnvironmentalPreviewUnits(preview.Vegetation), Rows: preview.Rows},
	}
	result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
	if err != nil || result.FirstVegetationRow != 36 {
		t.Fatal("existing thirty-row strict-Go owned preview cannot compose", err)
	}
	book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
	before, _ := json.Marshal(input)
	if !bytes.Equal(before, summaryEnvironmentalWorkbookSource(t, book)) {
		t.Fatal("full mixed source witness was reduced")
	}
}

func TestSummaryVegetationWorkbookRefusalsAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*summaryVegetationWorkbookInput)
	}{
		{"pivot-subset", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Columns = p.Vegetation.Columns[:1] }},
		{"pivot-order", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Columns = []string{"B", "A"} }},
		{"row-order", func(p *summaryVegetationWorkbookInput) {
			p.Vegetation.Rows[0], p.Vegetation.Rows[1] = p.Vegetation.Rows[1], p.Vegetation.Rows[0]
		}},
		{"duplicate-group", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[1] = p.Vegetation.Rows[0] }},
		{"partial-cover", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[0].Covers = nil }},
		{"missing-witness", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Rows[0].Witnesses = nil }},
		{"scalar-coercion", func(p *summaryVegetationWorkbookInput) {
			value := "1"
			p.Vegetation.Rows[0].Covers[0] = ProjectMetadataCell{Storage: "integer", Integer: &value}
		}},
		{"grouped", func(p *summaryVegetationWorkbookInput) { p.Vegetation.GroupAndOrder = 12 }},
		{"unknown-options", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Display = 7 }},
		{"sheet-alteration", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Project = "not/a/sheet" }},
		{"reserved-sheet", func(p *summaryVegetationWorkbookInput) { p.Vegetation.Project = "_VPRO_Source" }},
		{"broken-unicode", func(p *summaryVegetationWorkbookInput) { p.Title = string([]byte{0xff}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := summaryVegetationWorkbookFixture()
			test.edit(&input)
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err == nil || !reflect.DeepEqual(result, summaryVegetationWorkbook{}) {
				t.Fatal("invalid source returned success or partial bytes", err)
			}
		})
	}
	for _, ctx := range []context.Context{nil, &summaryHeaderCancellingContext{Context: context.Background(), remaining: 10}} {
		result, err := prepareSummaryVegetationWorkbook(ctx, summaryVegetationWorkbookFixture())
		if err == nil || !reflect.DeepEqual(result, summaryVegetationWorkbook{}) || ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("missing/cancelled context returned partial bytes or lost cancellation", err)
		}
	}
}
