package main

import (
	"context"
	"database/sql"
	"errors"
)

const summaryVegetationFieldListFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_FIELD_LIST"
const summaryVegetationFieldListSource = "owned imported SQLite VPro64.USysShortVegSummaryFieldList; complete physical rows; Label read-only"

func summaryVegetationFieldListFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(summaryVegetationFieldListFeatureEnvironment, lookup)
}

type SummaryVegetationFieldListReview struct {
	ContextID   string               `json:"contextId"`
	Project     string               `json:"project"`
	ProjectPath string               `json:"projectPath"`
	SU          string               `json:"su"`
	SUPath      string               `json:"suPath"`
	SourcePath  string               `json:"sourcePath"`
	Source      string               `json:"source"`
	Physical    ProjectMetadataTable `json:"physical"`
}

type SummaryVegetationFieldListSelection struct {
	Review   SummaryVegetationFieldListReview `json:"review"`
	Visible  []string                         `json:"visible"`
	Selected []string                         `json:"selected"`
}

type SummaryVegetationFieldListService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewSummaryVegetationFieldListService(contexts *ContextService, enabled bool) *SummaryVegetationFieldListService {
	return &SummaryVegetationFieldListService{contexts: contexts, enabled: enabled}
}

func (s *SummaryVegetationFieldListService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary environmental field-list requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Summary environmental field-list is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Summary environmental field-list context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *SummaryVegetationFieldListService) GetSummaryVegetationFieldList(ctx context.Context, contextID string) (*SummaryVegetationFieldListReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SummaryVegetationFieldListReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*SummaryVegetationFieldListReview, error) {
			physical, err := readSummaryVegetationFieldList(ctx, tx)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationFieldListReview{
				ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, SourcePath: owner.attachments["VPro64"],
				Source: summaryVegetationFieldListSource, Physical: physical,
			}, nil
		})
	})
}

func (s *SummaryVegetationFieldListService) GetSummaryVegetationFieldListIndex(ctx context.Context, contextID string) (*SummaryVegetationFieldListSelection, error) {
	review, err := s.GetSummaryVegetationFieldList(ctx, contextID)
	if err != nil {
		return nil, err
	}
	index, err := indexSummaryVegetationFieldList(ctx, review.Physical)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationFieldListSelection{Review: *review, Visible: index.Visible, Selected: index.Selected}, nil
}
