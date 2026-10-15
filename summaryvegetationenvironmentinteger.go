package main

import (
	"context"
	"errors"
	"fmt"
	"math"
)

type summaryEnvironmentalIntegerAggregate struct {
	Count, Nulls, Sum                  int64
	Minimum, Maximum                   int64
	FormattedMinimum, FormattedMaximum int64
}

func readSummaryEnvironmentalIntegerAggregate(ctx context.Context, field string, siteUnit *string,
	env, su ProjectMetadataTable) (summaryEnvironmentalIntegerAggregate, error) {
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
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := siteUnitTransferColumns(env, "PlotNumber", field)
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vegetationReportIdentity(row.Cells[sc["PlotNumber"]])
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
		plot, err := vegetationReportIdentity(row.Cells[ec["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s PlotNumber: %w", row.RowID, err))
		}
		cell := row.Cells[ec[field]]
		value, err := metadataCellValue(cell)
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
