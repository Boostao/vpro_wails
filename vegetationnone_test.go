package main

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"
)

func TestLongVegetationNoneRetainsLayerStatisticsAndSourceOrdering(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	options := layerTestOptions()
	options.ConstantSpeciesList = false
	options.PresenceGreaterThan, options.MeanCoverGreaterThan = -1, -999
	for _, order := range []string{"species", "presence"} {
		options.Order = order
		options.NoneGrouping = false
		layer, err := planLongVegetationLayers(context.Background(), prepared, species, layers, options)
		if err != nil {
			t.Fatal(err)
		}
		options.NoneGrouping = true
		none, err := planLongVegetationLayers(context.Background(), prepared, species, layers, options)
		if err != nil {
			t.Fatal(err)
		}
		for i, unit := range none.Units {
			if unit.NumPlots != layer.Units[i].NumPlots || len(unit.Rows) != len(layer.Units[i].Rows) {
				t.Fatal("None pooled or removed layer observations", unit)
			}
			for _, row := range unit.Rows {
				found := false
				for _, original := range layer.Units[i].Rows {
					if reflect.DeepEqual(original, row) {
						found = true
						break
					}
				}
				if !found {
					t.Fatal("None changed an existing statistic/projection", row)
				}
			}
			for j := 1; j < len(unit.Rows); j++ {
				a, b := unit.Rows[j-1], unit.Rows[j]
				if order == "presence" && *a.Presence != *b.Presence {
					if *a.Presence < *b.Presence {
						t.Fatal("source None presence order differs", unit.Rows)
					}
				} else if vegetationTextKey(a.Species) > vegetationTextKey(b.Species) {
					t.Fatal("source None species order differs", unit.Rows)
				}
			}
		}
	}
	options.ConstantSpeciesList = true
	options.NoneGrouping = false
	layer, err := planLongVegetationLayers(context.Background(), prepared, species, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	options.NoneGrouping = true
	none, err := planLongVegetationLayers(context.Background(), prepared, species, layers, options)
	if err != nil || !reflect.DeepEqual(layer, none) {
		t.Fatal("constant list lost source layer-first ordering", err)
	}
}

func TestLongVegetationNoneDecodeAndIndependentPublicGate(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["LVGroupBy"] = 4
	options, err := decodeLongVegetationOptions(values)
	if err != nil || !options.NoneGrouping || vegetationSettings(options).Grouping != "none" {
		t.Fatal("source mode4 unavailable", options, err)
	}
	for _, external := range []bool{false, true} {
		service, state := reportServiceFixture(t, external)
		if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 4}); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		if result, err := service.GetLongVegetationOptions(context.Background(), state.ContextID); err == nil || !reflect.DeepEqual(result, LongVegetationOptions{}) {
			t.Fatal("default options gate enabled", result, err)
		}
		if result, err := service.PreviewLongVegetation(context.Background(), state.ContextID); err == nil || !reflect.DeepEqual(result, LongVegetationPreview{}) {
			t.Fatal("default preview gate enabled", result, err)
		}
		if result, err := service.previewLongVegetationLayers(context.Background(), state.ContextID); err == nil || !reflect.DeepEqual(result, vegetationLayerReport{}) {
			t.Fatal("private context entry bypassed gate", result, err)
		}
		service.longVegetationNoneEnabled = true
		result, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
		if err != nil || result.Settings.Grouping != "none" {
			t.Fatal("enabled owned None preview failed", result, err)
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("None read repaired retained preferences", err)
		}
	}
	if enabled, err := siviFeature(longVegetationNoneFeatureEnvironment, func(string) (string, bool) { return "", false }); err != nil || enabled {
		t.Fatal("default gate enabled", err)
	}
	if _, err := siviFeature(longVegetationNoneFeatureEnvironment, func(string) (string, bool) { return "TRUE", true }); err == nil {
		t.Fatal("malformed gate accepted")
	}
}
