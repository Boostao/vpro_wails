package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestSummarySpeciesListLifeformPhysicalWeightsAndNullableNames(t *testing.T) {
	input := summaryLifeformCoverFixture(t)
	got, err := planSiteUnitSpeciesList(context.Background(), input, siteUnitSpeciesListOptions{2, 1, 1, 0, 0})
	if err != nil || len(got) != 3 {
		t.Fatal("owned species list unavailable", got, err)
	}
	for _, unit := range got[:2] {
		if len(unit.Groups) < 1 || unit.Groups[0].Index != 1 || unit.Groups[0].Caption != "Coniferous Tree" ||
			len(unit.Groups[0].Rows) != 2 {
			t.Fatal("whole-row UNION or source lifeform grouping changed", unit)
		}
		for _, row := range unit.Groups[0].Rows {
			if row.Species != "A" || row.Cover != "36.00" || row.Presence != "100.0" ||
				!row.Included || row.CodeType == nil || *row.CodeType.Text != "U" || len(row.ReferenceRowIDs) != 1 {
				t.Fatal("final reference grouping or second SU weight changed", row)
			}
		}
	}
	empty := got[2].Groups[0].Rows[0]
	if empty.Species != "" || empty.Cover != "-5.00" || empty.Included ||
		empty.EnglishName.Text == nil || *empty.EnglishName.Text != "" {
		t.Fatal("empty species/name or historical negative cover changed", empty)
	}
	refs := input.References.Table
	row := refs.Rows[0]
	row.RowID = "999"
	row.Cells = append([]ProjectMetadataCell{}, row.Cells...)
	row.Cells[3] = ProjectMetadataCell{Storage: "null"}
	input.References.Table.Rows = append(append([]ProjectMetadataRow{}, refs.Rows...), row)
	nullable, err := planSiteUnitSpeciesList(context.Background(), input, siteUnitSpeciesListOptions{2, 2, 1, 0, 0})
	if err != nil || len(nullable[0].Groups[0].Rows) != 3 {
		t.Fatal("NULL/common-name grouping repaired or conflated", nullable, err)
	}
	if nullable[0].Groups[0].Rows[0].EnglishName.Storage != "null" {
		t.Fatal("NULL name was replaced with empty text", nullable)
	}
	*nullable[0].Groups[0].Rows[1].ScientificName.Text = "changed"
	*nullable[0].Groups[0].Rows[1].CodeType.Text = "changed"
	if *input.References.Table.Rows[0].Cells[1].Text == "changed" || *input.References.Table.Rows[0].Cells[4].Text == "changed" {
		t.Fatal("output aliases the owned reference table")
	}
}

