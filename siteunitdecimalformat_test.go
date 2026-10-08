package main

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestSiteUnitDecimalFormatSourceMasksAndConversion(t *testing.T) {
	for _, test := range []struct {
		value           float64
		cover, presence string
	}{
		{0, "0.00", "0.0"}, {.0001, "0.00", "0.0"}, {.0049, "0.00", "0.0"},
		{.005, "0.01", "0.0"}, {.015, "0.02", "0.0"}, {.025, "0.03", "0.0"},
		{.035, "0.04", "0.0"}, {.145, "0.15", "0.1"}, {1.005, "1.01", "1.0"},
		{1.015, "1.02", "1.0"}, {1.125, "1.13", "1.1"}, {1.0 / 3, "0.33", "0.3"},
		{98.995, "99.00", "99.0"}, {99, "99.00", "99.0"}, {99.005, "99.01", "99.0"},
		{999999999.995, "1000000000.00", "1000000000.0"},
	} {
		checkSourceDecimalMasks(t, test.value, test.cover, test.presence)
	}
	for _, test := range []struct {
		value                    float64
		decimal, cover, presence string
	}{
		{.14499999999999, ".14499999999999", "0.14", "0.1"},
		{.144999999999999, ".144999999999999", "0.14", "0.1"},
		{.1449999999999999, ".145", "0.15", "0.1"},
		{1.00499999999999, "1.00499999999999", "1.00", "1.0"},
		{1.004999999999999, "1.005", "1.01", "1.0"},
		{1.12499999999999, "1.12499999999999", "1.12", "1.1"},
		{1.124999999999999, "1.125", "1.13", "1.1"},
		{.14999999999999, ".14999999999999", "0.15", "0.1"},
		{.149999999999999, ".149999999999999", "0.15", "0.1"},
		{1.14999999999999, "1.14999999999999", "1.15", "1.1"},
		{1.149999999999999, "1.15", "1.15", "1.2"},
	} {
		if got := strings.TrimPrefix(strconv.FormatFloat(test.value, 'g', 15, 64), "0"); got != test.decimal {
			t.Fatal("DAO decimal conversion differs", test.value, got, test.decimal)
		}
		checkSourceDecimalMasks(t, test.value, test.cover, test.presence)
	}
}

func TestSiteUnitDecimalFormatAdjacentDoubleSourceOracle(t *testing.T) {
	for _, test := range []struct {
		midpoint        float64
		cover, presence string
	}{
		{.005, "0.01", "0.0"}, {.145, "0.15", "0.1"}, {1.005, "1.01", "1.0"},
		{1.125, "1.13", "1.1"}, {.05, "0.05", "0.1"},
		{.15, "0.15", "0.2"}, {1.15, "1.15", "1.2"},
	} {
		for _, value := range []float64{math.Nextafter(test.midpoint, math.Inf(-1)), test.midpoint,
			math.Nextafter(test.midpoint, math.Inf(1))} {
			checkSourceDecimalMasks(t, value, test.cover, test.presence)
		}
	}
	if siteUnitFixedNumber(-.0001, 2, 2) != "00.00" ||
		siteUnitFixedNumber(-1.005, 2, 3) != "-001.01" {
		t.Fatal("negative rounded zero or existing integer-width padding changed")
	}
}

func checkSourceDecimalMasks(t *testing.T, value float64, cover, presence string) {
	t.Helper()
	for _, sign := range []float64{1, -1} {
		expectedCover, expectedPresence := cover, presence
		if sign < 0 {
			if expectedCover != "0.00" {
				expectedCover = "-" + expectedCover
			}
			if expectedPresence != "0.0" {
				expectedPresence = "-" + expectedPresence
			}
		}
		if got := siteUnitFixedNumber(sign*value, 2, 1); got != expectedCover {
			t.Fatal("source cover mask differs", sign*value, got, expectedCover)
		}
		if got := siteUnitFixedNumber(sign*value, 1, 1); got != expectedPresence {
			t.Fatal("source presence/lifeform mask differs", sign*value, got, expectedPresence)
		}
	}
}
