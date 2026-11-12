package main

import (
	"context"
	"errors"

	vegreportpkg "github.com/boostao/vpro-wails/internal/vegreport"
)

type SiteUnitExclusionRule = vegreportpkg.SiteUnitExclusionRule
type ExcludedSiteUnitInfo = vegreportpkg.ExcludedSiteUnitInfo
type RetainedSiteUnitInfo = vegreportpkg.RetainedSiteUnitInfo
type SiteUnitExclusionSummary = vegreportpkg.SiteUnitExclusionSummary
type HierarchyBreakpointOptions = vegreportpkg.HierarchyBreakpointOptions
type HierarchyBreakpointUnitMapping = vegreportpkg.HierarchyBreakpointUnitMapping
type HierarchyBreakpointBucket = vegreportpkg.HierarchyBreakpointBucket
type HierarchyBreakpointResolution = vegreportpkg.HierarchyBreakpointResolution
type HierarchyBreakpointRequest = vegreportpkg.HierarchyBreakpointRequest
type SiteUnitExclusionPreviewRequest = vegreportpkg.SiteUnitExclusionPreviewRequest
type SiteUnitExclusionPreview = vegreportpkg.SiteUnitExclusionPreview
type SummaryVegetationExclusionRequest = vegreportpkg.SummaryVegetationExclusionRequest
type SummaryVegetationExclusionPreview = vegreportpkg.SummaryVegetationExclusionPreview
type LongVegetationExclusionRequest = vegreportpkg.LongVegetationExclusionRequest
type LongVegetationExclusionPreview = vegreportpkg.LongVegetationExclusionPreview

type VegetationReportService struct {
	contexts                       *ContextService
	summary                        *SummaryVegetationService
	summaryEnvironment             *SummaryVegetationEnvironmentalService
	summaryEnvironmentLifeform     *SummaryVegetationEnvironmentalLifeformService
	summaryWorkbook                *SummaryVegetationWorkbookService
	summaryPreferences             *SummaryVegetationPreferencesService
	summaryQualityPreferences      *SummaryVegetationQualityPreferencesService
	summaryPresentationPreferences *SummaryVegetationPresentationPreferencesService
	summaryFieldList               *SummaryVegetationFieldListService
	longPreferences                *LongVegetationPreferencesService
	longStatisticsPreferences      *LongVegetationStatisticsPreferencesService
	longSwitchPreferences          *LongVegetationSwitchPreferencesService
	longThresholdPreferences       *LongVegetationThresholdPreferencesService
	longGroupPreferences           *LongVegetationGroupPreferencesService
	longQualityNullPreferences     *LongVegetationQualityNullPreferencesService
	longQualityMinimumPreferences  *LongVegetationQualityMinimumPreferencesService
	longNamingPreferences          *LongVegetationNamingPreferencesService
	longSelectedFieldPreferences   *LongVegetationSelectedFieldPreferencesService
	longEnforcementPreferences     *LongVegetationEnforcementPreferencesService
	longUnitPreferences            *LongVegetationUnitPreferencesService
	longSelectedUnitSelection      *LongVegetationSelectedUnitSelectionService
	longReportScope                *LongVegetationReportScopeService
	longProjectWorkbook            *LongVegetationProjectWorkbookService
	vegetationWorkbook             *VegetationWorkbookService
	lifeformSummary                *LifeformSummaryService
	lifeformWorkbook               *LifeformWorkbookService
	speciesAttributeSummary        *SpeciesAttributeSummaryService
}

// Summary Vegetation report & workbook delegates

func (s *VegetationReportService) PreviewSummaryVegetation(ctx context.Context, contextID string, average, display int) (SummaryVegetationPreview, error) {
	if s == nil || s.summary == nil {
		return SummaryVegetationPreview{}, errors.New("summary vegetation service is unavailable")
	}
	return s.summary.Preview(ctx, contextID, average, display)
}

func (s *VegetationReportService) PreviewSummaryVegetationClass(ctx context.Context, contextID string, average, display, showClass int) (SummaryVegetationPreview, error) {
	if s == nil || s.summary == nil {
		return SummaryVegetationPreview{}, errors.New("summary vegetation service is unavailable")
	}
	return s.summary.PreviewClass(ctx, contextID, average, display, showClass)
}

