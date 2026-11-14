package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"

	classificationpkg "github.com/boostao/vpro-wails/internal/classification"
	cloudsyncpkg "github.com/boostao/vpro-wails/internal/cloudsync"
	envreportpkg "github.com/boostao/vpro-wails/internal/envreport"
	interchangepkg "github.com/boostao/vpro-wails/internal/interchange"
	plotentrypkg "github.com/boostao/vpro-wails/internal/plotentry"
	projectopspkg "github.com/boostao/vpro-wails/internal/projectops"
	sivipkg "github.com/boostao/vpro-wails/internal/sivi"
	sourceformpkg "github.com/boostao/vpro-wails/internal/sourceform"
	spatialpkg "github.com/boostao/vpro-wails/internal/spatial"
	twopagepkg "github.com/boostao/vpro-wails/internal/twopage"
	validationpkg "github.com/boostao/vpro-wails/internal/validation"
	vegreportpkg "github.com/boostao/vpro-wails/internal/vegreport"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type ContextService struct {
	core  *vprocorepkg.ContextService
	cloud *cloudsyncpkg.Manager
}

func NewCloudSyncManager(configDir string) *cloudsyncpkg.Manager {
	return cloudsyncpkg.NewManager(configDir, os.LookupEnv)
}

func (s *ContextService) cloudManager() (*cloudsyncpkg.Manager, error) {
	if s == nil {
		return nil, errors.New("project context is unavailable")
	}
	if s.cloud != nil {
		return s.cloud, nil
	}
	cfgDir := ""
	if s.core != nil && s.core.Projects != nil {
		cfgDir = s.core.Projects.Config
	}
	return cloudsyncpkg.NewManager(cfgDir, os.LookupEnv), nil
}

func unwrapContextService(s *ContextService) *vprocorepkg.ContextService {
	if s == nil {
		return nil
	}
	return s.core
}

func NewContextService(projects *vprocorepkg.ProjectService, plots *vprocorepkg.PlotService) (*ContextService, error) {
	core, err := vprocorepkg.NewContextService(projects, plots)
	if err != nil {
		return nil, err
	}
	cfgDir := ""
	if projects != nil {
		cfgDir = projects.Config
	}
	return &ContextService{
		core:  core,
		cloud: cloudsyncpkg.NewManager(cfgDir, os.LookupEnv),
	}, nil
}

type activatedProjectContext struct {
	preferences          *vprocorepkg.DesktopConfig
	releaseChannel       string
	configuredSpatialDir string
	projects             *vprocorepkg.ProjectService
	plots                *vprocorepkg.PlotService
	contextSvc           *ContextService
}

func openActiveProjectContext(
	dataDir, configDir string,
	refs *referenceCatalogBundle,
	cloud *cloudsyncpkg.Manager,
) (*activatedProjectContext, error) {
	preferences, err := vprocorepkg.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		return nil, err
	}
	releaseChannel, err := preferences.ReleaseChannel()
	if err != nil {
		return nil, err
	}
	projects, err := vprocorepkg.NewSQLiteProjectService(dataDir, configDir, preferences)
	if err != nil {
		return nil, err
	}
	if err := projects.EnsureActiveProjectWAL(context.Background()); err != nil {
		log.Printf("Warning: failed to initialize active project WAL mode: %v", err)
	}
	configuredSpatialDir := ""
	if snap, snapErr := preferences.Snapshot(); snapErr == nil {
		if spSec, secErr := vprocorepkg.ConfigSection(snap, "Spatial"); secErr == nil {
			configuredSpatialDir, _ = spSec["AssetsDir"].(string)
		}
	}
	plots, err := vprocorepkg.NewPlotServiceWithPreferences(projects)
	if err != nil {
		_ = projects.CloseSQLiteContext()
		return nil, err
	}
	if refs != nil {
		plots.SiteCodes, plots.SiteCodesError = refs.siteCodes, refs.siteCodesErr
		plots.ParentCodes, plots.ParentCodesError = refs.parentCodes, refs.parentCodesErr
	}
	builtContext, err := NewContextService(projects, plots)
	if err != nil {
		_ = projects.CloseSQLiteContext()
		return nil, err
	}
	builtContext.cloud = cloud
	return &activatedProjectContext{
		preferences:          preferences,
		releaseChannel:       releaseChannel,
		configuredSpatialDir: configuredSpatialDir,
		projects:             projects,
		plots:                plots,
		contextSvc:           builtContext,
	}, nil
}

