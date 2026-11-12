package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func TwoPageParentProjector(form string) (sourceform.SourceParentProjector, error) {
	if _, err := twoPageParentBindings(form); err != nil {
		return nil, err
	}
	return func(ctx context.Context, contextID, project, plot string, env, admin vprocore.ProjectMetadataTable) (*sourceform.SiviParentProjection, error) {
		return ProjectTwoPageParent(ctx, contextID, project, plot, form, env, admin)
	}, nil
}

func TwoPageParentExtraHistory(form string) (sourceform.SiviParentHistoryDomain, error) {
	return twoPageParentHistory(form, "Extra", "two-page additional fields", PlanTwoPageParentExtraProjection)
}

func twoPageParentHistory(form, scope, label string, plan func(context.Context, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error)) (sourceform.SiviParentHistoryDomain, error) {
	projectParent, err := TwoPageParentProjector(form)
	if err != nil {
		return sourceform.SiviParentHistoryDomain{}, err
	}
	table := "__VPRO_TwoPageParent" + scope + "History"
	if form == "FS882-8x6XL-CHARS" {
		table = "__VPRO_TwoPageCHARSParent" + scope + "History"
	}
	return sourceform.SiviParentHistoryDomain{
		Table: table,
		Schema: `CREATE TABLE ` + vprocore.QuoteHeaderIdentifier(table) +
			`(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`,
		Label: label,
		Assignments: func(ctx context.Context, event sourceform.SiviParentHistory, project, plot string) ([]sourceform.SiviParentScalarAssignment, error) {
			if event.ProjectAssignment != nil || event.EntryPlan != nil || event.Committed == nil || event.Committed.Form != form {
				return nil, fmt.Errorf("%s history belongs to another source domain", label)
			}
			original, err := sourceform.ValidateSourceParentHistoryOriginal(ctx, event, project, plot, projectParent)
			if err != nil {
				return nil, err
			}
			edits := make([]sourceform.SiviParentScalarEdit, 0, len(event.Changes))
			for _, change := range event.Changes {
				edits = append(edits, sourceform.SiviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
			}
			assignments, err := plan(ctx, original, edits)
			if err != nil || len(assignments) != len(event.Changes) {
				return nil, errors.Join(err, errors.New("two-page typed history contains repeated or unchanged assignments"))
			}
			return assignments, nil
		},
	}, nil
}

func PlanTwoPageParentExtraProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("two-page additional-field editing requires one owned physical pair")
	}
	return planTwoPageParentExtraFields(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}, edits)
}

func TwoPageParentWriteReader(form string) (func(context.Context, *sql.Tx, *vprocore.SqliteContext, string, string, string) (*sourceform.SiviParentProjection, error), error) {
	projectParent, err := TwoPageParentProjector(form)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, suAlias, contextID, plot string) (*sourceform.SiviParentProjection, error) {
		return sourceform.ReadSourceParentForWrite(ctx, tx, owner, suAlias, contextID, plot, projectParent)
	}, nil
}

func WriteTwoPageParentExtra(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) (*sourceform.SiviParentWriteResult, error) {
	history, err := TwoPageParentExtraHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.WriteSIVIParentPlannedWithHooks(s, ctx, contextID, plot, original, history,
		func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
			return PlanTwoPageParentExtraProjection(ctx, observed, edits)
		}, sourceform.SiviParentWriteHooks{Read: read})
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field writing: %w", err)
	}
	return result, nil
}

func RestoreTwoPageParentExtra(s *vprocore.ContextService, ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	history, err := TwoPageParentExtraHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.RestoreSourceParentHistory(s, ctx, contextID, plot, historyID, action, history, read)
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field restoration: %w", err)
	}
	return result, nil
}
