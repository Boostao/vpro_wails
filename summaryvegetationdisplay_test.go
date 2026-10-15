package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestSummaryVegetationDisplayOriginalCrosstabs(t *testing.T) {
	for _, scenario := range []string{"ordinary", "over-one", "null"} {
		for _, mode := range []int{1, 2, 3} {
			for _, average := range []int{1, 2} {
				raw, su, stage, master, personal, layers := summaryLayerFixture()
				if scenario == "over-one" || scenario == "null" {
					personal = lifeformReferenceFixture(lifeformReferenceRow("X", "Alpha", metadataInteger("1"), metadataText("alternate"), metadataText("U")))
				}
				if scenario == "null" {
					for i := range stage.Rows {
						stage.Rows[i].Cells[3] = ProjectMetadataCell{Storage: "null"}
					}
				}
				result, err := planSummaryVegetationLayerDisplay(context.Background(), raw, su, stage, master, personal, layers, average, mode)
				if scenario == "null" && mode != 2 {
					if err == nil || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
						t.Fatal("source modes1/3 all-NULL criteria failure replaced with empty success", mode, result, err)
					}
					continue
				}
				if err != nil {
					t.Fatal(scenario, mode, average, err)
				}
				if scenario == "null" {
					assertSummaryLayerRows(t, result, []string{"A", "B"}, [][]ProjectMetadataCell{})
					continue
				}
				a, b, s := "10.00", "5.00", "0.50"
				if average == 2 {
					a, s = "13.33", "2.00"
				} else if scenario == "over-one" {
					a, b = "20.00", "10.00"
				}
				pa, pb := "0.75", "1.00"
				if scenario == "over-one" {
					pa, pb = "1.50", "2.00"
				}
				order := metadataText(pb)
				sOrder := "0.25"
				if mode == 2 {
					a = "IV - 4"
					if average == 2 {
						a = "IV - 5"
					}
					b, s = "V - 3", "II - 1"
					if average == 2 {
						s = "II - 2"
					}
					order, sOrder = metadataText("V - 3"), s
				} else {
					if mode == 3 {
						pa, pb, sOrder = "IV", "V", "II"
						order = metadataText("V")
						if scenario == "over-one" {
							pa, pb = "", ""
							order = ProjectMetadataCell{Storage: "null"}
						}
					}
					a, b = pa+" - "+a, pb+" - "+b
					sp := "0.25"
					if mode == 3 {
						sp = "II"
					}
					s = sp + " - " + s
				}
				expected := [][]ProjectMetadataCell{
					{metadataText("A1"), metadataText("Alpha"), order, metadataText(a), metadataText(b)},
					{metadataText("A1"), metadataText("S"), metadataText(sOrder), metadataText(s), {Storage: "null"}},
				}
				if scenario == "over-one" && mode == 2 {
					expected = expected[1:]
				}
				assertSummaryLayerRows(t, result, []string{"A", "B"}, expected)
			}
		}
	}
}

