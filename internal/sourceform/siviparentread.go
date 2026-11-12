package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func ReadSIVIParent(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*SiviParentProjection, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiviParentProjection, error) {
		return readOwnedSIVIParent(ctx, plots, contextID, plot)
	})
}

func readOwnedSIVIParent(ctx context.Context, plots *vprocore.PlotService, contextID, plot string) (*SiviParentProjection, error) {
	return WithOwnedSIVIParentRead(ctx, plots, contextID, plot, func(parent *SiviParentProjection) (*SiviParentProjection, error) {
		return parent, nil
	})
}

func WithOwnedSIVIParentRead[T any](ctx context.Context, plots *vprocore.PlotService, contextID, plot string, project func(*SiviParentProjection) (T, error)) (T, error) {
	return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (T, error) {
		var zero T
		parent, err := readSIVIParentSnapshot(ctx, owner, tx, contextID, plot)
		if err != nil {
			return zero, err
		}
		return project(parent)
	})
}

func readSIVIParentSnapshot(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID, plot string) (*SiviParentProjection, error) {
	tables, err := ReadSourceParentTables(ctx, owner, tx, plot)
	if err != nil {
		return nil, err
	}
	return ProjectSIVIParent(ctx, contextID, owner.Selection.Project, plot, tables[0], tables[1])
}

func ReadSourceParentTables(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, plot string) ([2]vprocore.ProjectMetadataTable, error) {
	var tables [2]vprocore.ProjectMetadataTable
	for i, suffix := range []string{"_Env", "_Admin"} {
		table := owner.Selection.Project + suffix
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
			return tables, err
		}
		if count != 1 {
			return tables, fmt.Errorf("SIVI parent requires original physical project table %q", table)
		}
		if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "project", table, "parent originals"); err != nil {
			return tables, err
		}
		column := "PlotNumber"
		if i == 1 {
			column = "Plot"
		}
		rows, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", table, column, &plot, "")
		if err != nil {
			return tables, err
		}
		tables[i] = rows
	}
	return tables, nil
}

type SiviParentProposal struct {
	Original         *SiviParentProjection
	Scalars, Options []SiviParentScalarAssignment
}

type SiviParentStorageProposal struct {
	SiviParentProposal
	Text []SiviParentScalarAssignment
}

type SiviParentCategoricalProposal struct {
	SiviParentStorageProposal
	Categorical []SiviParentScalarAssignment
}

type SiviParentActionProposal struct {
	SiviParentCategoricalProposal
	Actions []SiviParentScalarAssignment
}

func PrepareSIVIParentProposal(s *vprocore.ContextService, ctx context.Context, contextID, plot string, scalarEdits, optionEdits []SiviParentScalarEdit) (*SiviParentProposal, error) {
	result, err := PrepareSIVIParentStorageProposal(s, ctx, contextID, plot, scalarEdits, optionEdits, nil)
	if err != nil {
		return nil, err
	}
	return &result.SiviParentProposal, nil
}

func PrepareSIVIParentStorageProposal(s *vprocore.ContextService, ctx context.Context, contextID, plot string, scalarEdits, optionEdits, textEdits []SiviParentScalarEdit) (*SiviParentStorageProposal, error) {
	result, err := PrepareSIVIParentCategoricalProposal(s, ctx, contextID, plot, scalarEdits, optionEdits, textEdits, nil)
	if err != nil {
		return nil, err
	}
	return &result.SiviParentStorageProposal, nil
}

func PrepareSIVIParentCategoricalProposal(s *vprocore.ContextService, ctx context.Context, contextID, plot string, scalarEdits, optionEdits, textEdits, categoricalEdits []SiviParentScalarEdit) (*SiviParentCategoricalProposal, error) {
	result, err := PrepareSIVIParentActionProposal(s, ctx, contextID, plot, scalarEdits, optionEdits, textEdits, categoricalEdits, nil)
	if err != nil {
		return nil, err
	}
	return &result.SiviParentCategoricalProposal, nil
}

func PrepareSIVIParentActionProposal(s *vprocore.ContextService, ctx context.Context, contextID, plot string, scalarEdits, optionEdits, textEdits, categoricalEdits []SiviParentScalarEdit, actionEdits []SiviParentActionEdit) (*SiviParentActionProposal, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiviParentActionProposal, error) {
		return WithOwnedSIVIParentRead(ctx, plots, contextID, plot, func(original *SiviParentProjection) (*SiviParentActionProposal, error) {
			if len(original.Rows) != 1 || len(scalarEdits)+len(optionEdits)+len(textEdits)+len(categoricalEdits)+len(actionEdits) == 0 {
				return nil, errors.New("SIVI parent proposal requires one physical pair and explicit domain edits")
			}
			env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
			admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
			result := &SiviParentActionProposal{
				SiviParentCategoricalProposal: SiviParentCategoricalProposal{
					SiviParentStorageProposal: SiviParentStorageProposal{
						SiviParentProposal: SiviParentProposal{Original: original, Scalars: []SiviParentScalarAssignment{}, Options: []SiviParentScalarAssignment{}},
						Text:               []SiviParentScalarAssignment{},
					},
					Categorical: []SiviParentScalarAssignment{},
				},
				Actions: []SiviParentScalarAssignment{},
			}
			var err error
			if len(scalarEdits) > 0 {
				result.Scalars, err = PlanSIVIParentScalars(ctx, contextID, original.Project, plot, env, admin, scalarEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(optionEdits) > 0 {
				result.Options, err = PlanSIVIParentOptions(ctx, contextID, original.Project, plot, env, admin, optionEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(textEdits) > 0 {
				result.Text, err = PlanSIVIParentText(ctx, contextID, original.Project, plot, env, admin, textEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(categoricalEdits) > 0 {
				result.Categorical, err = PlanSIVIParentCategorical(ctx, contextID, original.Project, plot, env, admin, categoricalEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(actionEdits) > 0 {
				result.Actions, err = PlanSIVIParentActions(ctx, contextID, original.Project, plot, env, admin, actionEdits)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		})
	})
}
