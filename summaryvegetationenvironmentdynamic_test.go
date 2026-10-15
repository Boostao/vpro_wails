package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalDynamicPhysicalFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]*string `json:"physical"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalDynamicSourceFixture), &source); err != nil {
		t.Fatal(err)
	}
	build := func(name string, columns []ProjectMetadataColumn) ProjectMetadataTable {
		table := ProjectMetadataTable{Columns: columns, Rows: []ProjectMetadataRow{}}
		for i, row := range source.Physical[name] {
			cells := []ProjectMetadataCell{}
			for _, text := range row {
				cell := ProjectMetadataCell{Storage: "null"}
				if text != nil {
					cell = ProjectMetadataCell{Storage: "text", Text: text}
				}
				cells = append(cells, cell)
			}
			table.Rows = append(table.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: cells})
		}
		return table
	}
	return build("Sample_Env", []ProjectMetadataColumn{{Name: "PlotNumber"},
			{Name: "BedrockGeology1"}, {Name: "BedrockGeology2"}, {Name: "BedrockGeology3"},
			{Name: "SiteDisturbance1"}, {Name: "SiteDisturbance2"}, {Name: "SiteDisturbance3"}}),
		build("UnitInputs", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}})
}

func TestSummaryEnvironmentalDynamicOriginalPhysicalCases(t *testing.T) {
	env, su := summaryEnvironmentalDynamicPhysicalFixture(t)
	var source struct {
		Observations []struct {
			Function string   `json:"function"`
			Argument int      `json:"argument"`
			Unit     *string  `json:"unit"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalDynamicSourceFixture), &source); err != nil || len(source.Observations) != 40 {
		t.Fatal("forty independent original observations required", err)
	}
	for _, observation := range source.Observations {
		got, err := prepareSummaryVegetationEnvironmentalDynamicCategory(t.Context(), observation.Function,
			observation.Argument, observation.Unit, env, su)
		if observation.Unit != nil && *observation.Unit == "MIXED" {
			if err != nil || got.Text == nil || *got.Text != "" ||
				!reflect.DeepEqual(observation.Actual, []string{"ok", "String", "(1)  A(1)  B(3)  (Null 1)"}) {
				t.Fatal("explicit literal scope adaptation differs", got, err, observation)
			}
			continue
		}
		if observation.Unit == nil {
			if err == nil || !reflect.DeepEqual(observation.Actual[:2], []string{"error", "94"}) {
				t.Fatal(got, err, observation)
			}
		} else if err != nil || got.Storage != "text" || got.Text == nil ||
			!reflect.DeepEqual(observation.Actual, []string{"ok", "String", *got.Text}) {
			t.Fatal(got, err, observation)
		}
	}
}

func TestSummaryEnvironmentalDynamicDistinctFieldsAndRefusals(t *testing.T) {
	env, su := summaryEnvironmentalDynamicPhysicalFixture(t)
	unit := "plain"
	for _, row := range env.Rows {
		if row.Cells[0].Text != nil && *row.Cells[0].Text == "C1" {
			for column := 1; column < len(row.Cells); column++ {
				label := "field" + strconv.Itoa(column)
				row.Cells[column] = ProjectMetadataCell{Storage: "text", Text: &label}
			}
		}
	}
	before, _ := json.Marshal(env)
	for _, spec := range []struct {
		function         string
		argument, column int
	}{{"SVBedrockType", 1, 1}, {"SVBedrockType", 2, 2}, {"SVBedrockType", 3, 3},
		{"SVSiteDisturbance", 1, 4}, {"SVSiteDisturbance", 2, 5}} {
		got, err := prepareSummaryVegetationEnvironmentalDynamicCategory(t.Context(), spec.function, spec.argument, &unit, env, su)
		if err != nil || *got.Text != "field"+strconv.Itoa(spec.column)+"(1)  " {
			t.Fatal(got, err)
		}
	}
	after, _ := json.Marshal(env)
	if string(before) != string(after) {
		t.Fatal("physical source mutated")
	}
	for _, argument := range []int{-32769, -1, 0, 4, 32768} {
		if _, err := prepareSummaryVegetationEnvironmentalDynamicCategory(t.Context(), "SVBedrockType", argument, &unit, env, su); err == nil {
			t.Fatal("uncalibrated bedrock argument accepted", argument)
		}
	}
	if _, err := prepareSummaryVegetationEnvironmentalDynamicCategory(t.Context(), "SVSiteDisturbance", 3, &unit, env, su); err == nil {
		t.Fatal("wrong-field source path accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareSummaryVegetationEnvironmentalDynamicCategory(ctx, "SVBedrockType", 1, &unit, env, su); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	bad := string([]byte{0xff})
	env.Rows[len(env.Rows)-1].Cells[1] = ProjectMetadataCell{Storage: "text", Text: &bad}
	if _, err := prepareSummaryVegetationEnvironmentalDynamicCategory(t.Context(), "SVBedrockType", 1, &unit, env, su); err == nil {
		t.Fatal("malformed nonjoined text accepted")
	}
}
