package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

const siviParentReviewFeatureEnvironment = "VPRO_SIVI_PARENT_REVIEW"

func SiviParentReviewFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(siviParentReviewFeatureEnvironment, lookup)
}

func GetSIVIParentOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !s.SiviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return sourceform.ReadSIVIParent(s, ctx, contextID, plot)
}

func GetSIVIParentJoinReview(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentJoinReview, error) {
	if !s.SiviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SIVIParentJoinReview, error) {
		return sourceform.ReadOwnedSIVIParentJoin(ctx, plots, contextID, plot)
	})
}

func GetSIVIProjectIDChoices(s *vprocore.ContextService, ctx context.Context, contextID string) (*sourceform.SIVIProjectChoices, error) {
	if !s.SiviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return sourceform.ReadSIVIProjectChoices(s, ctx, contextID)
}

func SetSIVIProjectIDSource(s *vprocore.ContextService, ctx context.Context, contextID string, expected, source int) (*sourceform.SIVIProjectChoices, error) {
	if !s.SiviParentReviewEnabled {
		return nil, errors.New("SIVI parent read-only review is disabled in this session")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SIVIProjectChoices, error) {
		choices, err := sourceform.ReadOwnedSIVIProjectChoicesAtSource(ctx, plots, contextID, source)
		if err != nil {
			return nil, err
		}
		if err := plots.Projects.Preferences.CompareAndSetProjectIDSource(ctx, expected, source); err != nil {
			return nil, err
		}
		return choices, nil
	})
}
