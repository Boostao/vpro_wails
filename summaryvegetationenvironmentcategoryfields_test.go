package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryEnvironmentalCategoryIndependentSixteenFieldBindings(t *testing.T) {
	cases := []struct{ function, field, label string }{
		{"SVStructureStage", "StructuralStage", "A"},
		{"SVSuccessional", "SuccessionalStatus", "B"},
		{"SVSurfaceTopographyType", "SurfaceTopographyType", "C"},
		{"SVSnr", "NutrientRegime", "D"},
		{"SVMesoSlopePosition", "MesoSlopePosition", "E"},
		{"SVHydroGeoSubSystem", "HydroGeoSubSystem", "F"},
		{"SVHydroGeoSystem", "HydroGeoSystem", "G"},
		{"SVSoilDrainage", "SoilDrainage", "H"},
		{"SVHumusForm", "HumusForm", "I"},
		{"SVRootRestrictingType", "RootRestrictingType", "J"},
		{"SVSoilClassSubGroup", "SoilClassSubGroup", "K"},
		{"SVSoilClassGroup", "SoilClassGroup", "L"},
		{"SVRootZonePartSize", "RootZoneParticleSize", "M"},
		{"SVSurficialMaterialSurf", "SurficialMaterialSurf", "N"},
		{"SVExposure", "Exposure1", "O"},
		{"SVBioGeoClimaticZone", "Zone", "P"},
	}
	unit, plot := "unit", "P"
	text := func(value *string) ProjectMetadataCell { return ProjectMetadataCell{Storage: "text", Text: value} }
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{text(&plot)}},
	}}
	for _, tc := range cases {
		label := tc.label
		env.Columns = append(env.Columns, ProjectMetadataColumn{Name: tc.field})
		env.Rows[0].Cells = append(env.Rows[0].Cells, text(&label))
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{text(&plot), text(&unit)}},
		{RowID: "2", Cells: []ProjectMetadataCell{text(&plot), text(&unit)}},
	}}
	for _, tc := range cases {
		t.Run(tc.function, func(t *testing.T) {
			field, ok := summaryVegetationEnvironmentalTextField(tc.function)
			if !ok || field != tc.field {
				t.Fatal("independent physical field mapping differs", field, ok)
			}
			want := tc.label + "(2)  "
			cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), tc.function, &unit, env, su)
			if err != nil || !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "text", Text: &want}) {
				t.Fatalf("field caller got %#v %v want %q", cell, err, want)
			}
		})
	}
	for _, function := range []string{"SVSmr", "SVBioGeoClimaticUnit", "SVBedrockType", "SVSiteDisturbance", "", "svSnr"} {
		if field, ok := summaryVegetationEnvironmentalTextField(function); field != "" || ok {
			t.Fatal("unsupported function mapped", function, field, ok)
		}
		if cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), function, &unit, env, su); err == nil || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
			t.Fatal("unsupported function returned output", function, cell, err)
		}
	}
}

func TestSummaryEnvironmentalCategoryDirectBatchOriginalFixtures(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "summary_environment_category_direct_batch_source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Failure      json.RawMessage `json:"failure"`
		Observations []struct {
			Function string   `json:"function"`
			Unit     *string  `json:"unit"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if string(receipt.Failure) != "null" || len(receipt.Observations) != 88 {
		t.Fatal("complete original88-case fixture required")
	}
	labels := map[string][4]string{
		"SVHydroGeoSubSystem":     {"A", "B", "Z", "C"},
		"SVHydroGeoSystem":        {"B", "C", "A", "D"},
		"SVSoilDrainage":          {"C", "D", "B", "E"},
		"SVHumusForm":             {"D", "E", "C", "F"},
		"SVRootRestrictingType":   {"E", "F", "D", "G"},
		"SVSoilClassSubGroup":     {"F", "G", "E", "H"},
		"SVSoilClassGroup":        {"G", "H", "F", "I"},
		"SVRootZonePartSize":      {"H", "I", "G", "J"},
		"SVSurficialMaterialSurf": {"I", "J", "H", "K"},
		"SVExposure":              {"J", "K", "I", "L"},
		"SVBioGeoClimaticZone":    {"K", "L", "J", "M"},
	}
	seen := make(map[string]bool)
	for _, observation := range receipt.Observations {
		binding, ok := labels[observation.Function]
		if !ok || len(observation.Actual) != 3 {
			t.Fatal("unexpected original function/result", observation)
		}
		unit := "<NULL>"
		var categories []summaryVegetationEnvironmentalCategory
		var nulls int64
		if observation.Unit != nil {
			unit = *observation.Unit
			switch unit {
			case "mixed", "MIXED":
				categories = []summaryVegetationEnvironmentalCategory{{"", 1}, {binding[0], 1}, {binding[1], 3}}
				nulls = 1
			case "allnull":
				nulls = 2
			case "":
				categories = []summaryVegetationEnvironmentalCategory{{binding[2], 1}}
			case "plain":
				categories = []summaryVegetationEnvironmentalCategory{{binding[3], 1}}
			case "missing", "noenv":
			default:
				t.Fatal("unexpected original unit", unit)
			}
		}
		key := observation.Function + "/" + unit
		if seen[key] {
			t.Fatal("duplicate original witness", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			cell, err := formatSummaryVegetationEnvironmentalCategories(t.Context(), observation.Function, observation.Unit, categories, nulls)
			if observation.Unit == nil {
				if err == nil || !strings.Contains(err.Error(), "error94") || observation.Actual[0] != "error" ||
					observation.Actual[1] != "94" || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
					t.Fatal("original NULL failure changed", cell, err)
				}
				return
			}
			want := observation.Actual[2]
			if err != nil || observation.Actual[0] != "ok" || observation.Actual[1] != "String" ||
				!reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "text", Text: &want}) {
				t.Fatal("original typed direct category output changed", cell, err)
			}
		})
	}
}
