package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"fmt"
	"strconv"
)

func prepareSummaryVegetationEnvironmentalSlope(ctx context.Context, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (summaryVegetationEnvironmentalDepthResult, error) {
	fail := func(err error) (summaryVegetationEnvironmentalDepthResult, error) {
		return summaryVegetationEnvironmentalDepthResult{}, fmt.Errorf("environmental SINGLE slope: %w", err)
	}
	aggregate, err := readSummaryEnvironmentalSingleAggregate(ctx, "SlopeGradient", siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	if aggregate.Count == 0 {
		return summaryVegetationEnvironmentalDepthResult{Empty: true}, nil
	}
	minText, err := formatSummaryVegetationSingle(aggregate.Minimum)
	if err != nil {
		return fail(err)
	}
	maxText, err := formatSummaryVegetationSingle(aggregate.Maximum)
	if err != nil {
		return fail(err)
	}
	text := minText + " - " + reportcore.SiteUnitFixedNumber(aggregate.Average, 0, 1) + " - " + maxText
	if aggregate.Nulls != 0 {
		text += "  (Null " + strconv.FormatInt(aggregate.Nulls, 10) + ")"
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return summaryVegetationEnvironmentalDepthResult{Text: text}, nil
}
