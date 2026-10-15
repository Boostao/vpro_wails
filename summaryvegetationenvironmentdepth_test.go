package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalDepthPhysicalFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]json.RawMessage `json:"physical"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalDepthSourceFixture), &source); err != nil {
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
						var number int64
						if err := json.Unmarshal(raw, &number); err != nil {
							t.Fatal(err)
						}
						cell = metadataInteger(strconv.FormatInt(number, 10))
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
	return build("Sample_Env", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SeepageDepth"}, {Name: "RootingDepth"}}),
		build("UnitInputs", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}})
}

func TestSummaryEnvironmentalDepthOriginalPhysicalCases(t *testing.T) {
	env, su := summaryEnvironmentalDepthPhysicalFixture(t)
	var source struct {
		Observations []struct {
			Function string   `json:"function"`
			Unit     *string  `json:"unit"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalDepthSourceFixture), &source); err != nil || len(source.Observations) != 22 {
		t.Fatal("twenty-two independent original observations required", err)
	}
	for _, observation := range source.Observations {
		got, err := prepareSummaryVegetationEnvironmentalDepth(t.Context(), observation.Function, observation.Unit, env, su)
		if observation.Unit == nil {
			if err == nil || !reflect.DeepEqual(observation.Actual[:2], []string{"error", "94"}) {
				t.Fatal(got, err, observation)
			}
			continue
		}
		kind := "String"
		if got.Empty {
			kind = "Empty"
		}
		if err != nil || !reflect.DeepEqual(observation.Actual, []string{"ok", kind, got.Text}) {
			t.Fatal(got, err, observation)
		}
	}
}

func TestSummaryEnvironmentalDepthRefusalsAndImmutableInputs(t *testing.T) {
	env, su := summaryEnvironmentalDepthPhysicalFixture(t)
	beforeEnv, _ := json.Marshal(env)
	beforeSU, _ := json.Marshal(su)
	unit := "mixed"
	for _, function := range []string{"SVSeepage", "SVRootingDepth"} {
		got, err := prepareSummaryVegetationEnvironmentalDepth(t.Context(), function, &unit, env, su)
		if err != nil || got.Empty || got.Text != "1 - 2.3 - 5  (Null 1)" {
			t.Fatal(got, err)
		}
	}
	afterEnv, _ := json.Marshal(env)
	afterSU, _ := json.Marshal(su)
	if string(beforeEnv) != string(afterEnv) || string(beforeSU) != string(afterSU) {
		t.Fatal("physical source mutated")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareSummaryVegetationEnvironmentalDepth(ctx, "SVSeepage", &unit, env, su); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := prepareSummaryVegetationEnvironmentalDepth(nil, "SVSeepage", &unit, env, su); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := prepareSummaryVegetationEnvironmentalDepth(t.Context(), "SVHumusDepth", &unit, env, su); err == nil {
		t.Fatal("absent uncalibrated field enabled")
	}
	number := 0.5
	text := "1"
	badText := string([]byte{0xff})
	for _, invalid := range []ProjectMetadataCell{
		metadataInteger("32768"), metadataInteger("-32769"),
		{Storage: "real", Real: &number}, {Storage: "text", Text: &text},
		{Storage: "null", Integer: &text}, {Storage: "integer", Integer: &text, Text: &text},
	} {
		env, su := summaryEnvironmentalDepthPhysicalFixture(t)
		env.Rows[len(env.Rows)-1].Cells[1] = invalid
		if _, err := prepareSummaryVegetationEnvironmentalDepth(t.Context(), "SVSeepage", &unit, env, su); err == nil {
			t.Fatal("invalid nonjoined storage accepted", invalid)
		}
	}
	env.Rows[len(env.Rows)-1].Cells[0] = ProjectMetadataCell{Storage: "text", Text: &badText}
	if _, err := prepareSummaryVegetationEnvironmentalDepth(t.Context(), "SVSeepage", &unit, env, su); err == nil {
		t.Fatal("malformed nonjoined identity accepted")
	}
	env, su = summaryEnvironmentalDepthPhysicalFixture(t)
	env.Rows[1].RowID = env.Rows[0].RowID
	if _, err := prepareSummaryVegetationEnvironmentalDepth(t.Context(), "SVSeepage", &unit, env, su); err == nil {
		t.Fatal("duplicate physical row identity accepted")
	}
}
