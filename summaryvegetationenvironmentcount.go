package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

func calculateSummaryVegetationEnvironmentalPlotCount(ctx context.Context, siteUnit *string,
	groupPresent bool, nonNullPlotCount int64) (ProjectMetadataCell, error) {
	if err := ctx.Err(); err != nil {
		return ProjectMetadataCell{}, fmt.Errorf("SVnPlots environmental count: %w", err)
	}
	if siteUnit == nil {
		return ProjectMetadataCell{}, errors.New("SVnPlots environmental count requires a non-NULL Site Unit argument (source error94)")
	}
	if !groupPresent {
		return ProjectMetadataCell{}, errors.New("SVnPlots environmental count has no grouped source record (source error3021)")
	}
	if nonNullPlotCount < 0 {
		return ProjectMetadataCell{}, errors.New("SVnPlots environmental count requires a nonnegative prepared Count(PlotNumber)")
	}
	if nonNullPlotCount > 32767 {
		return ProjectMetadataCell{}, errors.New("SVnPlots environmental count exceeds the source Integer range (source overflow6)")
	}
	integer := strconv.FormatInt(nonNullPlotCount, 10)
	return ProjectMetadataCell{Storage: "integer", Integer: &integer}, nil
}
