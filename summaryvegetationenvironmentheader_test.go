package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func summaryEnvironmentalHeaderFixture(t *testing.T, selected bool) (ProjectMetadataTable, []summaryVegetationUnit, []SummaryVegetationEnvironmentalNumericRow) {
	t.Helper()
	report := int64(0)
	if selected {
		report = -1
	}
	fields := summaryEnvironmentalConsumerFields(t, [][]any{
		{nil, "Env", "Null order", nil, report, "FixtureValue", nil},
		{int64(-1), "Env", "Negative order", nil, report, "FixtureValue", "number"},
		{int64(0), "Env", "n Plots", nil, report, "FixtureValue", "zero"},
		{int64(1), "Env", "N Plots", nil, report, "FixtureValue", ""},
		{int64(2), "Env", "Hidden false", nil, int64(0), "FixtureValue", nil},
		{int64(3), "Env", "Hidden null", nil, nil, "FixtureValue", nil},
	})
	rows := []SummaryVegetationEnvironmentalNumericRow{}
	if selected {
		empty := ProjectMetadataCell{Storage: "null"}
		number, zero, blank := "0007", "0", ""
		values := [][]SummaryVegetationEnvironmentalValue{
			{{Kind: "empty"}, {Kind: "cell", Cell: &empty}},
			{{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "text", Text: &number}},
				{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "text", Text: &number}}},
			{{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "integer", Integer: &zero}},
				{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "integer", Integer: &zero}}},
			{{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "text", Text: &blank}},
				{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "text", Text: &blank}}},
		}
		for i := range values {
			rows = append(rows, SummaryVegetationEnvironmentalNumericRow{
				Field: cloneSummaryVegetationFieldListRow(fields.Rows[i]), LabelColumn: 2, Values: values[i],
			})
		}
	}
	return fields, []summaryVegetationUnit{{Code: "A"}, {Code: "B"}}, rows
}

