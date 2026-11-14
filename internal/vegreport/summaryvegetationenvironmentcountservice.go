package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SummaryVegetationEnvironmentalCountReview struct {
	ContextID   string                        `json:"contextId"`
	Project     string                        `json:"project"`
	ProjectPath string                        `json:"projectPath"`
	SU          string                        `json:"su"`
	SUPath      string                        `json:"suPath"`
	SourcePath  string                        `json:"sourcePath"`
	SourceTable string                        `json:"sourceTable"`
	SiteUnit    string                        `json:"siteUnit"`
	Source      string                        `json:"source"`
	Physical    vprocore.ProjectMetadataTable `json:"physical"`
	Value       vprocore.ProjectMetadataCell  `json:"value"`
}

type SummaryVegetationEnvironmentalCountService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationEnvironmentalCountService(contexts *vprocore.ContextService) (*SummaryVegetationEnvironmentalCountService, error) {
	return &SummaryVegetationEnvironmentalCountService{contexts: contexts}, nil
}

func (s *SummaryVegetationEnvironmentalCountService) GetEnvironmentalPlotCount(ctx context.Context, contextID string,
	siteUnit *string) (*SummaryVegetationEnvironmentalCountReview, error) {
	if ctx == nil {
		return nil, errors.New("Summary environmental plot count requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return nil, errors.New("Summary environmental plot count context service is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	if siteUnit == nil {
		return nil, errors.New("Summary environmental plot count requires a non-NULL Site Unit argument (source error94)")
	}
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return nil, fmt.Errorf("Summary environmental plot count Site Unit: %w", err)
	}
	unit := *siteUnit
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationEnvironmentalCountReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationEnvironmentalCountReview, error) {
			if owner.Selection.SU == "None" || owner.Selection.SU == "USysSuTableDynamic" {
				return nil, errors.New("Summary environmental plot count requires selected normal SU; hierarchy/dynamic-break scope unavailable")
			}
			name := owner.Selection.SU + "_SU"
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM su.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
				return nil, fmt.Errorf("Summary environmental plot count physical SU lookup: %w", err)
			}
			if count != 1 {
				return nil, fmt.Errorf("Summary environmental plot count requires original physical su.%s table", name)
			}
			if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "su", name, "Summary environmental plot count"); err != nil {
				return nil, err
			}
			physical, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "su", name, "", nil, "")
			if err != nil {
				return nil, fmt.Errorf("Summary environmental plot count physical SU read: %w", err)
			}
			value, err := prepareSummaryVegetationEnvironmentalPlotCount(ctx, &unit, physical)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationEnvironmentalCountReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, SourcePath: owner.Attachments["su"],
				SourceTable: name, SiteUnit: unit,
				Source:   "owned imported SQLite SU; literal SiteUnit count of nonNULL PlotNumber; no DISTINCT/Env join or Access collation claim",
				Physical: physical, Value: value,
			}, nil
		})
	})
}
