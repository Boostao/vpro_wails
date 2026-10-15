package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func projectReportFixture(t *testing.T, none bool) (vegetationProjectPreparation, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	veg, _, _, env, admin := projectEntryFixture(t)
	_, master, layers := vegetationLayerFixture(t)
	var project vegetationProjectPreparation
	var err error
	if none {
		project, err = prepareLongVegetationProjectNoneUnits(context.Background(), "Project", veg, layers, env)
	} else {
		project, err = prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "None",
			veg, layers, env, admin, ProjectMetadataTable{}, []string{"Unit"}, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return project, master, layers
}

func TestProjectReportLayerOriginalDenominatorsAndPivots(t *testing.T) {
	for _, none := range []bool{false, true} {
		project, master, layers := projectReportFixture(t, none)
		prepared, err := projectReportPreparation(context.Background(), project)
		if err != nil || len(prepared.Memberships) != 0 || prepared.selectedUnitOccurrences != nil ||
			prepared.qualityMembershipCounts != nil || prepared.noneUnitMembershipCounts != nil {
			t.Fatal("project bridge invented physical SU rows/weights", prepared, err)
		}
		report, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
		if err != nil || report.ProjectUnits == nil {
			t.Fatal(report, err)
		}
		counts := map[string]int{"text:U": 2, "text:V": 2, "null": 1, "text:": 1}
		if none {
			counts = map[string]int{"text:All plots": 6}
		}
		if len(report.Units) != len(counts) {
			t.Fatal("original unit scope differs", none, report)
		}
		for _, unit := range report.Units {
			if unit.NumPlots != counts[vegetationTextKey(unit.Code)] || len(unit.MembershipIDs) != 0 ||
				len(unit.ProjectUnitIndexes) != unit.NumPlots {
				t.Fatal("original project tuple denominators/indexes became SU memberships", none, unit)
			}
			for _, index := range unit.ProjectUnitIndexes {
				if index < 0 || index >= len(report.ProjectUnits) ||
					!reflect.DeepEqual(report.ProjectUnits[index].SiteUnit, unit.Code) {
					t.Fatal("unit index lost original project tuple", unit, report.ProjectUnits)
				}
			}
			if vegetationTextKey(unit.Code) != "text:U" && vegetationTextKey(unit.Code) != "text:V" && !none {
				continue
			}
			found := false
			for _, row := range unit.Rows {
				if row.Layer.Text == nil || *row.Layer.Text != "L1" || row.Species.Text == nil || *row.Species.Text != "Scientific A" {
					continue
				}
				found = true
				if none {
					if row.Presence == nil || math.Abs(*row.Presence-2.0/6) > 1e-15 ||
						row.MeanCover == nil || *row.MeanCover != 1501199875790164.8 ||
						len(row.Plots) != 3 || row.Plots[0].PlotNumber != "" || row.Plots[0].Cover != nil ||
						row.Plots[1].PlotNumber != "P1" || row.Plots[1].Cover == nil || *row.Plots[1].Cover != -4 ||
						row.Plots[2].PlotNumber != "P2" || row.Plots[2].Cover == nil || *row.Plots[2].Cover != 9007199254740992 {
						t.Fatal("source None denominator includes original NULL Env, cover joins weight duplicate Env only", row)
					}
				} else if row.Presence == nil || *row.Presence != .5 || row.MeanCover == nil || *row.MeanCover != -2 ||
					len(row.Plots) != 1 || row.Plots[0].Cover == nil || *row.Plots[0].Cover != -4 {
					t.Fatal("source2 project tuple fanout/presence/mean/pivot differs", row)
				}
			}
			if !found {
				t.Fatal("independent original species/layer row missing", none, unit)
			}
		}
		if dto, err := longVegetationTransport(context.Background(), report); err == nil || !reflect.DeepEqual(dto, LongVegetationReport{}) {
			t.Fatal("legacy selected-SU wire silently discarded original project provenance", dto, err)
		}
		*report.ProjectUnits[0].PlotNumber.Text = "changed"
		if *prepared.projectUnitInputs[0].PlotNumber.Text != "P1" {
			t.Fatal("report aliases prepared tuple")
		}
	}
}

func TestProjectReportAllAdaptersPreserveOriginalUnitSource(t *testing.T) {
	for _, none := range []bool{false, true} {
		for _, mode := range []string{"layer", "none", "strata", "lifeform"} {
			project, master, layers := projectReportFixture(t, none)
			prepared, err := projectReportPreparation(context.Background(), project)
			if err != nil {
				t.Fatal(err)
			}
			options := layerTestOptions()
			var report vegetationLayerReport
			switch mode {
			case "strata":
				layers.Columns = append(layers.Columns, ProjectMetadataColumn{Name: "Strata"})
				for i := range layers.Rows {
					layers.Rows[i].Cells = append(layers.Rows[i].Cells, metadataText("Stratum"))
				}
				report, err = planLongVegetationStrata(context.Background(), prepared, master, layers, options)
			case "lifeform":
				master = lifeformMasterFixture()
				union := lifeformUnionFixture(t, master, ProjectMetadataTable{Columns: master.Columns, Rows: []ProjectMetadataRow{}})
				report, err = planLongVegetationLifeforms(context.Background(), prepared, master, union, layers, options)
			default:
				options.NoneGrouping = mode == "none"
				report, err = planLongVegetationLayers(context.Background(), prepared, master, layers, options)
			}
			if err != nil || len(report.ProjectUnits) != len(prepared.projectUnitInputs) || len(report.Units) == 0 {
				t.Fatal("adapter lost typed project scope", none, mode, report, err)
			}
			for _, unit := range report.Units {
				expected := 1
				if none {
					expected = 6
				} else if unit.Code.Text != nil && (*unit.Code.Text == "U" || *unit.Code.Text == "V") {
					expected = 2
				}
				if unit.NumPlots != expected || len(unit.MembershipIDs) != 0 || len(unit.ProjectUnitIndexes) != expected {
					t.Fatal("adapter fabricated membership IDs or lost original tuples", none, mode, unit)
				}
			}
		}
	}
}

func TestProjectReportMixedForgedAndCancelledInputs(t *testing.T) {
	for _, mutate := range []func(*vegetationProjectPreparation){
		func(p *vegetationProjectPreparation) { p.Project = "" },
		func(p *vegetationProjectPreparation) { p.SourceSU = "" },
		func(p *vegetationProjectPreparation) { p.SelectedUnits = nil },
		func(p *vegetationProjectPreparation) { p.NoneUnits = []vegetationProjectNoneUnitOccurrence{} },
		func(p *vegetationProjectPreparation) { p.SelectedUnits[0].EnvRowID = "01" },
		func(p *vegetationProjectPreparation) { p.SelectedUnits[0].AdminRowID = "" },
		func(p *vegetationProjectPreparation) { p.SelectedUnits[1] = p.SelectedUnits[0] },
		func(p *vegetationProjectPreparation) {
			p.SelectedUnits[0].PlotNumber = ProjectMetadataCell{Storage: "null"}
		},
		func(p *vegetationProjectPreparation) { p.SelectedUnits[0].SiteUnit = metadataInteger("7") },
	} {
		project, _, _ := projectReportFixture(t, false)
		mutate(&project)
		got, err := projectReportPreparation(context.Background(), project)
		if err == nil || !reflect.DeepEqual(got, VegetationReportPreparation{}) {
			t.Fatal("forged or ambiguous original project prep accepted", got, err)
		}
	}
	for _, mutate := range []func(*VegetationReportPreparation){
		func(p *VegetationReportPreparation) { p.Memberships = []VegetationReportMembership{{RowID: "1"}} },
		func(p *VegetationReportPreparation) { p.selectedUnitOccurrences = []vegetationSelectedUnitOccurrence{} },
		func(p *VegetationReportPreparation) { p.qualityMembershipCounts = map[string]int{} },
		func(p *VegetationReportPreparation) { p.noneUnitMembershipCounts = map[string]int{} },
		func(p *VegetationReportPreparation) { p.projectUnitInputs[1] = p.projectUnitInputs[0] },
		func(p *VegetationReportPreparation) { p.projectUnitInputs[0].AdminRowID = nil },
	} {
		project, master, layers := projectReportFixture(t, false)
		prepared, err := projectReportPreparation(context.Background(), project)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&prepared)
		report, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
		if err == nil || !reflect.DeepEqual(report, vegetationLayerReport{}) {
			t.Fatal("mixed or forged kernel project scope accepted", report, err)
		}
	}
	project, master, layers := projectReportFixture(t, true)
	for _, run := range []func(context.Context) (bool, error){
		func(ctx context.Context) (bool, error) {
			p, err := projectReportPreparation(ctx, project)
			return reflect.DeepEqual(p, VegetationReportPreparation{}), err
		},
		func(ctx context.Context) (bool, error) {
			p, err := projectReportPreparation(context.Background(), project)
			if err != nil {
				return true, err
			}
			r, err := planLongVegetationLayers(ctx, p, master, layers, layerTestOptions())
			return reflect.DeepEqual(r, vegetationLayerReport{}), err
		},
	} {
		probe := &projectUnitCancellationContext{Context: context.Background()}
		if empty, err := run(probe); err != nil || empty {
			t.Fatal("project kernel successful probe failed", probe, err)
		}
		for step := 1; step <= probe.calls; step++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: step}
			if empty, err := run(ctx); !errors.Is(err, context.Canceled) || !empty {
				t.Fatal("project kernel cancellation leaked output", step, empty, err)
			}
		}
	}
}
