package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func summaryCellNumber(value float64) *float64 {
	return &value
}

func TestSummaryVegetationCoverOriginalVariantHelperCases(t *testing.T) {
	for _, test := range []struct {
		value *float64
		text  string
		real  bool
	}{
		{nil, "", false},
		{summaryCellNumber(-2), "", true}, {summaryCellNumber(0), "", true},
		{summaryCellNumber(math.Nextafter(0.01, math.Inf(-1))), "", true},
		{summaryCellNumber(0.01), "0.01", false},
		{summaryCellNumber(math.Nextafter(0.01, math.Inf(1))), "0.01", false},
		{summaryCellNumber(math.Nextafter(1.005, math.Inf(-1))), "1.01", false},
		{summaryCellNumber(1.005), "1.01", false},
		{summaryCellNumber(math.Nextafter(1.005, math.Inf(1))), "1.01", false},
		{summaryCellNumber(math.Nextafter(0.15, math.Inf(-1))), "0.15", false},
		{summaryCellNumber(0.15), "0.15", false},
		{summaryCellNumber(math.Nextafter(0.15, math.Inf(1))), "0.15", false},
		{summaryCellNumber(99), "99.00", false},
	} {
		input := summaryVegetationCellInput{NumPlots: 1, Sum: test.value, Average: test.value}
		if test.value != nil {
			input.Count = 1
		}
		for _, average := range []int{1, 2} {
			cell, err := calculateSummaryVegetationCover(context.Background(), input, average)
			if err != nil {
				t.Fatal("source-calibrated original helper input refused", test, average, err)
			}
			switch {
			case test.value == nil:
				if !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "null"}) {
					t.Fatal("source NULL repaired", cell)
				}
			case test.real:
				if cell.Storage != "real" || cell.Real == nil || *cell.Real != 0.01 || cell.Text != nil {
					t.Fatal("original numeric floor converted to text or clamp lost", cell)
				}
			default:
				if cell.Storage != "text" || cell.Text == nil || *cell.Text != test.text || cell.Real != nil {
					t.Fatal("original Format result changed storage/value", test, cell)
				}
			}
		}
	}
}

func TestSummaryVegetationCoverOriginalCrosstabScalars(t *testing.T) {
	for _, test := range []struct {
		input summaryVegetationCellInput
		all   string
		mean  string
	}{
		{summaryVegetationCellInput{Count: 3, NumPlots: 4, Sum: summaryCellNumber(40), Average: summaryCellNumber(40.0 / 3)}, "10.00", "13.33"},
		{summaryVegetationCellInput{Count: 1, NumPlots: 1, Sum: summaryCellNumber(5), Average: summaryCellNumber(5)}, "5.00", "5.00"},
		{summaryVegetationCellInput{Count: 1, NumPlots: 4, Sum: summaryCellNumber(2), Average: summaryCellNumber(2)}, "0.50", "2.00"},
		{summaryVegetationCellInput{Count: 2, NumPlots: 1, Sum: summaryCellNumber(240), Average: summaryCellNumber(120)}, "240.00", "120.00"},
	} {
		for _, mode := range []int{1, 2} {
			expected := test.all
			if mode == 2 {
				expected = test.mean
			}
			result, err := calculateSummaryVegetationCover(context.Background(), test.input, mode)
			if err != nil || result.Storage != "text" || result.Text == nil || *result.Text != expected {
				t.Fatal("physical duplicate weights/source denominator or mean changed", mode, result, err)
			}
		}
	}
}

