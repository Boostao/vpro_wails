package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

const speciesAttributeSummaryFeatureEnvironment = "VPRO_SPECIES_ATTRIBUTE_SUMMARY"

type SpeciesAttributeSummaryPreview struct {
	ContextID   string                        `json:"contextId"`
	ProjectPath string                        `json:"projectPath"`
	SUPath      string                        `json:"suPath"`
	Report      SpeciesAttributeSummaryReport `json:"report"`
}

type SpeciesAttributeSummaryService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewSpeciesAttributeSummaryService(contexts *ContextService, lookup func(string) (string, bool)) (*SpeciesAttributeSummaryService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(speciesAttributeSummaryFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SpeciesAttributeSummaryService{contexts: contexts, enabled: enabled}, nil
}

func (s *SpeciesAttributeSummaryService) Preview(ctx context.Context, contextID string) (SpeciesAttributeSummaryPreview, error) {
	if ctx == nil {
		return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SpeciesAttributeSummaryPreview{}, err
	}
	if s == nil || !s.enabled {
		return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary context service is unavailable")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return SpeciesAttributeSummaryPreview{}, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (SpeciesAttributeSummaryPreview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (SpeciesAttributeSummaryPreview, error) {
			if owner.selection.SU == "None" || owner.selection.SU == "USysSuTableDynamic" {
				return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary requires a selected normal SU; hierarchy/dynamic-break scope is unavailable")
			}
			tables := make([]ProjectMetadataTable, 3)
			for i, source := range []struct{ role, table string }{
				{"project", owner.selection.Project + "_Veg"},
				{"su", owner.selection.SU + "_SU"},
				{"VLists", "USysSppAttributes"},
			} {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
					`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
					return SpeciesAttributeSummaryPreview{}, err
				}
				if count != 1 {
					return SpeciesAttributeSummaryPreview{}, fmt.Errorf("species attribute summary requires original physical table %s.%s", source.role, source.table)
				}
				var err error
				tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
				if err != nil {
					return SpeciesAttributeSummaryPreview{}, err
				}
			}
			report, err := planSpeciesAttributeSummary(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], tables[2])
			if err != nil {
				return SpeciesAttributeSummaryPreview{}, err
			}
			return SpeciesAttributeSummaryPreview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, report}, nil
		})
	})
}