func (s *VegetationReportService) PreviewSummaryVegetationGrouped(ctx context.Context, contextID string, average, groupAndOrder int) (SummaryVegetationPreview, error) {
	if s == nil || s.summary == nil {
		return SummaryVegetationPreview{}, errors.New("summary vegetation service is unavailable")
	}
	return s.summary.PreviewGrouped(ctx, contextID, average, groupAndOrder)
}

func (s *VegetationReportService) PreviewSummaryVegetationGroupedDisplay(ctx context.Context, contextID string, average, groupAndOrder, display, showClass int) (SummaryVegetationPreview, error) {
	if s == nil || s.summary == nil {
		return SummaryVegetationPreview{}, errors.New("summary vegetation service is unavailable")
	}
	return s.summary.PreviewGroupedDisplay(ctx, contextID, average, groupAndOrder, display, showClass)
}

func (s *VegetationReportService) PreviewSummaryVegetationEnvironmental(ctx context.Context, contextID string, average, display, showClass int) (*SummaryVegetationEnvironmentalPreview, error) {
	if s == nil || s.summaryEnvironment == nil {
		return nil, errors.New("summary vegetation environmental service is unavailable")
	}
	return s.summaryEnvironment.PreviewEnvironmental(ctx, contextID, average, display, showClass)
}

func (s *VegetationReportService) PreviewSummaryVegetationEnvironmentalNumeric(ctx context.Context, contextID string, average, display, showClass int) (*SummaryVegetationEnvironmentalNumericPreview, error) {
	if s == nil || s.summaryEnvironment == nil {
		return nil, errors.New("summary vegetation environmental service is unavailable")
	}
	return s.summaryEnvironment.PreviewEnvironmentalNumeric(ctx, contextID, average, display, showClass)
}

func (s *VegetationReportService) PreviewSummaryVegetationEnvironmentalLifeform(ctx context.Context, contextID string, average, display, showClass int) (*SummaryVegetationEnvironmentalLifeformPreview, error) {
	if s == nil || s.summaryEnvironmentLifeform == nil {
		return nil, errors.New("summary vegetation environmental lifeform service is unavailable")
	}
	return s.summaryEnvironmentLifeform.PreviewEnvironmentalLifeform(ctx, contextID, average, display, showClass)
}

