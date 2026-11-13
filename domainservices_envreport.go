package main

import (
	"context"
	"errors"

	rsidecarpkg "github.com/boostao/vpro-wails/internal/rsidecar"
)

type RPackageStatus = rsidecarpkg.RPackageStatus
type RRuntimeStatus = rsidecarpkg.RRuntimeStatus
type PlotClimateRow = rsidecarpkg.PlotClimateRow
type EdatopicGridCell = rsidecarpkg.EdatopicGridCell
type EdatopicGridMatrix = rsidecarpkg.EdatopicGridMatrix
type DualUnitEdatopicSummary = rsidecarpkg.DualUnitEdatopicSummary
type ClimateEdatopicRequest = rsidecarpkg.ClimateEdatopicRequest
type ClimateEdatopicResult = rsidecarpkg.ClimateEdatopicResult
type ClusterIndicatorSpecies = rsidecarpkg.ClusterIndicatorSpecies
type ClusterUnitSummary = rsidecarpkg.ClusterUnitSummary
type ClusterPlotMembership = rsidecarpkg.ClusterPlotMembership
type DendrogramSegment = rsidecarpkg.DendrogramSegment
type DendrogramLeaf = rsidecarpkg.DendrogramLeaf
type DendrogramGeometry = rsidecarpkg.DendrogramGeometry
type ClusteringRequest = rsidecarpkg.ClusteringRequest
type ClusteringResult = rsidecarpkg.ClusteringResult
type PromoteCandidateSURequest = rsidecarpkg.PromoteCandidateSURequest
type PromoteCandidateSUResult = rsidecarpkg.PromoteCandidateSUResult
type QuartoVegSummaryRow = rsidecarpkg.QuartoVegSummaryRow
type QuartoGuideRequest = rsidecarpkg.QuartoGuideRequest
type QuartoGuideResult = rsidecarpkg.QuartoGuideResult

type EnvironmentReportService struct {
	contexts                        *ContextService
	environmentWorkbook             *EnvironmentWorkbookService
	longEnvironmentPreferences      *LongEnvironmentPreferencesService
	siteUnitSummaryPreferences      *SiteUnitSummaryPreferencesService
	siteUnitSummaryWorkbook         *SiteUnitSummaryWorkbookService
	siteUnitSummaryExtendedWorkbook *SiteUnitSummaryExtendedWorkbookService
	subzoneMatrix                   *SubzoneMatrixService
	dataDir                         string
}

func (s *EnvironmentReportService) rRuntimeOptions() []rsidecarpkg.RuntimeOptions {
	if s == nil || s.dataDir == "" {
		return nil
	}
	return []rsidecarpkg.RuntimeOptions{{DataDir: s.dataDir}}
}

func (s *EnvironmentReportService) GetLongEnvironmentOptions(ctx context.Context, contextID string) (LongEnvironmentOptions, error) {
	if s == nil || s.contexts == nil {
		return LongEnvironmentOptions{}, errors.New("environment report context is unavailable")
	}
	return s.contexts.GetLongEnvironmentOptions(ctx, contextID)
}

func (s *EnvironmentReportService) PreviewLongEnvironment(ctx context.Context, contextID string, request LongEnvironmentRequest) (LongEnvironmentPreview, error) {
	if s == nil || s.contexts == nil {
		return LongEnvironmentPreview{}, errors.New("environment report context is unavailable")
	}
	return s.contexts.PreviewLongEnvironment(ctx, contextID, request)
}

func (s *EnvironmentReportService) GetLongEnvironmentPreferences(ctx context.Context, contextID string) (*LongEnvironmentPreferencesReview, error) {
	if s == nil || s.longEnvironmentPreferences == nil {
		return nil, errors.New("long environment preferences service is unavailable")
	}
	return s.longEnvironmentPreferences.GetLongEnvironmentPreferences(ctx, contextID)
}

