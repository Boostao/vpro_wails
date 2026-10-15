package main

import (
	"context"
	"errors"
)

const breakReportReviewEnvironment = "VPRO_BREAK_REPORT_REVIEW"

type BreakReportService struct {
	contexts *ContextService
	enabled  bool
}

func NewBreakReportService(contexts *ContextService, lookup func(string) (string, bool)) (*BreakReportService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("PC-ORD Break Report requires owned contexts and explicit feature lookup")
	}
	enabled, err := siviFeature(breakReportReviewEnvironment, lookup)
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
	return s.contexts.reviewBreakReport(ctx, contextID, offset, limit)
}
