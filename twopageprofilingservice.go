package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

const twoPageProfilingEnvironment = "VPRO_TWO_PAGE_PROFILING_REVIEW"
const twoPageProfilingScanMaximum = 100000
const twoPageProfilingPageMaximum = 200
const twoPageProfilingOrdering = "physical-rowid; not source Order execution"

type TwoPageProfilingReview struct {
	ContextID           string               `json:"contextId"`
	Project             string               `json:"project"`
	ProjectPath         string               `json:"projectPath"`
	SU                  string               `json:"su"`
	SUPath              string               `json:"suPath"`
	Plot                string               `json:"plot"`
	TargetForm          string               `json:"targetForm"`
	Source              PlotProfileSource    `json:"source"`
	ProfileTable        string               `json:"profileTable"`
	EnvOriginal         ProjectMetadataTable `json:"envOriginal"`
	AdminOriginal       ProjectMetadataTable `json:"adminOriginal"`
	ProfileOriginal     ProjectMetadataTable `json:"profileOriginal"`
	DescriptionsPresent bool                 `json:"descriptionsPresent"`
	DescriptionOriginal ProjectMetadataTable `json:"descriptionOriginal"`
	Offset              int                  `json:"offset"`
	Limit               int                  `json:"limit"`
	TotalRows           int                  `json:"totalRows"`
	OrderingPolicy      string               `json:"orderingPolicy"`
}

type TwoPageProfilingService struct {
	contexts *ContextService
	review   bool
}

func NewTwoPageProfilingService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageProfilingService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page profiling requires owned contexts and explicit feature lookup")
	}
	enabled, err := siviFeature(twoPageProfilingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageProfilingService{contexts, enabled}, nil
}

func (s *TwoPageProfilingService) GetReview(ctx context.Context, contextID, plot string, offset, limit int) (*TwoPageProfilingReview, error) {
	if ctx == nil {
		return nil, errors.New("two-page profiling requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || !s.review {
		return nil, errors.New("two-page profiling requires independently enabled saved-data review")
	}
	if plot == "" || !utf8.ValidString(plot) || offset < 0 || offset > twoPageProfilingScanMaximum ||
		limit < 1 || limit > twoPageProfilingPageMaximum {
		return nil, errors.New("two-page profiling requires a literal opening parent, offset0..100000 and limit1..200")
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageProfilingReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageProfilingReview, error) {
			if owner.profile == nil {
				return nil, errors.New("two-page profiling requires explicitly selected profile name/path; legacy project fallback is not source authority")
			}
			if err := owner.profile.validate(); err != nil {
				return nil, fmt.Errorf("two-page profiling selected source: %w", err)
			}
			if owner.profile.Name == "None" {
				return nil, errors.New("two-page profiling has no selected profile; no project-local fallback")
			}
			role, table, err := owner.profileLocation()
			if err != nil {
				return nil, err
			}
			path, attached := owner.attachments[role]
			if !attached || !sameDesktopPath(owner.profile.Path, path) {
				return nil, errors.New("two-page profiling selected source has no exact owned attachment")
			}
			project := owner.selection.Project
			envTable, adminTable := project+"_Env", project+"_Admin"
			if err := requireTwoPageChildReadScope(ctx, tx, project, envTable, plot, "profiling"); err != nil {
				return nil, err
			}
			env, err := readSQLiteLiteralTextRows(ctx, tx, "project", envTable, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page profiling Env original: %w", err)
			}
			admin, err := readSQLiteLiteralTextRows(ctx, tx, "project", adminTable, "Plot", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page profiling Admin original: %w", err)
			}
			if len(env.Rows) != 1 || len(admin.Rows) != 1 {
				return nil, errors.New("two-page profiling requires one complete literal physical parent pair")
			}
			rules, err := readTwoPageProfilingBoundedTable(ctx, tx, role, table)
			if err != nil {
				return nil, err
			}
			if reason := profileSchemaReason(rules.Columns, nil); reason != "" {
				return nil, fmt.Errorf("two-page profiling source schema: %s", reason)
			}
			descriptionColumns, err := profileMetadataColumns(ctx, tx, role)
			if err != nil {
				return nil, fmt.Errorf("two-page profiling Description schema: %w", err)
			}
			descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
			if descriptionColumns != nil {
				if reason := profileMetadataReason(descriptionColumns); reason != "" {
					return nil, fmt.Errorf("two-page profiling Description schema: %s", reason)
				}
				original, err := readTwoPageProfilingBoundedTable(ctx, tx, role, "_table_metadata")
				if err != nil {
					return nil, err
				}
				indices, err := siteUnitTransferColumns(original, "table_name", "description")
				if err != nil {
					return nil, err
				}
				descriptions.Columns = original.Columns
				for _, row := range original.Rows {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					identity := row.Cells[indices["table_name"]]
					if identity.Storage == "null" {
						continue
					}
					if identity.Storage != "text" || identity.Text == nil {
						return nil, errors.New("two-page profiling Description identities require literal TEXT/NULL storage; no coercion")
					}
					if *identity.Text == table {
						descriptions.Rows = append(descriptions.Rows, row)
					}
				}
			}
			page := ProjectMetadataTable{Columns: rules.Columns, Rows: []ProjectMetadataRow{}}
			if offset < len(rules.Rows) {
				end := min(offset+limit, len(rules.Rows))
				page.Rows = rules.Rows[offset:end]
			}
			return &TwoPageProfilingReview{
				ContextID: contextID, Project: project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, Plot: plot,
				TargetForm: "USysPlotProfiling", Source: PlotProfileSource{owner.profile.Name, path},
				ProfileTable: table, EnvOriginal: env, AdminOriginal: admin, ProfileOriginal: page,
				DescriptionsPresent: descriptionColumns != nil, DescriptionOriginal: descriptions,
				Offset: offset, Limit: limit, TotalRows: len(rules.Rows), OrderingPolicy: twoPageProfilingOrdering,
			}, nil
		})
	})
}

func readTwoPageProfilingBoundedTable(ctx context.Context, tx *sql.Tx, role, table string) (ProjectMetadataTable, error) {
	var physical int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteHeaderIdentifier(role)+
		".sqlite_master WHERE type='table' AND name COLLATE BINARY=?", table).Scan(&physical); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("two-page profiling physical schema: %w", err)
	}
	if physical != 1 {
		return ProjectMetadataTable{}, fmt.Errorf("two-page profiling requires original physical table %s.%s", role, table)
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, role, table, "profiling"); err != nil {
		return ProjectMetadataTable{}, err
	}
	var count int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteHeaderIdentifier(role)+"."+
		quoteHeaderIdentifier(table)).Scan(&count); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("two-page profiling physical bound: %w", err)
	}
	if count < 0 || count > twoPageProfilingScanMaximum {
		return ProjectMetadataTable{}, errors.New("two-page profiling exceeds the100000-row physical source bound; no partial review")
	}
	original, err := readPhysicalLocationTable(ctx, tx, role, table)
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("two-page profiling original %s: %w", table, err)
	}
	return original, nil
}
