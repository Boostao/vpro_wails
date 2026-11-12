package main

import (
	"context"
	"errors"
)

type ClassificationService struct {
	contexts              *ContextService
	bec                   *BECService
	workingUnits          *WorkingUnitService
	externalSUEnvironment *ExternalSUEnvironmentService
	twoPageProfiling      *TwoPageProfilingService
	speciesPresence       *ProjectProfileSpeciesPresenceService
}

func NewClassificationService(
	contexts *ContextService,
	bec *BECService,
	workingUnits *WorkingUnitService,
	externalSUEnvironment *ExternalSUEnvironmentService,
	twoPageProfiling *TwoPageProfilingService,
	speciesPresence *ProjectProfileSpeciesPresenceService,
) *ClassificationService {
	return &ClassificationService{
		contexts:              contexts,
		bec:                   bec,
		workingUnits:          workingUnits,
		externalSUEnvironment: externalSUEnvironment,
		twoPageProfiling:      twoPageProfiling,
		speciesPresence:       speciesPresence,
	}
}

func (s *ClassificationService) GetBECCatalogueStatus(ctx context.Context) (BECCatalogueStatus, error) {
	if s == nil || s.bec == nil {
		return BECCatalogueStatus{}, errors.New("BEC catalogue service is unavailable")
	}
	return s.bec.GetBECCatalogueStatus(ctx)
}

func (s *ClassificationService) ListBECZones(ctx context.Context) ([]BECZone, error) {
	if s == nil || s.bec == nil {
		return nil, errors.New("BEC catalogue service is unavailable")
	}
	return s.bec.ListBECZones(ctx)
}

func (s *ClassificationService) ListBECSubZones(ctx context.Context, zone *string) ([]BECSubZone, error) {
	if s == nil || s.bec == nil {
		return nil, errors.New("BEC catalogue service is unavailable")
	}
	return s.bec.ListBECSubZones(ctx, zone)
}

func (s *ClassificationService) ListBECSiteSeries(ctx context.Context, zone, subZone *string) ([]BECSiteSeries, error) {
	if s == nil || s.bec == nil {
		return nil, errors.New("BEC catalogue service is unavailable")
	}
	return s.bec.ListBECSiteSeries(ctx, zone, subZone)
}

func (s *ClassificationService) GetWorkingUnitMode() (WorkingUnitModeState, error) {
	if s == nil || s.workingUnits == nil {
		return WorkingUnitModeState{}, errors.New("working unit service is unavailable")
	}
	return s.workingUnits.GetWorkingUnitMode()
}

func (s *ClassificationService) SetWorkingUnitMode(mode string) (WorkingUnitModeState, error) {
	if s == nil || s.workingUnits == nil {
		return WorkingUnitModeState{}, errors.New("working unit service is unavailable")
	}
	return s.workingUnits.SetWorkingUnitMode(mode)
}

func (s *ClassificationService) GetWorkingUnitChoices(ctx context.Context, mode string) ([]WorkingUnitChoice, error) {
	if s == nil || s.workingUnits == nil {
		return nil, errors.New("working unit service is unavailable")
	}
	return s.workingUnits.GetWorkingUnitChoices(ctx, mode)
}

func (s *ClassificationService) GetMasterWorkingUnitChoices(ctx context.Context) ([]WorkingUnitChoice, error) {
	if s == nil || s.workingUnits == nil {
		return nil, errors.New("working unit service is unavailable")
	}
	return s.workingUnits.GetMasterWorkingUnitChoices(ctx)
}

func (s *ClassificationService) GetExternalSUEnvironmentReview(ctx context.Context, contextID string) (*ExternalSUEnvironmentReview, error) {
	if s == nil || s.externalSUEnvironment == nil {
		return nil, errors.New("external SU environment service is unavailable")
	}
	return s.externalSUEnvironment.GetReview(ctx, contextID)
}

func (s *ClassificationService) TransferExternalSUEnvironmentReviewed(ctx context.Context, contextID, requestJSON string) (*ExternalSUEnvironmentOutcome, error) {
	if s == nil || s.externalSUEnvironment == nil {
		return nil, errors.New("external SU environment service is unavailable")
	}
	return s.externalSUEnvironment.TransferReviewed(ctx, contextID, requestJSON)
}

