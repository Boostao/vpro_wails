package classification

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

const TwoPageProfilingEnvironment = "VPRO_TWO_PAGE_PROFILING_REVIEW"
const twoPageProfilingScanMaximum = 100000
const twoPageProfilingPageMaximum = 200
const TwoPageProfilingOrdering = "physical-rowid; not source Order execution"

type TwoPageProfilingReview struct {
	ContextID           string                        `json:"contextId"`
	Project             string                        `json:"project"`
	ProjectPath         string                        `json:"projectPath"`
	SU                  string                        `json:"su"`
	SUPath              string                        `json:"suPath"`
	Plot                string                        `json:"plot"`
	TargetForm          string                        `json:"targetForm"`
	Source              vprocore.PlotProfileSource    `json:"source"`
	ProfileTable        string                        `json:"profileTable"`
	EnvOriginal         vprocore.ProjectMetadataTable `json:"envOriginal"`
	AdminOriginal       vprocore.ProjectMetadataTable `json:"adminOriginal"`
	ProfileOriginal     vprocore.ProjectMetadataTable `json:"profileOriginal"`
	DescriptionsPresent bool                          `json:"descriptionsPresent"`
	DescriptionOriginal vprocore.ProjectMetadataTable `json:"descriptionOriginal"`
	Offset              int                           `json:"offset"`
	Limit               int                           `json:"limit"`
	TotalRows           int                           `json:"totalRows"`
	OrderingPolicy      string                        `json:"orderingPolicy"`
}

type TwoPageProfilingService struct {
	Contexts *vprocore.ContextService `json:"-" yaml:"-"`
	Review   bool                     `json:"-" yaml:"-"`
}

func NewTwoPageProfilingService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*TwoPageProfilingService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page profiling requires owned contexts and explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(TwoPageProfilingEnvironment, lookup)
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
	if s == nil || s.Contexts == nil || !s.Review {
		return nil, errors.New("two-page profiling requires independently enabled saved-data review")
	}
	if plot == "" || !utf8.ValidString(plot) || offset < 0 || offset > twoPageProfilingScanMaximum ||
		limit < 1 || limit > twoPageProfilingPageMaximum {
		return nil, errors.New("two-page profiling requires a literal opening parent, offset0..100000 and limit1..200")
	}
	return vprocore.WithContextPlotRequest(ctx, s.Contexts, contextID, func(plots *vprocore.PlotService) (*TwoPageProfilingReview, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*TwoPageProfilingReview, error) {
			if owner.Profile == nil {
				return nil, errors.New("two-page profiling requires explicitly selected profile name/path; legacy project fallback is not source authority")
			}
			if err := owner.Profile.Validate(); err != nil {
				return nil, fmt.Errorf("two-page profiling selected source: %w", err)
			}
			if owner.Profile.Name == "None" {
				return nil, errors.New("two-page profiling has no selected profile; no project-local fallback")
			}
			role, table, err := owner.ProfileLocation()
			if err != nil {
				return nil, err
			}
			path, attached := owner.Attachments[role]
			if !attached || !vprocore.SameDesktopPath(owner.Profile.Path, path) {
				return nil, errors.New("two-page profiling selected source has no exact owned attachment")
			}
			project := owner.Selection.Project
			envTable, adminTable := project+"_Env", project+"_Admin"
			if err := sourceform.RequireTwoPageChildReadScope(ctx, tx, project, envTable, plot, "profiling"); err != nil {
				return nil, err
			}
			env, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "project", envTable, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page profiling Env original: %w", err)
			}
			admin, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "project", adminTable, "Plot", plot, "")
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
			if reason := vprocore.ProfileSchemaReason(rules.Columns, nil); reason != "" {
				return nil, fmt.Errorf("two-page profiling source schema: %s", reason)
			}
			descriptionColumns, err := vprocore.ProfileMetadataColumns(ctx, tx, role)
			if err != nil {
				return nil, fmt.Errorf("two-page profiling Description schema: %w", err)
			}
			descriptions := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
			if descriptionColumns != nil {
				if reason := vprocore.ProfileMetadataReason(descriptionColumns); reason != "" {
					return nil, fmt.Errorf("two-page profiling Description schema: %s", reason)
				}
				original, err := readTwoPageProfilingBoundedTable(ctx, tx, role, "_table_metadata")
				if err != nil {
					return nil, err
				}
				indices, err := vprocore.SiteUnitTransferColumns(original, "table_name", "description")
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
			page := vprocore.ProjectMetadataTable{Columns: rules.Columns, Rows: []vprocore.ProjectMetadataRow{}}
			if offset < len(rules.Rows) {
				end := min(offset+limit, len(rules.Rows))
				page.Rows = rules.Rows[offset:end]
			}
			return &TwoPageProfilingReview{
				ContextID: contextID, Project: project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Plot: plot,
				TargetForm: "USysPlotProfiling", Source: vprocore.PlotProfileSource{Name: owner.Profile.Name, Path: path},
				ProfileTable: table, EnvOriginal: env, AdminOriginal: admin, ProfileOriginal: page,
				DescriptionsPresent: descriptionColumns != nil, DescriptionOriginal: descriptions,
				Offset: offset, Limit: limit, TotalRows: len(rules.Rows), OrderingPolicy: TwoPageProfilingOrdering,
			}, nil
		})
	})
}

func readTwoPageProfilingBoundedTable(ctx context.Context, tx *sql.Tx, role, table string) (vprocore.ProjectMetadataTable, error) {
	var physical int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+vprocore.QuoteHeaderIdentifier(role)+
		".sqlite_master WHERE type='table' AND name COLLATE BINARY=?", table).Scan(&physical); err != nil {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("two-page profiling physical schema: %w", err)
	}
	if physical != 1 {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("two-page profiling requires original physical table %s.%s", role, table)
	}
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, role, table, "profiling"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	var count int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+vprocore.QuoteHeaderIdentifier(role)+"."+
		vprocore.QuoteHeaderIdentifier(table)).Scan(&count); err != nil {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("two-page profiling physical bound: %w", err)
	}
	if count < 0 || count > twoPageProfilingScanMaximum {
		return vprocore.ProjectMetadataTable{}, errors.New("two-page profiling exceeds the100000-row physical source bound; no partial review")
	}
	original, err := vprocore.ReadPhysicalLocationTable(ctx, tx, role, table)
	if err != nil {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("two-page profiling original %s: %w", table, err)
	}
	return original, nil
}