func TestSummaryVegetationCoverFormattedCountGate(t *testing.T) {
	for _, test := range []struct {
		plots  int64
		refuse bool
	}{
		{19999, false}, {20000, false}, {20001, true}, {math.MaxInt32, true},
	} {
		input := summaryVegetationCellInput{Count: 1, NumPlots: test.plots, Sum: summaryCellNumber(1), Average: summaryCellNumber(1)}
		for _, mode := range []int{1, 2} {
			result, err := calculateSummaryVegetationCover(context.Background(), input, mode)
			if test.refuse {
				if err == nil || !reflect.DeepEqual(result, ProjectMetadataCell{}) {
					t.Fatal("empty source format/numeric comparison became successful NULL output", test, mode, result, err)
				}
			} else if err != nil || result.Storage == "null" {
				t.Fatal("source four-decimal formatted count gate differs at half boundary", test, mode, result, err)
			}
		}
	}
	input := summaryVegetationCellInput{Count: math.MaxInt32, NumPlots: 1, Sum: summaryCellNumber(1)}
	if result, err := calculateSummaryVegetationCover(context.Background(), input, 1); err != nil || result.Text == nil || *result.Text != "1.00" {
		t.Fatal("duplicate observation count is not limited to number of plots", result, err)
	}
}

func TestSummaryVegetationCoverRejectsInvalidAggregates(t *testing.T) {
	valid := summaryVegetationCellInput{Count: 1, NumPlots: 1, Sum: summaryCellNumber(1), Average: summaryCellNumber(1)}
	changes := []func(*summaryVegetationCellInput){
		func(i *summaryVegetationCellInput) { i.Count = -1 },
		func(i *summaryVegetationCellInput) { i.Count = int64(math.MaxInt32) + 1 },
		func(i *summaryVegetationCellInput) { i.NumPlots = 0 },
		func(i *summaryVegetationCellInput) { i.NumPlots = int64(math.MaxInt32) + 1 },
		func(i *summaryVegetationCellInput) { i.Sum = nil },
		func(i *summaryVegetationCellInput) { i.Sum = summaryCellNumber(math.NaN()) },
		func(i *summaryVegetationCellInput) { i.Sum = summaryCellNumber(math.Inf(1)) },
		func(i *summaryVegetationCellInput) { i.Average = summaryCellNumber(math.Inf(-1)) },
		func(i *summaryVegetationCellInput) { i.Count = 0 },
	}
	for _, change := range changes {
		input := valid
		change(&input)
		for _, mode := range []int{1, 2} {
			if result, err := calculateSummaryVegetationCover(context.Background(), input, mode); err == nil || !reflect.DeepEqual(result, ProjectMetadataCell{}) {
				t.Fatal("invalid aggregate produced success-shaped value", input, mode, result, err)
			}
		}
	}
	for _, mode := range []int{-1, 0, 3, 10, 20} {
		if result, err := calculateSummaryVegetationCover(context.Background(), valid, mode); err == nil || !reflect.DeepEqual(result, ProjectMetadataCell{}) {
			t.Fatal("LongVeg mode or invalid summary mode inferred", mode, result, err)
		}
	}
	valid.Average = nil
	if result, err := calculateSummaryVegetationCover(context.Background(), valid, 2); err == nil || !reflect.DeepEqual(result, ProjectMetadataCell{}) {
		t.Fatal("Characteristic fabricated missing mean", result, err)
	}
	if result, err := calculateSummaryVegetationCover(context.Background(), valid, 1); err != nil || result.Text == nil || *result.Text != "1.00" {
		t.Fatal("By nPlots unnecessarily requires unused mean", result, err)
	}
}

func TestSummaryVegetationCoverCancellationAndIndependentValues(t *testing.T) {
	input := summaryVegetationCellInput{Count: 1, NumPlots: 1, Sum: summaryCellNumber(0), Average: summaryCellNumber(0)}
	cell, err := calculateSummaryVegetationCover(context.Background(), input, 1)
	if err != nil || cell.Real == nil {
		t.Fatal(cell, err)
	}
	*cell.Real = 42
	if *input.Sum != 0 || *input.Average != 0 {
		t.Fatal("output floor aliases input aggregates")
	}
	for _, point := range []int{1, 2} {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		if cell, err := calculateSummaryVegetationCover(ctx, input, 1); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
			t.Fatal("cancelled scalar returned an output", point, cell, err)
		}
	}
	if cell, err := calculateSummaryVegetationCover(nil, input, 1); err == nil || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatal("nil context accepted", cell, err)
	}
}
