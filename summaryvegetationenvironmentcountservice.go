package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

const summaryVegetationEnvironmentalCountFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_ENVIRONMENT_COUNT"

type SummaryVegetationEnvironmentalCountReview struct {
	ContextID   string               `json:"contextId"`
	Project     string               `json:"project"`
	ProjectPath string               `json:"projectPath"`
	SU          string               `json:"su"`
	SUPath      string               `json:"suPath"`
	SourcePath  string               `json:"sourcePath"`
	SourceTable string               `json:"sourceTable"`
	SiteUnit    string               `json:"siteUnit"`
	Source      string               `json:"source"`
	Physical    ProjectMetadataTable `json:"physical"`
	Value       ProjectMetadataCell  `json:"value"`
}

type SummaryVegetationEnvironmentalCountService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewSummaryVegetationEnvironmentalCountService(contexts *ContextService, lookup func(string) (string, bool)) (*SummaryVegetationEnvironmentalCountService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(summaryVegetationEnvironmentalCountFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationEnvironmentalCountService{contexts: contexts, enabled: enabled}, nil
}

func (s *SummaryVegetationEnvironmentalCountService) GetEnvironmentalPlotCount(ctx context.Context, contextID string,
	siteUnit *string) (*SummaryVegetationEnvironmentalCountReview, error) {
	if ctx == nil {
		return nil, errors.New("Summary environmental plot count requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || !s.enabled {
		return nil, errors.New("Summary environmental plot count is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return nil, errors.New("Summary environmental plot count context service is unavailable")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	if siteUnit == nil {
		return nil, errors.New("Summary environmental plot count requires a non-NULL Site Unit argument (source error94)")
	}
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return nil, fmt.Errorf("Summary environmental plot count Site Unit: %w", err)
	}
	unit := *siteUnit
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SummaryVegetationEnvironmentalCountReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*SummaryVegetationEnvironmentalCountReview, error) {
			if owner.selection.SU == "None" || owner.selection.SU == "USysSuTableDynamic" {
				return nil, errors.New("Summary environmental plot count requires selected normal SU; hierarchy/dynamic-break scope unavailable")
			}
			name := owner.selection.SU + "_SU"
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM su.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
				return nil, fmt.Errorf("Summary environmental plot count physical SU lookup: %w", err)
			}
			if count != 1 {
				return nil, fmt.Errorf("Summary environmental plot count requires original physical su.%s table", name)
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "su", name, "Summary environmental plot count"); err != nil {
				return nil, err
			}
			physical, err := readSQLiteStorageRows(ctx, tx, "su", name, "", nil, "")
			if err != nil {
				return nil, fmt.Errorf("Summary environmental plot count physical SU read: %w", err)
			}
			value, err := prepareSummaryVegetationEnvironmentalPlotCount(ctx, &unit, physical)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationEnvironmentalCountReview{
				ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, SourcePath: owner.attachments["su"],
				SourceTable: name, SiteUnit: unit,
				Source:   "owned imported SQLite SU; literal SiteUnit count of nonNULL PlotNumber; no DISTINCT/Env join or Access collation claim",
				Physical: physical, Value: value,
			}, nil
		})
	})
}