func TestSummarySpeciesListLayerRawNotCappedAndHiddenCoverEightNine(t *testing.T) {
	input := summaryLifeformCoverFixture(t)
	input.Quick.Entries = []siteUnitQuickVegetationEntry{
		{PlotNumber: "P1", Species: metadataText("A"), Cover: 125, Layer: 1},
		{PlotNumber: "P1", Species: metadataText("A"), Cover: 10, Layer: 7},
		{PlotNumber: "P1", Species: metadataText("A"), Cover: 80, Layer: 8},
		{PlotNumber: "P1", Species: metadataText("A"), Cover: 90, Layer: 9},
	}
	input.Quick.Groups = []siteUnitQuickVegetationGroup{{PlotNumber: "P1", Species: metadataText("A"), MyCover: 99}}
	base := input.References.Table.Rows[0]
	other := base
	other.RowID = "991"
	other.Cells = append([]ProjectMetadataCell{}, base.Cells...)
	other.Cells[2] = metadataInteger("2")
	input.References.Table.Rows = []ProjectMetadataRow{base, other}
	layers, err := planSiteUnitSpeciesList(context.Background(), input, siteUnitSpeciesListOptions{1, 1, 1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(layers[0].Groups) != 2 || layers[0].Groups[0].Index != 1 || layers[0].Groups[1].Index != 7 {
		t.Fatal("source layers1..7 loop widened", layers)
	}
	row := layers[0].Groups[0].Rows[0]
	if len(layers[0].Groups[0].Rows) != 1 || row.Cover != "250.00" || row.Presence != "200.0" ||
		row.PhysicalValues != 4 || row.CodeType != nil || len(row.ReferenceRowIDs) != 2 {
		t.Fatal("raw layer cover/physical presence clamped or reference groups incorrectly split", row)
	}
	present, err := planSiteUnitSpeciesList(context.Background(), input, siteUnitSpeciesListOptions{1, 2, 1, 0, 0})
	if err != nil || present[0].Groups[0].Rows[0].Cover != "125.00" {
		t.Fatal("present average did not use final physical value count", present, err)
	}
	forms, err := planSiteUnitSpeciesList(context.Background(), input, siteUnitSpeciesListOptions{2, 1, 1, 0, 0})
	if err != nil || len(forms[0].Groups) != 2 || forms[0].Groups[0].Rows[0].Cover != "99.00" ||
		forms[0].Groups[0].Rows[0].Presence != "100.0" {
		t.Fatal("lifeform final grouping borrowed raw layer reduction", forms, err)
	}
}

func TestSummarySpeciesListRoundedStrictThresholdsAndEmptyHeaders(t *testing.T) {
	input := summaryLifeformCoverFixture(t)
	input.Environment.Report.Units = []SiteUnitSummaryUnit{{Code: "U", Plots: make([]SiteUnitSummaryPlot, 2001)}}
	input.Quick.Memberships = make([]VegetationReportMembership, 1001)
	for i := range input.Quick.Memberships {
		input.Quick.Memberships[i] = VegetationReportMembership{PlotNumber: metadataText("P1"), SiteUnit: metadataText("U")}
	}
	input.References.Table.Rows = input.References.Table.Rows[:1]
	for _, cover := range []float64{1.0049, 1.005} {
		input.Quick.Groups = []siteUnitQuickVegetationGroup{{PlotNumber: "P1", Species: metadataText("A"), MyCover: cover}}
		for _, operator := range []int{1, 2} {
			got, err := planSiteUnitSpeciesList(context.Background(), input, siteUnitSpeciesListOptions{2, 2, operator, 50, 1})
			if err != nil || len(got[0].Groups) != 1 || len(got[0].Groups[0].Rows) != 1 {
				t.Fatal("pre-threshold header disappeared", got, err)
			}
			row := got[0].Groups[0].Rows[0]
			expected := cover == 1.005 && operator == 2
			if row.Included != expected || row.Presence != "50.0" {
				t.Fatal("strict numeric threshold was tested before source Format or AND/OR changed", cover, operator, row)
			}
		}
	}
}

func TestSummarySpeciesListOptionsRefusalCancellationAndRetry(t *testing.T) {
	input := summaryLifeformCoverFixture(t)
	options := siteUnitSpeciesListOptions{2, 1, 1, 0, 0}
	for _, invalid := range []siteUnitSpeciesListOptions{
		{0, 1, 1, 0, 0}, {3, 1, 1, 0, 0}, {2, 0, 1, 0, 0},
		{2, 3, 1, 0, 0}, {2, 1, 0, 0, 0}, {2, 1, 3, 0, 0},
		{2, 1, 1, -32769, 0}, {2, 1, 1, 0, 32768},
	} {
		if got, err := planSiteUnitSpeciesList(context.Background(), input, invalid); err == nil || got != nil {
			t.Fatal("invalid source choices accepted", invalid, got, err)
		}
	}
	for _, mutate := range []func(*siteUnitQuickVegetationInput){
		func(v *siteUnitQuickVegetationInput) { v.Quick.Project = "foreign" },
		func(v *siteUnitQuickVegetationInput) { v.Quick.SU = "foreign" },
		func(v *siteUnitQuickVegetationInput) {
			v.Environment.Report.Units = append(v.Environment.Report.Units, v.Environment.Report.Units[0])
		},
		func(v *siteUnitQuickVegetationInput) { v.Environment.Report.Units[0].Plots = nil },
		func(v *siteUnitQuickVegetationInput) { v.Quick.Groups[0].MyCover = math.NaN() },
		func(v *siteUnitQuickVegetationInput) { v.Quick.Groups[0].MyCover = math.Inf(1) },
	} {
		value := summaryLifeformCoverFixture(t)
		mutate(&value)
		if got, err := planSiteUnitSpeciesList(context.Background(), value, options); err == nil || got != nil {
			t.Fatal("malformed/foreign preparation returned partial output", got, err)
		}
	}
	if got, err := planSiteUnitSpeciesList(nil, input, options); err == nil || got != nil {
		t.Fatal("missing context accepted", got, err)
	}
	for _, remaining := range []int{2, 8, 16, 30} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		if got, err := planSiteUnitSpeciesList(ctx, input, options); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled result returned partial output", remaining, got, err)
		}
	}
	got, err := planSiteUnitSpeciesList(context.Background(), input, options)
	if err != nil || reflect.DeepEqual(got, []siteUnitSpeciesListUnit{}) {
		t.Fatal("retry failed", got, err)
	}
}
