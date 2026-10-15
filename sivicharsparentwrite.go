package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type siviCHARSParentEntryApproval func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error

func readSIVICHARSParentForWrite(ctx context.Context, tx *sql.Tx, owner *sqliteContext,
	suAlias, contextID, plot string) (*siviParentProjection, error) {
	return readSourceParentForWrite(ctx, tx, owner, suAlias, contextID, plot, projectSIVICHARSParent)
}

func planSIVICHARSParentWritableEntry(ctx context.Context, original *siviParentProjection,
	edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	assignments, err := planSIVICHARSParentEntry(ctx, original, edits)
	if err != nil {
		return nil, err
	}
	for _, assignment := range assignments {
		if assignment.Column == "SubZone" {
			return nil, errors.New("CHARS SubZone assignment requires qualification of its foreign-form RowSource")
		}
		if assignment.Column == "SV_AhorizonType" && assignment.After.Text != nil && *assignment.After.Text == "" {
			return nil, errors.New("CHARS blank Ahorizon assignment requires bound NULL/empty qualification")
		}
	}
	return assignments, nil
}

func (s *ContextService) writeSIVICHARSParentEntry(ctx context.Context, contextID, plot string,
	original *siviParentProjection, edits []siviParentScalarEdit, approve siviCHARSParentEntryApproval) (*siviParentWriteResult, error) {
	return s.writeSIVICHARSParentEntryWithLifecycle(ctx, contextID, plot, original, edits, approve, sourceParentEntryLifecycle{})
}

func (s *ContextService) writeSIVICHARSParentEntryWithLifecycle(ctx context.Context, contextID, plot string,
	original *siviParentProjection, edits []siviParentScalarEdit, approve siviCHARSParentEntryApproval,
	lifecycle sourceParentEntryLifecycle) (*siviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("CHARS entry requires independent source/reference approval")
	}
	var entryPlan []siviParentScalarEdit
	var requestPlots *PlotService
	hooks := siviParentWriteHooks{
		authorize: func(plots *PlotService) error {
			requestPlots = plots
			return nil
		},
		read: readSIVICHARSParentForWrite,
		plan: func(tx *sql.Tx, observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			if !sourceParentProjectionStorageEqual(original, observed) {
				return nil, errors.New("CHARS reviewed physical parent storage changed; undo and reload")
			}
			assignments, err := planSIVICHARSParentWritableEntry(ctx, observed, edits)
			if err != nil {
				return nil, err
			}
			for _, assignment := range assignments {
				if assignment.Column == "SV_FloodPlain" && assignment.After.Storage == "null" {
					return nil, errors.New("CHARS source Yes/No converts NULL to No; select No explicitly instead of writing SQLite NULL")
				}
			}
			if err := approve(tx, observed, assignments); err != nil {
				return nil, fmt.Errorf("CHARS entry approval: %w", err)
			}
			entryPlan = make([]siviParentScalarEdit, 0, len(assignments))
			for _, assignment := range assignments {
				entryPlan = append(entryPlan, siviParentScalarEdit{assignment.ContextID, assignment.Table, assignment.RowID,
					assignment.Column, cloneSiteUnitCell(assignment.Before), cloneSiteUnitCell(assignment.After)})
			}
			return assignments, nil
		},
		decorate: func(event *siviParentHistory) {
			event.EntryPlan = entryPlan
			if lifecycle.decorate != nil {
				lifecycle.decorate(event)
			}
		},
	}
	if lifecycle.prepare != nil {
		hooks.prepare = func(conn *sql.Conn) (func(), error) {
			if requestPlots == nil {
				return nil, errors.New("CHARS preparation requires the immutable owned request")
			}
			return lifecycle.prepare(requestPlots, conn)
		}
	}
	if lifecycle.verify != nil {
		hooks.verify = func(tx *sql.Tx) error {
			if requestPlots == nil {
				return errors.New("CHARS verification requires the immutable owned request")
			}
			return lifecycle.verify(requestPlots, tx)
		}
	}
	return s.writeSIVIParentPlannedWithHooks(ctx, contextID, plot, original, siviCHARSParentEntryHistory, nil, hooks)
}

func (s *ContextService) restoreSIVICHARSParentEntry(ctx context.Context, contextID, plot, historyID string,
	action AuditRestoreAction) (*AuditRestoreResult, error) {
	return s.restoreSourceParentHistoryCompared(ctx, contextID, plot, historyID, action,
		siviCHARSParentEntryHistory, readSIVICHARSParentForWrite, nil, sourceParentProjectionStorageEqual)
}
