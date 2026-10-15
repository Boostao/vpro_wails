package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
)

func prepareSummaryVegetationEnvironmentalCover(ctx context.Context, function, suffix string, siteUnit *string,
	env, su ProjectMetadataTable) (summaryVegetationEnvironmentalDepthResult, error) {
	fail := func(err error) (summaryVegetationEnvironmentalDepthResult, error) {
		return summaryVegetationEnvironmentalDepthResult{}, fmt.Errorf("environmental numeric %s suffix%q: %w", function, suffix, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	field := ""
	switch function {
	case "SVSubstrate":
		switch suffix {
		case "DecWood", "BedRock", "Rocks", "MineralSoil", "OrganicMatter", "Water":
			field = "Substrate" + suffix
		}
	case "SVStrataCover":
		switch suffix {
		case "Tree", "Shrub", "Herb", "Moss":
			field = "StrataCover" + suffix
		}
	}
	if field == "" {
		return fail(errors.New("function/suffix has no calibrated original physical cover field"))
	}
	aggregate, err := readSummaryEnvironmentalSingleAggregate(ctx, field, siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	if aggregate.Count == 0 {
		return summaryVegetationEnvironmentalDepthResult{Empty: true}, nil
	}
	maximum := aggregate.FormattedMaximum
	if aggregate.Nulls != 0 {
		maximum = math.Max(maximum, 0)
	}
	text := siteUnitNumericString(aggregate.FormattedMinimum) + " - " +
		siteUnitFixedNumber(aggregate.Average, 1, 1) + " - " + siteUnitNumericString(maximum)
	if aggregate.Nulls != 0 {
		text += "  (Null " + strconv.FormatInt(aggregate.Nulls, 10) + ")"
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return summaryVegetationEnvironmentalDepthResult{Text: text}, nil
}
