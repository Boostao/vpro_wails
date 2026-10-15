package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalCoverPhysicalFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]json.RawMessage `json:"physical"`
		Schemas  map[string][][]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalCoverSourceFixture), &source); err != nil {
		t.Fatal(err)
	}
	build := func(name string) ProjectMetadataTable {
		table := ProjectMetadataTable{Rows: []ProjectMetadataRow{}}
		for _, column := range source.Schemas[name] {
			var label string
			if err := json.Unmarshal(column[0], &label); err != nil {
				t.Fatal(err)
			}
			table.Columns = append(table.Columns, ProjectMetadataColumn{Name: label})
		}
		for i, row := range source.Physical[name] {
			cells := []ProjectMetadataCell{}
			for j, raw := range row {
				cell := ProjectMetadataCell{Storage: "null"}
				if string(raw) != "null" {
					if name == "Sample_Env" && j != 0 {
						var number float64
						if err := json.Unmarshal(raw, &number); err != nil {
							t.Fatal(err)
						}
						cell = ProjectMetadataCell{Storage: "real", Real: &number}
					} else {
						var text string
						if err := json.Unmarshal(raw, &text); err != nil {
							t.Fatal(err)
						}
						cell = ProjectMetadataCell{Storage: "text", Text: &text}
					}
				}
				cells = append(cells, cell)
			}
			table.Rows = append(table.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: cells})
		}
		return table
	}
	return build("Sample_Env"), build("UnitInputs")
}

func TestSummaryEnvironmentalCoverOriginalPhysicalCases(t *testing.T) {
	env, su := summaryEnvironmentalCoverPhysicalFixture(t)
	var source struct {
		Observations []struct {
			Function string   `json:"function"`
			Suffix   string   `json:"suffix"`
			Unit     *string  `json:"unit"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalCoverSourceFixture), &source); err != nil || len(source.Observations) != 110 {
		t.Fatal("110 independent original observations required", err)
	}
	for _, observation := range source.Observations {
		got, err := prepareSummaryVegetationEnvironmentalCover(t.Context(), observation.Function, observation.Suffix, observation.Unit, env, su)
		if observation.Unit == nil {
			if err == nil || !reflect.DeepEqual(observation.Actual[:2], []string{"error", "94"}) {
				t.Fatal(got, err, observation)
			}
		} else {
			kind := "String"
			if got.Empty {
				kind = "Empty"
			}
			if err != nil || !reflect.DeepEqual(observation.Actual, []string{"ok", kind, got.Text}) {
				t.Fatal(got, err, observation)
			}
		}
	}
}

func TestSummaryEnvironmentalCoverDistinctFieldsAndRefusals(t *testing.T) {
	env, su := summaryEnvironmentalCoverPhysicalFixture(t)
	unit := "plain"
	for _, row := range env.Rows {
		if row.Cells[0].Text != nil && *row.Cells[0].Text == "C1" {
			for column := 1; column < len(row.Cells); column++ {
				value := float64(column)
				row.Cells[column] = ProjectMetadataCell{Storage: "real", Real: &value}
			}
		}
	}
	before, _ := json.Marshal(env)
	for column, field := range env.Columns {
		if column == 0 {
			continue
		}
		function, suffix := "SVSubstrate", field.Name[len("Substrate"):]
		if column > 6 {
			function, suffix = "SVStrataCover", field.Name[len("StrataCover"):]
		}
		got, err := prepareSummaryVegetationEnvironmentalCover(t.Context(), function, suffix, &unit, env, su)
		want := strconv.Itoa(column) + " - " + strconv.Itoa(column) + ".0 - " + strconv.Itoa(column)
		if err != nil || got.Empty || got.Text != want {
			t.Fatal(got, err, want)
		}
	}
	after, _ := json.Marshal(env)
	if string(before) != string(after) {
		t.Fatal("physical source mutated")
	}
	if _, err := prepareSummaryVegetationEnvironmentalCover(t.Context(), "SVSubstrate", "decwood", &unit, env, su); err == nil {
		t.Fatal("case-repaired suffix")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareSummaryVegetationEnvironmentalCover(ctx, "SVSubstrate", "DecWood", &unit, env, su); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	bad := 0.1
	env.Rows[len(env.Rows)-1].Cells[1] = ProjectMetadataCell{Storage: "real", Real: &bad}
	if _, err := prepareSummaryVegetationEnvironmentalCover(t.Context(), "SVSubstrate", "DecWood", &unit, env, su); err == nil {
		t.Fatal("non-SINGLE nonjoined value accepted")
	}
}
