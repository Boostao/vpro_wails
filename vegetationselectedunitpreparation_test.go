package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func selectedUnitReportFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	veg, su, _ := vegetationReportFixture(t)
	_, master, layers := vegetationLayerFixture(t)
	su.Rows = su.Rows[:2]
	layers.Columns = append(layers.Columns, ProjectMetadataColumn{Name: "Strata"})
	for i := range layers.Rows {
		layers.Rows[i].Cells = append(layers.Rows[i].Cells, metadataText("Stratum"))
	}
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Alpha")}},
			{RowID: "12", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Beta")}},
		}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "22", Cells: []ProjectMetadataCell{metadataText("P1")}},
		}}
	return veg, su, layers, master, env, admin
}

func TestSelectedUnitPreparationAllAdaptersPreserveDerivedLabelsAndOriginalIDs(t *testing.T) {
	veg, su, layers, master, env, admin := selectedUnitReportFixture(t)
	prepared, occurrences, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, nil)
	if err != nil || len(occurrences) != 8 || len(prepared.Memberships) != 2 ||
		prepared.noneUnitMembershipCounts != nil || prepared.qualityMembershipCounts != nil {
		t.Fatal("source2 original scope/derived2SU*2Env*2Admin", prepared, occurrences, err)
	}
	if *prepared.Memberships[0].SiteUnit.Text != "U" || *su.Rows[0].Cells[1].Text != "U" {
		t.Fatal("source2 changed original physical unit")
	}
	*occurrences[0].SiteUnit.Text = "mutated"
	*occurrences[0].EnvRowID = "mutated"
	*occurrences[0].PlotNumber.Text = "mutated"
	if *prepared.selectedUnitOccurrences[0].SiteUnit.Text != "Alpha" ||
		*prepared.selectedUnitOccurrences[0].EnvRowID != "11" || *prepared.selectedUnitOccurrences[0].PlotNumber.Text != "P1" {
		t.Fatal("returned witness aliases prepared/source")
	}
	for _, mode := range []string{"layer", "none", "strata", "lifeform"} {
		t.Run(mode, func(t *testing.T) {
			options := layerTestOptions()
			var result vegetationLayerReport
			var err error
			switch mode {
			case "strata":
				result, err = planLongVegetationStrata(context.Background(), prepared, master, layers, options)
			case "lifeform":
				lifeform := lifeformMasterFixture()
				union := lifeformUnionFixture(t, lifeform, ProjectMetadataTable{Columns: lifeform.Columns})
				result, err = planLongVegetationLifeforms(context.Background(), prepared, lifeform, union, layers, options)
			default:
				options.NoneGrouping = mode == "none"
				result, err = planLongVegetationLayers(context.Background(), prepared, master, layers, options)
			}
			if err != nil || len(result.Units) != 2 {
				t.Fatal("adapter lost two derived labels", mode, result, err)
			}
			for i, unit := range result.Units {
				if unit.Code.Text == nil || *unit.Code.Text != []string{"Alpha", "Beta"}[i] || unit.NumPlots != 4 ||
					!reflect.DeepEqual(unit.MembershipIDs, []string{"1", "2"}) {
					t.Fatal("derived joins must count4 but retain2 unique physical IDs per label", unit)
				}
			}
			if mode == "layer" {
				for _, unit := range result.Units {
					found := false
					for _, row := range unit.Rows {
						if row.Layer.Text != nil && *row.Layer.Text == "L1" && row.Species.Text != nil && *row.Species.Text == "Scientific A" {
							found = true
							if row.Presence == nil || *row.Presence != .25 || row.MeanCover == nil || *row.MeanCover != -2 ||
								len(row.Plots) != 1 || row.Plots[0].Cover == nil || *row.Plots[0].Cover != -8 {
								t.Fatal("independent four-join per-unit statistics differ", unit.Code, row)
							}
						}
					}
					if !found {
						t.Fatal("expected literal layer row absent")
					}
				}
			}
			counts := map[string]int{}
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Code == "selected_unit_join_multiplicity" {
					counts[diagnostic.Identity] = diagnostic.Count
				}
				if diagnostic.Code == "physical_membership_multiplicity" && diagnostic.Count != 2 {
					t.Fatal("derived join count mislabeled physical membership count", diagnostic)
				}
			}
			if !reflect.DeepEqual(counts, map[string]int{"1": 4, "2": 4}) {
				t.Fatal("source2 diagnostics lost separate derived multiplicity", counts)
			}
		})
	}
}

