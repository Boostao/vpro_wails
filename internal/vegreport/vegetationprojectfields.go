package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

type LongVegetationProjectFieldDefinitions struct {
	ContextID   string                            `json:"contextId"`
	Project     string                            `json:"project"`
	ProjectPath string                            `json:"projectPath"`
	SU          string                            `json:"su"`
	SUPath      string                            `json:"suPath"`
	SourceKind  string                            `json:"sourceKind"`
	Revision    string                            `json:"revision"`
	Definitions []LongVegetationSelectedUnitField `json:"definitions"`
}

func (s *LongVegetationReportScopeService) GetLongVegetationProjectFieldDefinitions(ctx context.Context,
	contextID string) (*LongVegetationProjectFieldDefinitions, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationProjectFieldDefinitions, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationProjectFieldDefinitions, error) {
			var tables [2]vprocore.ProjectMetadataTable
			for i, suffix := range []string{"_Env", "_Admin"} {
				name := owner.Selection.Project + suffix
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master
					WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
					return nil, err
				}
				if count != 1 {
					return nil, fmt.Errorf("project report field catalogue requires original physical table project.%s", name)
				}
				columns, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, "project", name)
				if err != nil {
					return nil, err
				}
				tables[i] = vprocore.ProjectMetadataTable{Columns: columns}
			}
			entry := s.contexts.ProjectReportScopes.Read(contextID)
			return &LongVegetationProjectFieldDefinitions{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, SourceKind: entry.SourceKind,
				Revision: strconv.FormatInt(entry.Revision, 10), Definitions: reportUnitFieldDefinitions(tables[0], tables[1]),
			}, nil
		})
	})
}