func (s *ContextService) GetState(ctx context.Context) (ProjectState, error) {
	if s == nil || s.core == nil || s.core.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.core.Projects.GetState(ctx)
}

func (s *ContextService) ListPlots(ctx context.Context, offset, limit int) (PlotPage, error) {
	if s == nil || s.core == nil || s.core.Projects == nil {
		return PlotPage{}, errors.New("project context is unavailable")
	}
	return s.core.Projects.ListPlots(ctx, offset, limit)
}

func (s *ContextService) GetHierarchyNodes(ctx context.Context) ([]HierarchyNode, error) {
	if s == nil || s.core == nil || s.core.Projects == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.Projects.GetHierarchyNodes(ctx)
}

func (s *ContextService) SelectProject(name string) (ProjectState, error) {
	if s == nil || s.core == nil || s.core.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.core.Projects.SelectProject(name)
}

func (s *ContextService) SelectSU(name string) (ProjectState, error) {
	if s == nil || s.core == nil || s.core.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.core.Projects.SelectSU(name)
}

func (s *ContextService) SelectHierarchy(name, file string) (ProjectState, error) {
	if s == nil || s.core == nil || s.core.Projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.core.Projects.SelectHierarchy(name, file)
}

func (s *ContextService) SwitchContext(expectedID string, requested vprocorepkg.ContextSelection) (vprocorepkg.ProjectState, error) {
	if s == nil || s.core == nil {
		return vprocorepkg.ProjectState{}, errors.New("project context is unavailable")
	}
	return s.core.SwitchContext(expectedID, requested)
}

func (s *ContextService) GetPlot(ctx context.Context, contextID, plot string) (*vprocorepkg.FS882Header, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.GetPlot(ctx, contextID, plot)
}

func (s *ContextService) GetHeaderCapabilities(ctx context.Context, contextID string) (map[string]bool, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.GetHeaderCapabilities(ctx, contextID)
}

func (s *ContextService) CanEditMasterBEC(ctx context.Context, contextID string) (bool, error) {
	if s == nil || s.core == nil {
		return false, errors.New("project context is unavailable")
	}
	return s.core.CanEditMasterBEC(ctx, contextID)
}

func (s *ContextService) GetChildCapabilities(ctx context.Context, contextID, kind string) (map[string]bool, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.GetChildCapabilities(ctx, contextID, kind)
}

func (s *ContextService) ListVegRecords(ctx context.Context, contextID, plot string) ([]vprocorepkg.VegRecord, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListVegRecords(ctx, contextID, plot)
}

func (s *ContextService) ListHumusRecords(ctx context.Context, contextID, plot string) ([]vprocorepkg.HumusRecord, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListHumusRecords(ctx, contextID, plot)
}

func (s *ContextService) ListMineralRecords(ctx context.Context, contextID, plot string) ([]vprocorepkg.MineralRecord, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListMineralRecords(ctx, contextID, plot)
}

func (s *ContextService) ListOtherRecords(ctx context.Context, contextID, plot string) ([]vprocorepkg.OtherRecord, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListOtherRecords(ctx, contextID, plot)
}

func (s *ContextService) ListAuditEntries(ctx context.Context, contextID, plot string) ([]vprocorepkg.AuditEntry, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListAuditEntries(ctx, contextID, plot)
}

func (s *ContextService) ListAuditReadEntries(ctx context.Context, contextID, plot string) ([]vprocorepkg.OrdinaryAuditReadEntry, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListAuditReadEntries(ctx, contextID, plot)
}

