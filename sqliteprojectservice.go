package main

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

func (service *ProjectService) projectFile(project ProjectInfo) string {
	if project.Path != "" {
		return project.Path
	}
	return filepath.Join(service.root, "projects", project.File)
}

func newSQLiteProjectService(root, config string, preferences *desktopConfig) (_ *ProjectService, resultErr error) {
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
	support, err := installDatabaseFamily(root)
	if err != nil {
		return nil, err
	}
	values, err := preferences.snapshot()
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
	service := &ProjectService{root: root, config: config, preferences: preferences, supportPaths: support}
	selection, err := preferences.selection()
	if err != nil {
		return nil, err
	}
	if selection.ProjectPath == "" {
		projects, _, err := service.discover()
		if err != nil {
			return nil, err
		}
		selected := compatibleProject(projects, selection.Project)
		if selected.Name == "" {
			return nil, fmt.Errorf("configured project %q is unavailable; YAML selection was retained", selection.Project)
		}
		selection.ProjectPath = service.projectFile(selected)
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
	candidate, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		return nil, fmt.Errorf("configured context could not be opened; YAML was retained: %w", err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, candidate.Close())
		}
	}()
	service.sqlite = candidate
	if _, err := service.sqliteStateLocked(); err != nil {
		return nil, err
	}
	id, err := newContextIdentity()
	if err != nil {
		return nil, err
	}
	if err := service.persistSQLiteSelection(candidate.selection); err != nil {
		return nil, err
	}
	service.publishSQLiteSelection(candidate, id)
	return service, nil
}

func newContextIdentity() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("initialize project context identity: %w", err)
	}
	return hex.EncodeToString(entropy[:]), nil
}

func (service *ProjectService) persistSQLiteSelection(selection desktopSelection) error {
	current, err := service.preferences.selection()
	if err != nil {
		return err
	}
	if current == selection {
		return nil
	}
	return service.preferences.update("Current", map[string]any{
		"CurrProject": selection.Project, "ProjectPath": selection.ProjectPath,
		"CurrPlotlist": selection.SU, "SUPath": selection.SUPath,
		"CurrHierarchy": selection.Hierarchy, "HierarchyPath": selection.HierarchyPath,
	})
}

func (service *ProjectService) publishSQLiteSelection(candidate *sqliteContext, id string) {
	service.sqlite, service.contextID = candidate, id
	service.active, service.activeSU = candidate.selection.Project, candidate.selection.SU
	service.activeHierarchy, service.hierarchyFile = candidate.selection.Hierarchy, ""
	if candidate.selection.Hierarchy != "None" {
		service.hierarchyFile = filepath.Base(candidate.selection.HierarchyPath)
	}
}

func (service *ProjectService) switchSQLiteLocked(selection desktopSelection) (ProjectState, error) {
	if service.sqlite == nil || service.sqlite.conn == nil {
		return ProjectState{}, errors.New("project context is closed")
	}
	candidate, err := newSQLiteContext(context.Background(), selection, service.supportPaths)
	if err != nil {
		return ProjectState{}, err
	}
	id, err := newContextIdentity()
	if err != nil {
		return ProjectState{}, errors.Join(err, candidate.Close())
	}
	old, oldID := service.sqlite, service.contextID
	service.sqlite, service.contextID = candidate, id
	state, err := service.sqliteStateLocked()
	service.sqlite, service.contextID = old, oldID
	if err != nil {
		return ProjectState{}, errors.Join(err, candidate.Close())
	}
	if err := service.persistSQLiteSelection(candidate.selection); err != nil {
		return ProjectState{}, errors.Join(err, candidate.Close())
	}
	service.publishSQLiteSelection(candidate, id)
	if err := old.Close(); err != nil {
		log.Printf("Warning: new context committed, but previous owned SQLite context could not close: %v", err)
	}
	return state, nil
}

