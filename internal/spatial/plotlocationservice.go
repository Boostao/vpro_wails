package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

const plotLocationReviewFeatureEnvironment = "VPRO_PLOT_LOCATION_REVIEW"

func PlotLocationReviewFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(plotLocationReviewFeatureEnvironment, lookup)
}

func GetPlotLocationReview(s *vprocore.ContextService, ctx context.Context, contextID string) (*vprocore.PlotLocationReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.PlotLocationReviewEnabled {
		return nil, errors.New("plot location read-only review is disabled in this session")
	}
	review, err := s.ReadPlotLocations(ctx, contextID)
	if err != nil {
		return nil, err
	}
	return review, nil
}
