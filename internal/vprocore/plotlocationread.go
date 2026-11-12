package vprocore

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

func ReadPhysicalLocationTable(ctx context.Context, tx *sql.Tx, alias, name string) (ProjectMetadataTable, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+QuoteHeaderIdentifier(alias)+
		`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
		return ProjectMetadataTable{}, err
	}
	if count != 1 {
		return ProjectMetadataTable{}, fmt.Errorf("plot locations require original physical table %s.%s", alias, name)
	}
	return ReadSQLiteStorageRows(ctx, tx, alias, name, "", nil, "")
}

func (s *ContextService) ReadPlotLocations(ctx context.Context, contextID string) (*PlotLocationReview, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*PlotLocationReview, error) {
		return WithOwnedSIVISnapshot(ctx, plots, func(owner *SqliteContext, tx *sql.Tx) (*PlotLocationReview, error) {
			sources := []struct{ alias, table string }{
				{"project", owner.Selection.Project + "_Env"},
				{"project", owner.Selection.Project + "_Admin"},
			}
			if owner.Selection.SU != "None" {
				sources = append(sources, struct{ alias, table string }{"su", owner.Selection.SU + "_SU"})
			}
			tables := []ProjectMetadataTable{}
			for _, source := range sources {
				table, err := ReadPhysicalLocationTable(ctx, tx, source.alias, source.table)
				if err != nil {
					return nil, err
				}
				tables = append(tables, table)
			}
			selected := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
			if len(tables) == 3 {
				selected = tables[2]
			}
			report, err := PlanPlotLocations(ctx, owner.Selection.Project, owner.Selection.SU, tables[0], tables[1], selected)
			if err != nil {
				return nil, err
			}
			return &PlotLocationReview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, *report}, nil
		})
	})
}
