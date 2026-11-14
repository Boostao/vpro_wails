package vegreport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
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

func prepareSummaryVegetationEnvironmentalCover(ctx context.Context, function, suffix string, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (summaryVegetationEnvironmentalDepthResult, error) {
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
	text := reportcore.SiteUnitNumericString(aggregate.FormattedMinimum) + " - " +
		reportcore.SiteUnitFixedNumber(aggregate.Average, 1, 1) + " - " + reportcore.SiteUnitNumericString(maximum)
	if aggregate.Nulls != 0 {
		text += "  (Null " + strconv.FormatInt(aggregate.Nulls, 10) + ")"
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return summaryVegetationEnvironmentalDepthResult{Text: text}, nil
}

type summaryVegetationEnvironmentalDepthResult struct {
	Empty bool
	Text  string
}

func prepareSummaryVegetationEnvironmentalDepth(ctx context.Context, function string, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (summaryVegetationEnvironmentalDepthResult, error) {
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
		reportcore.SiteUnitFixedNumber(float64(aggregate.Sum)/float64(aggregate.Count), 1, 1) + " - " + strconv.FormatInt(maximum, 10)
	if aggregate.Nulls != 0 {
		text += "  (Null " + strconv.FormatInt(aggregate.Nulls, 10) + ")"
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return summaryVegetationEnvironmentalDepthResult{Text: text}, nil
}

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

func summaryVegetationSMRClassCode(code string) string {
	switch code {
	case "0":
		return "VX"
	case "1":
		return "X"
	case "2":
		return "SX"
	case "3":
		return "SM"
	case "4":
		return "M"
	case "5":
		return "SHG"
	case "6":
		return "HG"
	case "7":
		return "SHD"
	case "8":
		return "HD"
	default:
		return ""
	}
}

type summaryEnvironmentalIntegerAggregate struct {
	Count, Nulls, Sum                  int64
	Minimum, Maximum                   int64
	FormattedMinimum, FormattedMaximum int64
}

func readSummaryEnvironmentalIntegerAggregate(ctx context.Context, field string, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (summaryEnvironmentalIntegerAggregate, error) {
	fail := func(err error) (summaryEnvironmentalIntegerAggregate, error) {
		return summaryEnvironmentalIntegerAggregate{}, err
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if siteUnit == nil {
		return fail(errors.New("original String argument rejects NULL (94)"))
	}
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber", field)
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vprocore.VegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s PlotNumber: %w", row.RowID, err))
		}
		if unit != nil && *unit == *siteUnit && plot != nil {
			if memberships[*plot] == math.MaxInt32 {
				return fail(errors.New("plot membership count exceeds source Long range"))
			}
			memberships[*plot]++
		}
	}
	var result summaryEnvironmentalIntegerAggregate
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[ec["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s PlotNumber: %w", row.RowID, err))
		}
		cell := row.Cells[ec[field]]
		value, err := vprocore.MetadataCellValue(cell)
		if err != nil {
			return fail(fmt.Errorf("Env row %s %s: %w", row.RowID, field, err))
		}
		number, integer := value.(int64)
		if cell.Storage != "null" && (!integer || number < -32768 || number > 32767) {
			return fail(fmt.Errorf("Env row %s %s requires original signed16 INTEGER storage", row.RowID, field))
		}
		if plot == nil || memberships[*plot] == 0 {
			continue
		}
		weight := memberships[*plot]
		if weight > math.MaxInt32-result.Count-result.Nulls {
			return fail(errors.New("joined integer count exceeds source Long range"))
		}
		if cell.Storage == "null" {
			result.Nulls += weight
			continue
		}
		// VBA Val stops at the comma inserted by the original thousands format.
		formatted := number
		if number <= -1000 || number >= 1000 {
			formatted /= 1000
		}
		if result.Count == 0 || number < result.Minimum {
			result.Minimum = number
		}
		if result.Count == 0 || number > result.Maximum {
			result.Maximum = number
		}
		if result.Count == 0 || formatted < result.FormattedMinimum {
			result.FormattedMinimum = formatted
		}
		if result.Count == 0 || formatted > result.FormattedMaximum {
			result.FormattedMaximum = formatted
		}
		result.Sum += number * weight
		result.Count += weight
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

type summaryEnvironmentalSingleAggregate struct {
	Count, Nulls                                                  int64
	Minimum, Maximum, FormattedMinimum, FormattedMaximum, Average float64
}

func readSummaryEnvironmentalSingleAggregate(ctx context.Context, field string, siteUnit *string,
	env, su vprocore.ProjectMetadataTable) (summaryEnvironmentalSingleAggregate, error) {
	fail := func(err error) (summaryEnvironmentalSingleAggregate, error) {
		return summaryEnvironmentalSingleAggregate{}, fmt.Errorf("environmental SINGLE %s: %w", field, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if siteUnit == nil {
		return fail(errors.New("original String argument rejects NULL (94)"))
	}
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber", field)
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vprocore.VegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s PlotNumber: %w", row.RowID, err))
		}
		if unit != nil && *unit == *siteUnit && plot != nil {
			if memberships[*plot] == math.MaxInt32 {
				return fail(errors.New("plot membership count exceeds source Long range"))
			}
			memberships[*plot]++
		}
	}
	var count, nulls int64
	var minimum, maximum, formattedMinimum, formattedMaximum float64
	sum := new(big.Rat)
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[ec["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s PlotNumber: %w", row.RowID, err))
		}
		cell := row.Cells[ec[field]]
		value, err := vprocore.MetadataCellValue(cell)
		if err != nil {
			return fail(fmt.Errorf("Env row %s %s: %w", row.RowID, field, err))
		}
		var number float64
		switch value := value.(type) {
		case nil:
		case int64:
			number = float64(value)
			if number < -16777216 || number > 16777216 {
				return fail(fmt.Errorf("Env row %s INTEGER %s exceeds lossless SINGLE conversion", row.RowID, field))
			}
		case float64:
			number = value
		default:
			return fail(fmt.Errorf("Env row %s %s requires original numeric SINGLE storage", row.RowID, field))
		}
		if math.IsNaN(number) || math.IsInf(number, 0) || float64(float32(number)) != number {
			return fail(fmt.Errorf("Env row %s %s requires exactly assigned finite SINGLE", row.RowID, field))
		}
		if plot == nil || memberships[*plot] == 0 {
			continue
		}
		weight := memberships[*plot]
		if weight > math.MaxInt32-count-nulls {
			return fail(errors.New("joined slope count exceeds source Long range"))
		}
		if cell.Storage == "null" {
			nulls += weight
			continue
		}
		if count == 0 || number < minimum {
			minimum = number
		}
		if count == 0 || number > maximum {
			maximum = number
		}
		formatted := reportcore.SiteUnitGroundExtremum(number)
		if count == 0 || formatted < formattedMinimum {
			formattedMinimum = formatted
		}
		if count == 0 || formatted > formattedMaximum {
			formattedMaximum = formatted
		}
		sum.Add(sum, new(big.Rat).Mul(new(big.Rat).SetFloat64(number), new(big.Rat).SetInt64(weight)))
		count += weight
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if count == 0 {
		return summaryEnvironmentalSingleAggregate{Nulls: nulls}, nil
	}
	average, _ := new(big.Rat).Quo(sum, new(big.Rat).SetInt64(count)).Float64()
	return summaryEnvironmentalSingleAggregate{Count: count, Nulls: nulls, Minimum: minimum, Maximum: maximum,
		FormattedMinimum: formattedMinimum, FormattedMaximum: formattedMaximum, Average: average}, nil
}