func (s *ContextService) CreatePlot(contextID string, header vprocorepkg.FS882Header) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.CreatePlot(contextID, header)
}

func (s *ContextService) UpdatePlot(contextID string, header vprocorepkg.FS882Header) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdatePlot(contextID, header)
}

func (s *ContextService) SaveVegRecord(contextID string, record vprocorepkg.VegRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.SaveVegRecord(contextID, record)
}

func (s *ContextService) UpdateVegRecord(contextID string, record vprocorepkg.VegRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateVegRecord(contextID, record)
}

func (s *ContextService) DeleteVegRecord(contextID, plot string, id int64) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.DeleteVegRecord(contextID, plot, id)
}

func (s *ContextService) SaveHumusRecord(contextID string, record vprocorepkg.HumusRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.SaveHumusRecord(contextID, record)
}

func (s *ContextService) UpdateHumusRecord(contextID string, record vprocorepkg.HumusRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateHumusRecord(contextID, record)
}

func (s *ContextService) DeleteHumusRecord(contextID, plot string, id int64) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.DeleteHumusRecord(contextID, plot, id)
}

func (s *ContextService) SaveMineralRecord(contextID string, record vprocorepkg.MineralRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.SaveMineralRecord(contextID, record)
}

func (s *ContextService) UpdateMineralRecord(contextID string, record vprocorepkg.MineralRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateMineralRecord(contextID, record)
}

func (s *ContextService) DeleteMineralRecord(contextID, plot string, id int64) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.DeleteMineralRecord(contextID, plot, id)
}

func (s *ContextService) SaveOtherRecord(contextID string, record vprocorepkg.OtherRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.SaveOtherRecord(contextID, record)
}

func (s *ContextService) UpdateOtherRecord(contextID string, record vprocorepkg.OtherRecord) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateOtherRecord(contextID, record)
}

func (s *ContextService) UpdateOtherRecords(contextID, plot string, updates []vprocorepkg.OtherRecordUpdate) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateOtherRecords(contextID, plot, updates)
}

func (s *ContextService) UpdateSoilRecords(contextID, plot string, updates []vprocorepkg.SoilRecordUpdate) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateSoilRecords(contextID, plot, updates)
}

func (s *ContextService) DeleteOtherRecord(contextID, plot string, id int64) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.DeleteOtherRecord(contextID, plot, id)
}

func (s *ContextService) UpdateHeightRecords(contextID, plot string, updates []vprocorepkg.HeightRecordUpdate) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateHeightRecords(contextID, plot, updates)
}

func (s *ContextService) SetAuditRestoreSelection(contextID, plot string, rowIDs []string) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.SetAuditRestoreSelection(contextID, plot, rowIDs)
}

func (s *ContextService) UpdateCollectedRecords(contextID, plot string, updates []vprocorepkg.CollectedRecordUpdate) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateCollectedRecords(contextID, plot, updates)
}

func (s *ContextService) UpdateVegetationSpecies(contextID, plot string, updates []vprocorepkg.VegetationSpeciesUpdate) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateVegetationSpecies(contextID, plot, updates)
}

func (s *ContextService) ListVegetationSpecies(ctx context.Context, contextID, form string) ([]vprocorepkg.VegetationSpeciesOption, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListVegetationSpecies(ctx, contextID, form)
}

func (s *ContextService) ListVegetationSpeciesAliases(ctx context.Context, contextID string, lookup vprocorepkg.VegetationSpeciesLookup) ([]vprocorepkg.VegetationSpeciesAlias, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListVegetationSpeciesAliases(ctx, contextID, lookup)
}

func (s *ContextService) ListVegetationSpeciesUsers(ctx context.Context, contextID string, lookup vprocorepkg.VegetationSpeciesLookup) ([]vprocorepkg.VegetationSpeciesOption, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListVegetationSpeciesUsers(ctx, contextID, lookup)
}

func (s *ContextService) GetVegetationReadAvailability(ctx context.Context, contextID, plot string) (*vprocorepkg.VegetationReadAvailability, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.GetVegetationReadAvailability(ctx, contextID, plot)
}

