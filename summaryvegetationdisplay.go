package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

type summaryVegetationDisplayCell struct {
	Value ProjectMetadataCell
	Order ProjectMetadataCell
}

func validateSummaryVegetationDisplayOptions(ctx context.Context, average, display, showClass int) error {
	if ctx == nil {
		return errors.New("summary vegetation display requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if average != 1 && average != 2 {
		return errors.New("summary vegetation display requires original Calculate1 or2")
	}
	if showClass != 0 {
		return validateSummaryVegetationClassMode(ctx, display, showClass)
	}
	if display != 1 && display != 2 && display != 3 && display != 6 {
		return errors.New("summary vegetation display requires original value1,2,3 or6; class modes require explicit Show")
	}
	return nil
}

func calculateSummaryVegetationClassDisplay(ctx context.Context, input summaryVegetationCellInput,
	average, display, showClass int) (summaryVegetationDisplayCell, error) {
	fail := func(err error) (summaryVegetationDisplayCell, error) { return summaryVegetationDisplayCell{}, err }
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return fail(err)
	}
	if showClass == 0 {
		return fail(errors.New("summary vegetation class display requires explicit ShowClass1,10 or20"))
	}
	if err := validateSummaryVegetationCellInput(ctx, input, 1); err != nil {
		return fail(err)
	}
	cell, err := calculateSummaryVegetationClassFormula(ctx, input.Sum, input.Count, input.NumPlots, display, showClass)
	if err != nil {
		return fail(err)
	}
	null := ProjectMetadataCell{Storage: "null"}
	if cell.Storage == "null" {
		return summaryVegetationDisplayCell{null, null}, nil
	}
	var text string
	if cell.Integer != nil {
		text = *cell.Integer
	} else {
		text, err = formatSummaryVegetationSingle(*cell.Real)
		if err != nil {
			return fail(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	order := text
	return summaryVegetationDisplayCell{
		Value: ProjectMetadataCell{Storage: "text", Text: &text},
		Order: ProjectMetadataCell{Storage: "text", Text: &order},
	}, nil
}

func summaryVegetationPresenceClass(value float64) string {
	switch {
	case value < 0 || value > 1:
		return ""
	case value <= 0.2:
		return "I"
	case value <= 0.4:
		return "II"
	case value <= 0.6:
		return "III"
	case value <= 0.8:
		return "IV"
	default:
		return "V"
	}
}

func summaryVegetationSignificanceClass(value float64) string {
	for i, limit := range []float64{0.3, 1, 2.2, 5, 10, 20, 33, 50, 75} {
		if value <= limit {
			if i == 0 {
				return "+"
			}
			return strconv.Itoa(i)
		}
	}
	return "9"
}

// An absent unit group is handled by the pivot caller, not evaluated as a
// physical all-NULL group. Original modes1/3 fail on the latter projection.
func calculateSummaryVegetationDisplay(ctx context.Context, input summaryVegetationCellInput,
	average, display int) (summaryVegetationDisplayCell, error) {
	fail := func(err error) (summaryVegetationDisplayCell, error) { return summaryVegetationDisplayCell{}, err }
	if display != 1 && display != 2 && display != 3 && display != 6 {
		return fail(errors.New("summary vegetation display requires original value1,2,3 or6; other modes unavailable"))
	}
	if err := validateSummaryVegetationCellInput(ctx, input, average); err != nil {
		return fail(err)
	}
	null := ProjectMetadataCell{Storage: "null"}
	if input.Count == 0 {
		if display == 1 || display == 3 {
			return fail(fmt.Errorf("summary vegetation display%d all-NULL staged group has original criteria type mismatch; projection unavailable", display))
		}
		return summaryVegetationDisplayCell{null, null}, nil
	}
	ratio := float64(input.Count) / float64(input.NumPlots)
	prefix := ""
	if display == 1 || display == 6 {
		if siteUnitFixedNumber(ratio, 4, 1) == "0.0000" {
			return fail(errors.New("summary vegetation formatted cover count is empty; original numeric criteria comparison is unavailable"))
		}
	}
	if display == 1 {
		prefix = siteUnitFixedNumber(ratio, 2, 1)
	} else if display == 2 {
		formatted, err := strconv.ParseFloat(siteUnitFixedNumber(ratio, 2, 1), 64)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation formatted presence: %w", err))
		}
		prefix = summaryVegetationPresenceClass(formatted)
		if prefix == "" {
			return summaryVegetationDisplayCell{null, null}, nil
		}
	} else if display == 3 {
		prefix = summaryVegetationPresenceClass(ratio)
	}
	cover, err := formatSummaryVegetationCover(ctx, input, average)
	if err != nil {
		return fail(err)
	}
	text := "0.01"
	if cover.Text != nil {
		text = *cover.Text
	}
	if display == 2 {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation formatted significance: %w", err))
		}
		text = summaryVegetationSignificanceClass(value)
	}
	order := text
	if display != 6 {
		text = prefix + " - " + text
		order = text
		if display == 1 || display == 3 {
			order = prefix
		}
	}
	result := summaryVegetationDisplayCell{
		Value: ProjectMetadataCell{Storage: "text", Text: &text}, Order: null,
	}
	if order != "" {
		result.Order = ProjectMetadataCell{Storage: "text", Text: &order}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
