package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "context"

func CreateProjectPlotProfileRule(s *vprocore.ContextService, ctx context.Context, contextID string, request ProjectPlotProfileCreation) (vprocore.ProjectMetadataRow, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (vprocore.ProjectMetadataRow, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return vprocore.ProjectMetadataRow{}, err
		}
		return CreateProfileRule(plots.Projects.Sqlite, ctx, request, plots.CurrentUser)
	})
}

func DeleteProjectPlotProfileRule(s *vprocore.ContextService, ctx context.Context, contextID string, request ProjectPlotProfileDeletion) error {
	_, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, DeleteProfileRule(plots.Projects.Sqlite, ctx, request, plots.CurrentUser)
	})
	return err
}
