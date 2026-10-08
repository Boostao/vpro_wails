package main

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func siteUnitNumericCells(values ...float64) []ProjectMetadataCell {
	cells := make([]ProjectMetadataCell, len(values))
	for i, value := range values {
		cells[i] = ProjectMetadataCell{Storage: "real", Real: &value}
	}
	return cells
}

func TestSiteUnitNumericMean(t *testing.T) {
	tests := []struct {
		field  string
		values []float64
		nulls  int
		want   string
	}{
		{"Elevation", []float64{8, 2, 4}, 0, "2---05---8"},
		{"StandAge", []float64{0, 4, 4, 4}, 1, "0---03---4  (Null 1)"},
		{"SlopeGradient", []float64{-5, 0, 2}, 2, "-5----1---2  (Null 2)"},
		{"SubstrateRocks", []float64{1.234, 9.876}, 1, "1.23---5.6---9.88  (Null 1)"},
		{"HumusThickness", []float64{-3.125, -1.125}, 1, "-3.13----2.1---0  (Null 1)"},
		{"SeepageDepth", []float64{-3.125, -1.125}, 0, "-3.13----2.1----1.13"},
		{"RootingDepth", []float64{1000, 1234.5, 999}, 0, "1---1077.8---999"},
		{"StrataCoverTree", []float64{-1234, -999}, 0, "-999----1116.5----1"},
		{"Elevation", []float64{-0.0, 0, 0}, 0, "0---00---0"},
		{"StandAge", []float64{-3, -2}, 0, "-3----03----2"},
		{"SlopeGradient", []float64{2, 3}, 0, "2---3---3"},
		{"SlopeGradient", []float64{3, 4}, 0, "3---4---4"},
		{"SubstrateWater", []float64{1.25}, 0, "1.25---1.3---1.25"},
		{"SubstrateWater", []float64{1.35}, 0, "1.35---1.4---1.35"},
		{"SubstrateBedRock", []float64{100000, 1000000}, 0, "1---550000.0---100"},
		{"HumusThickness", []float64{math.Copysign(0, -1)}, 1, "0---0.0---0  (Null 1)"},
		{"SlopeGradient", []float64{1e20, 1e20}, 0, "1e+20---100000000000000000000---1e+20"},
		{"SlopeGradient", []float64{math.SmallestNonzeroFloat64}, 0, "5e-324---0---5e-324"},
		{"StandAge", []float64{0.5}, 0, "0.5---01---0.5"},
		{"StandAge", []float64{-0.5}, 0, "-0.5----01----0.5"},
		{"SubstrateDecWood", []float64{1234.567}, 1, "1---1234.6---1  (Null 1)"},
		{"SubstrateDecWood", []float64{-1234.567}, 1, "-1----1234.6---0  (Null 1)"},
	}
	for _, test := range tests {
		t.Run(test.field+"/"+test.want, func(t *testing.T) {
			cells := siteUnitNumericCells(test.values...)
			for i := 0; i < test.nulls; i++ {
				cells = append(cells, ProjectMetadataCell{Storage: "null"})
			}
			got, err := summarizeSiteUnitNumeric(cells, test.field, 1)
			if err != nil || got != test.want {
				t.Fatalf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestSiteUnitNumericEmpty(t *testing.T) {
	for _, field := range []string{"Elevation", "StandAge", "SlopeGradient", "HumusThickness"} {
		for _, method := range []int{1, 2} {
			for _, nulls := range []int{0, 1, 3} {
				cells := make([]ProjectMetadataCell, nulls)
				for i := range cells {
					cells[i].Storage = "null"
				}
				want := ""
				if method == 1 && (field == "Elevation" || field == "StandAge") {
					switch nulls {
					case 1:
						want = "  (Null 1)"
					case 3:
						want = "  (Null 3)"
					}
				}
				got, err := summarizeSiteUnitNumeric(cells, field, method)
				if err != nil || got != want {
					t.Fatalf("%s method %d NULLs %d: got %q, %v; want %q", field, method, nulls, got, err, want)
				}
			}
		}
	}
}

func TestSiteUnitNumericQuartiles(t *testing.T) {
	tests := []struct {
		values []float64
		want   string
	}{
		{[]float64{4}, "4---4---4"},
		{[]float64{4, 0}, "1---2---3"},
		{[]float64{8, 0, 4}, "2---4---6"},
		{[]float64{12, 0, 8, 4}, "3---6---9"},
		{[]float64{16, 0, 12, 4, 8}, "4---8---12"},
		{[]float64{4, 4, 4, 0, 0}, "0---4---4"},
		{[]float64{-8, -4, 0, 4, 8}, "-4---0---4"},
		{[]float64{1000, 2000}, "1250---1500---1750"},
		{[]float64{-1.5, -0.5}, "-1.25----1----0.75"},
	}
	for _, field := range []string{"Elevation", "StandAge", "SlopeGradient", "HumusThickness"} {
		for _, test := range tests {
			cells := append(siteUnitNumericCells(test.values...), ProjectMetadataCell{Storage: "null"})
			before := append([]ProjectMetadataCell(nil), cells...)
			got, err := summarizeSiteUnitNumeric(cells, field, 2)
			if err != nil || got != test.want+"  (Null 1)" {
				t.Fatalf("%s %v: got %q, %v; want %q", field, test.values, got, err, test.want+"  (Null 1)")
			}
			if !reflect.DeepEqual(cells, before) {
				t.Fatal("summary changed caller cells")
			}
		}
	}
}

func TestSiteUnitNumericAllowedFields(t *testing.T) {
	for _, field := range strings.Fields("SubstrateOrganicMatter SubstrateRocks SubstrateDecWood SubstrateMineralSoil SubstrateBedRock SubstrateWater StrataCoverTree StrataCoverShrub StrataCoverHerb StrataCoverMoss HumusThickness SeepageDepth RootingDepth") {
		for _, method := range []int{1, 2} {
			want := "7---7.0---7"
			if method == 2 {
				want = "7---7---7"
			}
			got, err := summarizeSiteUnitNumeric(siteUnitNumericCells(7), field, method)
			if err != nil || got != want {
				t.Fatalf("%s method %d: got %q, %v; want %q", field, method, got, err, want)
			}
		}
	}
}

func TestSiteUnitNumericExtremes(t *testing.T) {
	tests := []struct {
		values []float64
		want   string
	}{
		{[]float64{-math.MaxFloat64, math.MaxFloat64}, "-8.988465674311579e+307---0---8.988465674311579e+307"},
		{[]float64{math.MaxFloat64, math.MaxFloat64}, "1.7976931348623157e+308---1.7976931348623157e+308---1.7976931348623157e+308"},
		{[]float64{math.SmallestNonzeroFloat64}, "5e-324---5e-324---5e-324"},
		{[]float64{math.Copysign(0, -1)}, "0---0---0"},
	}
	for _, test := range tests {
		got, err := summarizeSiteUnitNumeric(siteUnitNumericCells(test.values...), "SlopeGradient", 2)
		if err != nil || got != test.want {
			t.Fatalf("%v: got %q, %v; want %q", test.values, got, err, test.want)
		}
	}
	got, err := summarizeSiteUnitNumeric(siteUnitNumericCells(-math.MaxFloat64, math.MaxFloat64), "Elevation", 1)
	if err != nil || got != "-1.7976931348623157e+308---00---1.7976931348623157e+308" {
		t.Fatalf("overflow-safe mean: %q, %v", got, err)
	}
	integer := "9223372036854775807"
	got, err = summarizeSiteUnitNumeric([]ProjectMetadataCell{{Storage: "integer", Integer: &integer}}, "SlopeGradient", 2)
	if err != nil || got != "9.223372036854776e+18---9.223372036854776e+18---9.223372036854776e+18" {
		t.Fatalf("signed64 input: %q, %v", got, err)
	}
}

func TestSiteUnitNumericRejection(t *testing.T) {
	text, hex, integer := "123", "00", "01"
	nan, inf, real := math.NaN(), math.Inf(1), 2.0
	bad := []ProjectMetadataCell{
		{Storage: "text", Text: &text},
		{Storage: "blob", BlobHex: &hex},
		{Storage: "integer", Integer: &integer},
		{Storage: "integer"},
		{Storage: "real"},
		{Storage: "real", Real: &nan},
		{Storage: "real", Real: &inf},
		{Storage: "real", Real: &real, Text: &text},
		{Storage: "null", Real: &real},
		{Storage: "unknown"},
	}
	for _, cell := range bad {
		for _, method := range []int{1, 2} {
			got, err := summarizeSiteUnitNumeric(append(siteUnitNumericCells(1), cell), "Elevation", method)
			if err == nil || got != "" {
				t.Fatalf("accepted %+v method %d: %q, %v", cell, method, got, err)
			}
		}
	}
	for _, field := range []string{"", "elevation", "Conductivity", "SiteUnit", "StrataCoverA"} {
		if got, err := summarizeSiteUnitNumeric(nil, field, 1); err == nil || got != "" {
			t.Fatalf("accepted field %q: %q, %v", field, got, err)
		}
	}
	for _, method := range []int{-1, 0, 3} {
		if got, err := summarizeSiteUnitNumeric(nil, "Elevation", method); err == nil || got != "" {
			t.Fatalf("accepted method %d: %q, %v", method, got, err)
		}
	}
}
