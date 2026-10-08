package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func summaryLifeformCoverFixture(t *testing.T) siteUnitQuickVegetationInput {
	t.Helper()
	veg, su, _ := vegetationReportFixture(t)
	quick, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", 2, veg, su)
	if err != nil {
		t.Fatal(err)
	}
	row := lifeformReferenceRow("A", "Exact", metadataInteger("1"), metadataText("Name"), metadataText("U"))
	master := lifeformReferenceFixture(row, row,
		lifeformReferenceRow("A", "Exact", metadataInteger("1"), metadataText("Different"), metadataText("U")),
		lifeformReferenceRow("A", "Excluded", metadataInteger("2"), metadataText("Name"), metadataText("s")),
		lifeformReferenceRow("A", "Excluded", metadataInteger("3"), metadataText("Name"), ProjectMetadataCell{Storage: "null"}),
		lifeformReferenceRow("", "Empty", metadataInteger("13"), metadataText(""), metadataText("U")))
	references, err := prepareVegetationLifeformReferences(context.Background(), master, lifeformReferenceFixture(row))
	if err != nil {
		t.Fatal(err)
	}
	return siteUnitQuickVegetationInput{
		Quick: quick, References: references,
		Environment: SiteUnitSummaryPreview{Report: SiteUnitSummaryReport{Project: "Project", SU: "Selected",
			Units: []SiteUnitSummaryUnit{
				{Code: "U", Plots: make([]SiteUnitSummaryPlot, 2)},
				{Code: "V", Plots: make([]SiteUnitSummaryPlot, 1)},
				{Code: "", Plots: make([]SiteUnitSummaryPlot, 1)},
			}}},
	}
}

func TestSummaryLifeformCoversPhysicalGuardSecondJoinAndWholeUnion(t *testing.T) {
	input := summaryLifeformCoverFixture(t)
	got, err := planSiteUnitLifeformCovers(context.Background(), input)
	if err != nil || len(got) != 3 {
		t.Fatal("source units unavailable", got, err)
	}
	for _, unit := range got {
		if len(unit.Rows) != 14 || unit.Rows[13].Lifeform != 13 {
			t.Fatal("source 0..13 range changed", unit)
		}
	}
	u, v, empty := got[0].Rows[1], got[1].Rows[1], got[2].Rows[13]
	if u.PhysicalPlots != 2 || u.DistinctPlots != 1 || u.ValueCount != 4 ||
		u.Minimum == nil || *u.Minimum != 0 || *u.Mean != 72 || *u.Maximum != 36 {
		t.Fatal("distinct-plot minimum guard, double SU join or whole-row UNION changed", u)
	}
	if v.PhysicalPlots != 1 || v.ValueCount != 2 || *v.Minimum != 36 || *v.Mean != 72 || *v.Maximum != 36 {
		t.Fatal("per-species extrema were pooled into a lifeform/plot total", v)
	}
	if u.Value != "0---72.0---36.0" || v.Value != "36.0---72.0---36.0" ||
		u.Caption != "Coniferous Tree" || empty.Caption != "Macroalgae" {
		t.Fatal("source labels/decimal strings or forced minimum changed", u, v, empty)
	}
	if *empty.Minimum != -5 || *empty.Mean != -5 || *empty.Maximum != -5 {
		t.Fatal("empty identities or negative historical covers repaired", empty)
	}
	for _, form := range []int{0, 2, 3, 12} {
		row := got[0].Rows[form]
		if row.Minimum != nil || row.Mean != nil || row.Maximum != nil || row.ValueCount != 0 {
			t.Fatal("missing/excluded lifeforms changed from source blank to zero", row)
		}
	}
	before := *input.Quick.Groups[0].Species.Text
	*got[0].Rows[1].Mean = 1
	if *input.Quick.Groups[0].Species.Text != before {
		t.Fatal("output changed preparation")
	}
}

func TestSummaryLifeformCoversAtomicCancellationAndOwnership(t *testing.T) {
	input := summaryLifeformCoverFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planSiteUnitLifeformCovers(ctx, input); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled source returned output", got, err)
	}
	for _, remaining := range []int{2, 8, 15, 30, 90} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		if got, err := planSiteUnitLifeformCovers(ctx, input); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("late cancellation returned partial units", remaining, got, err)
		}
	}
	foreign := input
	foreign.Quick.Project = "Other"
	if got, err := planSiteUnitLifeformCovers(context.Background(), foreign); err == nil || got != nil {
		t.Fatal("foreign preparation accepted", got, err)
	}
	duplicate := input
	duplicate.Environment.Report.Units = append([]SiteUnitSummaryUnit{}, input.Environment.Report.Units...)
	duplicate.Environment.Report.Units = append(duplicate.Environment.Report.Units, duplicate.Environment.Report.Units[0])
	if got, err := planSiteUnitLifeformCovers(context.Background(), duplicate); err == nil || got != nil {
		t.Fatal("duplicate owned units accepted", got, err)
	}
	got, err := planSiteUnitLifeformCovers(context.Background(), input)
	if err != nil || reflect.DeepEqual(got, []siteUnitLifeformCoverUnit{}) {
		t.Fatal("retry failed", got, err)
	}
}