func (s *EnvironmentReportService) SaveLongEnvironmentPreferences(ctx context.Context, contextID, requestJSON string) *LongEnvironmentPreferencesOutcome {
	if s == nil || s.longEnvironmentPreferences == nil {
		return &LongEnvironmentPreferencesOutcome{ContextID: contextID, ErrorMessage: "long environment preferences service is unavailable"}
	}
	return s.longEnvironmentPreferences.SaveLongEnvironmentPreferences(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) GetEnvironmentWorkbookReview(ctx context.Context, contextID, requestJSON string) (*EnvironmentWorkbookReview, error) {
	if s == nil || s.environmentWorkbook == nil {
		return nil, errors.New("environment workbook service is unavailable")
	}
	return s.environmentWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) ExportReviewedEnvironmentWorkbook(ctx context.Context, contextID, requestJSON string) (*EnvironmentWorkbookOutcome, error) {
	if s == nil || s.environmentWorkbook == nil {
		return nil, errors.New("environment workbook service is unavailable")
	}
	return s.environmentWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) GetSiteUnitSummaryOptions(ctx context.Context, contextID string) (SiteUnitSummaryOptions, error) {
	if s == nil || s.contexts == nil {
		return SiteUnitSummaryOptions{}, errors.New("environment report context is unavailable")
	}
	return s.contexts.GetSiteUnitSummaryOptions(ctx, contextID)
}

func (s *EnvironmentReportService) PreviewSiteUnitSummary(ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if s == nil || s.contexts == nil {
		return SiteUnitSummaryPreview{}, errors.New("environment report context is unavailable")
	}
	return s.contexts.PreviewSiteUnitSummary(ctx, contextID, request)
}

func (s *EnvironmentReportService) PreviewSiteUnitSummaryLifeforms(ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if s == nil || s.contexts == nil {
		return SiteUnitSummaryPreview{}, errors.New("environment report context is unavailable")
	}
	return s.contexts.PreviewSiteUnitSummaryLifeforms(ctx, contextID, request)
}

func (s *EnvironmentReportService) PreviewSiteUnitSummarySpecies(ctx context.Context, contextID string, request SiteUnitSpeciesListRequest) (SiteUnitSpeciesListPreview, error) {
	if s == nil || s.contexts == nil {
		return SiteUnitSpeciesListPreview{}, errors.New("environment report context is unavailable")
	}
	return s.contexts.PreviewSiteUnitSummarySpecies(ctx, contextID, request)
}

func (s *EnvironmentReportService) GetSiteUnitSummaryPreferences(ctx context.Context, contextID string) (*SiteUnitSummaryOptions, error) {
	if s == nil || s.siteUnitSummaryPreferences == nil {
		return nil, errors.New("site unit summary preferences service is unavailable")
	}
	return s.siteUnitSummaryPreferences.GetSiteUnitSummaryPreferences(ctx, contextID)
}

func (s *EnvironmentReportService) SaveSiteUnitSummaryPreferences(ctx context.Context, contextID, requestJSON string) *SiteUnitSummaryPreferencesOutcome {
	if s == nil || s.siteUnitSummaryPreferences == nil {
		return &SiteUnitSummaryPreferencesOutcome{ContextID: contextID, ErrorMessage: "site unit summary preferences service is unavailable"}
	}
	return s.siteUnitSummaryPreferences.SaveSiteUnitSummaryPreferences(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) GetSiteUnitSummaryWorkbookReview(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryWorkbookReview, error) {
	if s == nil || s.siteUnitSummaryWorkbook == nil {
		return nil, errors.New("site unit summary workbook service is unavailable")
	}
	return s.siteUnitSummaryWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) ExportReviewedSiteUnitSummaryWorkbook(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryWorkbookOutcome, error) {
	if s == nil || s.siteUnitSummaryWorkbook == nil {
		return nil, errors.New("site unit summary workbook service is unavailable")
	}
	return s.siteUnitSummaryWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) GetSiteUnitSummaryExtendedWorkbookReview(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryExtendedWorkbookReview, error) {
	if s == nil || s.siteUnitSummaryExtendedWorkbook == nil {
		return nil, errors.New("site unit summary extended workbook service is unavailable")
	}
	return s.siteUnitSummaryExtendedWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) ExportReviewedSiteUnitSummaryExtendedWorkbook(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryWorkbookOutcome, error) {
	if s == nil || s.siteUnitSummaryExtendedWorkbook == nil {
		return nil, errors.New("site unit summary extended workbook service is unavailable")
	}
	return s.siteUnitSummaryExtendedWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) GetSubzoneMatrixPreview(ctx context.Context, contextID, requestJSON string) (*SubzoneMatrixPreview, error) {
	if s == nil || s.subzoneMatrix == nil {
		return nil, errors.New("subzone matrix service is unavailable")
	}
	return s.subzoneMatrix.GetPreview(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) GetSubzoneMatrixReview(ctx context.Context, contextID string) (*SubzoneMatrixWorkbookReview, error) {
	if s == nil || s.subzoneMatrix == nil {
		return nil, errors.New("subzone matrix service is unavailable")
	}
	return s.subzoneMatrix.GetReview(ctx, contextID)
}

func (s *EnvironmentReportService) ExportReviewedSubzoneMatrix(ctx context.Context, contextID, requestJSON string) (*SubzoneMatrixWorkbookOutcome, error) {
	if s == nil || s.subzoneMatrix == nil {
		return nil, errors.New("subzone matrix service is unavailable")
	}
	return s.subzoneMatrix.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *EnvironmentReportService) DetectRRuntime(ctx context.Context) (*RRuntimeStatus, error) {
	return rsidecarpkg.DetectRRuntime(ctx, s.rRuntimeOptions()...)
}

func (s *EnvironmentReportService) GetCachedPlotClimate(ctx context.Context, contextID, refPeriod string) ([]PlotClimateRow, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("environment report context is unavailable")
	}
	return rsidecarpkg.GetCachedPlotClimate(ctx, unwrapContextService(s.contexts), contextID, refPeriod)
}

func (s *EnvironmentReportService) ComputePlotClimateAndDualEdatopic(ctx context.Context, contextID string, request ClimateEdatopicRequest) (*ClimateEdatopicResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("environment report context is unavailable")
	}
	return rsidecarpkg.ComputePlotClimateAndDualEdatopic(ctx, unwrapContextService(s.contexts), contextID, request, s.rRuntimeOptions()...)
}

func (s *EnvironmentReportService) RunMultivariateClustering(ctx context.Context, contextID string, request ClusteringRequest) (*ClusteringResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("environment report context is unavailable")
	}
	return rsidecarpkg.RunMultivariateClustering(ctx, unwrapContextService(s.contexts), contextID, request, s.rRuntimeOptions()...)
}

func (s *EnvironmentReportService) PromoteClusterCandidateToSU(ctx context.Context, contextID string, request PromoteCandidateSURequest) (*PromoteCandidateSUResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("environment report context is unavailable")
	}
	return rsidecarpkg.PromoteClusterCandidateToSU(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *EnvironmentReportService) RenderQuartoGuide(ctx context.Context, contextID string, request QuartoGuideRequest) (*QuartoGuideResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("environment report context is unavailable")
	}
	return rsidecarpkg.RenderQuartoGuide(ctx, unwrapContextService(s.contexts), contextID, request, s.rRuntimeOptions()...)
}
