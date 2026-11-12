package main

import (
	"context"
	"errors"

	classificationpkg "github.com/boostao/vpro-wails/internal/classification"
	diagnosticspkg "github.com/boostao/vpro-wails/internal/diagnostics"
)

type CreateSUTableRequest = classificationpkg.CreateSUTableRequest
type SaveSUTableAsRequest = classificationpkg.SaveSUTableAsRequest
type SUTableLifecycleResult = classificationpkg.SUTableLifecycleResult
type SUPlotLeaf = classificationpkg.SUPlotLeaf
type SUUnitBucket = classificationpkg.SUUnitBucket
type SUPlotTree = classificationpkg.SUPlotTree
type PlotInspectorSpecies = classificationpkg.PlotInspectorSpecies
type PlotInspectorSummary = classificationpkg.PlotInspectorSummary
type BECUnitPickerRequest = classificationpkg.BECUnitPickerRequest
type BECUnitPickerChoice = classificationpkg.BECUnitPickerChoice
type AssignPlotsToSiteUnitRequest = classificationpkg.AssignPlotsToSiteUnitRequest
type AssignPlotsToSiteUnitResult = classificationpkg.AssignPlotsToSiteUnitResult
type CreateSUFromEnvFieldsRequest = classificationpkg.CreateSUFromEnvFieldsRequest
type SUFromEnvPlotPreview = classificationpkg.SUFromEnvPlotPreview
type CreateSUFromEnvFieldsResult = classificationpkg.CreateSUFromEnvFieldsResult
type HierarchyExplorerNode = classificationpkg.HierarchyExplorerNode
type FlatHierarchyRow = classificationpkg.FlatHierarchyRow
type HierarchyDiagnostics = classificationpkg.HierarchyDiagnostics
type HierarchyExplorerView = classificationpkg.HierarchyExplorerView
type CreateHierarchyTableRequest = classificationpkg.CreateHierarchyTableRequest
type HierarchyTableLifecycleResult = classificationpkg.HierarchyTableLifecycleResult
type HierarchyAddNodeRequest = classificationpkg.HierarchyAddNodeRequest
type HierarchyRenameNodeRequest = classificationpkg.HierarchyRenameNodeRequest
type HierarchyReparentNodeRequest = classificationpkg.HierarchyReparentNodeRequest
type HierarchyDeleteNodeRequest = classificationpkg.HierarchyDeleteNodeRequest
type HierarchyMutationResult = classificationpkg.HierarchyMutationResult
type SaveFlatHierarchyRequest = classificationpkg.SaveFlatHierarchyRequest

type SynthesisLumpTableOption = diagnosticspkg.SynthesisLumpTableOption
type SynthesisOptions = diagnosticspkg.SynthesisOptions
type SynthesisSnapshot = diagnosticspkg.SynthesisSnapshot
type ReassignSynthesisPlotRequest = diagnosticspkg.ReassignSynthesisPlotRequest
type UndoSynthesisReassignmentRequest = diagnosticspkg.UndoSynthesisReassignmentRequest
type ReassignSynthesisPlotResult = diagnosticspkg.ReassignSynthesisPlotResult
type ExportDiagnosticWorkbookRequest = diagnosticspkg.ExportDiagnosticWorkbookRequest
type ExportDiagnosticWorkbookResult = diagnosticspkg.ExportDiagnosticWorkbookResult

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

