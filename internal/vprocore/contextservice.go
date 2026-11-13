package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

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

func (r *ContextSelection) UnmarshalJSON(data []byte) error {
	type plain ContextSelection
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type ContextService struct {
	Projects                           *ProjectService                `json:"-" yaml:"-"`
	Plots                              *PlotService                   `json:"-" yaml:"-"`
	SiviHeightEnabled                  bool                           `json:"-" yaml:"-"`
	SiviParentReviewEnabled            bool                           `json:"-" yaml:"-"`
	SiviParentEditingEnabled           bool                           `json:"-" yaml:"-"`
	SiviParentActionEditingEnabled     bool                           `json:"-" yaml:"-"`
	SiviProjectAssignmentEnabled       bool                           `json:"-" yaml:"-"`
	TableCSVReviewEnabled              bool                           `json:"-" yaml:"-"`
	PlotLocationReviewEnabled          bool                           `json:"-" yaml:"-"`
	SiteUnitSummaryEnabled             bool                           `json:"-" yaml:"-"`
	SiteUnitSummaryLifeformsEnabled    bool                           `json:"-" yaml:"-"`
	SiteUnitSummarySpeciesEnabled      bool                           `json:"-" yaml:"-"`
	LongVegetationNoneEnabled          bool                           `json:"-" yaml:"-"`
	LongVegetationNoneUnitsEnabled     bool                           `json:"-" yaml:"-"`
	LongVegetationLifeformEnabled      bool                           `json:"-" yaml:"-"`
	LongVegetationStrataEnabled        bool                           `json:"-" yaml:"-"`
	LongVegetationCodeEnabled          bool                           `json:"-" yaml:"-"`
	LongVegetationSelectedFieldEnabled bool                           `json:"-" yaml:"-"`
	LongVegetationSelectedUnitsEnabled bool                           `json:"-" yaml:"-"`
	SelectedUnitSelections             selectedUnitSelectionLedger    `json:"-" yaml:"-"`
	ProjectReportScopes                vegetationProjectScopeLedger   `json:"-" yaml:"-"`
	TwoPageSoilAttempts                twoPageSoilAttemptLedger       `json:"-" yaml:"-"`
	TwoPageVegetationAttempts          twoPageVegetationAttemptLedger `json:"-" yaml:"-"`
}

func NewContextService(projects *ProjectService, plots *PlotService) (*ContextService, error) {
	if projects == nil || plots == nil || plots.Projects != projects || projects.Sqlite == nil {
		return nil, errors.New("context service requires one SQLite project owner and its plot service")
	}
	return &ContextService{Projects: projects, Plots: plots}, nil
}

func (s *ContextService) SwitchContext(expectedID string, requested ContextSelection) (ProjectState, error) {
	s.Projects.OperationMu.Lock()
	defer s.Projects.OperationMu.Unlock()
	s.Projects.Mu.Lock()
	defer s.Projects.Mu.Unlock()
	if expectedID == "" || expectedID != s.Projects.ContextID {
		return ProjectState{}, errors.New("project context changed; reload before switching")
	}
	state, err := s.Projects.switchSQLiteLocked(DesktopSelection{
		Project: requested.Project, ProjectPath: requested.ProjectPath, SU: requested.SU, SUPath: requested.SUPath,
		Hierarchy: requested.Hierarchy, HierarchyPath: requested.HierarchyPath,
	})
	if err == nil {
		s.SelectedUnitSelections.Mu.Lock()
		s.SelectedUnitSelections.Entries = nil
		s.SelectedUnitSelections.Mu.Unlock()
		s.ProjectReportScopes.Mu.Lock()
		s.ProjectReportScopes.Entries = nil
		s.ProjectReportScopes.Mu.Unlock()
	}
	return state, err
}

func WithContextPlot[T any](s *ContextService, expectedID string, operation func(*PlotService) (T, error)) (T, error) {
	return WithContextPlotRequest(context.Background(), s, expectedID, operation)
}

func WithContextPlotRequest[T any](ctx context.Context, s *ContextService, expectedID string, operation func(*PlotService) (T, error)) (T, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.Projects.OperationMu); err != nil {
		var result T
		return result, err
	}
	defer s.Projects.OperationMu.RUnlock()
	if err := sqlutil.AcquireReadLease(ctx, &s.Projects.Mu); err != nil {
		var result T
		return result, err
	}
	if expectedID == "" || expectedID != s.Projects.ContextID || s.Projects.Sqlite.Conn == nil {
		s.Projects.Mu.RUnlock()
		var result T
		return result, errors.New("editor context is stale or closed; reload before editing")
	}
	// Immutable selection snapshot; the operation lease prevents its owner closing.
	projects := &ProjectService{Root: s.Projects.Root, Config: s.Projects.Config,
		Active: s.Projects.Active, ActiveSU: s.Projects.ActiveSU, activeHierarchy: s.Projects.activeHierarchy,
		hierarchyFile: s.Projects.hierarchyFile, Sqlite: s.Projects.Sqlite, ContextID: s.Projects.ContextID,
		Preferences: s.Projects.Preferences, SupportPaths: s.Projects.SupportPaths}
	s.Projects.Mu.RUnlock()
	if err := sqlutil.AcquireReadLease(ctx, &s.Plots.Mu); err != nil {
		var result T
		return result, err
	}
	plots := &PlotService{Projects: projects, AuditStrength: s.Plots.AuditStrength, CurrentUser: s.Plots.CurrentUser,
		SiteCodes: s.Plots.SiteCodes, SiteCodesError: s.Plots.SiteCodesError,
		ParentCodes: s.Plots.ParentCodes, ParentCodesError: s.Plots.ParentCodesError,
		ContextScoped: true, requestContext: ctx}
	s.Plots.Mu.RUnlock()
	return operation(plots)
}

