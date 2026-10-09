package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

type twoPageVegetationSpeciesReferences struct {
	ContextID string               `json:"contextId"`
	Project   string               `json:"project"`
	Plot      string               `json:"plot"`
	Form      string               `json:"form"`
	Source    string               `json:"source"`
	Master    ProjectMetadataTable `json:"master"`
	Personal  ProjectMetadataTable `json:"personal"`
}

func (s *ContextService) readTwoPageVegetationSpeciesReferences(ctx context.Context, contextID, plot, form string) (*twoPageVegetationSpeciesReferences, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page vegetation Species references require an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page vegetation Species references require an exact valid literal parent")
	}
	if _, _, err := twoPageVegetationSource(form); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageVegetationSpeciesReferences, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageVegetationSpeciesReferences, error) {
			project := owner.selection.Project
			if err := requireTwoPageChildReadScope(ctx, tx, project, project+"_Veg", plot, "vegetation"); err != nil {
				return nil, fmt.Errorf("two-page vegetation Species reference scope: %w", err)
			}
			for _, source := range []struct{ alias, table string }{{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"}} {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.alias)+
					`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
					return nil, fmt.Errorf("two-page vegetation Species physical schema: %w", err)
				}
				if count != 1 {
					return nil, fmt.Errorf("two-page vegetation Species requires original physical table %s.%s", source.alias, source.table)
				}
				if err := validateSIVIPhysicalSchema(ctx, tx, source.alias, source.table, "vegetation Species definitions"); err != nil {
					return nil, fmt.Errorf("two-page vegetation Species physical schema: %w", err)
				}
			}
			references, err := readPhysicalSpeciesReferences(ctx, tx, contextID, project, plot, "VLists", "VUser",
				projectTwoPageVegetationSpeciesDefinitions)
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation Species physical definitions: %w", err)
			}
			return &twoPageVegetationSpeciesReferences{
				ContextID: contextID, Project: project, Plot: plot, Form: form,
				Source: "configured-owned-imported-SQLite VLists.USysAllSpecs/VUser.USysUserSpp; physical definitions, not source union membership",
				Master: references.Master, Personal: references.Personal,
			}, nil
		})
	})
}

func projectTwoPageVegetationSpeciesDefinitions(ctx context.Context, source ProjectMetadataTable, personal bool) (ProjectMetadataTable, error) {
	if personal {
		return projectSIVISpeciesDefinitions(ctx, source, true)
	}
	return projectPhysicalSpeciesDefinitions(ctx, source, false, func(cell ProjectMetadataCell) error {
		if _, err := metadataCellValue(cell); err != nil {
			return err
		}
		if cell.Storage == "null" {
			return nil
		}
		if cell.Storage != "integer" {
			return errors.New("master Species Lifeform requires exact nullable source LONG integer storage; no coercion")
		}
		if _, err := strconv.ParseInt(*cell.Integer, 10, 32); err != nil {
			return errors.New("master Species Lifeform exceeds source signed32 LONG range")
		}
		return nil
	})
}
