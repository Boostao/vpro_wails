package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryEnvironmentalConsumerFields(t *testing.T, values [][]any) ProjectMetadataTable {
	t.Helper()
	table := ProjectMetadataTable{Columns: make([]ProjectMetadataColumn, 0, 7), Rows: []ProjectMetadataRow{}}
	for _, name := range summaryVegetationFieldListColumns {
		table.Columns = append(table.Columns, ProjectMetadataColumn{Name: name})
	}
	for i, row := range values {
		table.Rows = append(table.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: summaryFieldListExpected(t, row)})
	}
	return table
}

func TestSummaryEnvironmentalConsumerSelectedPhysicalOrderAndValues(t *testing.T) {
	env, su := summaryEnvironmentalCategoryPreparationFixture()
	fields := summaryEnvironmentalConsumerFields(t, [][]any{
		{int64(0), "Env", "n Plots", nil, int64(-1), "SVnPlots", nil},
		{int64(1), "Env", "Nutrient", "NutrientRegime", int64(-1), "SVSnr", nil},
		{int64(1), "Env", "duplicate", "NutrientRegime", int64(-1), "SVSnr", nil},
		{int64(-1), nil, "hidden", nil, int64(-1), "SVSnr", nil},
		{nil, nil, nil, nil, int64(-1), "SVSnr", nil},
		{int64(2), "Env", "disabled", nil, int64(0), "unknown", "wrong"},
	})
	before := cloneSummaryVegetationFieldListTable(fields)
	units := []summaryVegetationUnit{{Code: "mixed", NumPlots: 999}, {Code: "allnull", NumPlots: 999}}
	rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su)
	if err != nil || len(rows) != 5 {
		t.Fatal("source selected hidden/NULL rows omitted", rows, err)
	}
	ids := []string{"5", "4", "1", "2", "3"}
	for i, row := range rows {
		if row.Field.RowID != ids[i] || len(row.Field.Cells) != 7 || len(row.Values) != 2 {
			t.Fatal("stable selected order or complete shape changed", rows)
		}
		if i == 2 {
			first, second := "6", "2"
			if row.LabelColumn != 3 || !reflect.DeepEqual(row.Values, []ProjectMetadataCell{
				{Storage: "integer", Integer: &first}, {Storage: "integer", Integer: &second},
			}) {
				t.Fatal("source plot counts reused stale cache/Env join/dedup", row)
			}
		} else {
			first, second := "(1)  A(1)  B(3)  (Null 1)", "(Null 2)"
			if row.LabelColumn != 2 || !reflect.DeepEqual(row.Values, []ProjectMetadataCell{
				{Storage: "text", Text: &first}, {Storage: "text", Text: &second},
			}) {
				t.Fatal("joined categorical matrix changed", row)
			}
		}
	}
	wire, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []SummaryVegetationEnvironmentalRow
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(rows, decoded) {
		t.Fatal("full row/wire failed roundtrip", err)
	}
	*rows[2].Field.Cells[2].Text = "caller"
	*rows[3].Values[0].Text = "caller"
	if !reflect.DeepEqual(fields, before) || *rows[4].Values[0].Text == "caller" {
		t.Fatal("result aliased caller witness/peer calculation")
	}
}

func TestSummaryEnvironmentalConsumerUnsupportedAndCorrection(t *testing.T) {
	env, su := summaryEnvironmentalCategoryPreparationFixture()
	units := []summaryVegetationUnit{{Code: "mixed"}}
	for _, tc := range []struct {
		function  string
		number    any
		errorText string
	}{
		{"SVHumusDepth", nil, "unavailable"}, {"SVBedrockType", "4", "FieldNumber"},
		{"SVnPlots", "", "FieldNumber"}, {"SVSnr", int64(1), "FieldNumber"},
		{"unknown", nil, "unavailable"}, {"", nil, "unavailable"},
	} {
		t.Run(tc.function+"/"+tc.errorText, func(t *testing.T) {
			fields := summaryEnvironmentalConsumerFields(t, [][]any{
				{int64(0), "Env", "n Plots", nil, int64(-1), "SVnPlots", nil},
				{int64(1), "Env", "unsupported", nil, int64(-1), tc.function, tc.number},
			})
			rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su)
			if rows != nil || err == nil || !strings.Contains(err.Error(), tc.errorText) {
				t.Fatal("partial successful matrix or missing diagnostic", rows, err)
			}
			fields.Rows[1].Cells[4] = summaryFieldListExpected(t, []any{int64(0)})[0]
			if rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su); err != nil || len(rows) != 1 {
				t.Fatal("disabled unavailable function blocked correction", rows, err)
			}
		})
	}
	fields := summaryEnvironmentalConsumerFields(t, [][]any{{int64(0), "Env", "n Plots", nil, int64(-1), "SVnPlots", nil}})
	if rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, ProjectMetadataTable{}, su); err != nil || len(rows) != 1 {
		t.Fatal("count-only output acquired Env dependency", rows, err)
	}
}

