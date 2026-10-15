package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
)

type summaryEnvironmentalSingleAggregate struct {
	Count, Nulls                                                  int64
	Minimum, Maximum, FormattedMinimum, FormattedMaximum, Average float64
}

func readSummaryEnvironmentalSingleAggregate(ctx context.Context, field string, siteUnit *string,
	env, su ProjectMetadataTable) (summaryEnvironmentalSingleAggregate, error) {
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
	var count, nulls int64
	var minimum, maximum, formattedMinimum, formattedMaximum float64
	sum := new(big.Rat)
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
		formatted := siteUnitGroundExtremum(number)
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
