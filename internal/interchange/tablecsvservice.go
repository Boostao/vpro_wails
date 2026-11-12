package interchange

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

const tableCSVReviewFeatureEnvironment = "VPRO_TABLE_CSV_REVIEW"

func TableCSVReviewFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(tableCSVReviewFeatureEnvironment, lookup)
}

type ProjectTableCSVReview struct {
	ContextID                  string           `json:"contextId"`
	Project                    string           `json:"project"`
	ProjectPath                string           `json:"projectPath"`
	DescriptionMetadataPresent bool             `json:"descriptionMetadataPresent"`
	Manifest                   TableCSVManifest `json:"manifest"`
	CSV                        string           `json:"csv"`
}

func GetProjectTableCSVReview(s *vprocore.ContextService, ctx context.Context, contextID string, table string) (*ProjectTableCSVReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.TableCSVReviewEnabled {
		return nil, errors.New("project table CSV read-only review is disabled in this session")
	}
	review, err := ReadProjectTableCSV(s, ctx, contextID, table)
	if err != nil {
		return nil, err
	}
	return &ProjectTableCSVReview{
		ContextID: review.ContextID, Project: review.Project, ProjectPath: review.ProjectPath,
		DescriptionMetadataPresent: review.DescriptionMetadataPresent,
		Manifest:                   review.Document.Manifest, CSV: string(review.Document.Data),
	}, nil
}
