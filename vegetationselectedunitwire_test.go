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

func TestSelectedUnitStrictClientWireMatchesPreparedKernel(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationSelectedUnitsWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected LongVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		t.Fatal("shared selected-unit fixture is not production wire", err)
	}
	veg := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows:    []ProjectMetadataRow{{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("S")}}},
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
		Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Original unit")}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Another original unit")}},
		},
	}
	env := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Alpha")}},
			{RowID: "12", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Beta")}},
		},
	}
	admin := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "22", Cells: []ProjectMetadataCell{metadataText("P1")}},
		},
	}
	selectionRaw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationSelectedUnitSelectionWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selectionExpected LongVegetationSelectedUnitSelection
	selectionDecoder := json.NewDecoder(bytes.NewReader(selectionRaw))
	selectionDecoder.DisallowUnknownFields()
	if err := selectionDecoder.Decode(&selectionExpected); err != nil {
		t.Fatal("selector shared fixture is not production wire", err)
	}
	selectionOwner := &sqliteContext{selection: desktopSelection{
		Project: "Sample", ProjectPath: `C:\project.db`, SU: "Selected", SUPath: `C:\external.db`,
	}}
	selectionTables := [3]ProjectMetadataTable{env, admin, su}
	selectionBuild, err := buildSelectedUnitSelection(context.Background(), "owned", selectionOwner,
		selectionTables, []string{"Zone"}, configValues{"ReportOptions": map[string]any{"LVUnitGroups": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(selectionBuild.ApprovalHash) != 64 {
		t.Fatal("actual readonly Build lost approval hash")
	}
	// The shared transport fixture has a fixed opaque hash, not source authority.
	selectionBuild.ApprovalHash = selectionExpected.Build.ApprovalHash
	selectionActual := selectedUnitSelectionDTO("owned", selectionOwner, selectionTables,
		selectedUnitSelectionEntry{revision: 2, fields: []string{"Zone"}, build: selectionBuild})
	selectionActual.BuildCurrent = true
	selectionJSON, err := json.Marshal(selectionActual)
	if err != nil || !reflect.DeepEqual(selectionActual, &selectionExpected) {
		t.Fatalf("strict selector wire differs from actual production Build/DTO: %v\n%s", err, selectionJSON)
	}
	var selectionReopened LongVegetationSelectedUnitSelection
	if err := json.Unmarshal(selectionJSON, &selectionReopened); err != nil || !reflect.DeepEqual(selectionReopened, selectionExpected) {
		t.Fatal("production selector JSON lost physical definitions or original joins", err)
	}
	layers := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows:    []ProjectMetadataRow{{RowID: "31", Cells: []ProjectMetadataCell{metadataText("1"), metadataText("Tree"), metadataText("1")}}},
	}
	master := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows:    []ProjectMetadataRow{{RowID: "41", Cells: []ProjectMetadataCell{metadataText("S"), metadataText("SP"), {Storage: "null"}, {Storage: "null"}}}},
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
	names := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []ProjectMetadataRow{}}
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
