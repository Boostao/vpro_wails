package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func prepareSummaryVegetationEnvironmentalDynamicCategory(ctx context.Context, function string, argument int,
	siteUnit *string, env, su ProjectMetadataTable) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) {
		return ProjectMetadataCell{}, fmt.Errorf("environmental dynamic category %s argument%d: %w", function, argument, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var field string
	switch function {
	case "SVBedrockType":
		if argument < 1 || argument > 3 {
			return fail(errors.New("only original physical bedrock layers1/2/3 are calibrated"))
		}
		field = "BedrockGeology" + strconv.Itoa(argument)
	case "SVSiteDisturbance":
		if argument < 1 || argument > 2 {
			return fail(errors.New("only disturbance fields1/2 are calibrated; field3 active source loop reads unprojected field2"))
		}
		field = "SiteDisturbance" + strconv.Itoa(argument)
	default:
		return fail(errors.New("function has no calibrated dynamic TEXT category primitive"))
	}
	if siteUnit == nil {
		return fail(errors.New("original String argument rejects NULL (94)"))
	}
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	categories, nulls, err := prepareSummaryEnvironmentalLiteralCategories(ctx, field, *siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	var text strings.Builder
	for _, category := range categories {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		text.WriteString(category.label)
		text.WriteString("(" + strconv.FormatInt(category.count, 10) + ")  ")
	}
	if nulls != 0 {
		text.WriteString("(Null " + strconv.FormatInt(nulls, 10) + ")")
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	value := text.String()
	return ProjectMetadataCell{Storage: "text", Text: &value}, nil
}
