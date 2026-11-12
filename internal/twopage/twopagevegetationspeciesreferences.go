package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

const twoPageVegetationSpeciesReferenceSource = "configured-owned-imported-SQLite VLists.USysAllSpecs/VUser.USysUserSpp; physical definitions, not source union membership"

type twoPageVegetationSpeciesReferences struct {
	ContextID string                        `json:"contextId"`
	Project   string                        `json:"project"`
	Plot      string                        `json:"plot"`
	Form      string                        `json:"form"`
	Source    string                        `json:"source"`
	Master    vprocore.ProjectMetadataTable `json:"master"`
	Personal  vprocore.ProjectMetadataTable `json:"personal"`
}

func ReadTwoPageVegetationSpeciesReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string) (*twoPageVegetationSpeciesReferences, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page vegetation Species references require an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page vegetation Species references require an exact valid literal parent")
	}
	if _, _, err := twoPageVegetationSource(form); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*twoPageVegetationSpeciesReferences, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*twoPageVegetationSpeciesReferences, error) {
			project := owner.Selection.Project
			if err := sourceform.RequireTwoPageChildReadScope(ctx, tx, project, project+"_Veg", plot, "vegetation"); err != nil {
				return nil, fmt.Errorf("two-page vegetation Species reference scope: %w", err)
			}
			for _, source := range []struct{ alias, table string }{{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"}} {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.alias)+
					`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
					return nil, fmt.Errorf("two-page vegetation Species physical schema: %w", err)
				}
				if count != 1 {
					return nil, fmt.Errorf("two-page vegetation Species requires original physical table %s.%s", source.alias, source.table)
				}
				if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, source.alias, source.table, "vegetation Species definitions"); err != nil {
					return nil, fmt.Errorf("two-page vegetation Species physical schema: %w", err)
				}
			}
			references, err := sourceform.ReadPhysicalSpeciesReferences(ctx, tx, contextID, project, plot, "VLists", "VUser",
				projectTwoPageVegetationSpeciesDefinitions)
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation Species physical definitions: %w", err)
			}
			return &twoPageVegetationSpeciesReferences{
				ContextID: contextID, Project: project, Plot: plot, Form: form,
				Source: twoPageVegetationSpeciesReferenceSource,
				Master: references.Master, Personal: references.Personal,
			}, nil
		})
	})
}

func projectTwoPageVegetationSpeciesDefinitions(ctx context.Context, source vprocore.ProjectMetadataTable, personal bool) (vprocore.ProjectMetadataTable, error) {
	if personal {
		return sourceform.ProjectSIVISpeciesDefinitions(ctx, source, true)
	}
	return sourceform.ProjectPhysicalSpeciesDefinitions(ctx, source, false, func(cell vprocore.ProjectMetadataCell) error {
		if _, err := vprocore.MetadataCellValue(cell); err != nil {
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