func TestSummaryEnvironmentalConsumerInvalidFramesCancellation(t *testing.T) {
	env, su := summaryEnvironmentalCategoryPreparationFixture()
	fields := summaryEnvironmentalConsumerFields(t, [][]any{{int64(0), "Env", "n Plots", nil, int64(-1), "SVnPlots", nil}})
	for _, units := range [][]summaryVegetationUnit{
		nil, {{Code: "mixed"}, {Code: "mixed"}}, {{Code: string([]byte{0xff})}}, {{Code: "missing"}},
	} {
		if rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su); rows != nil || err == nil {
			t.Fatal("invalid/unavailable frame returned matrix", rows, err)
		}
	}
	units := []summaryVegetationUnit{{Code: "mixed"}}
	invalid := cloneSummaryVegetationFieldListTable(fields)
	invalid.Rows[0].RowID = "01"
	if rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), invalid, units, env, su); rows != nil || err == nil {
		t.Fatal("invalid witness returned matrix", rows, err)
	}
	if rows, err := planSummaryVegetationEnvironmentalRows(nil, fields, units, env, su); rows != nil || err == nil {
		t.Fatal("nil context returned matrix", rows, err)
	}
	base, cancel := context.WithCancel(context.Background())
	ctx := &summaryEnvironmentalCategoryCancelContext{Context: base, cancel: cancel, at: 5}
	rows, err := planSummaryVegetationEnvironmentalRows(ctx, fields, units, env, su)
	cancel()
	if rows != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight cancellation returned partial matrix", rows, err)
	}
	fields.Rows[0].Cells[4] = summaryFieldListExpected(t, []any{int64(0)})[0]
	rows, err = planSummaryVegetationEnvironmentalRows(t.Context(), fields, units, env, su)
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("explicit no selected fields changed", rows, err)
	}
}

func TestSummaryEnvironmentalConsumerEveryDirectFunction(t *testing.T) {
	unit, plot := "unit", "P"
	text := func(value string) ProjectMetadataCell { return ProjectMetadataCell{Storage: "text", Text: &value} }
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}}, Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{text(plot)}}}}
	fields := summaryEnvironmentalConsumerFields(t, nil)
	for i, binding := range []struct{ function, field, label string }{
		{"SVStructureStage", "StructuralStage", "A"}, {"SVSuccessional", "SuccessionalStatus", "B"},
		{"SVSurfaceTopographyType", "SurfaceTopographyType", "C"}, {"SVSnr", "NutrientRegime", "D"},
		{"SVMesoSlopePosition", "MesoSlopePosition", "E"}, {"SVHydroGeoSubSystem", "HydroGeoSubSystem", "F"},
		{"SVHydroGeoSystem", "HydroGeoSystem", "G"}, {"SVSoilDrainage", "SoilDrainage", "H"},
		{"SVHumusForm", "HumusForm", "I"}, {"SVRootRestrictingType", "RootRestrictingType", "J"},
		{"SVSoilClassSubGroup", "SoilClassSubGroup", "K"}, {"SVSoilClassGroup", "SoilClassGroup", "L"},
		{"SVRootZonePartSize", "RootZoneParticleSize", "M"}, {"SVSurficialMaterialSurf", "SurficialMaterialSurf", "N"},
		{"SVExposure", "Exposure1", "O"}, {"SVBioGeoClimaticZone", "Zone", "P"},
	} {
		env.Columns = append(env.Columns, ProjectMetadataColumn{Name: binding.field})
		env.Rows[0].Cells = append(env.Rows[0].Cells, text(binding.label))
		fields.Rows = append(fields.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: summaryFieldListExpected(t,
			[]any{int64(i), "Env", binding.label, nil, int64(-1), binding.function, nil})})
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}, Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{text(plot), text(unit)}}}}
	rows, err := planSummaryVegetationEnvironmentalRows(t.Context(), fields, []summaryVegetationUnit{{Code: unit}}, env, su)
	if err != nil || len(rows) != 16 {
		t.Fatal(rows, err)
	}
	for i, row := range rows {
		want := string(rune('A'+i)) + "(1)  "
		if len(row.Values) != 1 || row.Values[0].Text == nil || *row.Values[0].Text != want {
			t.Fatal("consumer field mapping differs", i, row)
		}
	}
}
