package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestLongVegetationNoneUnitsSummaryRefusesForgedWitnessesAndDetaches(t *testing.T) {
	env := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P")}},
		},
	}
	su := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("Original")}}},
	}
	for _, qualified := range []bool{false, true} {
		makeInputs := func() (vegetationWorkbookSummary, LongVegetationReport) {
			summary, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-10", env, su, vprocore.ProjectMetadataTable{})
			if err != nil {
				t.Fatal(err)
			}
			report := LongVegetationReport{NoneUnits: []LongVegetationNoneUnitOccurrence{}}
			if qualified {
				report.Quality = &LongVegetationQualitySelection{Occurrences: []LongVegetationQualityOccurrence{{
					MembershipID: "11", PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprotest.MetadataText("Original"), EnvRowID: "1",
				}}}
			}
			for _, envID := range []string{"1", "2"} {
				occurrence := LongVegetationNoneUnitOccurrence{EnvRowID: envID, MembershipID: "11",
					PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprotest.MetadataText("All plots")}
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
			func(r *LongVegetationReport) { r.NoneUnits[0].PlotNumber = vprotest.MetadataText("changed") },
			func(r *LongVegetationReport) {
				r.NoneUnits[0].PlotNumber = vprocore.ProjectMetadataCell{Storage: "null"}
			},
			func(r *LongVegetationReport) { r.NoneUnits[0].SiteUnit = vprotest.MetadataText("Original") },
			func(r *LongVegetationReport) { r.NoneUnits[0].SiteUnit = vprocore.ProjectMetadataCell{Storage: "null"} },
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
