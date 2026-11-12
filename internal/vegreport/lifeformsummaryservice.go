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

const lifeformSummaryFeatureEnvironment = "VPRO_LIFEFORM_SUMMARY"

type LifeformSummaryPreview struct {
	ContextID   string                `json:"contextId"`
	ProjectPath string                `json:"projectPath"`
	SUPath      string                `json:"suPath"`
	Report      LifeformSummaryReport `json:"report"`
}

type LifeformSummaryService struct {
	contexts *vprocore.ContextService
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLifeformSummaryService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*LifeformSummaryService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := vprocore.SiviFeature(lifeformSummaryFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &LifeformSummaryService{contexts: contexts, enabled: enabled}, nil
}

func (s *LifeformSummaryService) Preview(ctx context.Context, contextID string) (LifeformSummaryPreview, error) {
	if ctx == nil {
		return LifeformSummaryPreview{}, errors.New("Lifeform Summary requires a context")
	}
	if err := ctx.Err(); err != nil {
		return LifeformSummaryPreview{}, err
	}
	if s == nil || !s.enabled {
		return LifeformSummaryPreview{}, errors.New("Lifeform Summary is disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return LifeformSummaryPreview{}, errors.New("Lifeform Summary context service is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return LifeformSummaryPreview{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (LifeformSummaryPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (LifeformSummaryPreview, error) {
			if owner.Selection.SU == "None" || owner.Selection.SU == "USysSuTableDynamic" {
				return LifeformSummaryPreview{}, errors.New("Lifeform Summary requires a selected normal SU; hierarchy/dynamic-break scope is unavailable")
			}
			tables := make([]vprocore.ProjectMetadataTable, 4)
			for i, source := range []struct{ role, table string }{
				{"project", owner.Selection.Project + "_Veg"},
				{"su", owner.Selection.SU + "_SU"},
				{"VLists", "USysAllSpecs"},
				{"VPro64", "LifeformCodes"},
			} {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
					`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
					return LifeformSummaryPreview{}, err
				}
				if count != 1 {
					return LifeformSummaryPreview{}, fmt.Errorf("Lifeform Summary requires original physical table %s.%s", source.role, source.table)
				}
				var err error
				tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
				if err != nil {
					return LifeformSummaryPreview{}, err
				}
			}
			report, err := planLifeformSummary(ctx, owner.Selection.Project, owner.Selection.SU, tables[0], tables[1], tables[2], tables[3])
			if err != nil {
				return LifeformSummaryPreview{}, err
			}
			return LifeformSummaryPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, report}, nil
		})
	})
}
