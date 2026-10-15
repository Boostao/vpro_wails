package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const twoPageProjectMetadataEnvironment = "VPRO_TWO_PAGE_PROJECT_METADATA_REVIEW"
const twoPageProjectMetadataScanMaximum = 100000
const twoPageProjectMetadataPageMaximum = 200
const twoPageProjectMetadataMatching = "DAO-General1033-controlled-ASCII-alphanumeric-space; ignore-case-terminal-U0020; physical-row-order"

type TwoPageProjectMetadataReview struct {
	ContextID             string               `json:"contextId"`
	Project               string               `json:"project"`
	ProjectPath           string               `json:"projectPath"`
	SU                    string               `json:"su"`
	SUPath                string               `json:"suPath"`
	Plot                  string               `json:"plot"`
	ProjectID             string               `json:"projectId"`
	TargetForm            string               `json:"targetForm"`
	MetadataTable         string               `json:"metadataTable"`
	EnvOriginal           ProjectMetadataTable `json:"envOriginal"`
	AdminOriginal         ProjectMetadataTable `json:"adminOriginal"`
	MetadataOriginal      ProjectMetadataTable `json:"metadataOriginal"`
	Offset                int                  `json:"offset"`
	Limit                 int                  `json:"limit"`
	TotalRows             int                  `json:"totalRows"`
	MatchingPolicy        string               `json:"matchingPolicy"`
	GeneralLocaleVerified bool                 `json:"generalLocaleVerified"`
}

type TwoPageProjectMetadataService struct {
	contexts *ContextService
	review   bool
}

func NewTwoPageProjectMetadataService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageProjectMetadataService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page project metadata requires owned contexts and explicit feature lookup")
	}
	enabled, err := siviFeature(twoPageProjectMetadataEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageProjectMetadataService{contexts, enabled}, nil
}

func (s *TwoPageProjectMetadataService) GetReview(ctx context.Context, contextID, plot string, offset, limit int) (*TwoPageProjectMetadataReview, error) {
	if ctx == nil {
		return nil, errors.New("two-page project metadata requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || !s.review {
		return nil, errors.New("two-page project metadata requires independently enabled saved-data review")
	}
	if plot == "" || !utf8.ValidString(plot) || offset < 0 || offset > twoPageProjectMetadataScanMaximum ||
		limit < 1 || limit > twoPageProjectMetadataPageMaximum {
		return nil, errors.New("two-page project metadata requires a valid literal opening parent, offset0..100000 and limit1..200")
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageProjectMetadataReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageProjectMetadataReview, error) {
			project := owner.selection.Project
			envTable, adminTable, metadataTable := project+"_Env", project+"_Admin", project+"_Metadata"
			if err := requireTwoPageChildReadScope(ctx, tx, project, envTable, plot, "project metadata"); err != nil {
				return nil, err
			}
			env, err := readSQLiteLiteralTextRows(ctx, tx, "project", envTable, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page project metadata Env original: %w", err)
			}
			admin, err := readSQLiteLiteralTextRows(ctx, tx, "project", adminTable, "Plot", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page project metadata Admin original: %w", err)
			}
			if len(env.Rows) != 1 || len(admin.Rows) != 1 {
				return nil, errors.New("two-page project metadata requires one complete literal physical parent pair")
			}
			envIndices, err := siteUnitTransferColumns(env, "ProjectID")
			if err != nil {
				return nil, fmt.Errorf("two-page project metadata parent ProjectID: %w", err)
			}
			value, err := metadataCellValue(env.Rows[0].Cells[envIndices["ProjectID"]])
			if err != nil {
				return nil, fmt.Errorf("two-page project metadata parent ProjectID: %w", err)
			}
			projectID, ok := value.(string)
			if !ok {
				return nil, errors.New("two-page project metadata requires original TEXT parent ProjectID; NULL or other storage is not converted into an empty identity")
			}
			if _, err := twoPageProjectMetadataKey(projectID); err != nil {
				return nil, fmt.Errorf("two-page project metadata parent ProjectID: %w", err)
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", metadataTable, "project metadata"); err != nil {
				return nil, err
			}
			var count int64
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM project."+quoteHeaderIdentifier(metadataTable)).Scan(&count); err != nil {
				return nil, fmt.Errorf("two-page project metadata physical bound: %w", err)
			}
			if count < 0 || count > twoPageProjectMetadataScanMaximum {
				return nil, errors.New("two-page project metadata exceeds the100000-row physical snapshot bound; no partial review")
			}
			metadata, err := readPhysicalLocationTable(ctx, tx, "project", metadataTable)
			if err != nil {
				return nil, fmt.Errorf("two-page project metadata original: %w", err)
			}
			page, total, err := planTwoPageProjectMetadata(ctx, metadata, projectID, offset, limit)
			if err != nil {
				return nil, err
			}
			return &TwoPageProjectMetadataReview{
				ContextID: contextID, Project: project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, Plot: plot, ProjectID: projectID,
				TargetForm: "frmProjectMetaData", MetadataTable: metadataTable,
				EnvOriginal: env, AdminOriginal: admin, MetadataOriginal: page,
				Offset: offset, Limit: limit, TotalRows: total, MatchingPolicy: twoPageProjectMetadataMatching,
			}, nil
		})
	})
}

func twoPageProjectMetadataKey(value string) (string, error) {
	for _, char := range value {
		if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == ' ') {
			return "", errors.New("ProjectID is outside the controlled DAO ASCII alphanumeric/space comparison domain; no source criterion is escaped or repaired")
		}
	}
	// Only the comparison key ignores source-calibrated terminal U0020; originals remain unchanged.
	return strings.ToUpper(strings.TrimRight(value, " ")), nil
}

func planTwoPageProjectMetadata(ctx context.Context, metadata ProjectMetadataTable, projectID string, offset, limit int) (ProjectMetadataTable, int, error) {
	key, err := twoPageProjectMetadataKey(projectID)
	if err != nil {
		return ProjectMetadataTable{}, 0, err
	}
	indices, err := siteUnitTransferColumns(metadata, "ProjectID")
	if err != nil {
		return ProjectMetadataTable{}, 0, fmt.Errorf("two-page project metadata schema: %w", err)
	}
	page := ProjectMetadataTable{Columns: metadata.Columns, Rows: []ProjectMetadataRow{}}
	total := 0
	for _, row := range metadata.Rows {
		if err := ctx.Err(); err != nil {
			return ProjectMetadataTable{}, 0, err
		}
		value, err := metadataCellValue(row.Cells[indices["ProjectID"]])
		if err != nil {
			return ProjectMetadataTable{}, 0, fmt.Errorf("two-page project metadata row %s ProjectID: %w", row.RowID, err)
		}
		if value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return ProjectMetadataTable{}, 0, errors.New("two-page project metadata requires original TEXT/NULL metadata ProjectID storage; no identity coerced")
		}
		rowKey, err := twoPageProjectMetadataKey(text)
		if err != nil {
			return ProjectMetadataTable{}, 0, fmt.Errorf("two-page project metadata row %s: %w", row.RowID, err)
		}
		if rowKey == key {
			if total >= offset && len(page.Rows) < limit {
				page.Rows = append(page.Rows, row)
			}
			total++
		}
	}
	return page, total, nil
}
