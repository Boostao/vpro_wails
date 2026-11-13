package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (service *ProjectService) ProjectFile(project ProjectInfo) string {
	if project.Path != "" {
		return project.Path
	}
	return filepath.Join(service.Root, "projects", project.File)
}

func NewSQLiteProjectService(root, config string, preferences *DesktopConfig) (_ *ProjectService, resultErr error) {
	if preferences == nil {
		return nil, errors.New("SQLite application context requires shared YAML preferences")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0700); err != nil {
		return nil, err
	}
	if err := installSample(filepath.Join(root, "projects", "Sample.db")); err != nil {
		return nil, err
	}
	support, err := InstallDatabaseFamily(root)
	if err != nil {
		return nil, err
	}
	values, err := preferences.Snapshot()
	if err != nil {
		return nil, err
	}
	if raw, present := values["Desktop"].(map[string]any)["DatabasePaths"]; present {
		paths, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("Desktop.DatabasePaths must be a mapping of support role to explicit absolute file path")
		}
		for role, value := range paths {
			path, valid := value.(string)
			if _, known := support[role]; !known || !valid || !filepath.IsAbs(path) {
				return nil, fmt.Errorf("Desktop.DatabasePaths.%s must name a known support role and absolute file path", role)
			}
			support[role] = path
		}
	}
	service := &ProjectService{Root: root, Config: config, Preferences: preferences, SupportPaths: support}
	selection, err := preferences.Selection()
	if err != nil {
		return nil, err
	}
	if selection.ProjectPath == "" {
		projects, _, err := service.discover()
		if err != nil {
			return nil, err
		}
		selected := CompatibleProject(projects, selection.Project)
		if selected.Name == "" {
			return nil, fmt.Errorf("configured project %q is unavailable; YAML selection was retained", selection.Project)
		}
		selection.ProjectPath = service.ProjectFile(selected)
	}
	if selection.SU != "None" && selection.SUPath == "" {
		selection.SUPath = selection.ProjectPath
	}
	if selection.Hierarchy != "None" && selection.HierarchyPath == "" {
		hierarchies, err := discoverHierarchies(filepath.Join(root, "projects"))
		if err != nil {
			return nil, err
		}
		for _, hierarchy := range hierarchies {
			if hierarchy.Name == selection.Hierarchy && hierarchy.Compatible {
				if selection.HierarchyPath != "" {
					return nil, errors.New("configured hierarchy name is ambiguous; YAML was retained")
				}
				selection.HierarchyPath = filepath.Join(root, "projects", hierarchy.File)
			}
		}
	}
	profile, err := preferences.PlotProfileSelection()
	if err != nil {
		return nil, err
	}
	candidate, err := NewSQLiteContext(context.Background(), selection, support, profile)
	if err != nil {
		return nil, fmt.Errorf("configured context could not be opened; YAML was retained: %w", err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, candidate.Close())
		}
	}()
	service.Sqlite = candidate
	if _, err := service.sqliteStateLocked(); err != nil {
		return nil, err
	}
	id, err := NewContextIdentity()
	if err != nil {
		return nil, err
	}
	if err := service.persistSQLiteContext(candidate); err != nil {
		return nil, err
	}
	service.PublishSQLiteSelection(candidate, id)
	return service, nil
}

func NewContextIdentity() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("initialize project context identity: %w", err)
	}
	return hex.EncodeToString(entropy[:]), nil
}

func (service *ProjectService) PublishSQLiteSelection(candidate *SqliteContext, id string) {
	service.Sqlite, service.ContextID = candidate, id
	service.Active, service.ActiveSU = candidate.Selection.Project, candidate.Selection.SU
	service.activeHierarchy, service.hierarchyFile = candidate.Selection.Hierarchy, ""
	if candidate.Selection.Hierarchy != "None" {
		service.hierarchyFile = filepath.Base(candidate.Selection.HierarchyPath)
	}
}

// EnsureActiveProjectWAL initializes the active project database pool in SQLite WAL journal mode
// so newly materialized Sample.db and newly activated project databases operate in WAL immediately.
func (service *ProjectService) EnsureActiveProjectWAL(ctx context.Context) error {
	if service == nil {
		return errors.New("project service is unavailable")
	}
	if err := sqlutil.AcquireReadLease(ctx, &service.Mu); err != nil {
		return err
	}
	defer service.Mu.RUnlock()
	if service.Sqlite == nil {
		return errors.New("project context is closed")
	}
	_, err := service.Sqlite.ProjectDatabase(ctx)
	return err
}

