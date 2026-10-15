package main

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryEnvironmentalCategoryPreparationFixture() (ProjectMetadataTable, ProjectMetadataTable) {
	cell := func(value *string) ProjectMetadataCell {
		if value == nil {
			return ProjectMetadataCell{Storage: "null"}
		}
		return ProjectMetadataCell{Storage: "text", Text: value}
	}
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "PlotNumber"}, {Name: "StructuralStage"}, {Name: "SuccessionalStatus"},
		{Name: "SurfaceTopographyType"}, {Name: "NutrientRegime"}, {Name: "MesoSlopePosition"},
	}}
	for i, row := range []struct {
		plot  string
		label *string
	}{
		{"P1", nil}, {"P2", nil}, {"P3", nil}, {"P4", nil}, {"P5", nil},
		{"N1", nil}, {"N2", nil}, {"E1", nil}, {"C1", nil}, {"orphan", nil},
	} {
		var label *string
		if i != 2 && i != 5 && i != 6 {
			value := []string{"B", "A", "", "", "B", "", "", "Z", "C", "X"}[i]
			label = &value
		}
		plot := row.plot
		cells := []ProjectMetadataCell{cell(&plot)}
		for j := 0; j < 5; j++ {
			cells = append(cells, cell(label))
		}
		env.Rows = append(env.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: cells})
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for i, pair := range [][2]string{
		{"P1", "mixed"}, {"P1", "mixed"}, {"P2", "mixed"}, {"P3", "mixed"}, {"P4", "mixed"}, {"P5", "mixed"},
		{"N1", "allnull"}, {"N2", "allnull"}, {"E1", ""}, {"C1", "plain"}, {"absent", "noenv"}, {"", "mixed"},
	} {
		plot, unit := pair[0], pair[1]
		plotCell := cell(&plot)
		if i == 11 {
			plotCell = cell(nil)
		}
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: []ProjectMetadataCell{plotCell, cell(&unit)}})
	}
	return env, su
}

func TestSummaryEnvironmentalCategoryPreparationOriginalJoinWitnesses(t *testing.T) {
	env, su := summaryEnvironmentalCategoryPreparationFixture()
	beforeEnv, beforeSU := cloneSummaryVegetationFieldListTable(env), cloneSummaryVegetationFieldListTable(su)
	for _, function := range []string{"SVStructureStage", "SVSuccessional", "SVSurfaceTopographyType", "SVSnr", "SVMesoSlopePosition"} {
		for _, tc := range []struct{ unit, want string }{
			{"mixed", "(1)  A(1)  B(3)  (Null 1)"}, {"allnull", "(Null 2)"},
			{"missing", ""}, {"", "Z(1)  "}, {"plain", "C(1)  "}, {"noenv", ""},
		} {
			t.Run(function+"/"+tc.unit, func(t *testing.T) {
				cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), function, &tc.unit, env, su)
				if err != nil || !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "text", Text: &tc.want}) {
					t.Fatalf("got %#v %v want %q", cell, err, tc.want)
				}
			})
		}
	}
	if !reflect.DeepEqual(env, beforeEnv) || !reflect.DeepEqual(su, beforeSU) {
		t.Fatal("caller source changed")
	}
	unit := "mixed"
	env.Rows = append(env.Rows, ProjectMetadataRow{RowID: "11", Cells: env.Rows[0].Cells})
	cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", &unit, env, su)
	if err != nil || cell.Text == nil || *cell.Text != "(1)  A(1)  B(5)  (Null 1)" {
		t.Fatal("duplicate Env multiplicity collapsed", cell, err)
	}
	env.Rows[10].Cells = append([]ProjectMetadataCell(nil), env.Rows[10].Cells...)
	env.Rows[10].Cells[0] = ProjectMetadataCell{Storage: "null"}
	cell, err = prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", &unit, env, su)
	if err != nil || cell.Text == nil || *cell.Text != "(1)  A(1)  B(3)  (Null 1)" {
		t.Fatal("NULL Env join key matched", cell, err)
	}
}

