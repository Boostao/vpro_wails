package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	contexts *vprocore.ContextService
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSpeciesAttributeSummaryService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*SpeciesAttributeSummaryService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := vprocore.SiviFeature(speciesAttributeSummaryFeatureEnvironment, lookup)
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
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary context service is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return SpeciesAttributeSummaryPreview{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (SpeciesAttributeSummaryPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (SpeciesAttributeSummaryPreview, error) {
			if owner.Selection.SU == "None" || owner.Selection.SU == "USysSuTableDynamic" {
				return SpeciesAttributeSummaryPreview{}, errors.New("species attribute summary requires a selected normal SU; hierarchy/dynamic-break scope is unavailable")
			}
			tables := make([]vprocore.ProjectMetadataTable, 3)
			for i, source := range []struct{ role, table string }{
				{"project", owner.Selection.Project + "_Veg"},
				{"su", owner.Selection.SU + "_SU"},
				{"VLists", "USysSppAttributes"},
			} {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
					`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
					return SpeciesAttributeSummaryPreview{}, err
				}
				if count != 1 {
					return SpeciesAttributeSummaryPreview{}, fmt.Errorf("species attribute summary requires original physical table %s.%s", source.role, source.table)
				}
				var err error
				tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
				if err != nil {
					return SpeciesAttributeSummaryPreview{}, err
				}
			}
			report, err := planSpeciesAttributeSummary(ctx, owner.Selection.Project, owner.Selection.SU, tables[0], tables[1], tables[2])
			if err != nil {
				return SpeciesAttributeSummaryPreview{}, err
			}
			return SpeciesAttributeSummaryPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, report}, nil
		})
	})
}
