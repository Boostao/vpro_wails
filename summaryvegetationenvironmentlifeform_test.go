package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalLifeformFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	return summaryEnvironmentalLifeformFixtureFromJSON(t, summaryEnvironmentalLifeformSourceFixture)
}

func summaryEnvironmentalLifeformFixtureFromJSON(t *testing.T, fixture string) (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]json.RawMessage `json:"physical"`
		Schemas  map[string][][]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal([]byte(fixture), &source); err != nil {
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
					switch {
					case name == "QuickVeg" && j == 2:
						var value float64
						if err := json.Unmarshal(raw, &value); err != nil {
							t.Fatal(err)
						}
						cell = ProjectMetadataCell{Storage: "real", Real: &value}
					case name == "USysAllSpecs" && j == 1:
						var value int64
						if err := json.Unmarshal(raw, &value); err != nil {
							t.Fatal(err)
						}
						cell = metadataInteger(strconv.FormatInt(value, 10))
					default:
						var value string
						if err := json.Unmarshal(raw, &value); err != nil {
							t.Fatal(err)
						}
						cell = ProjectMetadataCell{Storage: "text", Text: &value}
					}
				}
				cells = append(cells, cell)
			}
			table.Rows = append(table.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: cells})
		}
		return table
	}
	return build("QuickVeg"), build("USysAllSpecs"), build("UnitInputs")
}

func TestSummaryEnvironmentalLifeformOriginalCases(t *testing.T) {
	quick, master, su := summaryEnvironmentalLifeformFixture(t)
	var source struct {
		Observations []struct {
			Unit     *string  `json:"unit"`
			Argument *string  `json:"argument"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalLifeformSourceFixture), &source); err != nil || len(source.Observations) != 23 {
		t.Fatal("complete23 original cases required", err)
	}
	for _, item := range source.Observations {
		if item.Argument == nil {
			continue
		}
		got, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), *item.Argument, item.Unit, quick, master, su)
		if item.Unit == nil {
			if err == nil || !reflect.DeepEqual(item.Actual[:2], []string{"error", "94"}) {
				t.Fatal(got, err, item)
			}
			continue
		}
		if *item.Unit == "MIXED" {
			if err != nil || got.Text == nil || *got.Text != "" {
				t.Fatal("literal native scope adaptation", got, err)
			}
			continue
		}
		if err != nil || got.Storage != "text" || got.Text == nil || !reflect.DeepEqual(item.Actual, []string{"ok", "String", *got.Text}) {
			t.Fatal(got, err, item)
		}
	}
}

func TestSummaryEnvironmentalLifeformRefusalsAndSourceImmutability(t *testing.T) {
	quick, master, su := summaryEnvironmentalLifeformFixture(t)
	before, err := json.Marshal([]ProjectMetadataTable{quick, master, su})
	if err != nil {
		t.Fatal(err)
	}
	unit := "mixed"
	got, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), "1", &unit, quick, master, su)
	if err != nil || got.Text == nil || *got.Text != "8.0 - 9.5 - 11.0" {
		t.Fatal(got, err)
	}
	after, err := json.Marshal([]ProjectMetadataTable{quick, master, su})
	if err != nil || string(before) != string(after) {
		t.Fatal("source mutation", err)
	}
	for _, argument := range []string{"", "01", "1 ", "14", "Tree"} {
		if _, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), argument, &unit, quick, master, su); err == nil {
			t.Fatal(argument)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareSummaryVegetationEnvironmentalLifeform(ctx, "1", &unit, quick, master, su); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := prepareSummaryVegetationEnvironmentalLifeform(nil, "1", &unit, quick, master, su); err == nil {
		t.Fatal("nil context")
	}
	for _, target := range []string{"quick", "master", "su"} {
		quick, master, su := summaryEnvironmentalLifeformFixture(t)
		bad := string([]byte{0xff})
		switch target {
		case "quick":
			quick.Rows[len(quick.Rows)-1].Cells[0] = ProjectMetadataCell{Storage: "text", Text: &bad}
		case "master":
			master.Rows[len(master.Rows)-1].Cells[1] = metadataInteger("32768")
		case "su":
			su.Rows[len(su.Rows)-1].Cells[1] = ProjectMetadataCell{Storage: "text", Text: &bad}
		}
		if _, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), "1", &unit, quick, master, su); err == nil {
			t.Fatal("nonjoined invalid source", target)
		}
	}
	quick, master, su = summaryEnvironmentalLifeformFixture(t)
	number := 0.1
	quick.Rows[len(quick.Rows)-1].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &number}
	if _, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), "1", &unit, quick, master, su); err == nil {
		t.Fatal("non-SINGLE nonjoined source")
	}
}
