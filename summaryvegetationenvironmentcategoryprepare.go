package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

func prepareSummaryVegetationEnvironmentalCategories(ctx context.Context, function string, siteUnit *string,
	env, su ProjectMetadataTable) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) {
		return ProjectMetadataCell{}, fmt.Errorf("environmental categorical %s literal source preparation: %w", function, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if function == "SVBioGeoClimaticUnit" {
		return prepareSummaryVegetationBGCUnit(ctx, siteUnit, env, su)
	}
	if function == "SVAspect" {
		projected, err := projectSummaryVegetationAspect(ctx, env)
		if err != nil {
			return fail(err)
		}
		env = projected
	}
	field, supported := summaryVegetationEnvironmentalCategoryField(function)
	if !supported {
		return fail(errors.New("function has no calibrated TEXT category preparation"))
	}
	if siteUnit == nil {
		return formatSummaryVegetationEnvironmentalCategories(ctx, function, nil, nil, 0)
	}
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	categories, nulls, err := prepareSummaryEnvironmentalLiteralCategories(ctx, field, *siteUnit, env, su)
	if err != nil {
		return fail(err)
	}
	return formatSummaryVegetationEnvironmentalCategories(ctx, function, siteUnit, categories, nulls)
}

func prepareSummaryEnvironmentalLiteralCategories(ctx context.Context, field, siteUnit string,
	env, su ProjectMetadataTable) ([]summaryVegetationEnvironmentalCategory, int64, error) {
	fail := func(err error) ([]summaryVegetationEnvironmentalCategory, int64, error) {
		return nil, 0, err
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
		if unit != nil && *unit == siteUnit && plot != nil {
			memberships[*plot]++
		}
	}
	counts := map[string]int64{}
	var nulls int64
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vegetationReportIdentity(row.Cells[ec["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s PlotNumber: %w", row.RowID, err))
		}
		label, err := vegetationReportIdentity(row.Cells[ec[field]])
		if err != nil {
			return fail(fmt.Errorf("Env row %s %s: %w", row.RowID, field, err))
		}
		if plot == nil || memberships[*plot] == 0 {
			continue
		}
		weight := memberships[*plot]
		if label == nil {
			if weight > 2147483647-nulls {
				return fail(errors.New("joined NULL count exceeds source Long range"))
			}
			nulls += weight
		} else {
			if weight > 2147483647-counts[*label] {
				return fail(errors.New("joined category count exceeds source Long range"))
			}
			counts[*label] += weight
		}
	}
	if len(counts) > 32766 {
		return fail(errors.New("original large Integer category-loop boundary is unavailable"))
	}
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	categories := make([]summaryVegetationEnvironmentalCategory, 0, len(labels))
	for _, label := range labels {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		categories = append(categories, summaryVegetationEnvironmentalCategory{label: label, count: counts[label]})
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return categories, nulls, nil
}
