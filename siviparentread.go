package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *ContextService) readSIVIParent(ctx context.Context, contextID, plot string) (*siviParentProjection, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviParentProjection, error) {
		return readOwnedSIVIParent(ctx, plots, contextID, plot)
	})
}

func readOwnedSIVIParent(ctx context.Context, plots *PlotService, contextID, plot string) (*siviParentProjection, error) {
	return withOwnedSIVIParentRead(ctx, plots, contextID, plot, func(parent *siviParentProjection) (*siviParentProjection, error) {
		return parent, nil
	})
}

func withOwnedSIVIParentRead[T any](ctx context.Context, plots *PlotService, contextID, plot string, project func(*siviParentProjection) (T, error)) (T, error) {
	return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (T, error) {
		var zero T
		tables := []ProjectMetadataTable{}
		for i, suffix := range []string{"_Env", "_Admin"} {
			table := owner.selection.Project + suffix
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
				return zero, err
			}
			if count != 1 {
				return zero, fmt.Errorf("SIVI parent requires original physical project table %q", table)
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "parent originals"); err != nil {
				return zero, err
			}
			column := "PlotNumber"
			if i == 1 {
				column = "Plot"
			}
			rows, err := readSQLiteStorageRows(ctx, tx, "project", table, column, &plot, "")
			if err != nil {
				return zero, err
			}
			tables = append(tables, rows)
		}
		parent, err := projectSIVIParent(ctx, contextID, owner.selection.Project, plot, tables[0], tables[1])
		if err != nil {
			return zero, err
		}
		result, err := project(parent)
		if err != nil {
			return zero, err
		}
		return result, nil
	})
}

type siviParentProposal struct {
	Original         *siviParentProjection
	Scalars, Options []siviParentScalarAssignment
}

type siviParentStorageProposal struct {
	siviParentProposal
	Text []siviParentScalarAssignment
}

type siviParentCategoricalProposal struct {
	siviParentStorageProposal
	Categorical []siviParentScalarAssignment
}

type siviParentActionProposal struct {
	siviParentCategoricalProposal
	Actions []siviParentScalarAssignment
}

func (s *ContextService) prepareSIVIParentProposal(ctx context.Context, contextID, plot string, scalarEdits, optionEdits []siviParentScalarEdit) (*siviParentProposal, error) {
	result, err := s.prepareSIVIParentStorageProposal(ctx, contextID, plot, scalarEdits, optionEdits, nil)
	if err != nil {
		return nil, err
	}
	return &result.siviParentProposal, nil
}

func (s *ContextService) prepareSIVIParentStorageProposal(ctx context.Context, contextID, plot string, scalarEdits, optionEdits, textEdits []siviParentScalarEdit) (*siviParentStorageProposal, error) {
	result, err := s.prepareSIVIParentCategoricalProposal(ctx, contextID, plot, scalarEdits, optionEdits, textEdits, nil)
	if err != nil {
		return nil, err
	}
	return &result.siviParentStorageProposal, nil
}

func (s *ContextService) prepareSIVIParentCategoricalProposal(ctx context.Context, contextID, plot string, scalarEdits, optionEdits, textEdits, categoricalEdits []siviParentScalarEdit) (*siviParentCategoricalProposal, error) {
	result, err := s.prepareSIVIParentActionProposal(ctx, contextID, plot, scalarEdits, optionEdits, textEdits, categoricalEdits, nil)
	if err != nil {
		return nil, err
	}
	return &result.siviParentCategoricalProposal, nil
}

func (s *ContextService) prepareSIVIParentActionProposal(ctx context.Context, contextID, plot string, scalarEdits, optionEdits, textEdits, categoricalEdits []siviParentScalarEdit, actionEdits []siviParentActionEdit) (*siviParentActionProposal, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviParentActionProposal, error) {
		return withOwnedSIVIParentRead(ctx, plots, contextID, plot, func(original *siviParentProjection) (*siviParentActionProposal, error) {
			if len(original.Rows) != 1 || len(scalarEdits)+len(optionEdits)+len(textEdits)+len(categoricalEdits)+len(actionEdits) == 0 {
				return nil, errors.New("SIVI parent proposal requires one physical pair and explicit domain edits")
			}
			env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
			admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
			result := &siviParentActionProposal{
				siviParentCategoricalProposal: siviParentCategoricalProposal{
					siviParentStorageProposal: siviParentStorageProposal{
						siviParentProposal: siviParentProposal{Original: original, Scalars: []siviParentScalarAssignment{}, Options: []siviParentScalarAssignment{}},
						Text:               []siviParentScalarAssignment{},
					},
					Categorical: []siviParentScalarAssignment{},
				},
				Actions: []siviParentScalarAssignment{},
			}
			var err error
			if len(scalarEdits) > 0 {
				result.Scalars, err = planSIVIParentScalars(ctx, contextID, original.Project, plot, env, admin, scalarEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(optionEdits) > 0 {
				result.Options, err = planSIVIParentOptions(ctx, contextID, original.Project, plot, env, admin, optionEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(textEdits) > 0 {
				result.Text, err = planSIVIParentText(ctx, contextID, original.Project, plot, env, admin, textEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(categoricalEdits) > 0 {
				result.Categorical, err = planSIVIParentCategorical(ctx, contextID, original.Project, plot, env, admin, categoricalEdits)
				if err != nil {
					return nil, err
				}
			}
			if len(actionEdits) > 0 {
				result.Actions, err = planSIVIParentActions(ctx, contextID, original.Project, plot, env, admin, actionEdits)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		})
	})
}
