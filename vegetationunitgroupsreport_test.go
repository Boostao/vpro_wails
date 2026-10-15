package main

import (
	"context"
	"reflect"
	"testing"
)

func TestLongVegetationNoneUnitsPreparationAllAdaptersKeepOuterJoinWeights(t *testing.T) {
	veg, su, _ := vegetationReportFixture(t)
	_, master, layers := vegetationLayerFixture(t)
	layers.Columns = append(layers.Columns, ProjectMetadataColumn{Name: "Strata"})
	for i := range layers.Rows {
		layers.Rows[i].Cells = append(layers.Rows[i].Cells, metadataText("Stratum"))
	}
	su.Rows = su.Rows[:2]
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P1")}},
		{RowID: "2", Cells: []ProjectMetadataCell{metadataText("P1")}},
	}}
	prepared, occurrences, err := prepareLongVegetationNoneUnits(context.Background(), "Project", "Selected", veg, su, layers, env, nil)
	if err != nil || len(occurrences) != 4 || len(prepared.Memberships) != 2 ||
		!reflect.DeepEqual(prepared.noneUnitMembershipCounts, map[string]int{"1": 2, "2": 2}) || prepared.qualityMembershipCounts != nil {
		t.Fatal("preparation lost physical IDs or double weighted joins", prepared, occurrences, err)
	}
	if *su.Rows[0].Cells[1].Text != "U" || *prepared.Memberships[0].SiteUnit.Text != "All plots" {
		t.Fatal("None adaptation mutated original unit identity")
	}
	for _, mode := range []string{"layer", "none", "strata", "lifeform"} {
		t.Run(mode, func(t *testing.T) {
			options := layerTestOptions()
			var result vegetationLayerReport
			var err error
			switch mode {
			case "lifeform":
				lifeform := lifeformMasterFixture()
				personal := ProjectMetadataTable{Columns: lifeform.Columns, Rows: []ProjectMetadataRow{}}
				union := lifeformUnionFixture(t, lifeform, personal)
				result, err = planLongVegetationLifeforms(context.Background(), prepared, lifeform, union, layers, options)
			case "strata":
				result, err = planLongVegetationStrata(context.Background(), prepared, master, layers, options)
			default:
				options.NoneGrouping = mode == "none"
				result, err = planLongVegetationLayers(context.Background(), prepared, master, layers, options)
			}
			if err != nil || len(result.Units) != 1 {
				t.Fatal("group adapter lost derived scope", mode, result, err)
			}
			unit := result.Units[0]
			if unit.NumPlots != 4 || *unit.Code.Text != "All plots" || !reflect.DeepEqual(unit.MembershipIDs, []string{"1", "2"}) {
				t.Fatal("outer Env multiplicity lost or became invented physical records", unit)
			}
			if mode == "layer" {
				found := false
				for _, row := range unit.Rows {
					if row.Layer.Text != nil && *row.Layer.Text == "L1" && row.Species.Text != nil && *row.Species.Text == "Scientific A" {
						found = true
						if row.Presence == nil || *row.Presence != .25 || row.MeanCover == nil || *row.MeanCover != -2 ||
							len(row.Plots) != 1 || row.Plots[0].Cover == nil || *row.Plots[0].Cover != -8 {
							t.Fatal("independent outer weighted statistics/pivot differ", row)
						}
					}
				}
				if !found {
					t.Fatal("expected literal weighted layer row absent")
				}
			}
		})
	}
	prepared.qualityMembershipCounts = map[string]int{"1": 2, "2": 2}
	if _, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions()); err == nil {
		t.Fatal("None/QCLV counts were applied twice")
	}
}
