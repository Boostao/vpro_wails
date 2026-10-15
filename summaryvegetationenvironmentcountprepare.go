package main

import (
	"context"
	"errors"
	"fmt"
)

func prepareSummaryVegetationEnvironmentalPlotCount(ctx context.Context, siteUnit *string,
	su ProjectMetadataTable) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) {
		return ProjectMetadataCell{}, fmt.Errorf("SVnPlots literal SU source preparation: %w", err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if siteUnit == nil {
		return calculateSummaryVegetationEnvironmentalPlotCount(ctx, nil, false, 0)
	}
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	columns, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	var count int64
	present := false
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vegetationReportIdentity(row.Cells[columns["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vegetationReportIdentity(row.Cells[columns["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("row %s PlotNumber: %w", row.RowID, err))
		}
		if unit != nil && *unit == *siteUnit {
			present = true
			if plot != nil {
				count++
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return calculateSummaryVegetationEnvironmentalPlotCount(ctx, siteUnit, present, count)
}
