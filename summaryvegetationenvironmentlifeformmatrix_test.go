package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func TestSummaryEnvironmentalLifeformMatrixAllCanonicalRows(t *testing.T) {
	quick, master, su := summaryEnvironmentalLifeformFixtureFromJSON(t, summaryEnvironmentalLifeformDomainSourceFixture)
	env, _ := summaryEnvironmentalCategoryPreparationFixture()
	values := [][]any{}
	for form := 1; form <= 13; form++ {
		var order any = int64(form)
		if form == 3 {
			order = nil
		} else if form == 4 {
			order = int64(-1)
		}
		argument := strconv.Itoa(form)
		values = append(values, []any{order, "Calc", "Lifeform " + argument, "Lifeform " + argument,
			int64(-1), "SVLifeformCover", argument})
	}
	values = append(values,
		[]any{int64(0), nil, "n Plots", nil, int64(-1), "SVnPlots", nil},
		[]any{int64(20), "Env", "Nutrient", nil, int64(-1), "SVSnr", nil},
		[]any{int64(21), "Calc", "disabled", nil, int64(0), "unknown", "wrong"})
	fields := summaryEnvironmentalConsumerFields(t, values)
	units := []summaryVegetationUnit{{Code: "mixed", NumPlots: 999}, {Code: "allnull", NumPlots: 999}}
	before, err := json.Marshal([]ProjectMetadataTable{fields, env, su, quick, master})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := planSummaryVegetationEnvironmentalLifeformRows(t.Context(), fields, units, env, su, quick, master)
	if err != nil || len(rows) != 15 || rows[0].Field.RowID != "3" || rows[1].Field.RowID != "4" ||
		rows[2].Field.RowID != "14" || rows[2].LabelColumn != 3 {
		t.Fatal("selected hidden/NULL/order/count behavior changed", rows, err)
	}
	forms := map[string]bool{}
	for _, row := range rows {
		if len(row.Values) != 2 || len(row.Field.Cells) != 7 {
			t.Fatal("complete physical witness and unit values required", row)
		}
		function := *row.Field.Cells[5].Text
		switch function {
		case "SVLifeformCover":
			argument := *row.Field.Cells[6].Text
			expected := "2.5 - 3.1 - 3.8"
			if argument == "1" {
				expected = "5.0 - 6.3 - 7.5"
			}
			forms[argument] = true
			for i, text := range []string{expected, ""} {
				value := row.Values[i]
				if value.Kind != "cell" || value.Cell == nil || value.Cell.Storage != "text" ||
					value.Cell.Text == nil || *value.Cell.Text != text {
					t.Fatal("source lifeform String result must never become Empty", argument, value)
				}
			}
		case "SVnPlots":
			for i, text := range []string{"4", "1"} {
				value := row.Values[i]
				if value.Kind != "cell" || value.Cell == nil || value.Cell.Storage != "integer" ||
					value.Cell.Integer == nil || *value.Cell.Integer != text {
					t.Fatal("count must use physical SU rows, not cached unit frame", value)
				}
			}
		case "SVSnr":
			legacy, err := prepareSummaryEnvironmentalSelectedCell(t.Context(), function, row.Field.Cells[6], &units[0].Code, env, su)
			if err != nil || row.Values[0].Kind != "cell" || !reflect.DeepEqual(row.Values[0].Cell, &legacy) {
				t.Fatal("existing category semantics changed", row, err)
			}
		default:
			t.Fatal("disabled unavailable field leaked into result", row)
		}
	}
	if len(forms) != 13 {
		t.Fatal("all13 selected lifeform rows required", forms)
	}
	after, err := json.Marshal([]ProjectMetadataTable{fields, env, su, quick, master})
	if err != nil || string(before) != string(after) {
		t.Fatal("planner mutated complete source inputs", err)
	}
	*rows[0].Field.Cells[2].Text = "caller"
	*rows[0].Values[0].Cell.Text = "caller"
	after, err = json.Marshal([]ProjectMetadataTable{fields, env, su, quick, master})
	if err != nil || string(before) != string(after) {
		t.Fatal("caller mutation aliased source witness/result", err)
	}
	if old, err := planSummaryVegetationEnvironmentalNumericRows(t.Context(), fields, units, env, su); err == nil || old != nil {
		t.Fatal("numeric-only planner implicitly enabled lifeform", old, err)
	}
}

func TestSummaryEnvironmentalLifeformMatrixPreservesNumericAndWholeRefusal(t *testing.T) {
	env, su := summaryEnvironmentalDepthPhysicalFixture(t)
	quick, master, _ := summaryEnvironmentalLifeformFixtureFromJSON(t, summaryEnvironmentalLifeformDomainSourceFixture)
	fields := summaryEnvironmentalConsumerFields(t, [][]any{
		{int64(0), "Env", "Seepage", nil, int64(-1), "SVSeepage", nil},
		{int64(1), "Calc", "Lifeform 3", nil, int64(-1), "SVLifeformCover", "3"},
	})
	units := []summaryVegetationUnit{{Code: "mixed"}, {Code: "allnull"}}
	rows, err := planSummaryVegetationEnvironmentalLifeformRows(t.Context(), fields, units, env, su, quick, master)
	if err != nil || len(rows) != 2 || rows[0].Values[0].Cell == nil || rows[0].Values[0].Cell.Text == nil ||
		*rows[0].Values[0].Cell.Text != "1 - 2.3 - 5  (Null 1)" ||
		!reflect.DeepEqual(rows[0].Values[1], SummaryVegetationEnvironmentalValue{Kind: "empty"}) ||
		rows[1].Values[1].Kind != "cell" || rows[1].Values[1].Cell.Text == nil || *rows[1].Values[1].Cell.Text != "" {
		t.Fatal("numeric Empty and lifeform String empty must remain different", rows, err)
	}
	for _, argument := range []ProjectMetadataCell{
		{Storage: "null"}, metadataInteger("3"),
		{Storage: "text", Text: func() *string { s := "03"; return &s }()},
		{Storage: "text", Text: func() *string { s := "14"; return &s }()},
		{Storage: "text", Text: func() *string { s := string([]byte{0xff}); return &s }()},
	} {
		bad := cloneSummaryVegetationFieldListTable(fields)
		bad.Rows[1].Cells[6] = argument
		if result, err := planSummaryVegetationEnvironmentalLifeformRows(t.Context(), bad, units, env, su, quick, master); err == nil || result != nil {
			t.Fatal("invalid selected argument returned partial result", argument, result, err)
		}
	}
	bad := cloneSummaryVegetationFieldListTable(fields)
	bad.Rows[1].Cells = bad.Rows[1].Cells[:6]
	if result, err := planSummaryVegetationEnvironmentalLifeformRows(t.Context(), bad, units, env, su, quick, master); err == nil || result != nil {
		t.Fatal("incomplete physical witness returned partial result", result, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := planSummaryVegetationEnvironmentalLifeformRows(ctx, fields, units, env, su, quick, master); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled matrix must refuse completely", result, err)
	}
}
