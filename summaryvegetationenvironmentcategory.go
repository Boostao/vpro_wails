package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type summaryVegetationEnvironmentalCategory struct {
	label string
	count int64
}

func formatSummaryVegetationEnvironmentalCategories(ctx context.Context, function string, siteUnit *string,
	categories []summaryVegetationEnvironmentalCategory, nullCount int64) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) {
		return ProjectMetadataCell{}, fmt.Errorf("environmental categorical summary %s: %w", function, err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if _, supported := summaryVegetationEnvironmentalCategoryField(function); !supported {
		return fail(errors.New("function has no calibrated TEXT category renderer"))
	}
	if siteUnit == nil {
		return fail(errors.New("requires a non-NULL Site Unit argument (source error94)"))
	}
	if !utf8.ValidString(*siteUnit) || strings.ContainsRune(*siteUnit, 0) {
		return fail(errors.New("prepared Site Unit contains malformed text"))
	}
	if nullCount < 0 || nullCount > 2147483647 {
		return fail(errors.New("prepared NULL count is outside the nonnegative source Long range"))
	}
	var result strings.Builder
	for _, category := range categories {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if !utf8.ValidString(category.label) || strings.ContainsRune(category.label, 0) {
			return fail(errors.New("prepared category label contains malformed text"))
		}
		if category.count <= 0 || category.count > 2147483647 {
			return fail(errors.New("prepared category count is outside the positive source Long range"))
		}
		label := category.label
		if function == "SVSmr" {
			label = summaryVegetationSMRClassCode(label)
		}
		result.WriteString(label)
		result.WriteByte('(')
		result.WriteString(strconv.FormatInt(category.count, 10))
		result.WriteString(")  ")
	}
	if nullCount > 0 {
		result.WriteString("(Null ")
		result.WriteString(strconv.FormatInt(nullCount, 10))
		result.WriteByte(')')
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	text := result.String()
	return ProjectMetadataCell{Storage: "text", Text: &text}, nil
}
