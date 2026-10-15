package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestLongVegetationNoneUnitsSummaryRefusesForgedWitnessesAndDetaches(t *testing.T) {
	env := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}},
		Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P")}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("P")}},
		},
	}
	su := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows:    []ProjectMetadataRow{{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P"), metadataText("Original")}}},
	}
	for _, qualified := range []bool{false, true} {
		makeInputs := func() (vegetationWorkbookSummary, LongVegetationReport) {
			summary, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-10", env, su, ProjectMetadataTable{})
			if err != nil {
				t.Fatal(err)
			}
			report := LongVegetationReport{NoneUnits: []LongVegetationNoneUnitOccurrence{}}
			if qualified {
				report.Quality = &LongVegetationQualitySelection{Occurrences: []LongVegetationQualityOccurrence{{
					MembershipID: "11", PlotNumber: metadataText("P"), SiteUnit: metadataText("Original"), EnvRowID: "1",
				}}}
			}
			for _, envID := range []string{"1", "2"} {
				occurrence := LongVegetationNoneUnitOccurrence{EnvRowID: envID, MembershipID: "11",
					PlotNumber: metadataText("P"), SiteUnit: metadataText("All plots")}
				if qualified {
					index := 0
					occurrence.QualityOccurrence = &index
				}
				report.NoneUnits = append(report.NoneUnits, occurrence)
			}
			return summary, report
		}
		summary, report := makeInputs()
		actual, err := vegetationWorkbookNoneUnitSummary(context.Background(), summary, env, report)
		if err != nil || actual.SelectedPlotRows != 2 || len(actual.Memberships) != 2 ||
			*summary.Memberships[0].SiteUnit.Text != "Original" || summary.SelectedPlotRows != 1 {
			t.Fatal("derived summary mutated or collapsed original membership", actual, summary, err)
		}
		*actual.Memberships[0].SiteUnit.Text = "changed"
		*actual.Memberships[0].NoneUnit.PlotNumber.Text = "changed"
		if qualified {
			*actual.Memberships[0].NoneUnit.QualityOccurrence = 9
			if *report.NoneUnits[0].QualityOccurrence != 0 || *actual.Memberships[1].NoneUnit.QualityOccurrence != 0 {
				t.Fatal("summary quality index aliases source or derived peer")
			}
		}
		if *report.NoneUnits[0].SiteUnit.Text != "All plots" || *report.NoneUnits[0].PlotNumber.Text != "P" ||
			*actual.Memberships[1].SiteUnit.Text != "All plots" || *actual.Memberships[1].NoneUnit.PlotNumber.Text != "P" {
			t.Fatal("summary aliases source or derived peer")
		}
		for _, mutate := range []func(*LongVegetationReport){
			func(r *LongVegetationReport) { r.NoneUnits = nil },
			func(r *LongVegetationReport) { r.NoneUnits[0].EnvRowID = "3" },
			func(r *LongVegetationReport) { r.NoneUnits[0].MembershipID = "12" },
			func(r *LongVegetationReport) { r.NoneUnits[0].PlotNumber = metadataText("changed") },
			func(r *LongVegetationReport) { r.NoneUnits[0].PlotNumber = ProjectMetadataCell{Storage: "null"} },
			func(r *LongVegetationReport) { r.NoneUnits[0].SiteUnit = metadataText("Original") },
			func(r *LongVegetationReport) { r.NoneUnits[0].SiteUnit = ProjectMetadataCell{Storage: "null"} },
			func(r *LongVegetationReport) { r.NoneUnits = append(r.NoneUnits, r.NoneUnits[0]) },
			func(r *LongVegetationReport) {
				index := -1
				r.NoneUnits[0].QualityOccurrence = &index
			},
			func(r *LongVegetationReport) {
				index := 1
				r.NoneUnits[0].QualityOccurrence = &index
			},
		} {
			summary, report := makeInputs()
			mutate(&report)
			if value, err := vegetationWorkbookNoneUnitSummary(context.Background(), summary, env, report); err == nil ||
				!reflect.DeepEqual(value, vegetationWorkbookSummary{}) {
				t.Fatal("forged summary emitted partial derived memberships", value, err)
			}
		}
		summary, report = makeInputs()
		if qualified {
			report.NoneUnits[0].QualityOccurrence = nil
		} else {
			index := 0
			report.NoneUnits[0].QualityOccurrence = &index
		}
		if value, err := vegetationWorkbookNoneUnitSummary(context.Background(), summary, env, report); err == nil ||
			!reflect.DeepEqual(value, vegetationWorkbookSummary{}) {
			t.Fatal("missing/invented quality link accepted", value, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		value, err := vegetationWorkbookNoneUnitSummary(ctx, vegetationWorkbookSummary{}, env, LongVegetationReport{})
		if err == nil || !reflect.DeepEqual(value, vegetationWorkbookSummary{}) ||
			ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("summary ignored missing/cancelled context", value, err)
		}
	}
}
