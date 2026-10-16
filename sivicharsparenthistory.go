package main

import (
	"context"
	"errors"
)

const siviCHARSParentEntryHistoryTable = "__VPRO_SIVICHARSParentEntryHistory"
const siviCHARSParentEntryHistorySQL = `CREATE TABLE "__VPRO_SIVICHARSParentEntryHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviCHARSParentEntryHistory = siviParentHistoryDomain{
	siviCHARSParentEntryHistoryTable, siviCHARSParentEntryHistorySQL, "CHARS parent entry", siviCHARSParentEntryHistoryAssignments,
}

func siviCHARSParentEntryHistoryAssignments(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
	if event.ProjectAssignment != nil || event.EntryPlan == nil || event.Original == nil || event.Committed == nil ||
		event.Original.Form != siviCHARSParentForm || event.Committed.Form != siviCHARSParentForm {
		return nil, errors.New("CHARS typed entry history requires its exact source form and full plan without ProjectID provenance")
	}
	original, err := validateSourceParentHistoryOriginal(ctx, event, project, plot, projectSIVICHARSParent)
	if err != nil {
		return nil, err
	}
	auditedEdits := make([]siviParentScalarEdit, 0, len(event.Changes))
	for _, change := range event.Changes {
		auditedEdits = append(auditedEdits, siviParentScalarEdit{original.ContextID, change.Table, change.RowID,
			change.Column, change.Before, change.After})
	}
	audited, err := planSIVICHARSParentHistoryPlan(ctx, original, auditedEdits)
	if err != nil || len(audited) != len(event.Changes) {
		return nil, errors.Join(err, errors.New("CHARS typed history contains repeated or unchanged audited assignments"))
	}
	planned, err := planSIVICHARSParentHistoryPlan(ctx, original, event.EntryPlan)
	if err != nil || len(planned) != len(event.EntryPlan) {
		return nil, errors.Join(err, errors.New("CHARS typed history contains repeated, unchanged or malformed full source assignments"))
	}
	if err := verifySourceParentAuditedSubset(audited, planned, "CHARS entry"); err != nil {
		return nil, err
	}
	if err := verifySourceParentEntryPlan(ctx, original, event.Committed, planned, projectSIVICHARSParent,
		sourceParentProjectionStorageEqual, "CHARS entry"); err != nil {
		return nil, err
	}
	return audited, nil
}