func (service *ProjectService) sqliteStateLocked() (ProjectState, error) {
	if service.sqlite.conn == nil {
		return ProjectState{}, errors.New("project context is closed")
	}
	selection := service.sqlite.selection
	projects, diagnostics, err := service.discover()
	if err != nil {
		return ProjectState{}, err
	}
	for i := range projects {
		projects[i].Path, err = existingDatabasePath(service.projectFile(projects[i]))
		if err != nil {
			return ProjectState{}, err
		}
	}
	found, err := inspectFile(selection.ProjectPath)
	if err != nil {
		return ProjectState{}, err
	}
	for _, project := range found {
		project.Path = selection.ProjectPath
		if !slices.ContainsFunc(projects, func(existing ProjectInfo) bool {
			return existing.Name == project.Name && sameDesktopPath(existing.Path, project.Path)
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
	sus, err := service.discoverSUs(ProjectInfo{Path: selection.ProjectPath})
	if err != nil {
		return ProjectState{}, err
	}
	for i := range sus {
		sus[i].Path = selection.ProjectPath
	}
	if selection.SU != "None" && !sameDesktopPath(selection.SUPath, selection.ProjectPath) {
		external, err := service.discoverSUs(ProjectInfo{Path: selection.SUPath})
		if err != nil {
			return ProjectState{}, err
		}
		for _, su := range external {
			su.Path = selection.SUPath
			if !slices.ContainsFunc(sus, func(existing SUInfo) bool {
				return existing.Name == su.Name && sameDesktopPath(existing.Path, su.Path)
			}) {
				sus = append(sus, su)
			}
		}
	}
	hierarchies, err := discoverHierarchies(filepath.Join(service.root, "projects"))
	if err != nil {
		return ProjectState{}, err
	}
	for i := range hierarchies {
		hierarchies[i].Path, err = existingDatabasePath(filepath.Join(service.root, "projects", hierarchies[i].File))
		if err != nil {
			return ProjectState{}, err
		}
	}
	if selection.Hierarchy != "None" {
		db, err := openReadOnly(selection.HierarchyPath)
		if err != nil {
			return ProjectState{}, err
		}
		external, inspectErr := inspectHierarchies(db, filepath.Base(selection.HierarchyPath))
		inspectErr = errors.Join(inspectErr, db.Close())
		if inspectErr != nil {
			return ProjectState{}, inspectErr
		}
		for _, hierarchy := range external {
			hierarchy.Path = selection.HierarchyPath
			if !slices.ContainsFunc(hierarchies, func(existing HierarchyInfo) bool {
				return existing.Name == hierarchy.Name && sameDesktopPath(existing.Path, hierarchy.Path)
			}) {
				hierarchies = append(hierarchies, hierarchy)
			}
		}
	}
	hierarchyFile := ""
	if selection.Hierarchy != "None" {
		hierarchyFile = filepath.Base(selection.HierarchyPath)
	}
	return ProjectState{ActiveProject: selection.Project, ActiveSU: selection.SU, ActiveHierarchy: selection.Hierarchy,
		HierarchyFile: hierarchyFile, Projects: projects, SUs: sus, Hierarchies: hierarchies,
		Diagnostics: diagnostics, ContextID: service.contextID, ProjectPath: selection.ProjectPath,
		SUPath: selection.SUPath, HierarchyPath: selection.HierarchyPath}, nil
}

func (service *ProjectService) sqlitePlotsLocked(offset, limit int) (PlotPage, error) {
	coordinator := service.sqlite
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if coordinator.conn == nil {
		return PlotPage{}, errors.New("project context is closed")
	}
	ctx := context.Background()
	page := PlotPage{Plots: []PlotSummary{}}
	if err := coordinator.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM USysEnv").Scan(&page.Total); err != nil {
		return PlotPage{}, err
	}
	rows, err := coordinator.conn.QueryContext(ctx, "SELECT PlotNumber,FieldNumber,PlotRepresenting,Zone,SubZone,SiteSeries FROM USysEnv ORDER BY PlotNumber LIMIT ? OFFSET ?", limit, offset)
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

func (service *ProjectService) sqliteHierarchyLocked() ([]HierarchyNode, error) {
	coordinator := service.sqlite
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if coordinator.conn == nil {
		return nil, errors.New("project context is closed")
	}
	nodes := []HierarchyNode{}
	if coordinator.selection.Hierarchy == "None" {
		return nodes, nil
	}
	rows, err := coordinator.conn.QueryContext(context.Background(), "SELECT ID,Name,Parent,Level FROM USysHierarchy ORDER BY Level,ID")
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

func (service *ProjectService) closeSQLiteContext() error {
	service.operationMu.Lock()
	defer service.operationMu.Unlock()
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.sqlite == nil {
		return nil
	}
	return service.sqlite.Close()
}
