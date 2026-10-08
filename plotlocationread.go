package main

import (
	"context"
	"database/sql"
	"fmt"
)

type PlotLocationReview struct {
	ContextID   string             `json:"contextId"`
	ProjectPath string             `json:"projectPath"`
	SUPath      string             `json:"suPath"`
	Report      PlotLocationReport `json:"report"`
}

func readPhysicalLocationTable(ctx context.Context, tx *sql.Tx, alias, name string) (ProjectMetadataTable, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(alias)+
		`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
		return ProjectMetadataTable{}, err
	}
	if count != 1 {
		return ProjectMetadataTable{}, fmt.Errorf("plot locations require original physical table %s.%s", alias, name)
	}
	return readSQLiteStorageRows(ctx, tx, alias, name, "", nil, "")
}

func (s *ContextService) readPlotLocations(ctx context.Context, contextID string) (*PlotLocationReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*PlotLocationReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*PlotLocationReview, error) {
			sources := []struct{ alias, table string }{
				{"project", owner.selection.Project + "_Env"},
				{"project", owner.selection.Project + "_Admin"},
			}
			if owner.selection.SU != "None" {
				sources = append(sources, struct{ alias, table string }{"su", owner.selection.SU + "_SU"})
			}
			tables := []ProjectMetadataTable{}
			for _, source := range sources {
				table, err := readPhysicalLocationTable(ctx, tx, source.alias, source.table)
				if err != nil {
					return nil, err
				}
				tables = append(tables, table)
			}
			selected := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
			if len(tables) == 3 {
				selected = tables[2]
			}
			report, err := planPlotLocations(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], selected)
			if err != nil {
				return nil, err
			}
			return &PlotLocationReview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, *report}, nil
		})
	})
}
