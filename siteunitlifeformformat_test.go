package main

import (
	"math"
	"testing"
)

func TestSummaryLifeformSourceOneDecimalDAOFormat(t *testing.T) {
	for _, tc := range []struct {
		number float64
		text   string
	}{
		{0, "0.0"}, {math.Copysign(0, -1), "0.0"}, {0.0001, "0.0"}, {-0.0001, "0.0"},
		{0.05, "0.1"}, {-0.05, "-0.1"}, {0.15, "0.2"}, {-0.15, "-0.2"},
		{0.25, "0.3"}, {-0.25, "-0.3"}, {0.35, "0.4"}, {-0.35, "-0.4"},
		{0.45, "0.5"}, {-0.45, "-0.5"}, {1.05, "1.1"}, {-1.05, "-1.1"},
		{1.15, "1.2"}, {-1.15, "-1.2"}, {1.25, "1.3"}, {-1.25, "-1.3"},
		{1.0 / 3, "0.3"}, {-1.0 / 3, "-0.3"}, {98.95, "99.0"}, {-98.95, "-99.0"},
		{99, "99.0"}, {99.05, "99.1"}, {99.15, "99.2"},
		{999999999.95, "1000000000.0"}, {-999999999.95, "-1000000000.0"},
	} {
		if got := siteUnitFixedNumber(tc.number, 1, 1); got != tc.text {
			t.Fatal("DAO16/General1033 source 0.0 formatting differs", tc.number, got, tc.text)
		}
	}
}

func TestSummaryLifeformCaptionsGuardedMinimumAndBlankValues(t *testing.T) {
	minimum, mean, maximum := 0.0, 0.25, 1.05
	row := siteUnitLifeformCover{Lifeform: 13, PhysicalPlots: 2, DistinctPlots: 1, ValueCount: 1,
		Minimum: &minimum, Mean: &mean, Maximum: &maximum}
	got, err := formatSiteUnitLifeformCover(row)
	if err != nil || got != "0---0.3---1.1" {
		t.Fatal("literal guarded zero or source decimal precision changed", got, err)
	}
	row.DistinctPlots, row.ValueCount = 2, 2
	if got, err = formatSiteUnitLifeformCover(row); err != nil || got != "0.0---0.3---1.1" {
		t.Fatal("observed numeric zero must not become guarded literal zero", got, err)
	}
	if got, err = formatSiteUnitLifeformCover(siteUnitLifeformCover{Lifeform: 0}); err != nil || got != "" {
		t.Fatal("source absent lifeform changed to guessed zero values", got, err)
	}
	if len(siteUnitLifeformCaptions) != 14 || siteUnitLifeformCaptions[0] != "Genus-level and mixed" ||
		siteUnitLifeformCaptions[13] != "Macroalgae" {
		t.Fatal("source caption range changed")
	}
}

func TestSummaryLifeformFormattingRejectsInconsistentStatistics(t *testing.T) {
	value := 1.0
	for _, row := range []siteUnitLifeformCover{
		{Lifeform: -1}, {Lifeform: 14},
		{Lifeform: 1, Minimum: &value},
		{Lifeform: 1, Minimum: &value, Mean: &value, Maximum: &value},
		{Lifeform: 1, PhysicalPlots: 2, DistinctPlots: 1, ValueCount: 1, Minimum: &value, Mean: &value, Maximum: &value},
	} {
		if got, err := formatSiteUnitLifeformCover(row); err == nil || got != "" {
			t.Fatal("invalid statistics returned successful-shaped caption", row, got, err)
		}
	}
	for _, number := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		row := siteUnitLifeformCover{Lifeform: 1, PhysicalPlots: 1, DistinctPlots: 1, ValueCount: 1,
			Minimum: &number, Mean: &value, Maximum: &value}
		if got, err := formatSiteUnitLifeformCover(row); err == nil || got != "" {
			t.Fatal("nonfinite scalar formatted", got, err)
		}
	}
}
