package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

type summaryVegetationEnvironmentalDepthResult struct {
	Empty bool
	Text  string
}

func prepareSummaryVegetationEnvironmentalDepth(ctx context.Context, function string, siteUnit *string,
	env, su ProjectMetadataTable) (summaryVegetationEnvironmentalDepthResult, error) {
	fail := func(err error) (summaryVegetationEnvironmentalDepthResult, error) {
		return summaryVegetationEnvironmentalDepthResult{}, fmt.Errorf("environmental integer depth %s: %w", function, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var field string
	switch function {
	case "SVSeepage":
		field = "SeepageDepth"
	case "SVRootingDepth":
		field = "RootingDepth"
	default:
		return fail(errors.New("function has no calibrated integer depth primitive"))
	}
	aggregate, err := readSummaryEnvironmentalIntegerAggregate(ctx, field, siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	if aggregate.Count == 0 {
		return summaryVegetationEnvironmentalDepthResult{Empty: true}, nil
	}
	maximum := aggregate.FormattedMaximum
	if aggregate.Nulls != 0 && maximum < 0 {
		maximum = 0
	}
	text := strconv.FormatInt(aggregate.FormattedMinimum, 10) + " - " +
		siteUnitFixedNumber(float64(aggregate.Sum)/float64(aggregate.Count), 1, 1) + " - " + strconv.FormatInt(maximum, 10)
	if aggregate.Nulls != 0 {
		text += "  (Null " + strconv.FormatInt(aggregate.Nulls, 10) + ")"
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return summaryVegetationEnvironmentalDepthResult{Text: text}, nil
}
