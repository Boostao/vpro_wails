package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

func requireSIVIParentReview(s *vprocore.ContextService, ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI parent review requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errors.New("SIVI parent review requires owned contexts")
	}
	return nil
}

func GetSIVIParentOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentProjection, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIParent(s, ctx, contextID, plot)
}

func GetSIVIParentJoinReview(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentJoinReview, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SIVIParentJoinReview, error) {
		return sourceform.ReadOwnedSIVIParentJoin(ctx, plots, contextID, plot)
	})
}

func GetSIVIProjectIDChoices(s *vprocore.ContextService, ctx context.Context, contextID string) (*sourceform.SIVIProjectChoices, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIProjectChoices(s, ctx, contextID)
}

func SetSIVIProjectIDSource(s *vprocore.ContextService, ctx context.Context, contextID string, expected, source int) (*sourceform.SIVIProjectChoices, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
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
