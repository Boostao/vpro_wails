package main

import (
	"context"
	"errors"
	classificationpkg "github.com/boostao/vpro-wails/internal/classification"
	envreportpkg "github.com/boostao/vpro-wails/internal/envreport"
	interchangepkg "github.com/boostao/vpro-wails/internal/interchange"
	plotentrypkg "github.com/boostao/vpro-wails/internal/plotentry"
	sivipkg "github.com/boostao/vpro-wails/internal/sivi"
	sourceformpkg "github.com/boostao/vpro-wails/internal/sourceform"
	spatialpkg "github.com/boostao/vpro-wails/internal/spatial"
	validationpkg "github.com/boostao/vpro-wails/internal/validation"
	vegreportpkg "github.com/boostao/vpro-wails/internal/vegreport"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type ContextService struct {
	*vprocorepkg.ContextService
}

func unwrapContextService(s *ContextService) *vprocorepkg.ContextService {
	if s == nil {
		return nil
	}
	return s.ContextService
}

func NewContextService(projects *vprocorepkg.ProjectService, plots *vprocorepkg.PlotService) (*ContextService, error) {
	core, err := vprocorepkg.NewContextService(projects, plots)
	if err != nil {
		return nil, err
	}
	return &ContextService{ContextService: core}, nil
}

func (s *ContextService) GetState(ctx context.Context) (ProjectState, error) {
	if s == nil || s.ContextService == nil || s.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.Projects.GetState(ctx)
}

func (s *ContextService) ListPlots(ctx context.Context, offset, limit int) (PlotPage, error) {
	if s == nil || s.ContextService == nil || s.Projects == nil {
		return PlotPage{}, errors.New("project context is unavailable")
	}
	return s.Projects.ListPlots(ctx, offset, limit)
}

func (s *ContextService) GetHierarchyNodes(ctx context.Context) ([]HierarchyNode, error) {
	if s == nil || s.ContextService == nil || s.Projects == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.Projects.GetHierarchyNodes(ctx)
}

func (s *ContextService) SelectProject(name string) (ProjectState, error) {
	if s == nil || s.ContextService == nil || s.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.Projects.SelectProject(name)
}

func (s *ContextService) SelectSU(name string) (ProjectState, error) {
	if s == nil || s.ContextService == nil || s.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.Projects.SelectSU(name)
}

func (s *ContextService) SelectHierarchy(name, file string) (ProjectState, error) {
	if s == nil || s.ContextService == nil || s.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.Projects.SelectHierarchy(name, file)
}

func (s *ContextService) CreateBlankProjectMetadata(p0 context.Context, p1 string, p2 ProjectMetadataCreate) (ProjectMetadataRow, error) {
	return classificationpkg.CreateBlankProjectMetadata(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) CreatePersonalSpeciesDefinition(p0 context.Context, p1 string, p2 PersonalSpeciesDefinitionRequest) (VegetationSpeciesOption, error) {
	return plotentrypkg.CtxCreatePersonalSpeciesDefinition(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) CreatePlotProfileFile(p0 context.Context, p1 string, p2 PlotProfileFileCreation) (PlotProfileFileCreated, error) {
	return classificationpkg.CreatePlotProfileFile(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) CreatePlotProfileTable(p0 context.Context, p1 string, p2 PlotProfileTableCreation) (PlotProfileFileCreated, error) {
	return classificationpkg.CreatePlotProfileTable(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) CreateProjectMetadataFromTemplate(p0 context.Context, p1 string, p2 ProjectMetadataTemplateCreate) (ProjectMetadataRow, error) {
	return classificationpkg.CreateProjectMetadataFromTemplate(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) CreateProjectPlotProfileRule(p0 context.Context, p1 string, p2 ProjectPlotProfileCreation) (ProjectMetadataRow, error) {
	return classificationpkg.CreateProjectPlotProfileRule(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) DeleteProjectPlotProfileRule(p0 context.Context, p1 string, p2 ProjectPlotProfileDeletion) error {
	return classificationpkg.DeleteProjectPlotProfileRule(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) DeleteReviewedVegetation(p0 string, p1 string, p2 VegetationDeletionRequest) error {
	return plotentrypkg.CtxDeleteReviewedVegetation(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetLongEnvironmentOptions(p0 context.Context, p1 string) (LongEnvironmentOptions, error) {
	return envreportpkg.GetLongEnvironmentOptions(unwrapContextService(s), p0, p1)
}

func (s *ContextService) GetLongVegetationOptions(p0 context.Context, p1 string) (LongVegetationOptions, error) {
	return vegreportpkg.GetLongVegetationOptions(unwrapContextService(s), p0, p1)
}

func (s *ContextService) GetPlotLocationReview(p0 context.Context, p1 string) (*PlotLocationReview, error) {
	return spatialpkg.GetPlotLocationReview(unwrapContextService(s), p0, p1)
}

func (s *ContextService) GetProjectTableCSVReview(p0 context.Context, p1 string, p2 string) (*ProjectTableCSVReview, error) {
	return interchangepkg.GetProjectTableCSVReview(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetSIVIParentActionOriginal(p0 context.Context, p1 string, p2 string) (*SIVIParentProjection, error) {
	return sivipkg.GetSIVIParentActionOriginal(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetSIVIParentDirectOriginal(p0 context.Context, p1 string, p2 string) (*SIVIParentProjection, error) {
	return sourceformpkg.GetSIVIParentDirectOriginal(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetSIVIParentJoinReview(p0 context.Context, p1 string, p2 string) (*SIVIParentJoinReview, error) {
	return sivipkg.GetSIVIParentJoinReview(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetSIVIParentOriginal(p0 context.Context, p1 string, p2 string) (*SIVIParentProjection, error) {
	return sivipkg.GetSIVIParentOriginal(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetSIVIProjectAssignmentOriginal(p0 context.Context, p1 string, p2 string) (*SIVIProjectAssignmentOriginal, error) {
	return sourceformpkg.GetSIVIProjectAssignmentOriginal(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) GetSIVIProjectIDChoices(p0 context.Context, p1 string) (*SIVIProjectChoices, error) {
	return sivipkg.GetSIVIProjectIDChoices(unwrapContextService(s), p0, p1)
}

func (s *ContextService) GetSIVIVegetation(p0 context.Context, p1 string, p2 string, p3 bool) ([]SIVIVegetationProjection, error) {
	return sivipkg.GetSIVIVegetation(unwrapContextService(s), p0, p1, p2, p3)
}

func (s *ContextService) GetSiteUnitSummaryOptions(p0 context.Context, p1 string) (SiteUnitSummaryOptions, error) {
	return envreportpkg.GetSiteUnitSummaryOptions(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ImportAccessProject(p0 context.Context, p1 string, p2 AccessProjectImportRequest) (AccessProjectImportResult, error) {
	return interchangepkg.ImportAccessProject(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) InspectAccessDatabase(p0 context.Context, p1 string, p2 string) (AccessDatabaseInspection, error) {
	return interchangepkg.InspectAccessDatabase(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) ListProjectMetadataFields(p0 context.Context, p1 string) ([]ProjectMetadataEditorField, error) {
	return classificationpkg.ListProjectMetadataFields(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ListProjectMetadataRestoreHistory(p0 context.Context, p1 string, p2 string) (ProjectMetadataRestoreHistory, error) {
	return classificationpkg.ListProjectMetadataRestoreHistory(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) ListProjectPlotProfileChoices(p0 context.Context, p1 string) (ProjectPlotProfileChoices, error) {
	return classificationpkg.ListProjectPlotProfileChoices(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ListSoilSuggestions(p0 context.Context, p1 string) ([]SoilSuggestion, error) {
	return plotentrypkg.ListSoilSuggestions(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ListVegetationAttributeSuggestions(p0 context.Context, p1 string) ([]SoilSuggestion, error) {
	return plotentrypkg.ListVegetationAttributeSuggestions(unwrapContextService(s), p0, p1)
}

func (s *ContextService) LookupSpeciesCodeCheckTarget(p0 context.Context, p1 string, p2 VegetationSpeciesLookup) ([]SpeciesCodeCheckOption, error) {
	return validationpkg.LookupSpeciesCodeCheckTarget(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) PreviewLongEnvironment(p0 context.Context, p1 string, p2 LongEnvironmentRequest) (LongEnvironmentPreview, error) {
	return envreportpkg.PreviewLongEnvironment(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) PreviewLongVegetation(p0 context.Context, p1 string) (LongVegetationPreview, error) {
	return vegreportpkg.PreviewLongVegetation(unwrapContextService(s), p0, p1)
}

func (s *ContextService) PreviewSiteUnitSummary(p0 context.Context, p1 string, p2 SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	return envreportpkg.PreviewSiteUnitSummary(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) PreviewSiteUnitSummaryLifeforms(p0 context.Context, p1 string, p2 SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	return envreportpkg.PreviewSiteUnitSummaryLifeforms(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) PreviewSiteUnitSummarySpecies(p0 context.Context, p1 string, p2 SiteUnitSpeciesListRequest) (SiteUnitSpeciesListPreview, error) {
	return envreportpkg.PreviewSiteUnitSummarySpecies(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) ResolveProjectPlotProfileNavigation(p0 context.Context, p1 string, p2 ProjectPlotProfileFilterRequest) (ProjectPlotProfileNavigation, error) {
	return classificationpkg.ResolveProjectPlotProfileNavigation(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) RestoreProjectMetadata(p0 context.Context, p1 string, p2 ProjectMetadataRestore) (*AuditRestoreResult, error) {
	return classificationpkg.RestoreProjectMetadata(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) RestoreSIVIHeights(p0 context.Context, p1 string, p2 string, p3 string, p4 AuditRestoreAction) (*AuditRestoreResult, error) {
	return sivipkg.RestoreSIVIHeights(unwrapContextService(s), p0, p1, p2, p3, vprocorepkg.AuditRestoreAction(p4))
}

func (s *ContextService) RestoreSIVIParentActions(p0 context.Context, p1 string, p2 string, p3 string, p4 AuditRestoreAction) (*AuditRestoreResult, error) {
	return sivipkg.RestoreSIVIParentActions(unwrapContextService(s), p0, p1, p2, p3, vprocorepkg.AuditRestoreAction(p4))
}

func (s *ContextService) RestoreSIVIParentDirect(p0 context.Context, p1 string, p2 string, p3 string, p4 AuditRestoreAction) (*AuditRestoreResult, error) {
	return sourceformpkg.RestoreSIVIParentDirect(unwrapContextService(s), p0, p1, p2, p3, vprocorepkg.AuditRestoreAction(p4))
}

func (s *ContextService) RestoreSIVIProjectAssignment(p0 context.Context, p1 string, p2 string, p3 string, p4 AuditRestoreAction) (*AuditRestoreResult, error) {
	return sourceformpkg.RestoreSIVIProjectAssignment(unwrapContextService(s), p0, p1, p2, p3, vprocorepkg.AuditRestoreAction(p4))
}

func (s *ContextService) ReviewEnvironmentSiteUnits(p0 context.Context, p1 string) (EnvironmentSiteUnitReview, error) {
	return classificationpkg.ReviewEnvironmentSiteUnits(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewPlotProfileFileCreation(p0 context.Context, p1 string) (PlotProfileFileReview, error) {
	return classificationpkg.ReviewPlotProfileFileCreation(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewPlotProfileTableCreation(p0 context.Context, p1 string) (PlotProfileTableReview, error) {
	return classificationpkg.ReviewPlotProfileTableCreation(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewProjectMetadataRestoration(p0 context.Context, p1 string, p2 string, p3 string) (ProjectMetadataRestoreReview, error) {
	return classificationpkg.ReviewProjectMetadataRestoration(unwrapContextService(s), p0, p1, p2, p3)
}

func (s *ContextService) ReviewProjectPlotProfile(p0 context.Context, p1 string) (ProjectPlotProfileReview, error) {
	return classificationpkg.ReviewProjectPlotProfile(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewProjectPlotProfileLump(p0 context.Context, p1 string) (ProjectMetadataTable, error) {
	return classificationpkg.ReviewProjectPlotProfileLump(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewProjectPlotProfileSU(p0 context.Context, p1 string, p2 ProjectPlotProfileFilterRequest) (ProfileSUReview, error) {
	return classificationpkg.ReviewProjectPlotProfileSU(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) ReviewProjectPlotProfileSUInProject(p0 context.Context, p1 string, p2 ProjectPlotProfileFilterRequest) (ProfileSUProjectReview, error) {
	return classificationpkg.ReviewProjectPlotProfileSUInProject(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) ReviewSiteUnitEnvironment(p0 context.Context, p1 string) (SiteUnitEnvironmentReview, error) {
	return classificationpkg.ReviewSiteUnitEnvironment(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewSpeciesCodes(p0 context.Context, p1 string) (SpeciesCodeCheckReview, error) {
	return validationpkg.ReviewSpeciesCodes(unwrapContextService(s), p0, p1)
}

func (s *ContextService) ReviewVegetationDeletion(p0 context.Context, p1 string, p2 string, p3 string, p4 int64) (VegetationDeletionReview, error) {
	return plotentrypkg.ReviewVegetationDeletion(unwrapContextService(s), p0, p1, p2, p3, p4)
}

func (s *ContextService) RunProjectPlotProfile(p0 context.Context, p1 string, p2 ProjectPlotProfileRunRequest) (ProjectPlotProfileResult, error) {
	return classificationpkg.RunProjectPlotProfile(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) SaveProjectPlotProfile(p0 context.Context, p1 string, p2 ProjectPlotProfileEdit) error {
	return classificationpkg.SaveProjectPlotProfile(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) SaveProjectPlotProfileSU(p0 context.Context, p1 string, p2 ProfileSUCreation) (ProfileSUCreated, error) {
	return classificationpkg.SaveProjectPlotProfileSU(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) SaveProjectPlotProfileSUInProject(p0 context.Context, p1 string, p2 ProfileSUProjectCreation) (ProfileSUCreated, error) {
	return classificationpkg.SaveProjectPlotProfileSUInProject(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) SaveSIVIHeights(p0 context.Context, p1 string, p2 string, p3 bool, p4 []SIVIVegetationProjection, p5 []SIVIHeightEdit) (*SIVIHeightWriteResult, error) {
	return sivipkg.SaveSIVIHeights(unwrapContextService(s), p0, p1, p2, p3, p4, p5)
}

func (s *ContextService) SaveSIVIParentActions(p0 context.Context, p1 string, p2 string, p3 SIVIParentActionWrite) (*SIVIParentActionSaveResult, error) {
	return sivipkg.SaveSIVIParentActions(unwrapContextService(s), p0, p1, p2, p3)
}

func (s *ContextService) SaveSIVIParentDirect(p0 context.Context, p1 string, p2 string, p3 SIVIParentDirectWrite) (*SIVIParentWriteResult, error) {
	return sourceformpkg.SaveSIVIParentDirect(unwrapContextService(s), p0, p1, p2, p3)
}

func (s *ContextService) SaveSIVIProjectAssignment(p0 context.Context, p1 string, p2 string, p3 SIVIProjectAssignmentWrite) (*SIVIParentWriteResult, error) {
	return sourceformpkg.SaveSIVIProjectAssignment(unwrapContextService(s), p0, p1, p2, p3)
}

func (s *ContextService) SaveSpeciesCodeCheck(p0 string, p1 []SpeciesCodeCheckUpdate) error {
	return validationpkg.SaveSpeciesCodeCheck(unwrapContextService(s), p0, p1)
}

func (s *ContextService) SetPlotProfileEditing(p0 context.Context, p1 string, p2 PlotProfileWriteRequest) (ProjectState, error) {
	return classificationpkg.SetPlotProfileEditing(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) SetSIVIProjectIDSource(p0 context.Context, p1 string, p2 int, p3 int) (*SIVIProjectChoices, error) {
	return sivipkg.SetSIVIProjectIDSource(unwrapContextService(s), p0, p1, p2, p3)
}

func (s *ContextService) TransferEnvironmentSiteUnits(p0 context.Context, p1 string, p2 EnvironmentSiteUnitTransfer) (*EnvironmentSiteUnitResult, error) {
	return classificationpkg.TransferEnvironmentSiteUnits(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) TransferSiteUnitEnvironment(p0 context.Context, p1 string, p2 SiteUnitEnvironmentTransfer) (*SiteUnitEnvironmentResult, error) {
	return classificationpkg.TransferSiteUnitEnvironment(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) UpdateVegetationAttributes(p0 string, p1 string, p2 []VegetationAttributeUpdate) error {
	return plotentrypkg.CtxUpdateVegetationAttributes(unwrapContextService(s), p0, p1, p2)
}

func (s *ContextService) RestoreSelectedAuditRecords(contextID, plot string, rowIDs []string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.ContextService == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.ContextService.RestoreSelectedAuditRecords(contextID, plot, rowIDs, vprocorepkg.AuditRestoreAction(action))
}
