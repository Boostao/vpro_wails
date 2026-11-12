package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	text := func(value string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "text", Text: &value}
	}
	veg := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "21", Cells: []vprocore.ProjectMetadataCell{text("P1"), text("S")}}},
	}
	for _, cover := range longVegetationCoverColumns() {
		veg.Columns = append(veg.Columns, vprocore.ProjectMetadataColumn{Name: cover})
		cell := vprocore.ProjectMetadataCell{Storage: "null"}
		if cover == "Cover1" {
			cell = vprotest.MetadataInteger("2")
		}
		veg.Rows[0].Cells = append(veg.Rows[0].Cells, cell)
	}
	su := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{text("P1"), text("Original unit")}}},
	}
	env := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{text("P1")}},
			{RowID: "12", Cells: []vprocore.ProjectMetadataCell{text("P1")}},
		},
	}
	layers := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "31", Cells: []vprocore.ProjectMetadataCell{text("1"), text("Tree"), text("1")}}},
	}
	master := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "41", Cells: []vprocore.ProjectMetadataCell{text("S"), text("SP"), {Storage: "null"}, {Storage: "null"}}}},
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
	names := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []vprocore.ProjectMetadataRow{}}
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
