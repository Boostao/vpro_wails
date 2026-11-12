package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

type BreakReportService struct {
	contexts *vprocore.ContextService
	enabled  bool
}

func NewBreakReportService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*BreakReportService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("PC-ORD Break Report requires owned contexts and explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(vprocore.BreakReportReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &BreakReportService{contexts: contexts, enabled: enabled}, nil
}

func (s *BreakReportService) GetReview(ctx context.Context, contextID string, offset, limit int) (BreakReportReview, error) {
	if ctx == nil {
		return BreakReportReview{}, errors.New("PC-ORD Break Report requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return BreakReportReview{}, err
	}
	if s == nil || !s.enabled || s.contexts == nil {
		return BreakReportReview{}, errors.New("PC-ORD Break Report review is independently disabled in this session")
	}
	return ReviewBreakReport(s.contexts, ctx, contextID, offset, limit)
}