func TestSummaryEnvironmentalHeaderOriginalFourLayouts(t *testing.T) {
	var source []struct {
		Selected bool    `json:"selected"`
		English  int     `json:"english"`
		Return   int     `json:"returnRow"`
		Values   [][]any `json:"values"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalHeaderOriginalFixture), &source); err != nil || len(source) != 4 {
		t.Fatal("independently reopened four source layouts required", err)
	}
	for _, original := range source {
		t.Run(fmt.Sprintf("%t/%d", original.Selected, original.English), func(t *testing.T) {
			fields, units, rows := summaryEnvironmentalHeaderFixture(t, original.Selected)
			plan, err := prepareSummaryEnvironmentalHeader(t.Context(), 4, original.English != 0, fields, units, rows)
			if err != nil {
				t.Fatal(err)
			}
			if plan.NextRow != original.Return || plan.NextRow != 6+len(rows) || plan.LastDataRow != 3+len(rows) {
				t.Fatal("original header return/last row differs", plan)
			}
			first := 4
			if original.English != 0 {
				first = 5
			}
			if plan.FirstValueColumn != first || plan.LastValueColumn != first+1 {
				t.Fatal("original English/non-English column placement differs", plan)
			}
			at := map[string]SummaryVegetationEnvironmentalValue{}
			for _, cell := range plan.Cells {
				key := fmt.Sprintf("%d/%d", cell.Row, cell.Column)
				if _, exists := at[key]; exists {
					t.Fatal("overlapping original placement", key)
				}
				at[key] = cell.Value
			}
			if *at[fmt.Sprintf("3/%d", first)].Cell.Text != "A" || *at[fmt.Sprintf("3/%d", first+1)].Cell.Text != "B" {
				t.Fatal("SV1 order changed")
			}
			if !original.Selected {
				if len(plan.Cells) != 2 || len(plan.Fields) != 0 {
					t.Fatal("unselected hidden rows emitted")
				}
				return
			}
			if *at["4/1"].Cell.Text != "Env" || *at["4/2"].Cell.Text != "Null order" ||
				*at["5/2"].Cell.Text != "Negative order" || *at["6/3"].Cell.Text != "n Plots" ||
				*at["7/3"].Cell.Text != "N Plots" {
				t.Fatal("hidden NULL/negative/zero fields or case-equivalent special label changed")
			}
			if _, exists := at["7/2"]; exists {
				t.Fatal("original case-insensitive special label also emitted ordinary caption")
			}
			if at[fmt.Sprintf("4/%d", first)].Kind != "empty" ||
				at[fmt.Sprintf("4/%d", first+1)].Cell.Storage != "null" ||
				*at[fmt.Sprintf("5/%d", first)].Cell.Text != "0007" ||
				*at[fmt.Sprintf("6/%d", first)].Cell.Integer != "0" ||
				*at[fmt.Sprintf("7/%d", first)].Cell.Text != "" {
				t.Fatal("source tags were replaced by Excel scalar coercion")
			}
			if original.Values[2][first-1] != float64(7) || original.Values[1][first-1] != nil ||
				original.Values[3][first-1] != float64(0) || original.Values[4][first-1] != nil {
				t.Fatal("independent native Excel conversion observations changed")
			}
		})
	}
}

func TestSummaryEnvironmentalHeaderMixedWireAndClones(t *testing.T) {
	var preview SummaryVegetationEnvironmentalLifeformPreview
	if err := json.Unmarshal([]byte(summaryEnvironmentalHeaderMixedWire), &preview); err != nil || len(preview.Rows) != 30 {
		t.Fatal("strict-Go full thirty-row fixture required", err)
	}
	before, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	units := summaryEnvironmentalPreviewUnits(preview.Vegetation)
	plan, err := prepareSummaryEnvironmentalHeader(t.Context(), 4, false, preview.FieldList, units, preview.Rows)
	if err != nil || plan.NextRow != 36 || plan.LastDataRow != 33 || len(plan.Fields) != 30 {
		t.Fatal("full lifeform/numeric/count/category header unavailable", plan, err)
	}
	var lifeformBlank, numericEmpty bool
	for _, cell := range plan.Cells {
		if cell.Value.Kind == "empty" {
			numericEmpty = true
		}
		if cell.Row == 9 && cell.Column == 5 && cell.Value.Cell != nil && cell.Value.Cell.Text != nil && *cell.Value.Cell.Text == "" {
			lifeformBlank = true
		}
	}
	if !lifeformBlank || !numericEmpty {
		t.Fatal("independent row9 lifeform2 String-empty versus numeric Empty lost")
	}
	*plan.Fields[0].Cells[2].Text = "caller field"
	*plan.Cells[0].Value.Cell.Text = "caller unit"
	for _, cell := range plan.Cells {
		if cell.Value.Cell != nil && cell.Value.Cell.Text != nil {
			*cell.Value.Cell.Text = "caller value"
		}
	}
	after, err := json.Marshal(preview)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("header aliases source matrix, field list or peer values", err)
	}
}

type summaryHeaderCancellingContext struct {
	context.Context
	remaining int
}

func (c *summaryHeaderCancellingContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestSummaryEnvironmentalHeaderRefusalAndBounds(t *testing.T) {
	cases := []struct {
		name string
		edit func(*ProjectMetadataTable, *[]summaryVegetationUnit, *[]SummaryVegetationEnvironmentalNumericRow)
	}{
		{"missing row", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			*rows = (*rows)[:3]
		}},
		{"reordered", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			(*rows)[0], (*rows)[1] = (*rows)[1], (*rows)[0]
		}},
		{"peer witness", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			*(*rows)[0].Field.Cells[2].Text = "changed"
		}},
		{"short values", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			(*rows)[0].Values = (*rows)[0].Values[:1]
		}},
		{"empty with cell", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			(*rows)[0].Values[0].Cell = &ProjectMetadataCell{Storage: "null"}
		}},
		{"cell without body", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			(*rows)[0].Values[1].Cell = nil
		}},
		{"unknown tag", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			(*rows)[0].Values[0].Kind = "string"
		}},
		{"malformed integer", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			*(*rows)[2].Values[0].Cell.Integer = "bad"
		}},
		{"malformed Unicode", func(_ *ProjectMetadataTable, _ *[]summaryVegetationUnit, rows *[]SummaryVegetationEnvironmentalNumericRow) {
			*(*rows)[1].Values[0].Cell.Text = string([]byte{0xff})
		}},
		{"XML control", func(_ *ProjectMetadataTable, units *[]summaryVegetationUnit, _ *[]SummaryVegetationEnvironmentalNumericRow) {
			(*units)[0].Code = "A\x00"
		}},
		{"no units", func(_ *ProjectMetadataTable, units *[]summaryVegetationUnit, _ *[]SummaryVegetationEnvironmentalNumericRow) {
			*units = nil
		}},
		{"duplicate units", func(_ *ProjectMetadataTable, units *[]summaryVegetationUnit, _ *[]SummaryVegetationEnvironmentalNumericRow) {
			(*units)[1].Code = "A"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields, units, rows := summaryEnvironmentalHeaderFixture(t, true)
			tc.edit(&fields, &units, &rows)
			plan, err := prepareSummaryEnvironmentalHeader(t.Context(), 4, false, fields, units, rows)
			if err == nil || !reflect.DeepEqual(plan, summaryEnvironmentalHeaderPlan{}) {
				t.Fatal("invalid shape returned a success/partial header", plan, err)
			}
		})
	}
	fields, units, rows := summaryEnvironmentalHeaderFixture(t, true)
	for _, first := range []int{1, 32762, 32768, int(^uint(0) >> 1)} {
		if plan, err := prepareSummaryEnvironmentalHeader(t.Context(), first, false, fields, units, rows); err == nil ||
			!reflect.DeepEqual(plan, summaryEnvironmentalHeaderPlan{}) {
			t.Fatal("source Integer bound not checked", first, err)
		}
	}
	if plan, err := prepareSummaryEnvironmentalHeader(t.Context(), 32761, true, fields, units, rows); err != nil || plan.NextRow != 32767 {
		t.Fatal("exact original Integer return bound rejected", plan, err)
	}
	for _, ctx := range []context.Context{nil, &summaryHeaderCancellingContext{Context: t.Context(), remaining: 12}} {
		plan, err := prepareSummaryEnvironmentalHeader(ctx, 4, false, fields, units, rows)
		if err == nil || !reflect.DeepEqual(plan, summaryEnvironmentalHeaderPlan{}) || (ctx != nil && !errors.Is(err, context.Canceled)) {
			t.Fatal("missing/in-flight canceled context returned partial header", plan, err)
		}
	}
	fields, _, _ = summaryEnvironmentalHeaderFixture(t, false)
	for _, count := range []int{250, 251} {
		frame := make([]summaryVegetationUnit, count)
		for i := range frame {
			frame[i].Code = fmt.Sprintf("U%d", i)
		}
		plan, err := prepareSummaryEnvironmentalHeader(t.Context(), 2, false, fields, frame, nil)
		if count == 250 && (err != nil || plan.LastValueColumn != 253 || plan.NextRow != 4) {
			t.Fatal("exact source group/header boundary rejected", plan, err)
		}
		if count == 251 && (err == nil || !reflect.DeepEqual(plan, summaryEnvironmentalHeaderPlan{})) {
			t.Fatal("source group boundary exceeded", plan, err)
		}
	}
}
