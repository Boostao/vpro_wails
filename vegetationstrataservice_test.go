package main

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLongVegetationStrataTwoSingleAssignmentsAndFiniteHeldFields(t *testing.T) {
	prepared := strataFixture(map[string]ProjectMetadataCell{
		"Cover1": strataReal(0.1), "Cover2": strataReal(0.2), "Cover6": strataReal(0.1),
	})
	before := strataFixture(map[string]ProjectMetadataCell{
		"Cover1": strataReal(0.1), "Cover2": strataReal(0.2), "Cover6": strataReal(0.1),
	})
	result, err := prepareLongVegetationStrataSingle(context.Background(), prepared)
	if err != nil || len(result.Observations) != 2 || !reflect.DeepEqual(prepared, before) {
		t.Fatal("SINGLE wrapper mutated borrowed preparation or lost rows", result, err)
	}
	want := map[string]float64{
		"1": float64(float32(float64(float32(0.1)) + float64(float32(0.2)))),
		"6": float64(float32(0.1)),
	}
	for _, row := range result.Observations {
		if row.Cover.Real == nil || *row.Cover.Real != want[row.Layer] {
			t.Fatal("held or inserted SINGLE assignment lost", row)
		}
	}
	overflow := strataFixture(map[string]ProjectMetadataCell{"Cover10": strataReal(math.MaxFloat64)})
	if value, err := prepareLongVegetationStrataSingle(context.Background(), overflow); err == nil ||
		!reflect.DeepEqual(value, VegetationReportPreparation{}) {
		t.Fatal("unused but physically held overflow was silently repaired", value, err)
	}
}

func TestLongVegetationStrataPhysicalJoinsAndNullableLabels(t *testing.T) {
	prepared := strataFixture(map[string]ProjectMetadataCell{"Cover1": strataReal(10)})
	prepared.Memberships = []VegetationReportMembership{
		{"1", metadataText("P"), metadataText("U")},
		{"2", metadataText("P"), metadataText("U")},
		{"3", metadataText("Q"), metadataText("U")},
	}
	species := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"},
	}, Rows: []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText("S"), metadataText("Species"), metadataText("Name"), metadataText("U")}},
		{"2", []ProjectMetadataCell{metadataText("S"), metadataText("Species"), metadataText("Name"), metadataText("U")}},
		{"3", []ProjectMetadataCell{metadataText("S"), metadataText("Excluded"), metadataText("Name"), metadataText("s")}},
	}}
	layers := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Strata"}},
		Rows: []ProjectMetadataRow{
			{"1", []ProjectMetadataCell{metadataText("1"), metadataText("A")}},
			{"2", []ProjectMetadataCell{metadataText("1"), metadataText("A")}},
		}}
	options := layerTestOptions()
	options.ConstantSpeciesList = false
	options.PresenceGreaterThan, options.MeanCoverGreaterThan = -1, -999
	report, err := planLongVegetationStrata(context.Background(), prepared, species, layers, options)
	if err != nil || len(report.Units) != 1 || len(report.Units[0].Rows) != 1 {
		t.Fatal(report, err)
	}
	unit, row := report.Units[0], report.Units[0].Rows[0]
	if unit.NumPlots != 3 || len(unit.MembershipIDs) != 3 || *row.Layer.Text != "A" ||
		*row.Species.Text != "Species" || *row.Presence != 1.0/3 || *row.MeanCover != 80.0/3 ||
		len(row.Plots) != 1 || *row.Plots[0].Cover != 80 {
		t.Fatal("physical fanout or source Strata display lost", unit)
	}
	options.Average = "observations"
	observations, err := planLongVegetationStrata(context.Background(), prepared, species, layers, options)
	if err != nil || *observations.Units[0].Rows[0].MeanCover != 10 {
		t.Fatal("characteristic average lost joined multiplicity", observations, err)
	}
	layers.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "null"}
	layers.Rows = layers.Rows[:1]
	options.ConstantSpeciesList = true
	null, err := planLongVegetationStrata(context.Background(), prepared, species, layers, options)
	if err != nil || null.Units[0].Rows[0].Layer.Storage != "null" || null.Units[0].Rows[0].MeanCover != nil {
		t.Fatal("NULL constant-list key became a matching Strata", null, err)
	}
	layers.Rows[0].Cells[1] = metadataInteger("1")
	if value, err := planLongVegetationStrata(context.Background(), prepared, species, layers, options); err == nil ||
		!reflect.DeepEqual(value, vegetationLayerReport{}) {
		t.Fatal("physical Strata INTEGER was coerced to a label", value, err)
	}
}

func TestLongVegetationStrataIndependentGateAndOwnedInternalExternalReads(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["LVGroupBy"] = 2
	options, err := decodeLongVegetationOptions(values)
	if err != nil || !options.StrataGrouping || options.NoneGrouping || options.LifeformGrouping ||
		vegetationSettings(options).Grouping != "strata" {
		t.Fatal("source grouping2 lost identity", options, err)
	}
	if _, err := siviFeature(longVegetationStrataFeatureEnvironment,
		func(string) (string, bool) { return "TRUE", true }); err == nil {
		t.Fatal("malformed gate accepted")
	}
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "internal", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 2}); err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			for _, other := range []bool{false, true} {
				service.longVegetationLifeformEnabled, service.longVegetationNoneEnabled = other, other
				if _, err := service.GetLongVegetationOptions(context.Background(), state.ContextID); err == nil ||
					!strings.Contains(err.Error(), "disabled in this session") {
					t.Fatal("independent Strata options gate bypassed", err)
				}
				if _, err := service.PreviewLongVegetation(context.Background(), state.ContextID); err == nil {
					t.Fatal("independent preview gate bypassed")
				}
			}
			service.longVegetationStrataEnabled = true
			preview, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || preview.Settings.Grouping != "strata" || len(preview.Report.Units) == 0 {
				t.Fatal(preview, err)
			}
			again, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(again, preview) {
				t.Fatal("repeated owned Strata preview differs", again, err)
			}
			if _, err := service.PreviewLongVegetation(context.Background(), "stale"); err == nil {
				t.Fatal("stale context accepted")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := service.PreviewLongVegetation(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled owned Strata accepted", err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("read-only Strata changed retained options", err)
			}
			for _, flags := range [][3]bool{{true, true, false}, {true, false, true}, {false, true, true}} {
				options.NoneGrouping, options.LifeformGrouping, options.StrataGrouping = flags[0], flags[1], flags[2]
				if err := service.checkLongVegetationGrouping(options); err == nil {
					t.Fatal("conflicting source grouping accepted")
				}
			}
		})
	}
}
