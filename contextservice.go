package main

import (
	"context"
	"errors"
)

type ContextSelection struct {
	Project       string `json:"project"`
	ProjectPath   string `json:"projectPath"`
	SU            string `json:"su"`
	SUPath        string `json:"suPath"`
	Hierarchy     string `json:"hierarchy"`
	HierarchyPath string `json:"hierarchyPath"`
}

type ContextService struct {
	projects                        *ProjectService
	plots                           *PlotService
	siviHeightEnabled               bool
	siviParentReviewEnabled         bool
	siviParentEditingEnabled        bool
	siviParentActionEditingEnabled  bool
	siviProjectAssignmentEnabled    bool
	tableCSVReviewEnabled           bool
	plotLocationReviewEnabled       bool
	siteUnitSummaryEnabled          bool
	siteUnitSummaryLifeformsEnabled bool
	siteUnitSummarySpeciesEnabled   bool
	longVegetationNoneEnabled       bool
	longVegetationLifeformEnabled   bool
	longVegetationStrataEnabled     bool
	longVegetationCodeEnabled       bool
}

func NewContextService(projects *ProjectService, plots *PlotService) (*ContextService, error) {
	if projects == nil || plots == nil || plots.projects != projects || projects.sqlite == nil {
		return nil, errors.New("context service requires one SQLite project owner and its plot service")
	}
	return &ContextService{projects: projects, plots: plots}, nil
}

func (s *ContextService) SwitchContext(expectedID string, requested ContextSelection) (ProjectState, error) {
	s.projects.operationMu.Lock()
	defer s.projects.operationMu.Unlock()
	s.projects.mu.Lock()
	defer s.projects.mu.Unlock()
	if expectedID == "" || expectedID != s.projects.contextID {
		return ProjectState{}, errors.New("project context changed; reload before switching")
	}
	return s.projects.switchSQLiteLocked(desktopSelection{
		Project: requested.Project, ProjectPath: requested.ProjectPath, SU: requested.SU, SUPath: requested.SUPath,
		Hierarchy: requested.Hierarchy, HierarchyPath: requested.HierarchyPath,
	})
}

func withContextPlot[T any](s *ContextService, expectedID string, operation func(*PlotService) (T, error)) (T, error) {
	return withContextPlotRequest(context.Background(), s, expectedID, operation)
}

func withContextPlotRequest[T any](ctx context.Context, s *ContextService, expectedID string, operation func(*PlotService) (T, error)) (T, error) {
	if err := acquireReadLease(ctx, &s.projects.operationMu); err != nil {
		var result T
		return result, err
	}
	defer s.projects.operationMu.RUnlock()
	if err := acquireReadLease(ctx, &s.projects.mu); err != nil {
		var result T
		return result, err
	}
	if expectedID == "" || expectedID != s.projects.contextID || s.projects.sqlite.conn == nil {
		s.projects.mu.RUnlock()
		var result T
		return result, errors.New("editor context is stale or closed; reload before editing")
	}
	// Immutable selection snapshot; the operation lease prevents its owner closing.
	projects := &ProjectService{root: s.projects.root, config: s.projects.config,
		active: s.projects.active, activeSU: s.projects.activeSU, activeHierarchy: s.projects.activeHierarchy,
		hierarchyFile: s.projects.hierarchyFile, sqlite: s.projects.sqlite, contextID: s.projects.contextID,
		preferences: s.projects.preferences, supportPaths: s.projects.supportPaths}
	s.projects.mu.RUnlock()
	if err := acquireReadLease(ctx, &s.plots.mu); err != nil {
		var result T
		return result, err
	}
	plots := &PlotService{projects: projects, auditStrength: s.plots.auditStrength, currentUser: s.plots.currentUser,
		siteCodes: s.plots.siteCodes, siteCodesError: s.plots.siteCodesError,
		parentCodes: s.plots.parentCodes, parentCodesError: s.plots.parentCodesError,
		contextScoped: true, requestContext: ctx}
	s.plots.mu.RUnlock()
	return operation(plots)
}

func (s *ContextService) edit(expectedID string, operation func(*PlotService) error) error {
	_, err := withContextPlot(s, expectedID, func(plots *PlotService) (struct{}, error) {
		return struct{}{}, operation(plots)
	})
	return err
}

