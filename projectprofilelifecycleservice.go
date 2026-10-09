package main

import "context"

func (s *ContextService) CreateProjectPlotProfileRule(ctx context.Context, contextID string, request ProjectPlotProfileCreation) (ProjectMetadataRow, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectMetadataRow, error) {
		if err := plots.requireContextEdit(); err != nil {
			return ProjectMetadataRow{}, err
		}
		return plots.projects.sqlite.createProfileRule(ctx, request, plots.currentUser)
	})
}

func (s *ContextService) DeleteProjectPlotProfileRule(ctx context.Context, contextID string, request ProjectPlotProfileDeletion) error {
	_, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		if err := plots.requireContextEdit(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, plots.projects.sqlite.deleteProfileRule(ctx, request, plots.currentUser)
	})
	return err
}