func (s *ContextService) Edit(expectedID string, operation func(*PlotService) error) error {
	_, err := WithContextPlot(s, expectedID, func(plots *PlotService) (struct{}, error) {
		return struct{}{}, operation(plots)
	})
	return err
}

func (s *ContextService) GetPlot(ctx context.Context, contextID, plot string) (*FS882Header, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*FS882Header, error) { return plots.GetPlot(plot) })
}

func (s *ContextService) GetHeaderCapabilities(ctx context.Context, contextID string) (map[string]bool, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (map[string]bool, error) { return plots.GetHeaderCapabilities() })
}

func (s *ContextService) CanEditMasterBEC(ctx context.Context, contextID string) (bool, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (bool, error) {
		return plots.CanEditMasterBEC(), nil
	})
}

func (s *ContextService) GetChildCapabilities(ctx context.Context, contextID, kind string) (map[string]bool, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (map[string]bool, error) { return plots.GetChildCapabilities(kind) })
}

func (s *ContextService) ListVegRecords(ctx context.Context, contextID, plot string) ([]VegRecord, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]VegRecord, error) { return plots.ListVegRecords(plot) })
}

func (s *ContextService) ListHumusRecords(ctx context.Context, contextID, plot string) ([]HumusRecord, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]HumusRecord, error) { return plots.ListHumusRecords(plot) })
}

func (s *ContextService) ListMineralRecords(ctx context.Context, contextID, plot string) ([]MineralRecord, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]MineralRecord, error) { return plots.ListMineralRecords(plot) })
}

func (s *ContextService) ListOtherRecords(ctx context.Context, contextID, plot string) ([]OtherRecord, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]OtherRecord, error) { return plots.ListOtherRecords(plot) })
}

func (s *ContextService) ListAuditEntries(ctx context.Context, contextID, plot string) ([]AuditEntry, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]AuditEntry, error) { return plots.ListAuditEntries(plot) })
}

func (s *ContextService) CreatePlot(contextID string, header FS882Header) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.CreatePlot(header) })
}

func (s *ContextService) UpdatePlot(contextID string, header FS882Header) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdatePlot(header) })
}

func (s *ContextService) SaveVegRecord(contextID string, record VegRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.SaveVegRecord(record) })
}

func (s *ContextService) UpdateVegRecord(contextID string, record VegRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateVegRecord(record) })
}

func (s *ContextService) DeleteVegRecord(contextID, plot string, id int64) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.DeleteVegRecord(plot, id) })
}

func (s *ContextService) SaveHumusRecord(contextID string, record HumusRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.SaveHumusRecord(record) })
}

func (s *ContextService) UpdateHumusRecord(contextID string, record HumusRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateHumusRecord(record) })
}

func (s *ContextService) DeleteHumusRecord(contextID, plot string, id int64) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.DeleteHumusRecord(plot, id) })
}

func (s *ContextService) SaveMineralRecord(contextID string, record MineralRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.SaveMineralRecord(record) })
}

func (s *ContextService) UpdateMineralRecord(contextID string, record MineralRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateMineralRecord(record) })
}

func (s *ContextService) DeleteMineralRecord(contextID, plot string, id int64) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.DeleteMineralRecord(plot, id) })
}

func (s *ContextService) SaveOtherRecord(contextID string, record OtherRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.SaveOtherRecord(record) })
}

func (s *ContextService) UpdateOtherRecord(contextID string, record OtherRecord) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateOtherRecord(record) })
}

func (s *ContextService) UpdateOtherRecords(contextID, plot string, updates []OtherRecordUpdate) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateOtherRecords(plot, updates) })
}

func (s *ContextService) UpdateSoilRecords(contextID, plot string, updates []SoilRecordUpdate) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateSoilRecords(plot, updates) })
}

func (s *ContextService) DeleteOtherRecord(contextID, plot string, id int64) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.DeleteOtherRecord(plot, id) })
}

func (s *ContextService) UpdateHeightRecords(contextID, plot string, updates []HeightRecordUpdate) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.UpdateHeightRecords(plot, updates) })
}

func (s *ContextService) SetAuditRestoreSelection(contextID, plot string, rowIDs []string) error {
	return s.Edit(contextID, func(plots *PlotService) error { return plots.SetAuditRestoreSelection(plot, rowIDs) })
}

func (s *ContextService) RestoreSelectedAuditRecords(contextID, plot string, rowIDs []string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return WithContextPlot(s, contextID, func(plots *PlotService) (*AuditRestoreResult, error) {
		return plots.RestoreSelectedAuditRecords(plot, rowIDs, action)
	})
}
