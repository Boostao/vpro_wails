package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageParentSourceVariantsAndDuplicateBindings(t *testing.T) {
	sources, err := twoPageParentSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		bindings, err := twoPageParentBindings(form)
		if err != nil {
			t.Fatal(err)
		}
		columns, controls := map[string]int{}, map[string]bool{}
		for _, binding := range bindings {
			columns[binding.Binding]++
			if controls[binding.ControlID] || binding.Implicit {
				t.Fatal("source instances collapsed or implicit target invented", binding)
			}
			controls[binding.ControlID] = true
		}
		if columns["PlotNumber"] != 3 {
			t.Fatal("source repeated plot identity evidence changed", columns["PlotNumber"])
		}
		wantColumns, wantAge, wantHeight, tree := 118, 2, 0, "SubVegAXL_BC"
		if strings.HasSuffix(form, "-CHARS") {
			wantColumns, wantAge, wantHeight, tree = 120, 1, 1, "SubVegAXL"
		}
		if len(columns) != wantColumns || columns["SV_StandAgeEstMeas"] != wantAge || columns["SV_StandHeightEstMeas"] != wantHeight {
			t.Fatal("normal duplicate age binding silently repaired or variants collapsed", columns)
		}
		root := sources[form].Forms[0]
		if root.Pages[0].Name != "Site/Veg" || root.Pages[1].Name != "Soil/Terrain" || len(root.Embedded) != 6 {
			t.Fatal("source grouping or six child instances changed", root.Pages, root.Embedded)
		}
		wantChildren := []string{tree, "SubVegCXL", "SubVegDXL", "frmVPicsXL", "SoilHumusXL", "SoilMineralXL"}
		for i, child := range root.Embedded {
			if child.Form != wantChildren[i] || !child.Resolved || !child.ReadOnly ||
				!reflect.DeepEqual(child.MasterFields, []string{"PlotNumber"}) || !reflect.DeepEqual(child.ChildFields, []string{"PlotNumber"}) {
				t.Fatal("source child identity/link/availability changed", child)
			}
		}
		bindings[0].Binding = "mutated"
		again, err := twoPageParentBindings(form)
		if err != nil || again[0].Binding == "mutated" {
			t.Fatal("caller mutated cached source registry", err)
		}
	}
}

func TestTwoPageParentRawProjectionPairsAndCloneSafety(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		env, admin := siviParentTables(t)
		env.Rows = append(env.Rows, env.Rows[0])
		env.Rows[1].RowID = "-9"
		admin.Rows = append(admin.Rows, admin.Rows[0])
		admin.Rows[1].RowID = "9223372036854775807"
		got, err := projectTwoPageParent(context.Background(), "owned", "Sample", "108050", form, env, admin)
		if err != nil || got == nil {
			t.Fatal(err)
		}
		expected := 121
		if strings.HasSuffix(form, "-CHARS") {
			expected = 122
		}
		if got.Form != form || got.Query != "USysEnv" || got.Membership != "literal-binary-inner-pairs" ||
			len(got.Bindings) != expected || len(got.Rows) != 4 ||
			!reflect.DeepEqual(got.EnvColumns, env.Columns) || !reflect.DeepEqual(got.AdminColumns, admin.Columns) ||
			!reflect.DeepEqual(got.Rows[0], siviParentRow{env.Rows[0], admin.Rows[0]}) {
			t.Fatal("physical raw provenance or duplicate-pair shape changed", got)
		}
		*got.Rows[0].Env.Cells[0].Text = "mutated"
		got.EnvColumns[0].DeclaredType = "mutated"
		if *got.Rows[1].Env.Cells[0].Text != "108050" || *env.Rows[0].Cells[0].Text != "108050" {
			t.Fatal("pairs share mutable original storage")
		}
	}
}

func TestTwoPageParentRefusesUnknownVariantAndCancelledOrUnownedProjection(t *testing.T) {
	env, admin := siviParentTables(t)
	for _, form := range []string{"", "fs882-8x6xl", "FS882-8x6XL ", "frmSIVIsite", "FS882-6x4XL"} {
		if got, err := projectTwoPageParent(context.Background(), "owned", "Sample", "108050", form, env, admin); err == nil || got != nil {
			t.Fatal("variant normalized or defaulted", form, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := projectTwoPageParent(ctx, "owned", "Sample", "108050", "FS882-8x6XL", env, admin); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled source projection published", got, err)
	}
	for _, identity := range [][2]string{{"", "Sample"}, {"owned", ""}} {
		if got, err := projectTwoPageParent(context.Background(), identity[0], identity[1], "108050", "FS882-8x6XL", env, admin); err == nil || got != nil {
			t.Fatal("unowned source projection published", got, err)
		}
	}
}

func TestTwoPageParentRequiresEveryAdditionalPhysicalOwnerAndLiteralMembership(t *testing.T) {
	for _, column := range []string{"ActiveLayerDepth", "SV_WaterTableCM", "SV_FullCruiseCard", "PlotSize",
		"ProvinceStateTerritory", "SiteUnitLongName", "GIS_BGC", "GIS_BGC_VER", "BEC_Use"} {
		env, admin := siviParentTables(t)
		for _, table := range []*ProjectMetadataTable{&env, &admin} {
			for i := range table.Columns {
				if table.Columns[i].Name == column {
					table.Columns[i].Name = "missing_" + column
				}
			}
		}
		if got, err := projectTwoPageParent(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin); err == nil || got != nil {
			t.Fatal("missing additional field completed or defaulted", column, got, err)
		}
	}
	env, admin := siviParentTables(t)
	for _, plot := range []string{"108050 ", "10805", "é", "e\u0301"} {
		got, err := projectTwoPageParent(context.Background(), "owned", "Sample", plot, "FS882-8x6XL", env, admin)
		if err != nil || got == nil || len(got.Rows) != 0 {
			t.Fatal("literal source identity trimmed or completed", plot, got, err)
		}
	}
}