func (s *ClassificationService) CreateSUTable(ctx context.Context, contextID string, request CreateSUTableRequest) (*SUTableLifecycleResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.CreateSUTable(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) SaveSUTableAs(ctx context.Context, contextID string, request SaveSUTableAsRequest) (*SUTableLifecycleResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.SaveSUTableAs(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) GetSUPlotTree(ctx context.Context, contextID, filterText string) (*SUPlotTree, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.GetSUPlotTree(ctx, unwrapContextService(s.contexts), contextID, filterText)
}

func (s *ClassificationService) GetPlotInspectorSummary(ctx context.Context, contextID, plotNumber string) (*PlotInspectorSummary, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.GetPlotInspectorSummary(ctx, unwrapContextService(s.contexts), contextID, plotNumber)
}

func (s *ClassificationService) SearchBECUnitPicker(ctx context.Context, contextID string, request BECUnitPickerRequest) ([]BECUnitPickerChoice, error) {
	if s == nil {
		return nil, errors.New("classification service is unavailable")
	}
	return classificationpkg.SearchBECUnitPicker(ctx, unwrapContextService(s.contexts), s.bec, contextID, request)
}

func (s *ClassificationService) AssignPlotsToSiteUnit(ctx context.Context, contextID string, request AssignPlotsToSiteUnitRequest) (*AssignPlotsToSiteUnitResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.AssignPlotsToSiteUnit(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) UndoSUAssignment(ctx context.Context, contextID string, historyID int64) (*AssignPlotsToSiteUnitResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.UndoSUAssignment(ctx, unwrapContextService(s.contexts), contextID, historyID)
}

func (s *ClassificationService) CreateSUFromEnvFields(ctx context.Context, contextID string, request CreateSUFromEnvFieldsRequest) (*CreateSUFromEnvFieldsResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.CreateSUFromEnvFields(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) GetHierarchyExplorer(ctx context.Context, contextID string) (*HierarchyExplorerView, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.GetHierarchyExplorer(ctx, unwrapContextService(s.contexts), contextID)
}

func (s *ClassificationService) CreateHierarchyTable(ctx context.Context, contextID string, request CreateHierarchyTableRequest) (*HierarchyTableLifecycleResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.CreateHierarchyTable(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) AddHierarchyNode(ctx context.Context, contextID string, request HierarchyAddNodeRequest) (*HierarchyMutationResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.AddHierarchyNode(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) RenameHierarchyNode(ctx context.Context, contextID string, request HierarchyRenameNodeRequest) (*HierarchyMutationResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.RenameHierarchyNode(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) ReparentHierarchyNode(ctx context.Context, contextID string, request HierarchyReparentNodeRequest) (*HierarchyMutationResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.ReparentHierarchyNode(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) ToggleHierarchyBreakpoint(ctx context.Context, contextID string, nodeID int64) (*HierarchyMutationResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.ToggleHierarchyBreakpoint(ctx, unwrapContextService(s.contexts), contextID, nodeID)
}

func (s *ClassificationService) DeleteHierarchyNode(ctx context.Context, contextID string, request HierarchyDeleteNodeRequest) (*HierarchyMutationResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.DeleteHierarchyNode(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) SaveFlatHierarchyRows(ctx context.Context, contextID string, request SaveFlatHierarchyRequest) (*HierarchyMutationResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return classificationpkg.SaveFlatHierarchyRows(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) ListSynthesisLumpTables(ctx context.Context, contextID string) ([]SynthesisLumpTableOption, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return diagnosticspkg.ListSynthesisLumpTables(ctx, unwrapContextService(s.contexts), contextID)
}

func (s *ClassificationService) ComputeSynthesisSnapshot(ctx context.Context, contextID string, opts SynthesisOptions) (*SynthesisSnapshot, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return diagnosticspkg.ComputeSynthesisSnapshot(ctx, unwrapContextService(s.contexts), contextID, opts)
}

func (s *ClassificationService) ReassignSynthesisPlotUnit(ctx context.Context, contextID string, request ReassignSynthesisPlotRequest) (*ReassignSynthesisPlotResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return diagnosticspkg.ReassignSynthesisPlotUnit(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) UndoSynthesisReassignment(ctx context.Context, contextID string, request UndoSynthesisReassignmentRequest) (*ReassignSynthesisPlotResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return diagnosticspkg.UndoSynthesisReassignment(ctx, unwrapContextService(s.contexts), contextID, request)
}

func (s *ClassificationService) ExportDiagnosticWorkbook(ctx context.Context, contextID string, request ExportDiagnosticWorkbookRequest) (*ExportDiagnosticWorkbookResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("classification context is unavailable")
	}
	return diagnosticspkg.ExportDiagnosticWorkbook(ctx, unwrapContextService(s.contexts), contextID, request)
}

