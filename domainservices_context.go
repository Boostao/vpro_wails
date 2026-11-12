package main

import (
	"context"
	"errors"
)

func (s *ContextService) GetState(ctx context.Context) (ProjectState, error) {
	if s == nil || s.projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.projects.GetState(ctx)
}

func (s *ContextService) ListPlots(ctx context.Context, offset, limit int) (PlotPage, error) {
	if s == nil || s.projects == nil {
		return PlotPage{}, errors.New("project context is unavailable")
	}
	return s.projects.ListPlots(ctx, offset, limit)
}

func (s *ContextService) GetHierarchyNodes(ctx context.Context) ([]HierarchyNode, error) {
	if s == nil || s.projects == nil {
		return nil, errors.New("project context is unavailable")
	}
	return s.projects.GetHierarchyNodes(ctx)
}

func (s *ContextService) SelectProject(name string) (ProjectState, error) {
	if s == nil || s.projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.projects.SelectProject(name)
}

func (s *ContextService) SelectSU(name string) (ProjectState, error) {
	if s == nil || s.projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.projects.SelectSU(name)
}

func (s *ContextService) SelectHierarchy(name, file string) (ProjectState, error) {
	if s == nil || s.projects == nil {
		return ProjectState{}, errors.New("project context is unavailable")
	}
	return s.projects.SelectHierarchy(name, file)
}
