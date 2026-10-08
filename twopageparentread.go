package main

import (
	"context"
	"database/sql"
)

func (s *ContextService) readTwoPageParent(ctx context.Context, contextID, plot, form string) (*siviParentProjection, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviParentProjection, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviParentProjection, error) {
			tables, err := readSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			return projectTwoPageParent(ctx, contextID, owner.selection.Project, plot, form, tables[0], tables[1])
		})
	})
}