func TestSummaryEnvironmentalCategoryPreparationLiteralAndRefusals(t *testing.T) {
	env, su := summaryEnvironmentalCategoryPreparationFixture()
	upper := "MIXED"
	cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", &upper, env, su)
	if err != nil || cell.Text == nil || *cell.Text != "" {
		t.Fatal("literal scope case was merged", cell, err)
	}
	unit := "mixed"
	lower, spaced := "b", " A "
	env.Rows[0].Cells[4] = ProjectMetadataCell{Storage: "text", Text: &lower}
	env.Rows[1].Cells[4] = ProjectMetadataCell{Storage: "text", Text: &spaced}
	cell, err = prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", &unit, env, su)
	if err != nil || cell.Text == nil || *cell.Text != "(1)   A (1)  B(1)  b(2)  (Null 1)" {
		t.Fatal("literal category spelling/ordering changed", cell, err)
	}
	for _, mutate := range []func(*ProjectMetadataTable, *ProjectMetadataTable){
		func(e, s *ProjectMetadataTable) { e.Columns[4].Name = "missing" },
		func(e, s *ProjectMetadataTable) { s.Rows[1].RowID = s.Rows[0].RowID },
		func(e, s *ProjectMetadataTable) { e.Rows[0].Cells = e.Rows[0].Cells[:1] },
		func(e, s *ProjectMetadataTable) {
			value := string([]byte{0xff})
			e.Rows[0].Cells[4] = ProjectMetadataCell{Storage: "text", Text: &value}
		},
		func(e, s *ProjectMetadataTable) {
			value := "1"
			s.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "integer", Integer: &value}
		},
	} {
		e, s := summaryEnvironmentalCategoryPreparationFixture()
		mutate(&e, &s)
		cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", &unit, e, s)
		if err == nil || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
			t.Fatal("invalid physical input returned output", cell, err)
		}
		e, s = summaryEnvironmentalCategoryPreparationFixture()
		if _, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", &unit, e, s); err != nil {
			t.Fatal("corrected retry", err)
		}
	}
	cell, err = prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVHumusDepth", &unit, env, su)
	if err == nil || !strings.Contains(err.Error(), "no calibrated") || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatal(cell, err)
	}
	cell, err = prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVSnr", nil, ProjectMetadataTable{}, ProjectMetadataTable{})
	if err == nil || !strings.Contains(err.Error(), "error94") || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatal(cell, err)
	}
	base, cancel := context.WithCancel(context.Background())
	ctx := &summaryEnvironmentalCategoryCancelContext{Context: base, cancel: cancel, at: 4}
	cell, err = prepareSummaryVegetationEnvironmentalCategories(ctx, "SVSnr", &unit, env, su)
	cancel()
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatal(cell, err)
	}
}

func TestSummaryEnvironmentalCategoryPreparationIndependentFieldBindings(t *testing.T) {
	text := func(value string) ProjectMetadataCell {
		return ProjectMetadataCell{Storage: "text", Text: &value}
	}
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "PlotNumber"}, {Name: "StructuralStage"}, {Name: "SuccessionalStatus"},
		{Name: "SurfaceTopographyType"}, {Name: "NutrientRegime"}, {Name: "MesoSlopePosition"},
	}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{text("P1"), text("A"), text("U"), text("T"), text("N"), text("M")}},
		{RowID: "2", Cells: []ProjectMetadataCell{text("P2"), text("B"), {Storage: "null"}, text(""), text("N"), text("X")}},
	}}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{text("P1"), text("unit")}},
		{RowID: "2", Cells: []ProjectMetadataCell{text("P1"), text("unit")}},
		{RowID: "3", Cells: []ProjectMetadataCell{text("P2"), text("unit")}},
	}}
	unit := "unit"
	for _, tc := range []struct{ function, want string }{
		{"SVStructureStage", "A(2)  B(1)  "},
		{"SVSuccessional", "U(2)  (Null 1)"},
		{"SVSurfaceTopographyType", "(1)  T(2)  "},
		{"SVSnr", "N(3)  "},
		{"SVMesoSlopePosition", "M(2)  X(1)  "},
	} {
		t.Run(tc.function, func(t *testing.T) {
			cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), tc.function, &unit, env, su)
			if err != nil || !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "text", Text: &tc.want}) {
				t.Fatalf("field binding got %#v %v want %q", cell, err, tc.want)
			}
		})
	}
}

func TestSummaryEnvironmentalCategoryPreparationLoopAvailability(t *testing.T) {
	unit, plot := "A", "P"
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "SiteUnit"}, {Name: "PlotNumber"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{{Storage: "text", Text: &unit}, {Storage: "text", Text: &plot}}},
	}}
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "StructuralStage"}}}
	for i := 0; i < 32766; i++ {
		label := strconv.Itoa(i)
		env.Rows = append(env.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: []ProjectMetadataCell{
			{Storage: "text", Text: &plot}, {Storage: "text", Text: &label},
		}})
	}

	cell, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVStructureStage", &unit, env, su)
	if err != nil || cell.Text == nil || strings.Count(*cell.Text, ")  ") != 32766 {
		t.Fatal("allowed distinct boundary", err)
	}
	label := "32766"
	env.Rows = append(env.Rows, ProjectMetadataRow{RowID: "32767", Cells: []ProjectMetadataCell{
		{Storage: "text", Text: &plot}, {Storage: "text", Text: &label},
	}})
	cell, err = prepareSummaryVegetationEnvironmentalCategories(t.Context(), "SVStructureStage", &unit, env, su)
	if err == nil || !strings.Contains(err.Error(), "large Integer category-loop") || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatal("uncalibrated source loop boundary became output", err)
	}
}
