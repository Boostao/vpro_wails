package main

import (
	"context"
	"errors"
	"fmt"
)

type ProjectPlotProfileChoices struct {
	EnvFields []string              `json:"envFields"`
	Species   []ProjectMetadataCell `json:"species"`
}

func (s *ContextService) ListProjectPlotProfileChoices(ctx context.Context, contextID string) (ProjectPlotProfileChoices, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result ProjectPlotProfileChoices, resultErr error) {
		c := plots.projects.sqlite
		if err := profileOwnedFiles(c); err != nil {
			return result, err
		}
		columns, err := readSQLiteStorageColumns(ctx, c.conn, "project", c.selection.Project+"_Env")
		if err != nil {
			return result, fmt.Errorf("selected project Env field suggestions unavailable: %w", err)
		}
		result.EnvFields = []string{}
		for _, column := range columns {
			result.EnvFields = append(result.EnvFields, column.Name)
		}
		rows, err := c.conn.QueryContext(ctx, `SELECT typeof(Code),CAST(Code AS BLOB)
			FROM "VLists"."USysAllSpecs" GROUP BY Code ORDER BY Code COLLATE BINARY`)
		if err != nil {
			return result, fmt.Errorf("source profile species suggestions unavailable: %w", err)
		}
		defer func() {
			resultErr = errors.Join(resultErr, rows.Close())
			if resultErr != nil {
				result = ProjectPlotProfileChoices{}
			}
		}()
		result.Species = []ProjectMetadataCell{}
		for rows.Next() {
			var storage string
			var raw []byte
			if err := rows.Scan(&storage, &raw); err != nil {
				return result, err
			}
			cell, err := projectMetadataCell(storage, raw)
			if err != nil {
				return result, err
			}
			result.Species = append(result.Species, cell)
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
		return result, profileOwnedFiles(c)
	})
}
