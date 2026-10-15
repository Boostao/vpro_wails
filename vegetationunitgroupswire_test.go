package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationNoneUnitsStrictClientWireMatchesPreparedKernel(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationNoneUnitsWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected LongVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		t.Fatal("shared client fixture is not production Go wire", err)
	}
	text := func(value string) ProjectMetadataCell { return ProjectMetadataCell{Storage: "text", Text: &value} }
	veg := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows:    []ProjectMetadataRow{{RowID: "21", Cells: []ProjectMetadataCell{text("P1"), text("S")}}},
	}
	for _, cover := range longVegetationCoverColumns() {
		veg.Columns = append(veg.Columns, ProjectMetadataColumn{Name: cover})
		cell := ProjectMetadataCell{Storage: "null"}
		if cover == "Cover1" {
			cell = metadataInteger("2")
		}
		veg.Rows[0].Cells = append(veg.Rows[0].Cells, cell)
	}
	su := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows:    []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{text("P1"), text("Original unit")}}},
	}
	env := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{text("P1")}},
			{RowID: "12", Cells: []ProjectMetadataCell{text("P1")}},
		},
	}
	layers := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows:    []ProjectMetadataRow{{RowID: "31", Cells: []ProjectMetadataCell{text("1"), text("Tree"), text("1")}}},
	}
	master := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows:    []ProjectMetadataRow{{RowID: "41", Cells: []ProjectMetadataCell{text("S"), text("SP"), {Storage: "null"}, {Storage: "null"}}}},
	}
	prepared, occurrences, err := prepareLongVegetationNoneUnits(context.Background(), "Sample", "Selected", veg, su, layers, env, nil)
	if err != nil {
		t.Fatal(err)
	}
	options := longVegetationOptions{Title: "None units", Average: "all-plots", Order: "species",
		PresenceGreaterThan: -100, MeanCoverGreaterThan: -100, NoneUnits: true}
	report, err := planLongVegetationLayers(context.Background(), prepared, master, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	names := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []ProjectMetadataRow{}}
	if err := addLongVegetationUnitNames(context.Background(), &report, names); err != nil {
		t.Fatal(err)
	}
	report.NoneUnits = occurrences
	transport, err := longVegetationTransport(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	actual := LongVegetationPreview{ContextID: "owned", ProjectPath: `C:\project.db`, SUPath: `C:\external.db`,
		Settings: vegetationSettings(options), Report: transport}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("independent client wire differs from production derivation/kernel:\nactual %#v\nexpected %#v", actual, expected)
	}
	again, err := json.Marshal(actual)
	var reopened LongVegetationPreview
	if err != nil || json.Unmarshal(again, &reopened) != nil || !reflect.DeepEqual(reopened, expected) {
		t.Fatal("production None wire changed through serialization", err)
	}
	*transport.NoneUnits[0].PlotNumber.Text = "changed"
	if *report.NoneUnits[0].PlotNumber.Text != "P1" || *transport.NoneUnits[1].PlotNumber.Text != "P1" {
		t.Fatal("transport aliases original or peer derived occurrences")
	}
}
