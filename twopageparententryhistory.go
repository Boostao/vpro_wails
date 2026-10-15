package main

import (
	"context"
	"errors"
	"reflect"
)

func twoPageEntryProjectHistoryTable(table string) bool {
	return table == "__VPRO_TwoPageParentEntryHistory" || table == "__VPRO_TwoPageCHARSParentEntryHistory"
}

func twoPageEntryHistoryAssignments(ctx context.Context, form string, masterAllowed bool,
	event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
	if event.Original == nil || event.Committed == nil || event.Original.Form != form || event.Committed.Form != form {
		return nil, errors.New("complete-entry history belongs to another exact source form")
	}
	projectParent, err := twoPageParentProjector(form)
	if err != nil {
		return nil, err
	}
	original, err := validateSourceParentHistoryOriginal(ctx, event, project, plot, projectParent)
	if err != nil {
		return nil, err
	}
	edits := make([]siviParentScalarEdit, 0, len(event.Changes))
	for _, change := range event.Changes {
		edits = append(edits, siviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
	}
	audited, err := planTwoPageEntryProjection(ctx, original, edits, masterAllowed)
	if err != nil || len(audited) != len(event.Changes) {
		return nil, errors.Join(err, errors.New("complete-entry history contains repeated or unchanged source assignments"))
	}
	if event.EntryPlan == nil {
		if event.ProjectAssignment != nil {
			return nil, errors.New("complete-entry ProjectID provenance requires the full committed source plan")
		}
		for _, assignment := range audited {
			if assignment.Column == "ProjectID" {
				return nil, errors.New("complete-entry ProjectID history lacks physical source provenance")
			}
		}
		return audited, nil
	}
	// Full plan integrity is separate from permission to restore audited cells.
	planned, err := planTwoPageEntryProjection(ctx, original, event.EntryPlan, true)
	if err != nil || len(planned) != len(event.EntryPlan) {
		return nil, errors.Join(err, errors.New("complete-entry history has a repeated, unchanged or malformed full source plan"))
	}
	if err := verifySourceParentAuditedSubset(audited, planned, "complete-entry"); err != nil {
		return nil, err
	}
	var projectChange *siviParentHistoryChange
	for _, assignment := range planned {
		if assignment.Column == "ProjectID" {
			projectChange = &siviParentHistoryChange{Table: assignment.Table, RowID: assignment.RowID, Column: assignment.Column,
				Before: assignment.Before, After: assignment.After}
		}
	}
	if projectChange == nil {
		if event.ProjectAssignment != nil {
			return nil, errors.New("complete-entry history has unused ProjectID source provenance")
		}
	} else {
		projectPlan, err := validateSourceProjectAssignmentHistory(ctx, original, *projectChange, event.ProjectAssignment, "form:"+form+"/ProjectID")
		if err != nil {
			return nil, err
		}
		found := false
		for _, assignment := range planned {
			if assignment.Column == "ProjectID" {
				found = reflect.DeepEqual(assignment, projectPlan[0])
			}
		}
		if !found {
			return nil, errors.New("complete-entry ProjectID source proof differs from its mixed assignment")
		}
	}
	if err := verifySourceParentEntryPlan(ctx, original, event.Committed, planned, projectParent,
		func(left, right *siviParentProjection) bool { return reflect.DeepEqual(left, right) }, "complete-entry"); err != nil {
		return nil, err
	}
	return audited, nil
}