func (s *ContextService) UpdateVegetationNumbers(contextID, plot string, updates []vprocorepkg.VegetationNumberUpdate) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.UpdateVegetationNumbers(contextID, plot, updates)
}

func (s *ContextService) CreateSourceVegetation(contextID, plot string, request vprocorepkg.VegetationCreationRequest) (int64, error) {
	if s == nil || s.core == nil {
		return 0, errors.New("project context is unavailable")
	}
	return s.core.CreateSourceVegetation(contextID, plot, request)
}

func (s *ContextService) LookupScopedPlot(ctx context.Context, contextID string, request vprocorepkg.ScopedPlotLookup) (vprocorepkg.ScopedPlotLookupResult, error) {
	if s == nil || s.core == nil {
		return vprocorepkg.ScopedPlotLookupResult{}, errors.New("project context is unavailable")
	}
	return s.core.LookupScopedPlot(ctx, contextID, request)
}

func (s *ContextService) SaveProjectMetadata(ctx context.Context, contextID string, request vprocorepkg.ProjectMetadataEdit) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.SaveProjectMetadata(ctx, contextID, request)
}

func (s *ContextService) ReviewProjectMetadata(ctx context.Context, contextID, plot string) (vprocorepkg.ProjectMetadataReview, error) {
	if s == nil || s.core == nil {
		return vprocorepkg.ProjectMetadataReview{}, errors.New("project context is unavailable")
	}
	return s.core.ReviewProjectMetadata(ctx, contextID, plot)
}

func (s *ContextService) InspectExternalSQLiteFile(ctx context.Context, contextID, path string) (vprocorepkg.ExternalSQLiteFileInspection, error) {
	if s == nil || s.core == nil {
		return vprocorepkg.ExternalSQLiteFileInspection{}, errors.New("project context is unavailable")
	}
	return s.core.InspectExternalSQLiteFile(ctx, contextID, path)
}

func (s *ContextService) ChooseExternalSQLiteFile(ctx context.Context, contextID string) (vprocorepkg.ExternalSQLiteFileInspection, error) {
	if s == nil || s.core == nil {
		return vprocorepkg.ExternalSQLiteFileInspection{}, errors.New("project context is unavailable")
	}
	return s.core.ChooseExternalSQLiteFile(ctx, contextID)
}

func (s *ContextService) ChooseAccessDatabaseFile(ctx context.Context, contextID string) (string, error) {
	if s == nil || s.core == nil {
		return "", errors.New("project context is unavailable")
	}
	return s.core.ChooseAccessDatabaseFile(ctx, contextID)
}

func (s *ContextService) ChooseNewSQLiteProjectFile(ctx context.Context, contextID, suggestedProject string) (string, error) {
	if s == nil || s.core == nil {
		return "", errors.New("project context is unavailable")
	}
	return s.core.ChooseNewSQLiteProjectFile(ctx, contextID, suggestedProject)
}

func (s *ContextService) RevealInFileExplorer(ctx context.Context, contextID, target string) (string, error) {
	if s == nil || s.core == nil {
		return "", errors.New("project context is unavailable")
	}
	return s.core.RevealInFileExplorer(ctx, contextID, target)
}

func (s *ContextService) ReadNewProjectTemplates(ctx context.Context, contextID string) (*vprocorepkg.NewProjectTemplateReview, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ReadNewProjectTemplates(ctx, contextID)
}

func (s *ContextService) SelectPlotProfile(expectedID string, requested vprocorepkg.PlotProfileSource) (vprocorepkg.ProjectState, error) {
	if s == nil || s.core == nil {
		return vprocorepkg.ProjectState{}, errors.New("project context is unavailable")
	}
	return s.core.SelectPlotProfile(expectedID, requested)
}

func (s *ContextService) ListPlotProfileSources(ctx context.Context, contextID, path string) ([]vprocorepkg.PlotProfileSourceInfo, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ListPlotProfileSources(ctx, contextID, path)
}

