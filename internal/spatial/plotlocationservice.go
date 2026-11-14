package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

func GetPlotLocationReview(s *vprocore.ContextService, ctx context.Context, contextID string) (*vprocore.PlotLocationReview, error) {
	if ctx == nil {
		return nil, errors.New("plot location review requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("plot location review requires owned contexts")
	}
	return s.ReadPlotLocations(ctx, contextID)
}
