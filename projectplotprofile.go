package main

import (
	"context"
	"fmt"
)

type ProjectPlotProfileReview struct {
	Project      string               `json:"project"`
	Table        string               `json:"table"`
	Rules        ProjectMetadataTable `json:"rules"`
	Descriptions ProjectMetadataTable `json:"descriptions"`
}

func (s *ContextService) ReviewProjectPlotProfile(ctx context.Context, contextID string) (ProjectPlotProfileReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectPlotProfileReview, error) {
		c := plots.projects.sqlite
		table := c.selection.Project + "_Profile"
		if err := c.requireColumns(ctx, "project", table, []string{
			"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount",
		}); err != nil {
			return ProjectPlotProfileReview{}, fmt.Errorf("project-local plot profile schema unavailable: %w", err)
		}
		rules, err := readSQLiteStorageRows(ctx, c.conn, "project", table, "", nil, "Order")
		if err != nil {
			return ProjectPlotProfileReview{}, fmt.Errorf("project-local plot profile rules unavailable: %w", err)
		}
		descriptions, err := readSQLiteStorageRows(ctx, c.conn, "project", "_table_metadata", "table_name", &table, "")
		if err != nil {
			return ProjectPlotProfileReview{}, fmt.Errorf("plot profile table-object descriptions unavailable: %w", err)
		}
		return ProjectPlotProfileReview{c.selection.Project, table, rules, descriptions}, nil
	})
}