func (s *ContextService) ReadPlotLocations(ctx context.Context, contextID string) (*vprocorepkg.PlotLocationReview, error) {
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.ReadPlotLocations(ctx, contextID)
}

func (s *ContextService) RequireSelectedListReportScope(contextID string) error {
	if s == nil || s.core == nil {
		return errors.New("project context is unavailable")
	}
	return s.core.RequireSelectedListReportScope(contextID)
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
	if s == nil || s.core == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.core.RestoreSelectedAuditRecords(contextID, plot, rowIDs, vprocorepkg.AuditRestoreAction(action))
}

func (s *ContextService) CreateNewProject(ctx context.Context, contextID string, req projectopspkg.NewProjectRequest) (projectopspkg.NewProjectResult, error) {
	return projectopspkg.CreateNewProject(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) SaveProjectAs(ctx context.Context, contextID string, req projectopspkg.SaveProjectAsRequest) (projectopspkg.SaveProjectAsResult, error) {
	return projectopspkg.SaveProjectAs(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) ExportSplinterProject(ctx context.Context, contextID string, req projectopspkg.ExportSplinterRequest) (projectopspkg.ExportSplinterResult, error) {
	return projectopspkg.ExportSplinterProject(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) CompareProjects(ctx context.Context, contextID string, req projectopspkg.CompareProjectsRequest) (projectopspkg.CompareProjectsResult, error) {
	return projectopspkg.CompareProjects(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) PreviewOptimizeVeg(ctx context.Context, contextID string, opts projectopspkg.OptimizeVegOptions) (projectopspkg.OptimizeVegPreview, error) {
	return projectopspkg.PreviewOptimizeVeg(unwrapContextService(s), ctx, contextID, opts)
}

func (s *ContextService) ApplyOptimizeVeg(ctx context.Context, contextID string, req projectopspkg.ApplyOptimizeVegRequest) (projectopspkg.OptimizeVegResult, error) {
	return projectopspkg.ApplyOptimizeVeg(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) PreviewThreeWayMerge(ctx context.Context, contextID string, req projectopspkg.ThreeWayMergePreviewRequest) (projectopspkg.ThreeWayMergePreview, error) {
	return projectopspkg.PreviewThreeWayMerge(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) ExecuteThreeWayMerge(ctx context.Context, contextID string, req projectopspkg.ThreeWayMergeExecuteRequest) (projectopspkg.ThreeWayMergeResult, error) {
	return projectopspkg.ExecuteThreeWayMerge(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) DeriveProjectMetadataFromPlots(ctx context.Context, contextID, projectID string) (projectopspkg.DerivedProjectMetadata, error) {
	return projectopspkg.DeriveProjectMetadataFromPlots(unwrapContextService(s), ctx, contextID, projectID)
}

func (s *ContextService) ListProjectMetadataPresets() []projectopspkg.ProjectMetadataPreset {
	return projectopspkg.ListProjectMetadataPresets()
}

func (s *ContextService) ApplyProjectMetadataAutomation(ctx context.Context, contextID string, req projectopspkg.ApplyProjectMetadataAutomationRequest) (projectopspkg.ApplyProjectMetadataAutomationResult, error) {
	return projectopspkg.ApplyProjectMetadataAutomation(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) RunComplianceValidation(ctx context.Context, contextID string, opts validationpkg.ComplianceOptions) (validationpkg.ComplianceValidationReport, error) {
	return validationpkg.RunComplianceValidation(unwrapContextService(s), ctx, contextID, opts)
}

func (s *ContextService) QuarantineOrphanRecords(ctx context.Context, contextID string, req validationpkg.QuarantineOrphanRequest) (validationpkg.QuarantineOrphanResult, error) {
	return validationpkg.QuarantineOrphanRecords(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) DeleteTwoPageSoilHorizon(ctx context.Context, contextID string, req twopagepkg.DeleteTwoPageSoilHorizonRequest) (twopagepkg.DeleteTwoPageSoilHorizonResult, error) {
	return twopagepkg.DeleteTwoPageSoilHorizon(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) RestoreTwoPageSoilHorizon(ctx context.Context, contextID string, req twopagepkg.RestoreTwoPageSoilHorizonRequest) (twopagepkg.RestoreTwoPageSoilHorizonResult, error) {
	return twopagepkg.RestoreTwoPageSoilHorizon(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) ListTwoPageSoilDeletedHorizons(ctx context.Context, contextID, plotNumber, form string) ([]twopagepkg.DeletedTwoPageSoilHorizonEntry, error) {
	return twopagepkg.ListTwoPageSoilDeletedHorizons(unwrapContextService(s), ctx, contextID, plotNumber, form)
}

func (s *ContextService) CreateSIVIPlot(ctx context.Context, contextID string, req sivipkg.CreateSIVIPlotRequest) (sivipkg.CreateSIVIPlotResult, error) {
	return sivipkg.CreateSIVIPlot(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) UndoCreateSIVIPlot(ctx context.Context, contextID string, req sivipkg.UndoCreateSIVIPlotRequest) (sivipkg.UndoCreateSIVIPlotResult, error) {
	return sivipkg.UndoCreateSIVIPlot(unwrapContextService(s), ctx, contextID, req)
}

func (s *ContextService) ListCreatedSIVIPlots(ctx context.Context, contextID string) ([]sivipkg.SIVICreatedPlotEntry, error) {
	return sivipkg.ListCreatedSIVIPlots(unwrapContextService(s), ctx, contextID)
}

// Provincial Cloud BECMaster PostgreSQL Sync, Staging Queue, Standards Sync & Custodian 3-Way Merge RPCs

func buildCloudComplianceValidator() cloudsyncpkg.ComplianceValidatorFunc {
	return func(ctx context.Context, core *vprocorepkg.ContextService, contextID string, useActiveSU bool) (cloudsyncpkg.ComplianceGateResult, error) {
		rep, err := validationpkg.RunComplianceValidation(core, ctx, contextID, validationpkg.ComplianceOptions{UseActiveSU: useActiveSU})
		if err != nil {
			return cloudsyncpkg.ComplianceGateResult{}, err
		}
		raw, _ := json.Marshal(rep)
		blocking := rep.Summary.EnvCodeErrors +
			rep.Summary.BoundsErrors +
			rep.Summary.EnvAdminMismatches +
			rep.Summary.OrphanChildRows +
			rep.Summary.DuplicateSpeciesSplits +
			rep.Summary.UnlistedSpecies +
			rep.Summary.SynonymSpecies
		return cloudsyncpkg.ComplianceGateResult{
			Passed:        rep.Passed,
			BlockingCount: blocking,
			Summary: map[string]int{
				"envCodeErrors":          rep.Summary.EnvCodeErrors,
				"boundsErrors":           rep.Summary.BoundsErrors,
				"envAdminMismatches":     rep.Summary.EnvAdminMismatches,
				"orphanChildRows":        rep.Summary.OrphanChildRows,
				"duplicateSpeciesSplits": rep.Summary.DuplicateSpeciesSplits,
				"unlistedSpecies":        rep.Summary.UnlistedSpecies,
				"synonymSpecies":         rep.Summary.SynonymSpecies,
				"userSpecies":            rep.Summary.UserSpecies,
				"plotsWithoutVeg":        rep.Summary.PlotsWithoutVeg,
			},
			ReportJSON: raw,
		}, nil
	}
}

func (s *ContextService) buildSnapshotBundleExporter() cloudsyncpkg.SnapshotBundleExporterFunc {
	return func(ctx context.Context, sqlitePath, projectName, format, destinationPath string) (string, string, error) {
		core := unwrapContextService(s)
		if core == nil || core.Projects == nil {
			return "", "", errors.New("project context is unavailable")
		}
		sel := vprocorepkg.DesktopSelection{
			Project:     projectName,
			ProjectPath: sqlitePath,
			SU:          "None",
			Hierarchy:   "None",
		}
		ephSqlite, err := vprocorepkg.NewSQLiteContext(ctx, sel, core.Projects.SupportPaths)
		if err != nil {
			return "", "", err
		}
		defer ephSqlite.Close()
		ephID, err := vprocorepkg.NewContextIdentity()
		if err != nil {
			return "", "", err
		}
		ephProjects := &vprocorepkg.ProjectService{
			Root:         core.Projects.Root,
			Config:       core.Projects.Config,
			Active:       projectName,
			ActiveSU:     "None",
			Sqlite:       ephSqlite,
			ContextID:    ephID,
			Preferences:  core.Projects.Preferences,
			SupportPaths: core.Projects.SupportPaths,
		}
		ephPlots := &vprocorepkg.PlotService{Projects: ephProjects, CurrentUser: "Custodian"}
		ephCtx, err := vprocorepkg.NewContextService(ephProjects, ephPlots)
		if err != nil {
			return "", "", err
		}
		switch format {
		case "rds":
			res, rErr := interchangepkg.ExportProjectRDS(ephCtx, ctx, ephID, interchangepkg.RDSExportRequest{
				DestinationPath: destinationPath,
			})
			if rErr != nil {
				return "", "", rErr
			}
			return res.Path, res.SHA256, nil
		case "gpkg":
			res, gErr := interchangepkg.ExportGeoPackage(ephCtx, ctx, ephID, interchangepkg.GeoPackageExportRequest{
				DestinationPath:        destinationPath,
				CRS:                    "EPSG:3005",
				IncludeAttributeTables: true,
			})
			if gErr != nil {
				return "", "", gErr
			}
			return res.Path, res.SHA256, nil
		default:
			return "", "", errors.New("unsupported snapshot export format")
		}
	}
}

func (s *ContextService) GetCloudSyncConfigStatus(ctx context.Context) (cloudsyncpkg.CloudConfigStatus, error) {
	if err := ctx.Err(); err != nil {
		return cloudsyncpkg.CloudConfigStatus{}, err
	}
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.CloudConfigStatus{}, err
	}
	return mgr.GetConfigStatus()
}

func (s *ContextService) ConfigureCloudSyncSession(ctx context.Context, req cloudsyncpkg.SessionConfigRequest) (cloudsyncpkg.CloudConfigStatus, error) {
	if err := ctx.Err(); err != nil {
		return cloudsyncpkg.CloudConfigStatus{}, err
	}
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.CloudConfigStatus{}, err
	}
	return mgr.ConfigureSession(req)
}

func (s *ContextService) TestCloudConnection(ctx context.Context) (cloudsyncpkg.CloudConnectionReport, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.CloudConnectionReport{}, err
	}
	return mgr.TestCloudConnection(ctx)
}

func (s *ContextService) BootstrapCloudSchemas(ctx context.Context) (cloudsyncpkg.SchemaBootstrapResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.SchemaBootstrapResult{}, err
	}
	return mgr.BootstrapSchemas(ctx)
}

func (s *ContextService) PublishStandardsToCloud(ctx context.Context, contextID string, req cloudsyncpkg.PublishStandardsRequest) (cloudsyncpkg.PublishStandardsResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.PublishStandardsResult{}, err
	}
	return mgr.PublishStandardsToCloud(ctx, unwrapContextService(s), contextID, req)
}

func (s *ContextService) SeedBECMasterBaseline(ctx context.Context, req cloudsyncpkg.SeedBaselineRequest) (cloudsyncpkg.SeedBaselineResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.SeedBaselineResult{}, err
	}
	return mgr.SeedBECMasterBaseline(ctx, req)
}

func (s *ContextService) PullCloudStandards(ctx context.Context, contextID string) (cloudsyncpkg.PullStandardsResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.PullStandardsResult{}, err
	}
	return mgr.PullCloudStandards(ctx, unwrapContextService(s), contextID)
}

func (s *ContextService) PreviewBECMasterSubset(ctx context.Context, filter cloudsyncpkg.SubsetFilter) (cloudsyncpkg.SubsetPreview, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.SubsetPreview{}, err
	}
	return mgr.PreviewBECMasterSubset(ctx, filter)
}

func (s *ContextService) PullBECMasterSubset(ctx context.Context, contextID string, req cloudsyncpkg.PullSubsetRequest) (cloudsyncpkg.PullSubsetResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.PullSubsetResult{}, err
	}
	return mgr.PullBECMasterSubset(ctx, unwrapContextService(s), contextID, req)
}

func (s *ContextService) PreviewStagingSubmission(ctx context.Context, contextID string, useActiveSU bool) (cloudsyncpkg.StagingPreflightPreview, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.StagingPreflightPreview{}, err
	}
	return mgr.PreviewStagingSubmission(ctx, unwrapContextService(s), contextID, useActiveSU, buildCloudComplianceValidator())
}

func (s *ContextService) SubmitProjectToStaging(ctx context.Context, contextID string, req cloudsyncpkg.SubmitStagingRequest) (cloudsyncpkg.SubmitStagingResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.SubmitStagingResult{}, err
	}
	return mgr.SubmitProjectToStaging(ctx, unwrapContextService(s), contextID, req, buildCloudComplianceValidator())
}

func (s *ContextService) ListStagingSubmissions(ctx context.Context, filter cloudsyncpkg.ListSubmissionsFilter) ([]cloudsyncpkg.StagingSubmissionSummary, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return nil, err
	}
	return mgr.ListStagingSubmissions(ctx, filter)
}