func (service *ProjectService) switchSQLiteLocked(selection DesktopSelection) (ProjectState, error) {
	if service.Sqlite == nil {
		return ProjectState{}, errors.New("project context is closed")
	}
	return service.switchSQLiteProfileLocked(selection, service.Sqlite.Profile)
}

func (service *ProjectService) switchSQLiteProfileLocked(selection DesktopSelection, profile *PlotProfileSource) (ProjectState, error) {
	if service.Sqlite == nil || service.Sqlite.Conn == nil {
		return ProjectState{}, errors.New("project context is closed")
	}
	candidate, err := NewSQLiteContext(context.Background(), selection, service.SupportPaths, profile)
	if err != nil {
		return ProjectState{}, err
	}
	id, err := NewContextIdentity()
	if err != nil {
		return ProjectState{}, errors.Join(err, candidate.Close())
	}
	old, oldID := service.Sqlite, service.ContextID
	service.Sqlite, service.ContextID = candidate, id
	state, err := service.sqliteStateLocked()
	service.Sqlite, service.ContextID = old, oldID
	if err != nil {
		return ProjectState{}, errors.Join(err, candidate.Close())
	}
	if err := service.persistSQLiteContext(candidate); err != nil {
		return ProjectState{}, errors.Join(err, candidate.Close())
	}
	service.PublishSQLiteSelection(candidate, id)
	if err := old.Close(); err != nil {
		log.Printf("Warning: new context committed, but previous owned SQLite context could not close: %v", err)
	}
	return state, nil
}

func (service *ProjectService) sqliteStateLocked() (ProjectState, error) {
	return service.SqliteStateContextLocked(context.Background())
}

func (service *ProjectService) SqliteStateContextLocked(ctx context.Context) (ProjectState, error) {
	if service.Sqlite.Conn == nil {
		return ProjectState{}, errors.New("project context is closed")
	}
	selection := service.Sqlite.Selection
	projects, diagnostics, err := service.DiscoverContext(ctx)
	if err != nil {
		return ProjectState{}, err
	}
	for i := range projects {
		if err := ctx.Err(); err != nil {
			return ProjectState{}, err
		}
		projects[i].Path, err = ExistingDatabasePath(service.ProjectFile(projects[i]))
		if err != nil {
			return ProjectState{}, err
		}
	}
	found, err := InspectFileContext(ctx, selection.ProjectPath)
	if err != nil {
		return ProjectState{}, err
	}
	for _, project := range found {
		project.Path = selection.ProjectPath
		if !slices.ContainsFunc(projects, func(existing ProjectInfo) bool {
			return existing.Name == project.Name && SameDesktopPath(existing.Path, project.Path)
		}) {
			projects = append(projects, project)
		}
	}
	slices.SortFunc(projects, func(left, right ProjectInfo) int {
		if order := strings.Compare(left.Name, right.Name); order != 0 {
			return order
		}
		return strings.Compare(left.Path, right.Path)
	})
	sus, err := service.DiscoverSUsContext(ctx, ProjectInfo{Path: selection.ProjectPath})
	if err != nil {
		return ProjectState{}, err
	}
	for i := range sus {
		sus[i].Path = selection.ProjectPath
	}
	if selection.SU != "None" && !SameDesktopPath(selection.SUPath, selection.ProjectPath) {
		external, err := service.DiscoverSUsContext(ctx, ProjectInfo{Path: selection.SUPath})
		if err != nil {
			return ProjectState{}, err
		}
		for _, su := range external {
			su.Path = selection.SUPath
			if !slices.ContainsFunc(sus, func(existing SUInfo) bool {
				return existing.Name == su.Name && SameDesktopPath(existing.Path, su.Path)
			}) {
				sus = append(sus, su)
			}
		}
	}
	hierarchies, err := DiscoverHierarchiesContext(ctx, filepath.Join(service.Root, "projects"))
	if err != nil {
		return ProjectState{}, err
	}
	for i := range hierarchies {
		hierarchies[i].Path, err = ExistingDatabasePath(filepath.Join(service.Root, "projects", hierarchies[i].File))
		if err != nil {
			return ProjectState{}, err
		}
	}
	if selection.Hierarchy != "None" {
		db, err := sqlutil.OpenReadOnlyContext(ctx, selection.HierarchyPath)
		if err != nil {
			return ProjectState{}, err
		}
		external, inspectErr := inspectHierarchies(sqlutil.ContextPlotDB{Db: db, Ctx: ctx}, filepath.Base(selection.HierarchyPath))
		inspectErr = errors.Join(inspectErr, db.Close())
		if inspectErr != nil {
			return ProjectState{}, inspectErr
		}
		for _, hierarchy := range external {
			hierarchy.Path = selection.HierarchyPath
			if !slices.ContainsFunc(hierarchies, func(existing HierarchyInfo) bool {
				return existing.Name == hierarchy.Name && SameDesktopPath(existing.Path, hierarchy.Path)
			}) {
				hierarchies = append(hierarchies, hierarchy)
			}
		}
	}
	hierarchyFile := ""
	if selection.Hierarchy != "None" {
		hierarchyFile = filepath.Base(selection.HierarchyPath)
	}
	if err := ctx.Err(); err != nil {
		return ProjectState{}, err
	}
	profile, err := service.Sqlite.ProfileInfo(ctx)
	if err != nil {
		return ProjectState{}, err
	}
	return ProjectState{ActiveProject: selection.Project, ActiveSU: selection.SU, ActiveHierarchy: selection.Hierarchy,
		HierarchyFile: hierarchyFile, Projects: projects, SUs: sus, Hierarchies: hierarchies,
		Diagnostics: diagnostics, ContextID: service.ContextID, ProjectPath: selection.ProjectPath,
		SUPath: selection.SUPath, HierarchyPath: selection.HierarchyPath, PlotProfile: profile}, nil
}

