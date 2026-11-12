package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
)

func prepareSummaryVegetationEnvironmentalPlotCount(ctx context.Context, siteUnit *string,
	su vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("SVnPlots literal SU source preparation: %w", err)
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
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	columns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	var count int64
	present := false
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vprocore.VegetationReportIdentity(row.Cells[columns["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[columns["PlotNumber"]])
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
