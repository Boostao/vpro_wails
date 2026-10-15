package main

import (
	"context"
	"errors"
	"math"
)

type summaryVegetationCellInput struct {
	Count    int64
	NumPlots int64
	Sum      *float64
	Average  *float64
}

// The caller owns the joined aggregates. This preserves the source Variant
// before the crosstab infers/coerces its column type.
func calculateSummaryVegetationCover(ctx context.Context, input summaryVegetationCellInput, average int) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) { return ProjectMetadataCell{}, err }
	if err := validateSummaryVegetationCellInput(ctx, input, average); err != nil {
		return fail(err)
	}
	if input.Count == 0 {
		return ProjectMetadataCell{Storage: "null"}, nil
	}
	if siteUnitFixedNumber(float64(input.Count)/float64(input.NumPlots), 4, 1) == "0.0000" {
		return fail(errors.New("summary vegetation formatted cover count is empty; original numeric criteria comparison is unavailable"))
	}
	return formatSummaryVegetationCover(ctx, input, average)
}

func validateSummaryVegetationCellInput(ctx context.Context, input summaryVegetationCellInput, average int) error {
	if ctx == nil {
		return errors.New("summary vegetation cover calculation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if average != 1 && average != 2 {
		return errors.New("summary vegetation cover requires original Average1 or2")
	}
	if input.NumPlots < 1 || input.NumPlots > math.MaxInt32 || input.Count < 0 || input.Count > math.MaxInt32 {
		return errors.New("summary vegetation cover requires positive original plot count and nonnegative signed32 cover count")
	}
	for _, value := range []*float64{input.Sum, input.Average} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return errors.New("summary vegetation aggregates require finite DOUBLE values")
		}
	}
	if input.Count == 0 {
		if input.Sum != nil || input.Average != nil {
			return errors.New("summary vegetation zero cover count requires NULL aggregates")
		}
	} else if input.Sum == nil || average == 2 && input.Average == nil {
		return errors.New("summary vegetation nonzero cover count lost its required aggregate")
	}
	return ctx.Err()
}

func formatSummaryVegetationCover(ctx context.Context, input summaryVegetationCellInput, average int) (ProjectMetadataCell, error) {
	value := *input.Sum / float64(input.NumPlots)
	if average == 2 {
		value = *input.Average
	}
	var result ProjectMetadataCell
	if value < 0.01 {
		floor := 0.01
		result = ProjectMetadataCell{Storage: "real", Real: &floor}
	} else {
		text := siteUnitFixedNumber(value, 2, 1)
		result = ProjectMetadataCell{Storage: "text", Text: &text}
	}
	if err := ctx.Err(); err != nil {
		return ProjectMetadataCell{}, err
	}
	return result, nil
}
