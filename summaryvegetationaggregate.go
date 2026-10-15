package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
)

// Covers must already have source SINGLE staging and all source join weights.
// The shared binary unit certifies exact DOUBLE partial sums in every order.
func aggregateSummaryVegetationCovers(ctx context.Context, covers []ProjectMetadataCell, numPlots int64) (summaryVegetationCellInput, error) {
	fail := func(err error) (summaryVegetationCellInput, error) { return summaryVegetationCellInput{}, err }
	if ctx == nil {
		return fail(errors.New("summary vegetation aggregation requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if numPlots < 1 || numPlots > math.MaxInt32 {
		return fail(errors.New("summary vegetation aggregation requires positive signed32 original plot count"))
	}
	count := int64(0)
	sum, absolute := new(big.Rat), new(big.Rat)
	minExponent, haveExponent := 0, false
	for i, cell := range covers {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		number, err := vegetationReportNumber(cell)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation joined cover %d: %w", i+1, err))
		}
		if number == nil {
			continue
		}
		value, _ := number.Float64()
		single := float64(float32(value))
		if math.IsInf(single, 0) || new(big.Rat).SetFloat64(single).Cmp(number) != 0 {
			return fail(fmt.Errorf("summary vegetation joined cover %d lacks exact original SINGLE staging", i+1))
		}
		if count == math.MaxInt32 {
			return fail(errors.New("summary vegetation non-NULL cover count exceeds original signed32 range"))
		}
		count++
		sum.Add(sum, number)
		absolute.Add(absolute, new(big.Rat).Abs(number))
		if number.Sign() != 0 {
			exponent := int(number.Num().TrailingZeroBits()) - (number.Denom().BitLen() - 1)
			if !haveExponent || exponent < minExponent {
				minExponent, haveExponent = exponent, true
			}
		}
	}
	unit := new(big.Rat).SetInt64(1)
	if haveExponent {
		magnitude := minExponent
		if magnitude < 0 {
			magnitude = -magnitude
		}
		power := new(big.Int).Lsh(big.NewInt(1), uint(magnitude))
		if minExponent >= 0 {
			unit.SetInt(power)
		} else {
			unit.SetFrac(big.NewInt(1), power)
		}
	}
	bound := new(big.Rat).Quo(absolute, unit)
	limit := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 53))
	if bound.Cmp(limit) > 0 {
		return fail(errors.New("summary vegetation DOUBLE sum is not certified order-independent; source arithmetic remains unavailable"))
	}
	result := summaryVegetationCellInput{NumPlots: numPlots, Count: count}
	if count > 0 {
		total, exact := sum.Float64()
		if !exact || math.IsInf(total, 0) || math.IsNaN(total) {
			return fail(errors.New("summary vegetation certified sum lost exact finite DOUBLE output"))
		}
		mean := total / float64(count)
		result.Sum, result.Average = &total, &mean
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
