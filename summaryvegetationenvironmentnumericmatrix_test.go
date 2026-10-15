package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestSummaryEnvironmentalNumericMatrixPreservesLegacyRows(t *testing.T) {
	env, su := summaryEnvironmentalCategoryPreparationFixture()
	fields := summaryEnvironmentalConsumerFields(t, [][]any{
		{nil, "Env", "Nutrient", nil, int64(-1), "SVSnr", nil},
		{int64(0), "Env", "n Plots", nil, int64(-1), "SVnPlots", nil},
	})
	units := []summaryVegetationUnit{{Code: "mixed"}, {Code: "allnull"}}
	old, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := planSummaryVegetationEnvironmentalNumericRows(t.Context(), fields, units, env, su)
	if err != nil || len(rows) != len(old) {
		t.Fatal(rows, err)
	}
	for i, row := range rows {
		if !reflect.DeepEqual(row.Field, old[i].Field) || row.LabelColumn != old[i].LabelColumn {
			t.Fatal(row, old[i])
		}
		for j, value := range row.Values {
			if value.Kind != "cell" || value.Cell == nil || !reflect.DeepEqual(*value.Cell, old[i].Values[j]) {
				t.Fatal(value, old[i])
			}
		}
	}
}

func TestSummaryEnvironmentalNumericMatrixEmptyAndWholeResultRefusal(t *testing.T) {
	env, su := summaryEnvironmentalDepthPhysicalFixture(t)
	fields := summaryEnvironmentalConsumerFields(t, [][]any{
		{nil, "Env", "Seepage", nil, int64(-1), "SVSeepage", nil},
		{int64(0), "Env", "n Plots", nil, int64(-1), "SVnPlots", nil},
		{int64(-1), nil, "Root", nil, int64(-1), "SVRootingDepth", nil},
	})
	units := []summaryVegetationUnit{{Code: "mixed"}, {Code: "allnull"}}
	rows, err := planSummaryVegetationEnvironmentalNumericRows(t.Context(), fields, units, env, su)
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	if rows[0].Field.RowID != "1" || rows[1].Field.RowID != "3" || rows[2].Field.RowID != "2" || rows[2].LabelColumn != 3 {
		t.Fatal(rows)
	}
	for _, row := range rows[:2] {
		if row.Values[0].Kind != "cell" || row.Values[0].Cell.Text == nil || *row.Values[0].Cell.Text != "1 - 2.3 - 5  (Null 1)" ||
			!reflect.DeepEqual(row.Values[1], SummaryVegetationEnvironmentalValue{Kind: "empty"}) {
			t.Fatal(row)
		}
	}
	if old, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su); err == nil || old != nil {
		t.Fatal("legacy facade unexpectedly enabled numeric", old, err)
	}
	for _, replacement := range []ProjectMetadataCell{
		{Storage: "text", Text: func() *string { s := ""; return &s }()},
		metadataInteger("1"),
	} {
		bad := cloneSummaryVegetationFieldListTable(fields)
		bad.Rows[0].Cells[6] = replacement
		if result, err := planSummaryVegetationEnvironmentalNumericRows(t.Context(), bad, units, env, su); err == nil || result != nil {
			t.Fatal("partial matrix", result, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := planSummaryVegetationEnvironmentalNumericRows(ctx, fields, units, env, su); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal(result, err)
	}
}

func TestSummaryEnvironmentalNumericMatrixAllCoverSuffixes(t *testing.T) {
	env, su := summaryEnvironmentalCoverPhysicalFixture(t)
	values := [][]any{}
	for _, spec := range []struct {
		function string
		suffixes []string
	}{
		{"SVSubstrate", []string{"DecWood", "BedRock", "Rocks", "MineralSoil", "OrganicMatter", "Water"}},
		{"SVStrataCover", []string{"Tree", "Shrub", "Herb", "Moss"}},
	} {
		for _, suffix := range spec.suffixes {
			values = append(values, []any{int64(len(values)), "Env", suffix, nil, int64(-1), spec.function, suffix})
		}
	}
	fields := summaryEnvironmentalConsumerFields(t, values)
	rows, err := planSummaryVegetationEnvironmentalNumericRows(t.Context(), fields, []summaryVegetationUnit{{Code: "mixed"}, {Code: "allnull"}}, env, su)
	if err != nil || len(rows) != 10 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.Values[0].Kind != "cell" || row.Values[0].Cell.Text == nil || *row.Values[0].Cell.Text != "1.25 - 2.7 - 5.75  (Null 1)" ||
			row.Values[1].Kind != "empty" || row.Values[1].Cell != nil {
			t.Fatal(row)
		}
	}
}
