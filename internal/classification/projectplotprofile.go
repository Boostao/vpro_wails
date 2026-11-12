package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"fmt"
)

type ProjectPlotProfileReview struct {
	Project      string                         `json:"project"`
	Table        string                         `json:"table"`
	Rules        vprocore.ProjectMetadataTable  `json:"rules"`
	Descriptions vprocore.ProjectMetadataTable  `json:"descriptions"`
	Source       vprocore.PlotProfileSourceInfo `json:"source"`
}

func ReviewProjectPlotProfile(s *vprocore.ContextService, ctx context.Context, contextID string) (ProjectPlotProfileReview, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (ProjectPlotProfileReview, error) {
		return readProjectPlotProfile(ctx, plots.Projects.Sqlite)
	})
}

func readProjectPlotProfile(ctx context.Context, c *vprocore.SqliteContext) (ProjectPlotProfileReview, error) {
	role, table, err := c.ProfileLocation()
	if err != nil {
		return ProjectPlotProfileReview{}, err
	}
	if err := vprocore.ProfileOwnedFiles(c); err != nil {
		return ProjectPlotProfileReview{}, err
	}
	if err := c.RequireColumns(ctx, role, table, []string{
		"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount",
	}); err != nil {
		return ProjectPlotProfileReview{}, fmt.Errorf("project-local plot profile schema unavailable: %w", err)
	}
	rules, err := vprocore.ReadSQLiteStorageRows(ctx, c.Conn, role, table, "", nil, "Order")
	if err != nil {
		return ProjectPlotProfileReview{}, fmt.Errorf("project-local plot profile rules unavailable: %w", err)
	}
	descriptions, err := vprocore.ReadProfileDescriptions(ctx, c.Conn, role, table)
	if err != nil {
		return ProjectPlotProfileReview{}, fmt.Errorf("plot profile table-object descriptions unavailable: %w", err)
	}
	info, err := c.ProfileInfo(ctx)
	if err != nil {
		return ProjectPlotProfileReview{}, err
	}
	return ProjectPlotProfileReview{c.Selection.Project, table, rules, descriptions, info}, vprocore.ProfileOwnedFiles(c)
}
