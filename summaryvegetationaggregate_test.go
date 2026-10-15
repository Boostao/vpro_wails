package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func summaryCoverCells(values ...*float64) []ProjectMetadataCell {
	result := []ProjectMetadataCell{}
	for _, value := range values {
		cell := ProjectMetadataCell{Storage: "null"}
		if value != nil {
			cell = ProjectMetadataCell{Storage: "real", Real: value}
		}
		result = append(result, cell)
	}
	return result
}

func TestSummaryVegetationAggregatesNativeSingleCases(t *testing.T) {
	for _, test := range []struct {
		covers []ProjectMetadataCell
		count  int64
		total  *float64
		mean   *float64
	}{
		{summaryCoverCells(nil, nil), 0, nil, nil},
		{summaryCoverCells(summaryCellNumber(10), summaryCellNumber(10), summaryCellNumber(20), nil), 3, summaryCellNumber(40), summaryCellNumber(40.0 / 3)},
		{summaryCoverCells(summaryCellNumber(float64(float32(0.1))), summaryCellNumber(float64(float32(0.1)))), 2,
			summaryCellNumber(0.20000000298023224), summaryCellNumber(0.10000000149011612)},
		{summaryCoverCells(summaryCellNumber(-2), summaryCellNumber(0), nil), 2, summaryCellNumber(-2), summaryCellNumber(-1)},
		{summaryCoverCells(summaryCellNumber(16777216), summaryCellNumber(1), summaryCellNumber(-16777216)), 3, summaryCellNumber(1), summaryCellNumber(1.0 / 3)},
	} {
		result, err := aggregateSummaryVegetationCovers(context.Background(), test.covers, 4)
		expected := summaryVegetationCellInput{Count: test.count, NumPlots: 4, Sum: test.total, Average: test.mean}
		if err != nil || !reflect.DeepEqual(result, expected) {
			t.Fatal("independent native SINGLE SUM/Avg/count witness differs", result, expected, err)
		}
	}
	result, err := aggregateSummaryVegetationCovers(context.Background(),
		[]ProjectMetadataCell{metadataInteger("10"), metadataInteger("10"), metadataInteger("20")}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{1, 2} {
		cell, err := calculateSummaryVegetationCover(context.Background(), result, mode)
		expected := "10.00"
		if mode == 2 {
			expected = "13.33"
		}
		if err != nil || cell.Text == nil || *cell.Text != expected {
			t.Fatal("aggregate caller lost calibrated source cell", mode, cell, err)
		}
	}
}

func TestSummaryVegetationAggregatesExactDomainBoundaryAndOrders(t *testing.T) {
	large := math.Ldexp(1, 52)
	for _, last := range []float64{0, 1} {
		cells := summaryCoverCells(summaryCellNumber(large), summaryCellNumber(-large), summaryCellNumber(last))
		result, err := aggregateSummaryVegetationCovers(context.Background(), cells, 1)
		if last == 0 {
			if err != nil || result.Sum == nil || *result.Sum != 0 {
				t.Fatal("large exact cancellation refused", result, err)
			}
		} else if err == nil || !reflect.DeepEqual(result, summaryVegetationCellInput{}) {
			t.Fatal("bound2^53+1 became certified via cancellation", result, err)
		}
	}
	boundary := summaryCoverCells(summaryCellNumber(large), summaryCellNumber(large-math.Ldexp(1, 29)),
		summaryCellNumber(math.Ldexp(1, 29)-32), summaryCellNumber(31), summaryCellNumber(1))
	boundaryResult, err := aggregateSummaryVegetationCovers(context.Background(), boundary, 1)
	if err != nil || boundaryResult.Sum == nil || *boundaryResult.Sum != math.Ldexp(1, 53) || boundaryResult.Count != 5 {
		t.Fatal("inclusive2^53 coefficients in a shared unit of1 refused", boundaryResult, err)
	}
	boundary = append(boundary, ProjectMetadataCell{Storage: "real", Real: summaryCellNumber(1)})
	if result, err := aggregateSummaryVegetationCovers(context.Background(), boundary, 1); err == nil ||
		!reflect.DeepEqual(result, summaryVegetationCellInput{}) {
		t.Fatal("measured2^53+1 coefficients accepted", result, err)
	}
	values := []float64{16777216, 1, -16777216}
	for _, order := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		cells := summaryCoverCells(summaryCellNumber(values[order[0]]), summaryCellNumber(values[order[1]]), summaryCellNumber(values[order[2]]))
		result, err := aggregateSummaryVegetationCovers(context.Background(), cells, 1)
		if err != nil || result.Sum == nil || *result.Sum != 1 || *result.Average != 1.0/3 {
			t.Fatal("safe source SINGLE permutations must agree", order, result, err)
		}
	}
	unsafe := []float64{math.Ldexp(1, 100), 1, -math.Ldexp(1, 100)}
	for _, order := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		cells := summaryCoverCells(summaryCellNumber(unsafe[order[0]]), summaryCellNumber(unsafe[order[1]]), summaryCellNumber(unsafe[order[2]]))
		if result, err := aggregateSummaryVegetationCovers(context.Background(), cells, 1); err == nil || !reflect.DeepEqual(result, summaryVegetationCellInput{}) {
			t.Fatal("observed source order sensitivity silently replaced with rational sum", order, result, err)
		}
	}
	// A large common binary unit is safe; magnitude alone is not the certificate.
	result, err := aggregateSummaryVegetationCovers(context.Background(),
		summaryCoverCells(summaryCellNumber(math.Ldexp(1, 100)), summaryCellNumber(math.Ldexp(1, 100))), 1)
	if err != nil || result.Sum == nil || *result.Sum != math.Ldexp(1, 101) {
		t.Fatal("common binary unit unnecessarily rejected", result, err)
	}
}

