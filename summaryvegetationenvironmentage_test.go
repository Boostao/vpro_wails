package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalAgePhysicalFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]json.RawMessage `json:"physical"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalAgeSourceFixture), &source); err != nil {
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
	return build("Sample_Env", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "StandAge"}, {Name: "Elevation"}}),
		build("UnitInputs", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}})
}

func TestSummaryEnvironmentalAgeElevationOriginalPhysicalCases(t *testing.T) {
	env, su := summaryEnvironmentalAgePhysicalFixture(t)
	var source struct {
		Observations []struct {
			Function string   `json:"function"`
			Unit     *string  `json:"unit"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalAgeSourceFixture), &source); err != nil || len(source.Observations) != 18 {
		t.Fatal("eighteen independent observations required", err)
	}
	for _, observation := range source.Observations {
		got, err := prepareSummaryVegetationEnvironmentalAgeElevation(t.Context(), observation.Function, observation.Unit, env, su)
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

func TestSummaryEnvironmentalAgeElevationSharedBoundaryAndRefusals(t *testing.T) {
	unit := "mixed"
	for _, spec := range []struct {
		function string
		index    int
	}{{"SVStandAge", 1}, {"SVElevation", 2}} {
		env, su := summaryEnvironmentalAgePhysicalFixture(t)
		got, err := prepareSummaryVegetationEnvironmentalAgeElevation(t.Context(), spec.function, &unit, env, su)
		if err != nil || *got.Text != "1 - 02 - 5  (Null 1)" {
			t.Fatal(got, err)
		}
		aggregate, err := readSummaryEnvironmentalIntegerAggregate(t.Context(), env.Columns[spec.index].Name, &unit, env, su)
		if err != nil || aggregate.Count != 4 || aggregate.Nulls != 1 || aggregate.Sum != 9 ||
			aggregate.Minimum != 1 || aggregate.Maximum != 5 {
			t.Fatal("independent joined count/sum differs", aggregate, err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := prepareSummaryVegetationEnvironmentalAgeElevation(ctx, spec.function, &unit, env, su); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		bad := 0.5
		env.Rows[len(env.Rows)-1].Cells[spec.index] = ProjectMetadataCell{Storage: "real", Real: &bad}
		if _, err := prepareSummaryVegetationEnvironmentalAgeElevation(t.Context(), spec.function, &unit, env, su); err == nil {
			t.Fatal("fractional nonjoined original integer accepted")
		}
	}
}
