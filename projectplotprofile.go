package main

import (
	"context"
	"fmt"
)

type ProjectPlotProfileReview struct {
	Project      string                `json:"project"`
	Table        string                `json:"table"`
	Rules        ProjectMetadataTable  `json:"rules"`
	Descriptions ProjectMetadataTable  `json:"descriptions"`
	Source       PlotProfileSourceInfo `json:"source"`
}

func (s *ContextService) ReviewProjectPlotProfile(ctx context.Context, contextID string) (ProjectPlotProfileReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectPlotProfileReview, error) {
		c := plots.projects.sqlite
		role, table, err := c.profileLocation()
		if err != nil {
			return ProjectPlotProfileReview{}, err
		}
		if err := profileOwnedFiles(c); err != nil {
			return ProjectPlotProfileReview{}, err
		}
		if err := c.requireColumns(ctx, role, table, []string{
			"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount",
		}); err != nil {
			return ProjectPlotProfileReview{}, fmt.Errorf("project-local plot profile schema unavailable: %w", err)
		}
		rules, err := readSQLiteStorageRows(ctx, c.conn, role, table, "", nil, "Order")
		if err != nil {
			return ProjectPlotProfileReview{}, fmt.Errorf("project-local plot profile rules unavailable: %w", err)
		}
		descriptions, err := readSQLiteStorageRows(ctx, c.conn, role, "_table_metadata", "table_name", &table, "")
		if err != nil {
			return ProjectPlotProfileReview{}, fmt.Errorf("plot profile table-object descriptions unavailable: %w", err)
		}
		info, err := c.profileInfo(ctx)
		if err != nil {
			return ProjectPlotProfileReview{}, err
		}
		return ProjectPlotProfileReview{c.selection.Project, table, rules, descriptions, info}, profileOwnedFiles(c)
	})
}
