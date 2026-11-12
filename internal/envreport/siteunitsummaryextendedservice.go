package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"os"
)

const SiteUnitSummaryExtendedWorkbookFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK"

type SiteUnitSummaryExtendedWorkbookService struct {
	contexts *vprocore.ContextService
	enabled  bool
}

func NewSiteUnitSummaryExtendedWorkbookService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*SiteUnitSummaryExtendedWorkbookService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := vprocore.SiviFeature(SiteUnitSummaryExtendedWorkbookFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SiteUnitSummaryExtendedWorkbookService{contexts, enabled}, nil
}

func (s *SiteUnitSummaryExtendedWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Extended Summary workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Extended Summary workbook is disabled in this session")
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
