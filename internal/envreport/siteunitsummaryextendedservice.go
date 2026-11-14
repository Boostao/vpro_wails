package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

type SiteUnitSummaryExtendedWorkbookService struct {
	contexts *vprocore.ContextService
}

func NewSiteUnitSummaryExtendedWorkbookService(contexts *vprocore.ContextService) (*SiteUnitSummaryExtendedWorkbookService, error) {
	return &SiteUnitSummaryExtendedWorkbookService{contexts: contexts}, nil
}

func (s *SiteUnitSummaryExtendedWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Extended Summary workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errors.New("Extended Summary workbook service is unavailable")
	}
	return nil
}

func (s *SiteUnitSummaryExtendedWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryExtendedWorkbookReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	return reviewSiteUnitSummaryExtendedWorkbook(ctx, s.contexts, contextID, requestJSON)
}

func (s *SiteUnitSummaryExtendedWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryWorkbookOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	return exportSiteUnitSummaryExtendedWorkbook(ctx, s.contexts, contextID, requestJSON, siteUnitSummaryWorkbookHooks{})
}
