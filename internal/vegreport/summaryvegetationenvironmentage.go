package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func prepareSummaryVegetationEnvironmentalAgeElevation(ctx context.Context, function string, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("environmental integer age/elevation %s: %w", function, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var field string
	switch function {
	case "SVStandAge":
		field = "StandAge"
	case "SVElevation":
		field = "Elevation"
	default:
		return fail(errors.New("function has no calibrated integer age/elevation primitive"))
	}
	aggregate, err := readSummaryEnvironmentalIntegerAggregate(ctx, field, siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	text := ""
	if aggregate.Count != 0 {
		text = strconv.FormatInt(aggregate.Minimum, 10) + " - " +
			reportcore.SiteUnitFixedNumber(float64(aggregate.Sum)/float64(aggregate.Count), 0, 2) + " - " +
			strconv.FormatInt(aggregate.Maximum, 10)
	}
	if aggregate.Nulls != 0 {
		text += "  (Null " + strconv.FormatInt(aggregate.Nulls, 10) + ")"
	}
	if function == "SVStandAge" {
		text = strings.Trim(text, " ")
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &text}, nil
}
