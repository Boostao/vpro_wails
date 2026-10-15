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

func TestProjectTransportStrictClientWireMatchesOriginalKernel(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationProjectUnitsWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected []LongVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil || len(expected) != 3 {
		t.Fatal("project fixture is not three complete production-Go previews", err)
	}
	veg := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows: []ProjectMetadataRow{{RowID: "51", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("S")}}}}
	for _, cover := range longVegetationCoverColumns() {
		veg.Columns = append(veg.Columns, ProjectMetadataColumn{Name: cover})
		cell := ProjectMetadataCell{Storage: "null"}
		if cover == "Cover1" {
			cell = metadataInteger("2")
		}
		veg.Rows[0].Cells = append(veg.Rows[0].Cells, cell)
	}
	env := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Alpha")}},
			{RowID: "12", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Beta")}},
		},
	}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "22", Cells: []ProjectMetadataCell{metadataText("P1")}},
		}}
	layers := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows:    []ProjectMetadataRow{{RowID: "31", Cells: []ProjectMetadataCell{metadataText("1"), metadataText("Tree"), metadataText("1")}}}}
	master := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows:    []ProjectMetadataRow{{RowID: "41", Cells: []ProjectMetadataCell{metadataText("S"), metadataText("SP"), {Storage: "null"}, {Storage: "null"}}}}}
	options := longVegetationOptions{Title: "Project units", Average: "all-plots", Order: "species",
		PresenceGreaterThan: -100, MeanCoverGreaterThan: -100}
	ctx := context.Background()
	for i, kind := range []string{vegetationProjectFieldsScope, vegetationProjectAllScope, vegetationProjectFieldsScope} {
		var project vegetationProjectPreparation
		currentOptions := options
		sourceSU, suPath := "None", ""
		var quality *vegetationQualitySelection
		if i == 2 {
			env.Rows, admin.Rows = env.Rows[:1], admin.Rows[:1]
			sourceSU, suPath = "Selected", `C:\external.db`
			su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
				Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Original")}}}}
			lists := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Item"}, {Name: "ListName"}, {Name: "ItemOrder"}},
				Rows: []ProjectMetadataRow{
					{RowID: "90", Cells: []ProjectMetadataCell{metadataText("Poor"), metadataText("DataQuality"), metadataInteger("1")}},
					{RowID: "100", Cells: []ProjectMetadataCell{metadataText("Good"), metadataText("DataQuality"), metadataInteger("3")}},
					{RowID: "101", Cells: []ProjectMetadataCell{metadataText("Good"), metadataText("DataQuality"), metadataInteger("3")}},
				}}
			for _, name := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
				admin.Columns = append(admin.Columns, ProjectMetadataColumn{Name: name})
				admin.Rows[0].Cells = append(admin.Rows[0].Cells, metadataText("Good"))
			}
			criteria := [3]vegetationQualityCriterion{{Minimum: "Poor"}, {Minimum: "Poor"}, {Minimum: "Poor"}}
			qualified, qualityErr := planLongVegetationQuality(ctx, su, env, admin, lists, criteria)
			if qualityErr != nil {
				t.Fatal(qualityErr)
			}
			quality, currentOptions.Quality = &qualified, &criteria
			project, err = prepareLongVegetationProjectSelectedUnits(ctx, "Sample", sourceSU, veg, layers, env, admin,
				su, []string{"Zone"}, quality)
		} else if i == 0 {
			project, err = prepareLongVegetationProjectSelectedUnits(ctx, "Sample", "None", veg, layers, env, admin,
				ProjectMetadataTable{}, []string{"Zone"}, nil)
		} else {
			project, err = prepareLongVegetationProjectNoneUnits(ctx, "Sample", veg, layers, env)
		}
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := projectReportPreparation(ctx, project)
		if err != nil {
			t.Fatal(err)
		}
		source, err := planLongVegetationLayers(ctx, prepared, master, layers, currentOptions)
		if err != nil {
			t.Fatal(err)
		}
		if quality != nil {
			source.Quality = quality
			if err := addLongVegetationQualityMultiplicity(ctx, &source, *quality); err != nil {
				t.Fatal(err)
			}
		}
		if err := addLongVegetationUnitNames(ctx, &source, ProjectMetadataTable{
			Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []ProjectMetadataRow{}}); err != nil {
			t.Fatal(err)
		}
		report, err := longVegetationProjectTransport(ctx, source, kind)
		if err != nil {
			t.Fatal(err)
		}
		settings := vegetationSettings(currentOptions)
		settings.UnitGrouping = "all-plots"
		if kind == vegetationProjectFieldsScope {
			settings.UnitGrouping, settings.UnitFields = "selected-fields", []string{"Zone"}
		}
		actual := LongVegetationPreview{ContextID: "owned", ProjectPath: `C:\project.db`, SUPath: suPath,
			Settings: settings, Report: report}
		actualJSON, err := json.MarshalIndent(actual, "", "  ")
		if err != nil || !reflect.DeepEqual(actual, expected[i]) {
			t.Fatalf("independent project fixture differs from actual %s kernel/transport: %v\n%s", kind, err, actualJSON)
		}
		var reopened LongVegetationPreview
		if err := json.Unmarshal(actualJSON, &reopened); err != nil || !reflect.DeepEqual(reopened, expected[i]) {
			t.Fatal("production-Go project serialization changed distinct ownership wire", kind, err)
		}
	}
}
