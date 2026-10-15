package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalSlopePhysicalFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]json.RawMessage `json:"physical"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalSlopeSourceFixture), &source); err != nil {
		t.Fatal(err)
	}
	build := func(name string, columns []ProjectMetadataColumn) ProjectMetadataTable {
		table := ProjectMetadataTable{Columns: columns, Rows: []ProjectMetadataRow{}}
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
	return build("Sample_Env", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SlopeGradient"}}),
		build("UnitInputs", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}})
}

func TestSummaryEnvironmentalSlopeOriginalPhysicalCases(t *testing.T) {
	env, su := summaryEnvironmentalSlopePhysicalFixture(t)
	var source struct {
		Observations []struct {
			Unit   *string  `json:"unit"`
			Actual []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalSlopeSourceFixture), &source); err != nil || len(source.Observations) != 9 {
		t.Fatal("nine independent original observations required", err)
	}
	for _, observation := range source.Observations {
		got, err := prepareSummaryVegetationEnvironmentalSlope(t.Context(), observation.Unit, env, su)
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

func TestSummaryEnvironmentalSlopeRefusalsAndImmutableSource(t *testing.T) {
	env, su := summaryEnvironmentalSlopePhysicalFixture(t)
	before, _ := json.Marshal(env)
	unit := "mixed"
	got, err := prepareSummaryVegetationEnvironmentalSlope(t.Context(), &unit, env, su)
	if err != nil || got.Text != "1.25 - 3 - 5.75  (Null 1)" {
		t.Fatal(got, err)
	}
	after, _ := json.Marshal(env)
	if string(before) != string(after) {
		t.Fatal("physical source mutated")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareSummaryVegetationEnvironmentalSlope(ctx, &unit, env, su); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	badReal := 0.1
	text := "1"
	for _, bad := range []ProjectMetadataCell{
		{Storage: "real", Real: &badReal}, {Storage: "text", Text: &text},
		{Storage: "null", Integer: &text}, metadataInteger("9007199254740993"),
	} {
		env, su := summaryEnvironmentalSlopePhysicalFixture(t)
		env.Rows[len(env.Rows)-1].Cells[1] = bad
		if _, err := prepareSummaryVegetationEnvironmentalSlope(t.Context(), &unit, env, su); err == nil {
			t.Fatal("invalid nonjoined storage accepted", bad)
		}
	}
}
