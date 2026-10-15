package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func summaryEnvironmentalQuickVegFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	var source struct {
		Physical map[string][][]json.RawMessage `json:"physical"`
		Schemas  map[string][][]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalQuickVegSourceFixture), &source); err != nil {
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
					case name == "Sample_Veg" && j >= 2:
						var number float64
						if err := json.Unmarshal(raw, &number); err != nil {
							t.Fatal(err)
						}
						cell = ProjectMetadataCell{Storage: "real", Real: &number}
					case name == "USysAllSpecs" && j == 1:
						var number int64
						if err := json.Unmarshal(raw, &number); err != nil {
							t.Fatal(err)
						}
						cell = metadataInteger(strconv.FormatInt(number, 10))
					default:
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
	return build("Sample_Veg"), build("USysAllSpecs"), build("UnitInputs")
}

func TestSummaryEnvironmentalQuickVegOriginalConstruction(t *testing.T) {
	veg, master, su := summaryEnvironmentalQuickVegFixture(t)
	quick, err := prepareSummaryEnvironmentalQuickVeg(t.Context(), "Sample", "SU", veg, su)
	if err != nil || len(quick.Rows) != 10 {
		t.Fatal(quick, err)
	}
	type key struct {
		plot, species string
		value         float64
	}
	counts := map[key]int{}
	for _, row := range quick.Rows {
		counts[key{*row.Cells[0].Text, *row.Cells[1].Text, *row.Cells[2].Real}]++
	}
	want := map[key]int{{"P1", "A", 1.25}: 3, {"P1", "A", 2.5}: 3, {"P1", "B", 4}: 3, {"P2", "A", 8}: 1}
	if !reflect.DeepEqual(counts, want) {
		t.Fatal(counts, want)
	}
	for _, item := range []struct{ unit, form, text string }{
		{"mixed", "1", "8.0 - 27.3 - 46.5"}, {"other", "1", "23.3 - 23.3 - 23.3"},
		{"mixed", "2", ""}, {"missing", "1", ""},
	} {
		got, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), item.form, &item.unit, quick, master, su)
		if err != nil || got.Text == nil || *got.Text != item.text {
			t.Fatal(got, err, item)
		}
	}
}

func TestSummaryEnvironmentalQuickVegRefusalsAndDetachedSource(t *testing.T) {
	veg, _, su := summaryEnvironmentalQuickVegFixture(t)
	before, err := json.Marshal([]ProjectMetadataTable{veg, su})
	if err != nil {
		t.Fatal(err)
	}
	quick, err := prepareSummaryEnvironmentalQuickVeg(t.Context(), "Sample", "SU", veg, su)
	if err != nil {
		t.Fatal(err)
	}
	*quick.Rows[0].Cells[1].Text = "caller"
	after, err := json.Marshal([]ProjectMetadataTable{veg, su})
	if err != nil || string(before) != string(after) {
		t.Fatal("source aliased or mutated", err)
	}
	number := 0.1
	veg.Rows[len(veg.Rows)-1].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &number}
	if _, err := prepareSummaryEnvironmentalQuickVeg(t.Context(), "Sample", "SU", veg, su); err == nil {
		t.Fatal("nonjoined non-SINGLE")
	}
	veg, _, su = summaryEnvironmentalQuickVegFixture(t)
	veg.Rows[len(veg.Rows)-1].Cells[0] = ProjectMetadataCell{Storage: "null"}
	if _, err := prepareSummaryEnvironmentalQuickVeg(t.Context(), "Sample", "SU", veg, su); err == nil {
		t.Fatal("historical NULL original required PlotNumber")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepareSummaryEnvironmentalQuickVeg(ctx, "Sample", "SU", veg, su); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := prepareSummaryEnvironmentalQuickVeg(nil, "Sample", "SU", veg, su); err == nil {
		t.Fatal("nil context")
	}
}
