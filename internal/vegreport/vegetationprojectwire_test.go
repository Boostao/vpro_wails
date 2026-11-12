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
	veg := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows: []vprocore.ProjectMetadataRow{{RowID: "51", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("S")}}}}
	for _, cover := range longVegetationCoverColumns() {
		veg.Columns = append(veg.Columns, vprocore.ProjectMetadataColumn{Name: cover})
		cell := vprocore.ProjectMetadataCell{Storage: "null"}
		if cover == "Cover1" {
			cell = vprotest.MetadataInteger("2")
		}
		veg.Rows[0].Cells = append(veg.Rows[0].Cells, cell)
	}
	env := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Alpha")}},
			{RowID: "12", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Beta")}},
		},
	}
	admin := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "21", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
			{RowID: "22", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
		}}
	layers := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "31", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("1"), vprotest.MetadataText("Tree"), vprotest.MetadataText("1")}}}}
	master := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "41", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("S"), vprotest.MetadataText("SP"), {Storage: "null"}, {Storage: "null"}}}}}
	options := longVegetationOptions{Title: "Project units", Average: "all-plots", Order: "species",
		PresenceGreaterThan: -100, MeanCoverGreaterThan: -100}
	ctx := context.Background()
	for i, kind := range []string{vprocore.VegetationProjectFieldsScope, vprocore.VegetationProjectAllScope, vprocore.VegetationProjectFieldsScope} {
		var project vegetationProjectPreparation
		currentOptions := options
		sourceSU, suPath := "None", ""
		var quality *vegetationQualitySelection
		if i == 2 {
			env.Rows, admin.Rows = env.Rows[:1], admin.Rows[:1]
			sourceSU, suPath = "Selected", `C:\external.db`
			su := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
				Rows: []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Original")}}}}
			lists := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Item"}, {Name: "ListName"}, {Name: "ItemOrder"}},
				Rows: []vprocore.ProjectMetadataRow{
					{RowID: "90", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Poor"), vprotest.MetadataText("DataQuality"), vprotest.MetadataInteger("1")}},
					{RowID: "100", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Good"), vprotest.MetadataText("DataQuality"), vprotest.MetadataInteger("3")}},
					{RowID: "101", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Good"), vprotest.MetadataText("DataQuality"), vprotest.MetadataInteger("3")}},
				}}
			for _, name := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
				admin.Columns = append(admin.Columns, vprocore.ProjectMetadataColumn{Name: name})
				admin.Rows[0].Cells = append(admin.Rows[0].Cells, vprotest.MetadataText("Good"))
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
				vprocore.ProjectMetadataTable{}, []string{"Zone"}, nil)
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
		if err := addLongVegetationUnitNames(ctx, &source, vprocore.ProjectMetadataTable{
			Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []vprocore.ProjectMetadataRow{}}); err != nil {
			t.Fatal(err)
		}
		report, err := longVegetationProjectTransport(ctx, source, kind)
		if err != nil {
			t.Fatal(err)
		}
		settings := vegetationSettings(currentOptions)
		settings.UnitGrouping = "all-plots"
		if kind == vprocore.VegetationProjectFieldsScope {
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
