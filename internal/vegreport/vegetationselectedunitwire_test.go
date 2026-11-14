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

func TestSelectedUnitStrictClientWireMatchesPreparedKernel(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "tests", "fixtures", "longVegetationSelectedUnitsWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected LongVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		t.Fatal("shared selected-unit fixture is not production wire", err)
	}
	veg := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "21", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("S")}}},
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
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Original unit")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Another original unit")}},
		},
	}
	env := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Alpha")}},
			{RowID: "12", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Beta")}},
		},
	}
	admin := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "21", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
			{RowID: "22", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
		},
	}
	selectionRaw, err := os.ReadFile(filepath.Join("frontend", "tests", "fixtures", "longVegetationSelectedUnitSelectionWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selectionExpected LongVegetationSelectedUnitSelection
	selectionDecoder := json.NewDecoder(bytes.NewReader(selectionRaw))
	selectionDecoder.DisallowUnknownFields()
	if err := selectionDecoder.Decode(&selectionExpected); err != nil {
		t.Fatal("selector shared fixture is not production wire", err)
	}
	selectionOwner := &vprocore.SqliteContext{Selection: vprocore.DesktopSelection{
		Project: "Sample", ProjectPath: `C:\project.db`, SU: "Selected", SUPath: `C:\external.db`,
	}}
	selectionTables := [3]vprocore.ProjectMetadataTable{env, admin, su}
	selectionBuild, err := buildSelectedUnitSelection(context.Background(), "owned", selectionOwner,
		selectionTables, []string{"Zone"}, vprocore.ConfigValues{"ReportOptions": map[string]any{"LVUnitGroups": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(selectionBuild.ApprovalHash) != 64 {
		t.Fatal("actual readonly Build lost approval hash")
	}
	// The shared transport fixture has a fixed opaque hash, not source authority.
	selectionBuild.ApprovalHash = selectionExpected.Build.ApprovalHash
	selectionActual := selectedUnitSelectionDTO("owned", selectionOwner, selectionTables,
		vprocore.SelectedUnitSelectionEntry{Revision: 2, Fields: []string{"Zone"}, Build: selectionBuild})
	selectionActual.BuildCurrent = true
	selectionJSON, err := json.Marshal(selectionActual)
	if err != nil || !reflect.DeepEqual(selectionActual, &selectionExpected) {
		t.Fatalf("strict selector wire differs from actual production Build/DTO: %v\n%s", err, selectionJSON)
	}
	var selectionReopened LongVegetationSelectedUnitSelection
	if err := json.Unmarshal(selectionJSON, &selectionReopened); err != nil || !reflect.DeepEqual(selectionReopened, selectionExpected) {
		t.Fatal("production selector JSON lost physical definitions or original joins", err)
	}
	layers := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "31", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("1"), vprotest.MetadataText("Tree"), vprotest.MetadataText("1")}}},
	}
	master := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "41", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("S"), vprotest.MetadataText("SP"), {Storage: "null"}, {Storage: "null"}}}},
	}
	options := longVegetationOptions{Title: "Selected units", Average: "all-plots", Order: "species",
		PresenceGreaterThan: -100, MeanCoverGreaterThan: -100, SelectedUnitFields: []string{"Zone"}}
	prepared, occurrences, err := prepareLongVegetationSelectedUnits(context.Background(), "Sample", "Selected",
		veg, su, layers, env, admin, options.SelectedUnitFields, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := planLongVegetationLayers(context.Background(), prepared, master, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	names := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []vprocore.ProjectMetadataRow{}}
	if err := addLongVegetationUnitNames(context.Background(), &report, names); err != nil {
		t.Fatal(err)
	}
	report.SelectedUnits = occurrences
	transport, err := longVegetationTransport(context.Background(), report)
	if err != nil {
		t.Fatal(err)
	}
	actual := LongVegetationPreview{ContextID: "owned", ProjectPath: `C:\project.db`, SUPath: `C:\external.db`,
		Settings: vegetationSettings(options), Report: transport}
	actualJSON, err := json.MarshalIndent(actual, "", "  ")
	if err != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("independent client fixture differs from production selected-unit kernel: %v\n%s", err, actualJSON)
	}
	var reopened LongVegetationPreview
	if err := json.Unmarshal(actualJSON, &reopened); err != nil || !reflect.DeepEqual(reopened, expected) {
		t.Fatal("production serialization changed source2 evidence", err)
	}
}
