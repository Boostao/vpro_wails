package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
)

type ProjectPlotProfileChoices struct {
	EnvFields []string                       `json:"envFields"`
	Species   []vprocore.ProjectMetadataCell `json:"species"`
}

func ListProjectPlotProfileChoices(s *vprocore.ContextService, ctx context.Context, contextID string) (ProjectPlotProfileChoices, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (result ProjectPlotProfileChoices, resultErr error) {
		c := plots.Projects.Sqlite
		if err := vprocore.ProfileOwnedFiles(c); err != nil {
			return result, err
		}
		columns, err := vprocore.ReadSQLiteStorageColumns(ctx, c.Conn, "project", c.Selection.Project+"_Env")
		if err != nil {
			return result, fmt.Errorf("selected project Env field suggestions unavailable: %w", err)
		}
		result.EnvFields = []string{}
		for _, column := range columns {
			result.EnvFields = append(result.EnvFields, column.Name)
		}
		rows, err := c.Conn.QueryContext(ctx, `SELECT typeof(Code),CAST(Code AS BLOB)
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
		result.Species = []vprocore.ProjectMetadataCell{}
		for rows.Next() {
			var storage string
			var raw []byte
			if err := rows.Scan(&storage, &raw); err != nil {
				return result, err
			}
			cell, err := vprocore.ProjectMetadataCellInternal(storage, raw)
			if err != nil {
				return result, err
			}
			result.Species = append(result.Species, cell)
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
		return result, vprocore.ProfileOwnedFiles(c)
	})
}