func (s *ClassificationService) RecoverExternalSUEnvironmentOutcome(ctx context.Context, contextID, requestJSON string) (*ExternalSUEnvironmentAttemptOutcome, error) {
	if s == nil || s.externalSUEnvironment == nil {
		return nil, errors.New("external SU environment service is unavailable")
	}
	return s.externalSUEnvironment.RecoverOutcome(ctx, contextID, requestJSON)
}

func (s *ClassificationService) GetTwoPageProfilingReview(ctx context.Context, contextID, plot string, offset, limit int) (*TwoPageProfilingReview, error) {
	if s == nil || s.twoPageProfiling == nil {
		return nil, errors.New("two-page profiling service is unavailable")
	}
	return s.twoPageProfiling.GetReview(ctx, contextID, plot, offset, limit)
}

func (s *ClassificationService) PreviewSpeciesPresence(ctx context.Context, contextID, requestJSON string) (*ProjectProfileSpeciesPresencePreview, error) {
	if s == nil || s.speciesPresence == nil {
		return nil, errors.New("project profile species presence service is unavailable")
	}
	return s.speciesPresence.Preview(ctx, contextID, requestJSON)
}

func (s *ClassificationService) GetHierarchyNodes(ctx context.Context) ([]HierarchyNode, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return s.contexts.GetHierarchyNodes(ctx)
}

func (s *ClassificationService) ListPlotProfileSources(ctx context.Context, contextID, path string) ([]PlotProfileSourceInfo, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return s.contexts.ListPlotProfileSources(ctx, contextID, path)
}

func (s *ClassificationService) SelectPlotProfile(expectedID string, requested PlotProfileSource) (ProjectState, error) {
	if s == nil || s.contexts == nil {
		return ProjectState{}, errors.New("classification context is unavailable")
	}
	return s.contexts.SelectPlotProfile(expectedID, requested)
}

func (s *ClassificationService) SetPlotProfileEditing(ctx context.Context, expectedID string, request PlotProfileWriteRequest) (ProjectState, error) {
	if s == nil || s.contexts == nil {
		return ProjectState{}, errors.New("classification context is unavailable")
	}
	return s.contexts.SetPlotProfileEditing(ctx, expectedID, request)
}

