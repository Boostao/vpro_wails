package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

const summaryVegetationFieldListSource = "owned imported SQLite VPro64.USysShortVegSummaryFieldList; complete physical rows; Label read-only"

type SummaryVegetationFieldListReview struct {
	ContextID   string                        `json:"contextId"`
	Project     string                        `json:"project"`
	ProjectPath string                        `json:"projectPath"`
	SU          string                        `json:"su"`
	SUPath      string                        `json:"suPath"`
	SourcePath  string                        `json:"sourcePath"`
	Source      string                        `json:"source"`
	Physical    vprocore.ProjectMetadataTable `json:"physical"`
}

type SummaryVegetationFieldListSelection struct {
	Review   SummaryVegetationFieldListReview `json:"review"`
	Visible  []string                         `json:"visible"`
	Selected []string                         `json:"selected"`
}

type SummaryVegetationFieldListService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationFieldListService(contexts *vprocore.ContextService) *SummaryVegetationFieldListService {
	return &SummaryVegetationFieldListService{contexts: contexts}
}

func (s *SummaryVegetationFieldListService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary environmental field-list requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Summary environmental field-list context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *SummaryVegetationFieldListService) GetSummaryVegetationFieldList(ctx context.Context, contextID string) (*SummaryVegetationFieldListReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationFieldListReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationFieldListReview, error) {
			physical, err := readSummaryVegetationFieldList(ctx, tx)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationFieldListReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, SourcePath: owner.Attachments["VPro64"],
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
