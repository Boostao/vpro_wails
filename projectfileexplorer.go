package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ExternalSQLiteFileInspection struct {
	SelectedPath string        `json:"selectedPath"`
	Projects     []ProjectInfo `json:"projects"`
	SUs          []SUInfo      `json:"sus"`
}

func chooseNativeOpenFileWithFilters(ctx context.Context, title string, filters []nativeFileFilter) (string, error) {
	app, window, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	handle := window.NativeWindow()
	if handle == nil {
		return "", errors.New("native file selection window closed before opening its dialog")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selected, err := promptNativeOpenFileWithTitle(app, window, handle, title, filters)
	if err != nil {
		return "", fmt.Errorf("native file selection failed: %w", err)
	}
	currentApp, current, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	if currentApp != app || current != window || current.NativeWindow() != handle {
		return "", errors.New("native file selection owner changed while the dialog was open")
	}
	return selected, nil
}

func chooseNativeSaveFileWithFilters(ctx context.Context, title, defaultFileName, defaultExt string, filters []nativeFileFilter) (string, error) {
	app, window, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	handle := window.NativeWindow()
	if handle == nil {
		return "", errors.New("native file save window closed before opening its dialog")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selected, err := promptNativeSaveFileWithTitle(app, window, handle, title, defaultFileName, defaultExt, filters)
	if err != nil {
		return "", fmt.Errorf("native file save selection failed: %w", err)
	}
	currentApp, current, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	if currentApp != app || current != window || current.NativeWindow() != handle {
		return "", errors.New("native file save owner changed while the dialog was open")
	}
	return selected, nil
}

func (s *ContextService) InspectExternalSQLiteFile(ctx context.Context, contextID, path string) (ExternalSQLiteFileInspection, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ExternalSQLiteFileInspection, error) {
		resolved, err := existingDatabasePath(path)
		if err != nil {
			return ExternalSQLiteFileInspection{}, err
		}
		projects, err := inspectFileContext(ctx, resolved)
		if err != nil {
			return ExternalSQLiteFileInspection{}, err
		}
		for i := range projects {
			projects[i].Path = resolved
		}
		sus, err := plots.projects.discoverSUsContext(ctx, ProjectInfo{Path: resolved})
		if err != nil {
			return ExternalSQLiteFileInspection{}, err
		}
		for i := range sus {
			sus[i].Path = resolved
		}
		return ExternalSQLiteFileInspection{
			SelectedPath: resolved,
			Projects:     projects,
			SUs:          sus,
		}, nil
	})
}

func (s *ContextService) ChooseExternalSQLiteFile(ctx context.Context, contextID string) (ExternalSQLiteFileInspection, error) {
	if _, err := withContextPlotRequest(ctx, s, contextID, func(*PlotService) (struct{}, error) {
		return struct{}{}, nil
	}); err != nil {
		return ExternalSQLiteFileInspection{}, err
	}
	selected, err := chooseNativeOpenFileWithFilters(ctx, "Select VPRO SQLite Database (.db)", []nativeFileFilter{
		{DisplayName: "VPRO SQLite Database (*.db;*.sqlite;*.sqlite3)", Pattern: "*.db;*.sqlite;*.sqlite3"},
		{DisplayName: "All Files (*.*)", Pattern: "*.*"},
	})
	if err != nil {
		return ExternalSQLiteFileInspection{}, err
	}
	if selected == "" {
		return ExternalSQLiteFileInspection{Projects: []ProjectInfo{}, SUs: []SUInfo{}}, nil
	}
	return s.InspectExternalSQLiteFile(ctx, contextID, selected)
}

func (s *ContextService) ChooseAccessDatabaseFile(ctx context.Context, contextID string) (string, error) {
	if _, err := withContextPlotRequest(ctx, s, contextID, func(*PlotService) (struct{}, error) {
		return struct{}{}, nil
	}); err != nil {
		return "", err
	}
	return chooseNativeOpenFileWithFilters(ctx, "Select VPRO Access Database (.mdb, .accdb)", []nativeFileFilter{
		{DisplayName: "Microsoft Access Database (*.mdb;*.accdb)", Pattern: "*.mdb;*.accdb"},
		{DisplayName: "All Files (*.*)", Pattern: "*.*"},
	})
}

func (s *ContextService) ChooseNewSQLiteProjectFile(ctx context.Context, contextID, suggestedProject string) (string, error) {
	var projectsDir string
	if _, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		projectsDir = filepath.Join(plots.projects.root, "projects")
		return struct{}{}, nil
	}); err != nil {
		return "", err
	}
	defaultName := "ImportedProject.db"
	trimmed := strings.TrimSpace(suggestedProject)
	if projectNamePattern.MatchString(trimmed) {
		defaultName = trimmed + ".db"
	}
	_ = projectsDir
	return chooseNativeSaveFileWithFilters(ctx, "Choose New VPRO SQLite Database File (.db)", defaultName, "db", []nativeFileFilter{
		{DisplayName: "VPRO SQLite Database (*.db)", Pattern: "*.db"},
	})
}

func (s *ContextService) RevealInFileExplorer(ctx context.Context, contextID, target string) (string, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (string, error) {
		owner := plots.projects.sqlite
		var rawPath string
		var isDir bool
		switch strings.TrimSpace(target) {
		case "", "root":
			rawPath = plots.projects.root
			isDir = true
		case "projects":
			rawPath = filepath.Join(plots.projects.root, "projects")
			isDir = true
		case "project":
			rawPath = owner.selection.ProjectPath
		case "su":
			if owner.selection.SU == "None" || owner.selection.SUPath == "" {
				return "", errors.New("no Site Unit database is currently active")
			}
			rawPath = owner.selection.SUPath
		case "hierarchy":
			if owner.selection.Hierarchy == "None" || owner.selection.HierarchyPath == "" {
				return "", errors.New("no Hierarchy database is currently active")
			}
			rawPath = owner.selection.HierarchyPath
		default:
			return "", fmt.Errorf("unknown file explorer target %q", target)
		}
		if rawPath == "" {
			return "", errors.New("requested database path is empty")
		}
		resolved, err := filepath.EvalSymlinks(filepath.Clean(rawPath))
		if err != nil {
			return "", fmt.Errorf("resolve path %q: %w", rawPath, err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return "", fmt.Errorf("stat path %q: %w", resolved, err)
		}
		if isDir && !info.IsDir() {
			return "", fmt.Errorf("expected directory at %s", resolved)
		}
		if !isDir && !info.Mode().IsRegular() {
			return "", fmt.Errorf("expected regular file at %s", resolved)
		}
		if err := openPathInSystemFileExplorer(resolved, isDir); err != nil {
			return "", fmt.Errorf("launch file explorer for %s: %w", resolved, err)
		}
		return resolved, nil
	})
}
