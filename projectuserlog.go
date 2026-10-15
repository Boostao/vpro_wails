package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const userLogMaxRows = 10000
const userLogMaxPageSize = 200

type UserLogReview struct {
	ContextID  string               `json:"contextId"`
	SourcePath string               `json:"sourcePath"`
	Mode       int                  `json:"mode"`
	Offset     int                  `json:"offset"`
	Limit      int                  `json:"limit"`
	Total      int                  `json:"total"`
	Table      ProjectMetadataTable `json:"table"`
}

func (s *ContextService) reviewUserLog(ctx context.Context, contextID string, mode, offset, limit int) (UserLogReview, error) {
	if ctx == nil {
		return UserLogReview{}, errors.New("user log review requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return UserLogReview{}, err
	}
	if s == nil || s.projects == nil || s.plots == nil {
		return UserLogReview{}, errors.New("user log context service is unavailable")
	}
	if mode != 1 && mode != 2 {
		return UserLogReview{}, errors.New("user log mode must be Show All (1) or Show Current (2)")
	}
	if offset < 0 || limit < 1 || limit > userLogMaxPageSize {
		return UserLogReview{}, errors.New("user log page requires a nonnegative offset and limit from 1 to 200")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (UserLogReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{}, func(owner *sqliteContext, tx *sql.Tx) (UserLogReview, error) {
			table, err := readUserLog(ctx, tx, mode)
			if err != nil {
				return UserLogReview{}, err
			}
			total := len(table.Rows)
			if offset < total {
				end := total
				if limit < total-offset {
					end = offset + limit
				}
				table.Rows = table.Rows[offset:end]
			} else {
				table.Rows = []ProjectMetadataRow{}
			}
			return UserLogReview{
				ContextID: contextID, SourcePath: owner.attachments["VPro64"], Mode: mode,
				Offset: offset, Limit: limit, Total: total, Table: table,
			}, nil
		})
	})
}

func readUserLog(ctx context.Context, tx *sql.Tx, mode int) (ProjectMetadataTable, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM "VPro64".sqlite_master WHERE name='USysUserLog' COLLATE BINARY`).Scan(&kind); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("user log physical table unavailable: %w", err)
	}
	if kind != "table" {
		return ProjectMetadataTable{}, errors.New("user log requires its original physical table, not a view")
	}
	columns, err := readSQLiteStorageColumns(ctx, tx, "VPro64", "USysUserLog")
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("user log schema unavailable: %w", err)
	}
	required := map[string]bool{"User": false, "InTime": false, "OutTime": false, "LocalMachine": false}
	if len(columns) != len(required) {
		return ProjectMetadataTable{}, errors.New("user log requires exactly its four original physical columns")
	}
	for _, column := range columns {
		if _, exists := required[column.Name]; !exists {
			return ProjectMetadataTable{}, fmt.Errorf("user log has an unsupported physical column %q", column.Name)
		}
		required[column.Name] = true
	}
	for name, present := range required {
		if !present {
			return ProjectMetadataTable{}, fmt.Errorf("user log physical column %q is unavailable", name)
		}
	}
	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "VPro64"."USysUserLog"`).Scan(&total); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("user log count unavailable: %w", err)
	}
	if total > userLogMaxRows {
		return ProjectMetadataTable{}, fmt.Errorf("user log exceeds the %d-row review bound; no rows were truncated", userLogMaxRows)
	}
	filter := ""
	if mode == 2 {
		filter = "OutTime"
	}
	table, err := readSQLiteStorageRows(ctx, tx, "VPro64", "USysUserLog", filter, nil, "")
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("user log rows unavailable: %w", err)
	}
	return table, nil
}