func (s *ContextService) GetPlot(ctx context.Context, contextID, plot string) (*FS882Header, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*FS882Header, error) { return plots.GetPlot(plot) })
}

func (s *ContextService) GetHeaderCapabilities(ctx context.Context, contextID string) (map[string]bool, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (map[string]bool, error) { return plots.GetHeaderCapabilities() })
}

func (s *ContextService) CanEditMasterBEC(ctx context.Context, contextID string) (bool, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (bool, error) {
		return plots.CanEditMasterBEC(), nil
	})
}

func (s *ContextService) GetChildCapabilities(ctx context.Context, contextID, kind string) (map[string]bool, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (map[string]bool, error) { return plots.GetChildCapabilities(kind) })
}

func (s *ContextService) ListVegRecords(ctx context.Context, contextID, plot string) ([]VegRecord, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]VegRecord, error) { return plots.ListVegRecords(plot) })
}

func (s *ContextService) ListHumusRecords(ctx context.Context, contextID, plot string) ([]HumusRecord, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]HumusRecord, error) { return plots.ListHumusRecords(plot) })
}

func (s *ContextService) ListMineralRecords(ctx context.Context, contextID, plot string) ([]MineralRecord, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]MineralRecord, error) { return plots.ListMineralRecords(plot) })
}

func (s *ContextService) ListOtherRecords(ctx context.Context, contextID, plot string) ([]OtherRecord, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]OtherRecord, error) { return plots.ListOtherRecords(plot) })
}

func (s *ContextService) ListAuditEntries(ctx context.Context, contextID, plot string) ([]AuditEntry, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]AuditEntry, error) { return plots.ListAuditEntries(plot) })
}

func (s *ContextService) CreatePlot(contextID string, header FS882Header) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.CreatePlot(header) })
}

func (s *ContextService) UpdatePlot(contextID string, header FS882Header) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdatePlot(header) })
}

func (s *ContextService) SaveVegRecord(contextID string, record VegRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.SaveVegRecord(record) })
}

func (s *ContextService) UpdateVegRecord(contextID string, record VegRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateVegRecord(record) })
}

func (s *ContextService) DeleteVegRecord(contextID, plot string, id int64) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.DeleteVegRecord(plot, id) })
}

func (s *ContextService) SaveHumusRecord(contextID string, record HumusRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.SaveHumusRecord(record) })
}

func (s *ContextService) UpdateHumusRecord(contextID string, record HumusRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateHumusRecord(record) })
}

func (s *ContextService) DeleteHumusRecord(contextID, plot string, id int64) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.DeleteHumusRecord(plot, id) })
}

func (s *ContextService) SaveMineralRecord(contextID string, record MineralRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.SaveMineralRecord(record) })
}

func (s *ContextService) UpdateMineralRecord(contextID string, record MineralRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateMineralRecord(record) })
}

func (s *ContextService) DeleteMineralRecord(contextID, plot string, id int64) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.DeleteMineralRecord(plot, id) })
}

func (s *ContextService) SaveOtherRecord(contextID string, record OtherRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.SaveOtherRecord(record) })
}

func (s *ContextService) UpdateOtherRecord(contextID string, record OtherRecord) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateOtherRecord(record) })
}

func (s *ContextService) UpdateOtherRecords(contextID, plot string, updates []OtherRecordUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateOtherRecords(plot, updates) })
}

func (s *ContextService) UpdateSoilRecords(contextID, plot string, updates []SoilRecordUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateSoilRecords(plot, updates) })
}

func (s *ContextService) DeleteOtherRecord(contextID, plot string, id int64) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.DeleteOtherRecord(plot, id) })
}

func (s *ContextService) UpdateHeightRecords(contextID, plot string, updates []HeightRecordUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateHeightRecords(plot, updates) })
}

func (s *ContextService) SetAuditRestoreSelection(contextID, plot string, rowIDs []string) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.SetAuditRestoreSelection(plot, rowIDs) })
}

func (s *ContextService) RestoreSelectedAuditRecords(contextID, plot string, rowIDs []string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return withContextPlot(s, contextID, func(plots *PlotService) (*AuditRestoreResult, error) {
		return plots.RestoreSelectedAuditRecords(plot, rowIDs, action)
	})
}
