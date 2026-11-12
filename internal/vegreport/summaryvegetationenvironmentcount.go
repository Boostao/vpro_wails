package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

func calculateSummaryVegetationEnvironmentalPlotCount(ctx context.Context, siteUnit *string,
	groupPresent bool, nonNullPlotCount int64) (vprocore.ProjectMetadataCell, error) {
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("SVnPlots environmental count: %w", err)
	}
	if siteUnit == nil {
		return vprocore.ProjectMetadataCell{}, errors.New("SVnPlots environmental count requires a non-NULL Site Unit argument (source error94)")
	}
	if !groupPresent {
		return vprocore.ProjectMetadataCell{}, errors.New("SVnPlots environmental count has no grouped source record (source error3021)")
	}
	if nonNullPlotCount < 0 {
		return vprocore.ProjectMetadataCell{}, errors.New("SVnPlots environmental count requires a nonnegative prepared Count(PlotNumber)")
	}
	if nonNullPlotCount > 32767 {
		return vprocore.ProjectMetadataCell{}, errors.New("SVnPlots environmental count exceeds the source Integer range (source overflow6)")
	}
	integer := strconv.FormatInt(nonNullPlotCount, 10)
	return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &integer}, nil
}
