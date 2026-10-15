package main

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
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationProjectFieldDefinitions, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationProjectFieldDefinitions, error) {
			var tables [2]ProjectMetadataTable
			for i, suffix := range []string{"_Env", "_Admin"} {
				name := owner.selection.Project + suffix
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master
					WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
					return nil, err
				}
				if count != 1 {
					return nil, fmt.Errorf("project report field catalogue requires original physical table project.%s", name)
				}
				columns, err := readSQLiteStorageColumns(ctx, tx, "project", name)
				if err != nil {
					return nil, err
				}
				tables[i] = ProjectMetadataTable{Columns: columns}
			}
			entry := s.contexts.projectReportScopes.read(contextID)
			return &LongVegetationProjectFieldDefinitions{
				ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, SourceKind: entry.sourceKind,
				Revision: strconv.FormatInt(entry.revision, 10), Definitions: reportUnitFieldDefinitions(tables[0], tables[1]),
			}, nil
		})
	})
}
