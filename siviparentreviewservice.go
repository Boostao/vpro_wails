package main

import (
	"context"
	"errors"
)

const siviParentReviewFeatureEnvironment = "VPRO_SIVI_PARENT_REVIEW"

func siviParentReviewFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(siviParentReviewFeatureEnvironment, lookup)
}

func (s *ContextService) GetSIVIParentOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !s.siviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return s.readSIVIParent(ctx, contextID, plot)
}

func (s *ContextService) GetSIVIParentJoinReview(ctx context.Context, contextID, plot string) (*SIVIParentJoinReview, error) {
	if !s.siviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*SIVIParentJoinReview, error) {
		return readOwnedSIVIParentJoin(ctx, plots, contextID, plot)
	})
}

func (s *ContextService) GetSIVIProjectIDChoices(ctx context.Context, contextID string) (*SIVIProjectChoices, error) {
	if !s.siviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return s.readSIVIProjectChoices(ctx, contextID)
}

func (s *ContextService) SetSIVIProjectIDSource(ctx context.Context, contextID string, expected, source int) (*SIVIProjectChoices, error) {
	if !s.siviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*SIVIProjectChoices, error) {
		choices, err := readOwnedSIVIProjectChoicesAtSource(ctx, plots, contextID, source)
		if err != nil {
			return nil, err
		}
		if err := plots.projects.preferences.compareAndSetProjectIDSource(ctx, expected, source); err != nil {
			return nil, err
		}
		return choices, nil
	})
}