func TestSummaryVegetationDisplayClassBoundariesAndFormattedPresence(t *testing.T) {
	for _, point := range []struct {
		value            float64
		below, at, above string
	}{
		{0, "", "I", "I"}, {0.2, "I", "I", "II"}, {0.4, "II", "II", "III"},
		{0.6, "III", "III", "IV"}, {0.8, "IV", "IV", "V"}, {1, "V", "V", ""},
	} {
		for i, value := range []float64{math.Nextafter(point.value, math.Inf(-1)), point.value, math.Nextafter(point.value, math.Inf(1))} {
			expected := []string{point.below, point.at, point.above}[i]
			if actual := summaryVegetationPresenceClass(value); actual != expected {
				t.Fatal("original presence threshold or adjacent DOUBLE differs", value, actual, expected)
			}
		}
	}
	for _, point := range []struct {
		value     float64
		at, above string
	}{{0.3, "+", "1"}, {1, "1", "2"}, {2.2, "2", "3"}, {5, "3", "4"}, {10, "4", "5"},
		{20, "5", "6"}, {33, "6", "7"}, {50, "7", "8"}, {75, "8", "9"}} {
		if summaryVegetationSignificanceClass(point.value) != point.at ||
			summaryVegetationSignificanceClass(math.Nextafter(point.value, math.Inf(1))) != point.above {
			t.Fatal("independent significance threshold differs", point)
		}
	}
	input := summaryVegetationCellInput{Count: 21, NumPlots: 100, Sum: summaryCellNumber(100), Average: summaryCellNumber(1)}
	for _, test := range []struct {
		count int64
		mode  int
		value string
	}{
		{204, 2, "I - 1"}, {204, 3, "II - 1.00"}, {1004, 2, "V - 1"},
	} {
		input.Count, input.NumPlots, input.Sum = test.count, 1000, summaryCellNumber(1000)
		result, err := calculateSummaryVegetationDisplay(context.Background(), input, 1, test.mode)
		if err != nil || result.Value.Text == nil || *result.Value.Text != test.value {
			t.Fatal("mode2 formatted presence and mode3 raw presence were conflated", test, result, err)
		}
	}
	input.Count, input.NumPlots, input.Sum = 1, 20001, summaryCellNumber(20001)
	result, err := calculateSummaryVegetationDisplay(context.Background(), input, 1, 3)
	if err != nil || result.Value.Text == nil || *result.Value.Text != "I - 1.00" {
		t.Fatal("mode3 inherited mode1/6 count mask", result, err)
	}
	for _, mode := range []int{1, 6} {
		if _, err := calculateSummaryVegetationDisplay(context.Background(), input, 1, mode); err == nil {
			t.Fatal("mode1/6 source ratio criteria error silently suppressed")
		}
	}
	input.Count, input.NumPlots, input.Sum = 1, 1, summaryCellNumber(1.005)
	result, err = calculateSummaryVegetationDisplay(context.Background(), input, 1, 2)
	if err != nil || result.Value.Text == nil || *result.Value.Text != "V - 2" {
		t.Fatal("significance used pre-format1.005 instead of source1.01", result, err)
	}
}

func TestSummaryVegetationDisplayCancellationErrorsAndOwnership(t *testing.T) {
	input := summaryVegetationCellInput{Count: 1, NumPlots: 1, Sum: summaryCellNumber(0), Average: summaryCellNumber(0)}
	for _, mode := range []int{1, 2, 3, 6} {
		probe := &projectUnitCancellationContext{Context: context.Background()}
		result, err := calculateSummaryVegetationDisplay(probe, input, 1, mode)
		if err != nil {
			t.Fatal(mode, err)
		}
		*result.Value.Text = "changed"
		if *input.Sum != 0 || *input.Average != 0 || *result.Order.Text == "changed" {
			t.Fatal("display output aliases caller values or Order")
		}
		for point := 1; point <= probe.calls; point++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
			result, err := calculateSummaryVegetationDisplay(ctx, input, 1, mode)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationDisplayCell{}) {
				t.Fatal("cancelled display leaked partial scalar", mode, point, result, err)
			}
		}
	}
	for _, mode := range []int{0, 4, 5, 7} {
		if _, err := calculateSummaryVegetationDisplay(context.Background(), input, 1, mode); err == nil {
			t.Fatal("unavailable source display guessed")
		}
	}
	if _, err := calculateSummaryVegetationDisplay(nil, input, 1, 1); err == nil {
		t.Fatal("nil context accepted")
	}
	invalid := input
	invalid.Count = 0
	if _, err := calculateSummaryVegetationDisplay(context.Background(), invalid, 1, 2); err == nil {
		t.Fatal("invalid NULL/count input bypassed shared validation")
	}
	raw, su, stage, master, personal, layers := summaryLayerFixture()
	probe := &projectUnitCancellationContext{Context: context.Background()}
	if _, err := planSummaryVegetationLayerDisplay(probe, raw, su, stage, master, personal, layers, 1, 3); err != nil {
		t.Fatal(err)
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		result, err := planSummaryVegetationLayerDisplay(ctx, raw, su, stage, master, personal, layers, 1, 3)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
			t.Fatal("mode-specific pivot cancellation leaked data", point, result, err)
		}
	}
}
