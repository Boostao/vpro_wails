package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type twoPageEntryApproval func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error

type twoPageEntryLifecycle struct {
	prepare  func(*PlotService, *sql.Conn) (func(), error)
	verify   func(*PlotService, *sql.Tx) error
	decorate func(*siviParentHistory)
}

func twoPageEntryHistory(form string, masterAllowed bool) (siviParentHistoryDomain, error) {
	return twoPageEntryHistoryAuthority(form, func() bool { return masterAllowed })
}

func twoPageEntryHistoryAuthority(form string, masterAllowed func() bool) (siviParentHistoryDomain, error) {
	history, err := twoPageParentHistory(form, "Entry", "two-page complete entry",
		func(ctx context.Context, original *siviParentProjection, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
			return planTwoPageEntryProjection(ctx, original, edits, masterAllowed())
		})
	if err != nil {
		return siviParentHistoryDomain{}, err
	}
	history.assignments = func(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
		return twoPageEntryHistoryAssignments(ctx, form, masterAllowed(), event, project, plot)
	}
	return history, nil
}

// This private boundary requires independent reference/role approval before publication.
func (s *ContextService) writeTwoPageEntry(ctx context.Context, contextID, plot, form string, original *siviParentProjection,
	edits []siviParentScalarEdit, masterAllowed bool, approve twoPageEntryApproval) (*siviParentWriteResult, error) {
	return s.writeTwoPageEntryWithLifecycle(ctx, contextID, plot, form, original, edits, masterAllowed, approve, twoPageEntryLifecycle{})
}

func (s *ContextService) writeTwoPageEntryWithLifecycle(ctx context.Context, contextID, plot, form string, original *siviParentProjection,
	edits []siviParentScalarEdit, masterAllowed bool, approve twoPageEntryApproval, lifecycle twoPageEntryLifecycle) (*siviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("two-page complete entry requires independent reference and role approval")
	}
	authorizedMaster := false
	history, err := twoPageEntryHistoryAuthority(form, func() bool { return authorizedMaster })
	if err != nil {
		return nil, err
	}
	read, err := twoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	var requestPlots *PlotService
	var entryPlan []siviParentScalarEdit
	hooks := siviParentWriteHooks{
		authorize: func(plots *PlotService) error {
			authorizedMaster = masterAllowed && masterBECAllowed(plots.currentUser)
			requestPlots = plots
			return nil
		},
		read: read,
		plan: func(tx *sql.Tx, observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			assignments, err := planTwoPageEntryProjection(ctx, observed, edits, authorizedMaster)
			if err != nil {
				return nil, err
			}
			if err := approve(tx, observed, assignments); err != nil {
				return nil, fmt.Errorf("two-page complete entry approval: %w", err)
			}
			entryPlan = make([]siviParentScalarEdit, 0, len(assignments))
			for _, assignment := range assignments {
				entryPlan = append(entryPlan, siviParentScalarEdit{assignment.ContextID, assignment.Table, assignment.RowID,
					assignment.Column, cloneSiteUnitCell(assignment.Before), cloneSiteUnitCell(assignment.After)})
			}
			return assignments, nil
		},
	}
	if lifecycle.prepare != nil {
		hooks.prepare = func(conn *sql.Conn) (func(), error) {
			if requestPlots == nil {
				return nil, errors.New("complete-entry preparation requires the immutable owned request")
			}
			return lifecycle.prepare(requestPlots, conn)
		}
	}
	if lifecycle.verify != nil {
		hooks.verify = func(tx *sql.Tx) error {
			if requestPlots == nil {
				return errors.New("complete-entry verification requires the immutable owned request")
			}
			return lifecycle.verify(requestPlots, tx)
		}
	}
	hooks.decorate = func(event *siviParentHistory) {
		event.EntryPlan = entryPlan
		if lifecycle.decorate != nil {
			lifecycle.decorate(event)
		}
	}
	return s.writeSIVIParentPlannedWithHooks(ctx, contextID, plot, original, history, nil, hooks)
}

func (s *ContextService) restoreTwoPageEntry(ctx context.Context, contextID, plot, form, historyID string,
	action AuditRestoreAction, masterAllowed bool) (*AuditRestoreResult, error) {
	authorizedMaster := false
	history, err := twoPageEntryHistoryAuthority(form, func() bool { return authorizedMaster })
	if err != nil {
		return nil, err
	}
	read, err := twoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := s.restoreSourceParentHistoryAuthorized(ctx, contextID, plot, historyID, action, history, read,
		func(plots *PlotService) error {
			authorizedMaster = masterAllowed && masterBECAllowed(plots.currentUser)
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("two-page complete entry restoration: %w", err)
	}
	return result, nil
}