func TestSummaryVegetationAggregatesRejectUnstagedAndMalformedCovers(t *testing.T) {
	for _, cell := range []ProjectMetadataCell{
		metadataText("1"), metadataInteger("16777217"), metadataInteger("9223372036854775807"),
		{Storage: "real", Real: summaryCellNumber(0.1)}, {Storage: "real", Real: summaryCellNumber(math.MaxFloat64)},
		{Storage: "real", Real: summaryCellNumber(math.NaN())}, {Storage: "real", Real: summaryCellNumber(math.Inf(1))},
		{Storage: "real"}, {Storage: "null", Text: new(string)},
	} {
		if result, err := aggregateSummaryVegetationCovers(context.Background(), []ProjectMetadataCell{cell}, 1); err == nil ||
			!reflect.DeepEqual(result, summaryVegetationCellInput{}) {
			t.Fatal("unstaged source SINGLE or malformed cover became accepted aggregate", cell, result, err)
		}
	}
	for _, count := range []int64{-1, 0, int64(math.MaxInt32) + 1} {
		if result, err := aggregateSummaryVegetationCovers(context.Background(), nil, count); err == nil ||
			!reflect.DeepEqual(result, summaryVegetationCellInput{}) {
			t.Fatal("invalid independently supplied denominator accepted", count, result, err)
		}
	}
	result, err := aggregateSummaryVegetationCovers(context.Background(),
		summaryCoverCells(summaryCellNumber(float64(math.SmallestNonzeroFloat32)), summaryCellNumber(float64(math.SmallestNonzeroFloat32))), 1)
	if err != nil || result.Sum == nil || *result.Sum != 2*float64(math.SmallestNonzeroFloat32) {
		t.Fatal("source SINGLE subnormal lost at exact shared unit", result, err)
	}
}

func TestSummaryVegetationAggregatesCancellationAndOwnership(t *testing.T) {
	covers := summaryCoverCells(summaryCellNumber(1), nil, summaryCellNumber(2))
	probe := &projectUnitCancellationContext{Context: context.Background()}
	result, err := aggregateSummaryVegetationCovers(probe, covers, 1)
	if err != nil || result.Sum == nil || probe.calls < 2 {
		t.Fatal(result, err, probe.calls)
	}
	*result.Sum, *result.Average = 42, 42
	if *covers[0].Real != 1 || *covers[2].Real != 2 {
		t.Fatal("aggregate result aliases source covers")
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		if result, err := aggregateSummaryVegetationCovers(ctx, covers, 1); !errors.Is(err, context.Canceled) ||
			!reflect.DeepEqual(result, summaryVegetationCellInput{}) {
			t.Fatal("cancelled aggregate leaked partial counts/sums", point, result, err)
		}
	}
	if result, err := aggregateSummaryVegetationCovers(nil, covers, 1); err == nil || !reflect.DeepEqual(result, summaryVegetationCellInput{}) {
		t.Fatal("nil context accepted", result, err)
	}
}
