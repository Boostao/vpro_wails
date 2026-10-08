package main

import (
	"context"
	"errors"
)

const plotLocationReviewFeatureEnvironment = "VPRO_PLOT_LOCATION_REVIEW"

func plotLocationReviewFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(plotLocationReviewFeatureEnvironment, lookup)
}

func (s *ContextService) GetPlotLocationReview(ctx context.Context, contextID string) (*PlotLocationReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.plotLocationReviewEnabled {
		return nil, errors.New("plot location read-only review is disabled in this session")
	}
	review, err := s.readPlotLocations(ctx, contextID)
	if err != nil {
		return nil, err
	}
	return review, nil
}