func (service *ProjectService) sqlitePlotsLocked(ctx context.Context, offset, limit int) (PlotPage, error) {
	coordinator := service.Sqlite
	if err := sqlutil.AcquireMutexLease(ctx, &coordinator.Mu); err != nil {
		return PlotPage{}, err
	}
	defer coordinator.Mu.Unlock()
	if coordinator.Conn == nil {
		return PlotPage{}, errors.New("project context is closed")
	}
	page := PlotPage{Plots: []PlotSummary{}}
	if err := coordinator.Conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM USysEnv").Scan(&page.Total); err != nil {
		return PlotPage{}, err
	}
	rows, err := coordinator.Conn.QueryContext(ctx, "SELECT PlotNumber,FieldNumber,PlotRepresenting,Zone,SubZone,SiteSeries FROM USysEnv ORDER BY PlotNumber LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return PlotPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var plot PlotSummary
		if err := rows.Scan(&plot.PlotNumber, &plot.FieldNumber, &plot.PlotRepresenting, &plot.Zone, &plot.SubZone, &plot.SiteSeries); err != nil {
			return PlotPage{}, err
		}
		page.Plots = append(page.Plots, plot)
	}
	return page, rows.Err()
}

func (service *ProjectService) sqliteHierarchyLocked(ctx context.Context) ([]HierarchyNode, error) {
	coordinator := service.Sqlite
	if err := sqlutil.AcquireMutexLease(ctx, &coordinator.Mu); err != nil {
		return nil, err
	}
	defer coordinator.Mu.Unlock()
	if coordinator.Conn == nil {
		return nil, errors.New("project context is closed")
	}
	nodes := []HierarchyNode{}
	if coordinator.Selection.Hierarchy == "None" {
		return nodes, nil
	}
	rows, err := coordinator.Conn.QueryContext(ctx, "SELECT ID,Name,Parent,Level FROM USysHierarchy ORDER BY Level,ID")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var node HierarchyNode
		if err := rows.Scan(&node.ID, &node.Name, &node.Parent, &node.Level); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (service *ProjectService) CloseSQLiteContext() error {
	service.OperationMu.Lock()
	defer service.OperationMu.Unlock()
	service.Mu.Lock()
	defer service.Mu.Unlock()
	if service.Sqlite == nil {
		return nil
	}
	return service.Sqlite.Close()
}