func (s *ContextService) GetStagingSubmissionDetail(ctx context.Context, submissionID string) (cloudsyncpkg.StagingSubmissionDetail, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.StagingSubmissionDetail{}, err
	}
	return mgr.GetStagingSubmissionDetail(ctx, submissionID)
}

func (s *ContextService) PreviewStagingToMasterMerge(ctx context.Context, req cloudsyncpkg.StagingMergePreviewRequest) (cloudsyncpkg.StagingMergePreview, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.StagingMergePreview{}, err
	}
	return mgr.PreviewStagingToMasterMerge(ctx, req)
}

func (s *ContextService) ExecuteStagingToMasterMerge(ctx context.Context, req cloudsyncpkg.StagingMergeExecuteRequest) (cloudsyncpkg.StagingMergeResult, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.StagingMergeResult{}, err
	}
	return mgr.ExecuteStagingToMasterMerge(ctx, req)
}

func (s *ContextService) RejectStagingSubmission(ctx context.Context, req cloudsyncpkg.RejectSubmissionRequest) (cloudsyncpkg.StagingSubmissionSummary, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.StagingSubmissionSummary{}, err
	}
	return mgr.RejectStagingSubmission(ctx, req)
}

func (s *ContextService) CreateBECMasterSnapshot(ctx context.Context, contextID string, req cloudsyncpkg.CreateSnapshotRequest) (cloudsyncpkg.SnapshotRecord, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.SnapshotRecord{}, err
	}
	return mgr.CreateBECMasterSnapshot(ctx, unwrapContextService(s), contextID, req, s.buildSnapshotBundleExporter())
}

func (s *ContextService) ListBECMasterSnapshots(ctx context.Context) ([]cloudsyncpkg.SnapshotRecord, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return nil, err
	}
	return mgr.ListBECMasterSnapshots(ctx)
}

func (s *ContextService) ListBECMasterDownloadLog(ctx context.Context, limit int) ([]cloudsyncpkg.DownloadLogEntry, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return nil, err
	}
	return mgr.ListBECMasterDownloadLog(ctx, limit)
}

func (s *ContextService) VerifyBECMasterAuditChain(ctx context.Context, limit int) (cloudsyncpkg.AuditChainVerification, error) {
	mgr, err := s.cloudManager()
	if err != nil {
		return cloudsyncpkg.AuditChainVerification{}, err
	}
	return mgr.VerifyBECMasterAuditChain(ctx, limit)
}
