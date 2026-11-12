package sivi

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

type siviCHARSParentEntryApproval func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error

func readSIVICHARSParentForWrite(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext,
	suAlias, contextID, plot string) (*sourceform.SiviParentProjection, error) {
	return sourceform.ReadSourceParentForWrite(ctx, tx, owner, suAlias, contextID, plot, projectSIVICHARSParent)
}

func planSIVICHARSParentWritableEntry(ctx context.Context, original *sourceform.SiviParentProjection,
	edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
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

func WriteSIVICHARSParentEntry(s *vprocore.ContextService, ctx context.Context, contextID, plot string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, approve siviCHARSParentEntryApproval) (*sourceform.SiviParentWriteResult, error) {
	return WriteSIVICHARSParentEntryWithLifecycle(s, ctx, contextID, plot, original, edits, approve, sourceform.SourceParentEntryLifecycle{})
}

func WriteSIVICHARSParentEntryWithLifecycle(s *vprocore.ContextService, ctx context.Context, contextID, plot string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, approve siviCHARSParentEntryApproval,
	lifecycle sourceform.SourceParentEntryLifecycle) (*sourceform.SiviParentWriteResult, error) {
	return WriteSIVICHARSParentProposal(s, ctx, contextID, plot, original, edits, nil, approve, lifecycle)
}

func WriteSIVICHARSParentProposal(s *vprocore.ContextService, ctx context.Context, contextID, plot string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, actions []sourceform.SiviParentActionEdit,
	approve siviCHARSParentEntryApproval, lifecycle sourceform.SourceParentEntryLifecycle) (*sourceform.SiviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("CHARS entry requires independent source/reference approval")
	}
	var entryPlan []sourceform.SiviParentScalarEdit
	var requestPlots *vprocore.PlotService
	hooks := sourceform.SiviParentWriteHooks{
		Authorize: func(plots *vprocore.PlotService) error {
			requestPlots = plots
			return nil
		},
		Read: readSIVICHARSParentForWrite,
		Plan: func(tx *sql.Tx, observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
			if !sourceform.SourceParentProjectionStorageEqual(original, observed) {
				return nil, errors.New("CHARS reviewed physical parent storage changed; undo and reload")
			}
			assignments, err := planSIVICHARSParentProposal(ctx, observed, edits, actions)
			if err != nil {
				return nil, err
			}
			if err := sourceform.ValidateSourceParentBooleanAssignments(assignments); err != nil {
				return nil, fmt.Errorf("CHARS entry: %w", err)
			}
			if err := approve(tx, observed, assignments); err != nil {
				return nil, fmt.Errorf("CHARS entry approval: %w", err)
			}
			entryPlan = make([]sourceform.SiviParentScalarEdit, 0, len(assignments))
			for _, assignment := range assignments {
				entryPlan = append(entryPlan, sourceform.SiviParentScalarEdit{assignment.ContextID, assignment.Table, assignment.RowID,
					assignment.Column, vprocore.CloneSiteUnitCell(assignment.Before), vprocore.CloneSiteUnitCell(assignment.After)})
			}
			return assignments, nil
		},
		Decorate: func(event *sourceform.SiviParentHistory) {
			event.EntryPlan = entryPlan
			if lifecycle.Decorate != nil {
				lifecycle.Decorate(event)
			}
		},
	}
	if lifecycle.Prepare != nil {
		hooks.Prepare = func(conn *sql.Conn) (func(), error) {
			if requestPlots == nil {
				return nil, errors.New("CHARS preparation requires the immutable owned request")
			}
			return lifecycle.Prepare(requestPlots, conn)
		}
	}
	if lifecycle.Verify != nil {
		hooks.Verify = func(tx *sql.Tx) error {
			if requestPlots == nil {
				return errors.New("CHARS verification requires the immutable owned request")
			}
			return lifecycle.Verify(requestPlots, tx)
		}
	}
	return sourceform.WriteSIVIParentPlannedWithHooks(s, ctx, contextID, plot, original, siviCHARSParentEntryHistory, nil, hooks)
}

func RestoreSIVICHARSParentEntry(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string,
	action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	return sourceform.RestoreSourceParentHistoryCompared(s, ctx, contextID, plot, historyID, action,
		siviCHARSParentEntryHistory, readSIVICHARSParentForWrite, nil, sourceform.SourceParentProjectionStorageEqual)
}
