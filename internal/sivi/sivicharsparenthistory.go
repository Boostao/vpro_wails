package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
)

import (
	"context"
	"errors"
)

const siviCHARSParentEntryHistorySQL = `CREATE TABLE "__VPRO_SIVICHARSParentEntryHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviCHARSParentEntryHistory = sourceform.SiviParentHistoryDomain{
	Table: sourceform.SiviCHARSParentEntryHistoryTable, Schema: siviCHARSParentEntryHistorySQL, Label: "CHARS parent entry", Assignments: siviCHARSParentEntryHistoryAssignments,
}

func siviCHARSParentEntryHistoryAssignments(ctx context.Context, event sourceform.SiviParentHistory, project, plot string) ([]sourceform.SiviParentScalarAssignment, error) {
	if event.ProjectAssignment != nil || event.EntryPlan == nil || event.Original == nil || event.Committed == nil ||
		event.Original.Form != siviCHARSParentForm || event.Committed.Form != siviCHARSParentForm {
		return nil, errors.New("CHARS typed entry history requires its exact source form and full plan without ProjectID provenance")
	}
	original, err := sourceform.ValidateSourceParentHistoryOriginal(ctx, event, project, plot, projectSIVICHARSParent)
	if err != nil {
		return nil, err
	}
	auditedEdits := make([]sourceform.SiviParentScalarEdit, 0, len(event.Changes))
	for _, change := range event.Changes {
		auditedEdits = append(auditedEdits, sourceform.SiviParentScalarEdit{original.ContextID, change.Table, change.RowID,
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
	if err := sourceform.VerifySourceParentAuditedSubset(audited, planned, "CHARS entry"); err != nil {
		return nil, err
	}
	if err := sourceform.VerifySourceParentEntryPlan(ctx, original, event.Committed, planned, projectSIVICHARSParent,
		sourceform.SourceParentProjectionStorageEqual, "CHARS entry"); err != nil {
		return nil, err
	}
	return audited, nil
}
