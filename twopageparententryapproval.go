package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
)

// Role authorization and preference leases remain the owned writer's responsibility.
func approveTwoPageEntryReferences(ctx context.Context, tx *sql.Tx, owner *sqliteContext,
	provider *twoPageEntryReferenceProvider, contextID string, original *siviParentProjection,
	assignments []siviParentScalarAssignment, acknowledgements []twoPageEntryCodeAcknowledgement,
	selection twoPageEntryReferenceSelection, aliases twoPageEntryReferenceAliases) error {
	return approveTwoPageEntryReferencesWithProject(ctx, tx, owner, provider, contextID, original,
		assignments, acknowledgements, selection, aliases, nil)
}

func approveTwoPageEntryReferencesWithProject(ctx context.Context, tx *sql.Tx, owner *sqliteContext,
	provider *twoPageEntryReferenceProvider, contextID string, original *siviParentProjection,
	assignments []siviParentScalarAssignment, acknowledgements []twoPageEntryCodeAcknowledgement,
	selection twoPageEntryReferenceSelection, aliases twoPageEntryReferenceAliases, projectPlan *siviProjectAssignmentPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || owner == nil || provider == nil || original == nil || len(original.Rows) != 1 ||
		original.ContextID != contextID || original.Project != owner.selection.Project {
		return errors.New("complete-entry reference approval requires the owned transaction and exact original")
	}
	projectParent, err := twoPageParentProjector(original.Form)
	if err != nil {
		return err
	}
	rebuilt, err := projectParent(ctx, contextID, original.Project, original.Plot,
		ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}},
		ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return errors.Join(err, errors.New("complete-entry reference original differs from its exact source projection"))
	}
	fields, err := twoPageEntryReferenceFields(original.Form)
	if err != nil {
		return err
	}
	if len(provider.fields) != len(fields) {
		return errors.New("complete-entry reference approval has a foreign provider cohort")
	}
	for _, field := range fields {
		if observed, exists := provider.fields[field.column]; !exists || observed != field {
			return errors.New("complete-entry reference approval has a foreign source policy")
		}
	}
	if selection.projectSource < 1 || selection.projectSource > 2 || selection.workingSource < 1 || selection.workingSource > 3 ||
		(aliases.project != "project" && aliases.project != "main") ||
		(aliases.lists != "VLists" && aliases.lists != "two_page_entry_refs") ||
		(aliases.su != "" && aliases.su != "su" && aliases.su != "sivi_su" && aliases.su != "main") {
		return errors.New("complete-entry reference approval requires exact configured source modes")
	}
	if err := profileOwnedFiles(owner); err != nil {
		return err
	}
	if err := verifyTwoPageEntryReferenceAliases(ctx, tx, owner, map[string]string{aliases.project: "project", aliases.lists: "VLists"}); err != nil {
		return err
	}
	edits := []siviParentScalarEdit{}
	projectAssigned := false
	for _, assignment := range assignments {
		if assignment.Column == "ProjectID" {
			if projectAssigned || projectPlan == nil || len(projectPlan.Assignments) != 1 ||
				projectPlan.SourceOption != selection.projectSource || !reflect.DeepEqual(assignment, projectPlan.Assignments[0]) {
				return errors.New("complete-entry ProjectID mutation requires its exact independently planned physical source proof")
			}
			projectAssigned = true
		}
		edits = append(edits, siviParentScalarEdit{ContextID: assignment.ContextID, Table: assignment.Table, RowID: assignment.RowID,
			Column: assignment.Column, Expected: assignment.Before, Value: assignment.After})
	}
	if !projectAssigned && projectPlan != nil && len(projectPlan.Assignments) != 0 {
		return errors.New("complete-entry approval has unused ProjectID assignment proof")
	}
	if len(edits) != 0 {
		// Recheck plan integrity, not role authorization, which precedes approval.
		planned, err := planTwoPageEntryProjection(ctx, original, edits, true)
		if err != nil || !reflect.DeepEqual(planned, assignments) {
			return errors.Join(err, errors.New("complete-entry reference assignments differ from their independent source plan"))
		}
	}
	filters := map[string]ProjectMetadataCell{}
	for _, binding := range original.Bindings {
		if binding.Binding == "Zone" || binding.Binding == "SubZone" {
			filters[binding.Binding] = cloneSiteUnitCell(original.Rows[0].Env.Cells[binding.Column])
		}
	}
	for _, assignment := range assignments {
		if assignment.Column == "Zone" || assignment.Column == "SubZone" {
			filters[assignment.Column] = cloneSiteUnitCell(assignment.After)
		}
	}
	if len(filters) != 2 {
		return errors.New("complete-entry reference approval requires both original BEC filter fields")
	}
	read := func(ctx context.Context, tx *sql.Tx, field twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
		switch field.reader {
		case "project", "master-unit", "working-unit":
			return readTwoPageEntryContextReference(ctx, tx, owner, contextID, original.Form, selection, aliases, field)
		default:
			return provider.read(ctx, tx, aliases.lists, field, filters["Zone"], filters["SubZone"])
		}
	}
	if err := approveTwoPageEntryRequiredReferences(ctx, tx, original, assignments,
		func(ctx context.Context, tx *sql.Tx, policy siviParentSharedReferencePolicy) (SIVIParentSharedReference, error) {
			field, exists := provider.fields[policy.column]
			if !exists || field.siviParentSharedReferencePolicy != policy {
				return SIVIParentSharedReference{}, errors.New("complete-entry required reference changed its source policy")
			}
			return read(ctx, tx, field)
		}); err != nil {
		return err
	}
	optional := make([]siviParentScalarAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.Column != "ProjectID" {
			optional = append(optional, assignment)
		}
	}
	if err := approveTwoPageEntryOptionalReferences(ctx, tx, original, optional, acknowledgements, read); err != nil {
		return err
	}
	if err := profileOwnedFiles(owner); err != nil {
		return err
	}
	return ctx.Err()
}