func (s *VegetationReportService) GetSummaryVegetationWorkbookReview(ctx context.Context, contextID, requestJSON string) (*SummaryVegetationWorkbookReview, error) {
	if s == nil || s.summaryWorkbook == nil {
		return nil, errors.New("summary vegetation workbook service is unavailable")
	}
	return s.summaryWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) ExportReviewedSummaryVegetationWorkbook(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookOutcome, error) {
	if s == nil || s.summaryWorkbook == nil {
		return nil, errors.New("summary vegetation workbook service is unavailable")
	}
	return s.summaryWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetSummaryVegetationPreferences(ctx context.Context, contextID string) (*SummaryVegetationPreferencesReview, error) {
	if s == nil || s.summaryPreferences == nil {
		return nil, errors.New("summary vegetation preferences service is unavailable")
	}
	return s.summaryPreferences.GetSummaryVegetationPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveSummaryVegetationPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationPreferencesOutcome {
	if s == nil || s.summaryPreferences == nil {
		return &SummaryVegetationPreferencesOutcome{ContextID: contextID, ErrorMessage: "summary vegetation preferences service is unavailable"}
	}
	return s.summaryPreferences.SaveSummaryVegetationPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetSummaryVegetationQualityPreferences(ctx context.Context, contextID string) (*SummaryVegetationQualityPreferencesReview, error) {
	if s == nil || s.summaryQualityPreferences == nil {
		return nil, errors.New("summary vegetation quality preferences service is unavailable")
	}
	return s.summaryQualityPreferences.GetSummaryVegetationQualityPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveSummaryVegetationQualityPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationQualityPreferencesOutcome {
	if s == nil || s.summaryQualityPreferences == nil {
		return &SummaryVegetationQualityPreferencesOutcome{ContextID: contextID, ErrorMessage: "summary vegetation quality preferences service is unavailable"}
	}
	return s.summaryQualityPreferences.SaveSummaryVegetationQualityPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetSummaryVegetationPresentationPreferences(ctx context.Context, contextID string) (*SummaryVegetationPresentationPreferencesReview, error) {
	if s == nil || s.summaryPresentationPreferences == nil {
		return nil, errors.New("summary vegetation presentation preferences service is unavailable")
	}
	return s.summaryPresentationPreferences.GetSummaryVegetationPresentationPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveSummaryVegetationPresentationPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationPresentationPreferencesOutcome {
	if s == nil || s.summaryPresentationPreferences == nil {
		return &SummaryVegetationPresentationPreferencesOutcome{ContextID: contextID, ErrorMessage: "summary vegetation presentation preferences service is unavailable"}
	}
	return s.summaryPresentationPreferences.SaveSummaryVegetationPresentationPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetSummaryVegetationFieldList(ctx context.Context, contextID string) (*SummaryVegetationFieldListReview, error) {
	if s == nil || s.summaryFieldList == nil {
		return nil, errors.New("summary vegetation field list service is unavailable")
	}
	return s.summaryFieldList.GetSummaryVegetationFieldList(ctx, contextID)
}

func (s *VegetationReportService) GetSummaryVegetationFieldListIndex(ctx context.Context, contextID string) (*SummaryVegetationFieldListSelection, error) {
	if s == nil || s.summaryFieldList == nil {
		return nil, errors.New("summary vegetation field list service is unavailable")
	}
	return s.summaryFieldList.GetSummaryVegetationFieldListIndex(ctx, contextID)
}

func (s *VegetationReportService) SaveSummaryVegetationFieldList(ctx context.Context, contextID, requestJSON string) *SummaryVegetationFieldListOutcome {
	if s == nil || s.summaryFieldList == nil {
		return &SummaryVegetationFieldListOutcome{ContextID: contextID, ErrorMessage: "summary vegetation field list service is unavailable"}
	}
	return s.summaryFieldList.SaveSummaryVegetationFieldList(ctx, contextID, requestJSON)
}

// Long Vegetation report, preferences, scope & workbook delegates

func (s *VegetationReportService) GetLongVegetationOptions(ctx context.Context, contextID string) (LongVegetationOptions, error) {
	if s == nil || s.contexts == nil {
		return LongVegetationOptions{}, errors.New("vegetation report context is unavailable")
	}
	return s.contexts.GetLongVegetationOptions(ctx, contextID)
}

func (s *VegetationReportService) PreviewLongVegetation(ctx context.Context, contextID string) (LongVegetationPreview, error) {
	if s == nil || s.contexts == nil {
		return LongVegetationPreview{}, errors.New("vegetation report context is unavailable")
	}
	return s.contexts.PreviewLongVegetation(ctx, contextID)
}

func (s *VegetationReportService) GetLongVegetationPreferences(ctx context.Context, contextID string) (*LongVegetationPreferencesReview, error) {
	if s == nil || s.longPreferences == nil {
		return nil, errors.New("long vegetation preferences service is unavailable")
	}
	return s.longPreferences.GetLongVegetationPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationPreferencesOutcome {
	if s == nil || s.longPreferences == nil {
		return &LongVegetationPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation preferences service is unavailable"}
	}
	return s.longPreferences.SaveLongVegetationPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationStatisticsPreferences(ctx context.Context, contextID string) (*LongVegetationStatisticsPreferencesReview, error) {
	if s == nil || s.longStatisticsPreferences == nil {
		return nil, errors.New("long vegetation statistics preferences service is unavailable")
	}
	return s.longStatisticsPreferences.GetLongVegetationStatisticsPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationStatisticsPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationStatisticsPreferencesOutcome {
	if s == nil || s.longStatisticsPreferences == nil {
		return &LongVegetationStatisticsPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation statistics preferences service is unavailable"}
	}
	return s.longStatisticsPreferences.SaveLongVegetationStatisticsPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationSwitchPreferences(ctx context.Context, contextID string) (*LongVegetationSwitchPreferencesReview, error) {
	if s == nil || s.longSwitchPreferences == nil {
		return nil, errors.New("long vegetation switch preferences service is unavailable")
	}
	return s.longSwitchPreferences.GetLongVegetationSwitchPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationSwitchPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSwitchPreferencesOutcome {
	if s == nil || s.longSwitchPreferences == nil {
		return &LongVegetationSwitchPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation switch preferences service is unavailable"}
	}
	return s.longSwitchPreferences.SaveLongVegetationSwitchPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationThresholdPreferences(ctx context.Context, contextID string) (*LongVegetationThresholdPreferencesReview, error) {
	if s == nil || s.longThresholdPreferences == nil {
		return nil, errors.New("long vegetation threshold preferences service is unavailable")
	}
	return s.longThresholdPreferences.GetLongVegetationThresholdPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationThresholdPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationThresholdPreferencesOutcome {
	if s == nil || s.longThresholdPreferences == nil {
		return &LongVegetationThresholdPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation threshold preferences service is unavailable"}
	}
	return s.longThresholdPreferences.SaveLongVegetationThresholdPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationGroupPreferences(ctx context.Context, contextID string) (*LongVegetationGroupPreferencesReview, error) {
	if s == nil || s.longGroupPreferences == nil {
		return nil, errors.New("long vegetation group preferences service is unavailable")
	}
	return s.longGroupPreferences.GetLongVegetationGroupPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationGroupPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationGroupPreferencesOutcome {
	if s == nil || s.longGroupPreferences == nil {
		return &LongVegetationGroupPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation group preferences service is unavailable"}
	}
	return s.longGroupPreferences.SaveLongVegetationGroupPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationQualityNullPreferences(ctx context.Context, contextID string) (*LongVegetationQualityNullPreferencesReview, error) {
	if s == nil || s.longQualityNullPreferences == nil {
		return nil, errors.New("long vegetation quality null preferences service is unavailable")
	}
	return s.longQualityNullPreferences.GetLongVegetationQualityNullPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationQualityNullPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityNullPreferencesOutcome {
	if s == nil || s.longQualityNullPreferences == nil {
		return &LongVegetationQualityNullPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation quality null preferences service is unavailable"}
	}
	return s.longQualityNullPreferences.SaveLongVegetationQualityNullPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationQualityMinimumPreferences(ctx context.Context, contextID string) (*LongVegetationQualityMinimumPreferencesReview, error) {
	if s == nil || s.longQualityMinimumPreferences == nil {
		return nil, errors.New("long vegetation quality minimum preferences service is unavailable")
	}
	return s.longQualityMinimumPreferences.GetLongVegetationQualityMinimumPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationQualityMinimumPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityMinimumPreferencesOutcome {
	if s == nil || s.longQualityMinimumPreferences == nil {
		return &LongVegetationQualityMinimumPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation quality minimum preferences service is unavailable"}
	}
	return s.longQualityMinimumPreferences.SaveLongVegetationQualityMinimumPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationNamingPreferences(ctx context.Context, contextID string) (*LongVegetationNamingPreferencesReview, error) {
	if s == nil || s.longNamingPreferences == nil {
		return nil, errors.New("long vegetation naming preferences service is unavailable")
	}
	return s.longNamingPreferences.GetLongVegetationNamingPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationNamingPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationNamingPreferencesOutcome {
	if s == nil || s.longNamingPreferences == nil {
		return &LongVegetationNamingPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation naming preferences service is unavailable"}
	}
	return s.longNamingPreferences.SaveLongVegetationNamingPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationSelectedFieldPreferences(ctx context.Context, contextID string) (*LongVegetationSelectedFieldPreferencesReview, error) {
	if s == nil || s.longSelectedFieldPreferences == nil {
		return nil, errors.New("long vegetation selected field preferences service is unavailable")
	}
	return s.longSelectedFieldPreferences.GetLongVegetationSelectedFieldPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationSelectedFieldPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSelectedFieldPreferencesOutcome {
	if s == nil || s.longSelectedFieldPreferences == nil {
		return &LongVegetationSelectedFieldPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation selected field preferences service is unavailable"}
	}
	return s.longSelectedFieldPreferences.SaveLongVegetationSelectedFieldPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationEnforcementPreferences(ctx context.Context, contextID string) (*LongVegetationEnforcementPreferencesReview, error) {
	if s == nil || s.longEnforcementPreferences == nil {
		return nil, errors.New("long vegetation enforcement preferences service is unavailable")
	}
	return s.longEnforcementPreferences.GetLongVegetationEnforcementPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationEnforcementPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationEnforcementPreferencesOutcome {
	if s == nil || s.longEnforcementPreferences == nil {
		return &LongVegetationEnforcementPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation enforcement preferences service is unavailable"}
	}
	return s.longEnforcementPreferences.SaveLongVegetationEnforcementPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationUnitPreferences(ctx context.Context, contextID string) (*LongVegetationUnitPreferencesReview, error) {
	if s == nil || s.longUnitPreferences == nil {
		return nil, errors.New("long vegetation unit preferences service is unavailable")
	}
	return s.longUnitPreferences.GetLongVegetationUnitPreferences(ctx, contextID)
}

func (s *VegetationReportService) SaveLongVegetationUnitPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationUnitPreferencesOutcome {
	if s == nil || s.longUnitPreferences == nil {
		return &LongVegetationUnitPreferencesOutcome{ContextID: contextID, ErrorMessage: "long vegetation unit preferences service is unavailable"}
	}
	return s.longUnitPreferences.SaveLongVegetationUnitPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) BuildLongVegetationSelectedUnitSelection(ctx context.Context, contextID, revision string) (*LongVegetationSelectedUnitSelection, error) {
	if s == nil || s.longSelectedUnitSelection == nil {
		return nil, errors.New("long vegetation selected unit selection service is unavailable")
	}
	return s.longSelectedUnitSelection.BuildLongVegetationSelectedUnitSelection(ctx, contextID, revision)
}

func (s *VegetationReportService) GetLongVegetationSelectedUnitSelection(ctx context.Context, contextID string) (*LongVegetationSelectedUnitSelection, error) {
	if s == nil || s.longSelectedUnitSelection == nil {
		return nil, errors.New("long vegetation selected unit selection service is unavailable")
	}
	return s.longSelectedUnitSelection.GetLongVegetationSelectedUnitSelection(ctx, contextID)
}

func (s *VegetationReportService) SetLongVegetationSelectedUnitFields(ctx context.Context, contextID, requestJSON string) (*LongVegetationSelectedUnitSelection, error) {
	if s == nil || s.longSelectedUnitSelection == nil {
		return nil, errors.New("long vegetation selected unit selection service is unavailable")
	}
	return s.longSelectedUnitSelection.SetLongVegetationSelectedUnitFields(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) BuildLongVegetationProjectScope(ctx context.Context, contextID, revision string) (*LongVegetationReportScope, error) {
	if s == nil || s.longReportScope == nil {
		return nil, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.BuildLongVegetationProjectScope(ctx, contextID, revision)
}

func (s *VegetationReportService) GetLongVegetationProjectFieldDefinitions(ctx context.Context, contextID string) (*LongVegetationProjectFieldDefinitions, error) {
	if s == nil || s.longReportScope == nil {
		return nil, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.GetLongVegetationProjectFieldDefinitions(ctx, contextID)
}

func (s *VegetationReportService) GetLongVegetationProjectOptions(ctx context.Context, contextID, requestJSON string) (LongVegetationProjectOptions, error) {
	if s == nil || s.longReportScope == nil {
		return LongVegetationProjectOptions{}, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.GetLongVegetationProjectOptions(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationProjectUnitPreferences(ctx context.Context, contextID, requestJSON string) (LongVegetationProjectUnitPreferences, error) {
	if s == nil || s.longReportScope == nil {
		return LongVegetationProjectUnitPreferences{}, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.GetLongVegetationProjectUnitPreferences(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationReportScope(ctx context.Context, contextID string) (*LongVegetationReportScope, error) {
	if s == nil || s.longReportScope == nil {
		return nil, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.GetLongVegetationReportScope(ctx, contextID)
}

func (s *VegetationReportService) PreviewLongVegetationProject(ctx context.Context, contextID, requestJSON string) (LongVegetationProjectPreview, error) {
	if s == nil || s.longReportScope == nil {
		return LongVegetationProjectPreview{}, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.PreviewLongVegetationProject(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) SaveLongVegetationProjectUnits(ctx context.Context, contextID, requestJSON string) LongVegetationProjectUnitOutcome {
	if s == nil || s.longReportScope == nil {
		return LongVegetationProjectUnitOutcome{
			Outcome: LongVegetationUnitPreferencesOutcome{
				ContextID:    contextID,
				ErrorMessage: "long vegetation report scope service is unavailable",
			},
		}
	}
	return s.longReportScope.SaveLongVegetationProjectUnits(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) SetLongVegetationProjectFields(ctx context.Context, contextID, requestJSON string) (*LongVegetationReportScope, error) {
	if s == nil || s.longReportScope == nil {
		return nil, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.SetLongVegetationProjectFields(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) SetLongVegetationReportScope(ctx context.Context, contextID, requestJSON string) (*LongVegetationReportScope, error) {
	if s == nil || s.longReportScope == nil {
		return nil, errors.New("long vegetation report scope service is unavailable")
	}
	return s.longReportScope.SetLongVegetationReportScope(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetLongVegetationProjectWorkbookReview(ctx context.Context, contextID, requestJSON string) (*LongVegetationProjectWorkbookReview, error) {
	if s == nil || s.longProjectWorkbook == nil {
		return nil, errors.New("long vegetation project workbook service is unavailable")
	}
	return s.longProjectWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) ExportReviewedLongVegetationProjectWorkbook(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookOutcome, error) {
	if s == nil || s.longProjectWorkbook == nil {
		return nil, errors.New("long vegetation project workbook service is unavailable")
	}
	return s.longProjectWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) GetVegetationWorkbookReview(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookReview, error) {
	if s == nil || s.vegetationWorkbook == nil {
		return nil, errors.New("vegetation workbook service is unavailable")
	}
	return s.vegetationWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) ExportReviewedVegetationWorkbook(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookOutcome, error) {
	if s == nil || s.vegetationWorkbook == nil {
		return nil, errors.New("vegetation workbook service is unavailable")
	}
	return s.vegetationWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

// Lifeform & Species Attribute Summary delegates

func (s *VegetationReportService) PreviewLifeformSummary(ctx context.Context, contextID string) (LifeformSummaryPreview, error) {
	if s == nil || s.lifeformSummary == nil {
		return LifeformSummaryPreview{}, errors.New("lifeform summary service is unavailable")
	}
	return s.lifeformSummary.Preview(ctx, contextID)
}

func (s *VegetationReportService) GetLifeformWorkbookReview(ctx context.Context, contextID, requestJSON string) (*LifeformWorkbookReview, error) {
	if s == nil || s.lifeformWorkbook == nil {
		return nil, errors.New("lifeform workbook service is unavailable")
	}
	return s.lifeformWorkbook.GetReview(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) ExportReviewedLifeformWorkbook(ctx context.Context, contextID, requestJSON string) (*LifeformWorkbookOutcome, error) {
	if s == nil || s.lifeformWorkbook == nil {
		return nil, errors.New("lifeform workbook service is unavailable")
	}
	return s.lifeformWorkbook.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *VegetationReportService) PreviewSpeciesAttributeSummary(ctx context.Context, contextID string) (SpeciesAttributeSummaryPreview, error) {
	if s == nil || s.speciesAttributeSummary == nil {
		return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary service is unavailable")
	}
	return s.speciesAttributeSummary.Preview(ctx, contextID)
}

// SiteUnit Exclusion Filters (.support, .poor, .nudum) & Dynamic Hierarchy Breakpoint (~) delegates

func (s *VegetationReportService) GetDefaultSiteUnitExclusions() []string {
	return vegreportpkg.DefaultSiteUnitExclusionPatterns()
}

func (s *VegetationReportService) PreviewSiteUnitExclusions(ctx context.Context, contextID string, request SiteUnitExclusionPreviewRequest) (*SiteUnitExclusionPreview, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("vegetation report context is unavailable")
	}
	return vegreportpkg.PreviewSiteUnitExclusions(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *VegetationReportService) ResolveHierarchyBreakpointUnits(ctx context.Context, contextID string, request HierarchyBreakpointRequest) (*HierarchyBreakpointResolution, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("vegetation report context is unavailable")
	}
	return vegreportpkg.ResolveHierarchyBreakpointUnits(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *VegetationReportService) PreviewSummaryVegetationWithExclusions(ctx context.Context, contextID string, request SummaryVegetationExclusionRequest) (*SummaryVegetationExclusionPreview, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("vegetation report context is unavailable")
	}
	return vegreportpkg.PreviewSummaryVegetationWithExclusions(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *VegetationReportService) PreviewLongVegetationWithExclusions(ctx context.Context, contextID string, request LongVegetationExclusionRequest) (*LongVegetationExclusionPreview, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("vegetation report context is unavailable")
	}
	return vegreportpkg.PreviewLongVegetationWithExclusions(ctx, unwrapContextService(s.contexts), contextID, request)
}