func (s *ClassificationService) ReviewPlotProfileFileCreation(ctx context.Context, contextID string) (PlotProfileFileReview, error) {
	if s == nil || s.contexts == nil {
		return PlotProfileFileReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewPlotProfileFileCreation(ctx, contextID)
}

func (s *ClassificationService) CreatePlotProfileFile(ctx context.Context, contextID string, request PlotProfileFileCreation) (PlotProfileFileCreated, error) {
	if s == nil || s.contexts == nil {
		return PlotProfileFileCreated{}, errors.New("classification context is unavailable")
	}
	return s.contexts.CreatePlotProfileFile(ctx, contextID, request)
}

func (s *ClassificationService) ReviewPlotProfileTableCreation(ctx context.Context, contextID string) (PlotProfileTableReview, error) {
	if s == nil || s.contexts == nil {
		return PlotProfileTableReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewPlotProfileTableCreation(ctx, contextID)
}

func (s *ClassificationService) CreatePlotProfileTable(ctx context.Context, contextID string, request PlotProfileTableCreation) (PlotProfileFileCreated, error) {
	if s == nil || s.contexts == nil {
		return PlotProfileFileCreated{}, errors.New("classification context is unavailable")
	}
	return s.contexts.CreatePlotProfileTable(ctx, contextID, request)
}

func (s *ClassificationService) ReviewProjectPlotProfile(ctx context.Context, contextID string) (ProjectPlotProfileReview, error) {
	if s == nil || s.contexts == nil {
		return ProjectPlotProfileReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewProjectPlotProfile(ctx, contextID)
}

func (s *ClassificationService) SaveProjectPlotProfile(ctx context.Context, contextID string, request ProjectPlotProfileEdit) error {
	if s == nil || s.contexts == nil {
		return errors.New("classification context is unavailable")
	}
	return s.contexts.SaveProjectPlotProfile(ctx, contextID, request)
}

func (s *ClassificationService) CreateProjectPlotProfileRule(ctx context.Context, contextID string, request ProjectPlotProfileCreation) (ProjectMetadataRow, error) {
	if s == nil || s.contexts == nil {
		return ProjectMetadataRow{}, errors.New("classification context is unavailable")
	}
	return s.contexts.CreateProjectPlotProfileRule(ctx, contextID, request)
}

func (s *ClassificationService) DeleteProjectPlotProfileRule(ctx context.Context, contextID string, request ProjectPlotProfileDeletion) error {
	if s == nil || s.contexts == nil {
		return errors.New("classification context is unavailable")
	}
	return s.contexts.DeleteProjectPlotProfileRule(ctx, contextID, request)
}

func (s *ClassificationService) ListProjectPlotProfileChoices(ctx context.Context, contextID string) (ProjectPlotProfileChoices, error) {
	if s == nil || s.contexts == nil {
		return ProjectPlotProfileChoices{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ListProjectPlotProfileChoices(ctx, contextID)
}

func (s *ClassificationService) ReviewProjectPlotProfileLump(ctx context.Context, contextID string) (ProjectMetadataTable, error) {
	if s == nil || s.contexts == nil {
		return ProjectMetadataTable{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewProjectPlotProfileLump(ctx, contextID)
}

func (s *ClassificationService) RunProjectPlotProfile(ctx context.Context, contextID string, request ProjectPlotProfileRunRequest) (ProjectPlotProfileResult, error) {
	if s == nil || s.contexts == nil {
		return ProjectPlotProfileResult{}, errors.New("classification context is unavailable")
	}
	return s.contexts.RunProjectPlotProfile(ctx, contextID, request)
}

func (s *ClassificationService) ResolveProjectPlotProfileNavigation(ctx context.Context, contextID string, request ProjectPlotProfileFilterRequest) (ProjectPlotProfileNavigation, error) {
	if s == nil || s.contexts == nil {
		return ProjectPlotProfileNavigation{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ResolveProjectPlotProfileNavigation(ctx, contextID, request)
}

func (s *ClassificationService) ReviewProjectPlotProfileSU(ctx context.Context, contextID string, filter ProjectPlotProfileFilterRequest) (ProfileSUReview, error) {
	if s == nil || s.contexts == nil {
		return ProfileSUReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewProjectPlotProfileSU(ctx, contextID, filter)
}

func (s *ClassificationService) SaveProjectPlotProfileSU(ctx context.Context, contextID string, request ProfileSUCreation) (ProfileSUCreated, error) {
	if s == nil || s.contexts == nil {
		return ProfileSUCreated{}, errors.New("classification context is unavailable")
	}
	return s.contexts.SaveProjectPlotProfileSU(ctx, contextID, request)
}

func (s *ClassificationService) ReviewProjectPlotProfileSUInProject(ctx context.Context, contextID string, filter ProjectPlotProfileFilterRequest) (ProfileSUProjectReview, error) {
	if s == nil || s.contexts == nil {
		return ProfileSUProjectReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewProjectPlotProfileSUInProject(ctx, contextID, filter)
}

func (s *ClassificationService) SaveProjectPlotProfileSUInProject(ctx context.Context, contextID string, request ProfileSUProjectCreation) (ProfileSUCreated, error) {
	if s == nil || s.contexts == nil {
		return ProfileSUCreated{}, errors.New("classification context is unavailable")
	}
	return s.contexts.SaveProjectPlotProfileSUInProject(ctx, contextID, request)
}

func (s *ClassificationService) ReviewEnvironmentSiteUnits(ctx context.Context, contextID string) (EnvironmentSiteUnitReview, error) {
	if s == nil || s.contexts == nil {
		return EnvironmentSiteUnitReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewEnvironmentSiteUnits(ctx, contextID)
}

func (s *ClassificationService) TransferEnvironmentSiteUnits(ctx context.Context, contextID string, request EnvironmentSiteUnitTransfer) (*EnvironmentSiteUnitResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return s.contexts.TransferEnvironmentSiteUnits(ctx, contextID, request)
}

func (s *ClassificationService) ReviewSiteUnitEnvironment(ctx context.Context, contextID string) (SiteUnitEnvironmentReview, error) {
	if s == nil || s.contexts == nil {
		return SiteUnitEnvironmentReview{}, errors.New("classification context is unavailable")
	}
	return s.contexts.ReviewSiteUnitEnvironment(ctx, contextID)
}

func (s *ClassificationService) TransferSiteUnitEnvironment(ctx context.Context, contextID string, request SiteUnitEnvironmentTransfer) (*SiteUnitEnvironmentResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return s.contexts.TransferSiteUnitEnvironment(ctx, contextID, request)
}