func TestSelectedUnitPreparationQualityAndNullUnits(t *testing.T) {
	veg, su, layers, master, env, admin := selectedUnitReportFixture(t)
	quality := vegetationQualitySelection{}
	for i := 0; i < 8; i++ {
		quality.Occurrences = append(quality.Occurrences, vegetationQualityOccurrence{
			Membership: VegetationReportMembership{RowID: "1", PlotNumber: metadataText("P1"), SiteUnit: metadataText("U")},
			EnvRowID:   "11", AdminRowID: "21",
		})
	}
	env.Rows[1].Cells[1] = ProjectMetadataCell{Storage: "null"}
	prepared, joins, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, &quality)
	if err != nil || len(joins) != 8 || len(prepared.Memberships) != 2 {
		t.Fatal("source2 quality existence retains physical peers, no64-fold weighting", prepared, joins, err)
	}
	result, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
	if err != nil || len(result.Units) != 2 {
		t.Fatal("nullable derived label lost", result, err)
	}
	for _, unit := range result.Units {
		if unit.NumPlots != 4 || !reflect.DeepEqual(unit.MembershipIDs, []string{"1", "2"}) {
			t.Fatal("NULL label counts original nonnull plots without quality multiplication", unit)
		}
	}
	quality.Occurrences = nil
	prepared, joins, err = prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, &quality)
	if err != nil || joins == nil || len(joins) != 0 || len(prepared.Memberships) != 0 {
		t.Fatal("empty quality fallback", prepared, joins, err)
	}
}

func TestSelectedUnitDestinationRefusesUnsafeExcludedRows(t *testing.T) {
	for _, edit := range []func(*ProjectMetadataTable, *ProjectMetadataTable){
		func(e, s *ProjectMetadataTable) { e.Rows[0].Cells[1] = metadataText("") },
		func(e, s *ProjectMetadataTable) { s.Rows[0].Cells[0] = metadataText("") },
		func(e, s *ProjectMetadataTable) { s.Rows[0].Cells[0] = metadataText("12345678") },
	} {
		veg, su, layers, _, env, admin := selectedUnitReportFixture(t)
		edit(&env, &su)
		for _, quality := range []*vegetationQualitySelection{nil, {}} {
			prepared, rows, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
				veg, su, layers, env, admin, []string{"Zone"}, quality)
			if err == nil || rows != nil || prepared.Project != "" {
				t.Fatal("unsafe destination normalized/dropped/pruned before refusal", prepared, rows, err)
			}
		}
	}
}

func TestSelectedUnitKernelWitnessGuardsAndCancellation(t *testing.T) {
	for _, mutate := range []func(*VegetationReportPreparation){
		func(p *VegetationReportPreparation) { p.qualityMembershipCounts = map[string]int{"1": 8} },
		func(p *VegetationReportPreparation) { p.noneUnitMembershipCounts = map[string]int{"1": 8} },
		func(p *VegetationReportPreparation) { p.selectedUnitOccurrences[0].MembershipRowID = "missing" },
		func(p *VegetationReportPreparation) { p.selectedUnitOccurrences[0].PlotNumber = metadataText("other") },
		func(p *VegetationReportPreparation) { p.selectedUnitOccurrences[0].AdminRowID = nil },
		func(p *VegetationReportPreparation) { p.selectedUnitOccurrences[0].SiteUnit = metadataText("") },
		func(p *VegetationReportPreparation) { *p.selectedUnitOccurrences[0].EnvRowID = "01" },
		func(p *VegetationReportPreparation) {
			p.selectedUnitOccurrences = append(p.selectedUnitOccurrences, p.selectedUnitOccurrences[0])
		},
	} {
		veg, su, layers, master, env, admin := selectedUnitReportFixture(t)
		prepared, _, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
			veg, su, layers, env, admin, []string{"Zone"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		mutate(&prepared)
		result, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
		if err == nil || result.Project != "" {
			t.Fatal("malformed source2 private witness accepted", result, err)
		}
	}
	veg, su, layers, _, env, admin := selectedUnitReportFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	prepared, rows, err := prepareLongVegetationSelectedUnits(ctx, "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, nil)
	if !errors.Is(err, context.Canceled) || prepared.Project != "" || rows != nil {
		t.Fatal("cancelled source2 constructor leaked result", prepared, rows, err)
	}
}
