package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func twoPageParentProjector(form string) (sourceParentProjector, error) {
	if _, err := twoPageParentBindings(form); err != nil {
		return nil, err
	}
	return func(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable) (*siviParentProjection, error) {
		return projectTwoPageParent(ctx, contextID, project, plot, form, env, admin)
	}, nil
}

func twoPageParentExtraHistory(form string) (siviParentHistoryDomain, error) {
	return twoPageParentHistory(form, "Extra", "two-page additional fields", planTwoPageParentExtraProjection)
}

func twoPageParentHistory(form, scope, label string, plan func(context.Context, *siviParentProjection, []siviParentScalarEdit) ([]siviParentScalarAssignment, error)) (siviParentHistoryDomain, error) {
	projectParent, err := twoPageParentProjector(form)
	if err != nil {
		return siviParentHistoryDomain{}, err
	}
	table := "__VPRO_TwoPageParent" + scope + "History"
	if form == "FS882-8x6XL-CHARS" {
		table = "__VPRO_TwoPageCHARSParent" + scope + "History"
	}
	return siviParentHistoryDomain{
		table: table,
		schema: `CREATE TABLE ` + quoteHeaderIdentifier(table) +
			`(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`,
		label: label,
		assignments: func(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
			if event.ProjectAssignment != nil || event.EntryPlan != nil || event.Committed == nil || event.Committed.Form != form {
				return nil, fmt.Errorf("%s history belongs to another source domain", label)
			}
			original, err := validateSourceParentHistoryOriginal(ctx, event, project, plot, projectParent)
			if err != nil {
				return nil, err
			}
			edits := make([]siviParentScalarEdit, 0, len(event.Changes))
			for _, change := range event.Changes {
				edits = append(edits, siviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
			}
			assignments, err := plan(ctx, original, edits)
			if err != nil || len(assignments) != len(event.Changes) {
				return nil, errors.Join(err, errors.New("two-page typed history contains repeated or unchanged assignments"))
			}
			return assignments, nil
		},
	}, nil
}

func planTwoPageParentExtraProjection(ctx context.Context, original *siviParentProjection, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("two-page additional-field editing requires one owned physical pair")
	}
	return planTwoPageParentExtraFields(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}},
		ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}, edits)
}

func twoPageParentWriteReader(form string) (func(context.Context, *sql.Tx, *sqliteContext, string, string, string) (*siviParentProjection, error), error) {
	projectParent, err := twoPageParentProjector(form)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, tx *sql.Tx, owner *sqliteContext, suAlias, contextID, plot string) (*siviParentProjection, error) {
		return readSourceParentForWrite(ctx, tx, owner, suAlias, contextID, plot, projectParent)
	}, nil
}

func (s *ContextService) writeTwoPageParentExtra(ctx context.Context, contextID, plot, form string, original *siviParentProjection, edits []siviParentScalarEdit) (*siviParentWriteResult, error) {
	history, err := twoPageParentExtraHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := twoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := s.writeSIVIParentPlannedWithHooks(ctx, contextID, plot, original, history,
		func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			return planTwoPageParentExtraProjection(ctx, observed, edits)
		}, siviParentWriteHooks{read: read})
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field writing: %w", err)
	}
	return result, nil
}

func (s *ContextService) restoreTwoPageParentExtra(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	history, err := twoPageParentExtraHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := twoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := s.restoreSourceParentHistory(ctx, contextID, plot, historyID, action, history, read)
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field restoration: %w", err)
	}
	return result, nil
}
