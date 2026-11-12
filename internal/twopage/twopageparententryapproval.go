package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
)

// Role authorization and preference leases remain the owned writer's responsibility.
func ApproveTwoPageEntryReferences(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext,
	provider *TwoPageEntryReferenceProvider, contextID string, original *sourceform.SiviParentProjection,
	assignments []sourceform.SiviParentScalarAssignment, acknowledgements []TwoPageEntryCodeAcknowledgementInternal,
	selection TwoPageEntryReferenceSelection, aliases TwoPageEntryReferenceAliases) error {
	return approveTwoPageEntryReferencesWithProject(ctx, tx, owner, provider, contextID, original,
		assignments, acknowledgements, selection, aliases, nil)
}

func approveTwoPageEntryReferencesWithProject(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext,
	provider *TwoPageEntryReferenceProvider, contextID string, original *sourceform.SiviParentProjection,
	assignments []sourceform.SiviParentScalarAssignment, acknowledgements []TwoPageEntryCodeAcknowledgementInternal,
	selection TwoPageEntryReferenceSelection, aliases TwoPageEntryReferenceAliases, projectPlan *sourceform.SiviProjectAssignmentPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || owner == nil || provider == nil || original == nil || len(original.Rows) != 1 ||
		original.ContextID != contextID || original.Project != owner.Selection.Project {
		return errors.New("complete-entry reference approval requires the owned transaction and exact original")
	}
	projectParent, err := TwoPageParentProjector(original.Form)
	if err != nil {
		return err
	}
	rebuilt, err := projectParent(ctx, contextID, original.Project, original.Plot,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return errors.Join(err, errors.New("complete-entry reference original differs from its exact source projection"))
	}
	fields, err := twoPageEntryReferenceFields(original.Form)
	if err != nil {
		return err
	}
	if len(provider.Fields) != len(fields) {
		return errors.New("complete-entry reference approval has a foreign provider cohort")
	}
	for _, field := range fields {
		if observed, exists := provider.Fields[field.Column]; !exists || observed != field {
			return errors.New("complete-entry reference approval has a foreign source policy")
		}
	}
	if selection.ProjectSource < 1 || selection.ProjectSource > 2 || selection.WorkingSource < 1 || selection.WorkingSource > 3 ||
		(aliases.Project != "project" && aliases.Project != "main") ||
		(aliases.Lists != "VLists" && aliases.Lists != "two_page_entry_refs") ||
		(aliases.Su != "" && aliases.Su != "su" && aliases.Su != "sivi_su" && aliases.Su != "main") {
		return errors.New("complete-entry reference approval requires exact configured source modes")
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return err
	}
	if err := verifyTwoPageEntryReferenceAliases(ctx, tx, owner, map[string]string{aliases.Project: "project", aliases.Lists: "VLists"}); err != nil {
		return err
	}
	edits := []sourceform.SiviParentScalarEdit{}
	projectAssigned := false
	for _, assignment := range assignments {
		if assignment.Column == "ProjectID" {
			if projectAssigned || projectPlan == nil || len(projectPlan.Assignments) != 1 ||
				projectPlan.SourceOption != selection.ProjectSource || !reflect.DeepEqual(assignment, projectPlan.Assignments[0]) {
				return errors.New("complete-entry ProjectID mutation requires its exact independently planned physical source proof")
			}
			projectAssigned = true
		}
		edits = append(edits, sourceform.SiviParentScalarEdit{ContextID: assignment.ContextID, Table: assignment.Table, RowID: assignment.RowID,
			Column: assignment.Column, Expected: assignment.Before, Value: assignment.After})
	}
	if !projectAssigned && projectPlan != nil && len(projectPlan.Assignments) != 0 {
		return errors.New("complete-entry approval has unused ProjectID assignment proof")
	}
	if len(edits) != 0 {
		// Recheck plan integrity, not role authorization, which precedes approval.
		planned, err := PlanTwoPageEntryProjection(ctx, original, edits, true)
		if err != nil || !reflect.DeepEqual(planned, assignments) {
			return errors.Join(err, errors.New("complete-entry reference assignments differ from their independent source plan"))
		}
	}
	filters := map[string]vprocore.ProjectMetadataCell{}
	for _, binding := range original.Bindings {
		if binding.Binding == "Zone" || binding.Binding == "SubZone" {
			filters[binding.Binding] = vprocore.CloneSiteUnitCell(original.Rows[0].Env.Cells[binding.Column])
		}
	}
	for _, assignment := range assignments {
		if assignment.Column == "Zone" || assignment.Column == "SubZone" {
			filters[assignment.Column] = vprocore.CloneSiteUnitCell(assignment.After)
		}
	}
	if len(filters) != 2 {
		return errors.New("complete-entry reference approval requires both original BEC filter fields")
	}
	read := func(ctx context.Context, tx *sql.Tx, field TwoPageEntryReferenceField) (sourceform.SIVIParentSharedReference, error) {
		switch field.Reader {
		case "project", "master-unit", "working-unit":
			return ReadTwoPageEntryContextReference(ctx, tx, owner, contextID, original.Form, selection, aliases, field)
		default:
			return provider.Read(ctx, tx, aliases.Lists, field, filters["Zone"], filters["SubZone"])
		}
	}
	if err := ApproveTwoPageEntryRequiredReferences(ctx, tx, original, assignments,
		func(ctx context.Context, tx *sql.Tx, policy sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
			field, exists := provider.Fields[policy.Column]
			if !exists || field.SiviParentSharedReferencePolicy != policy {
				return sourceform.SIVIParentSharedReference{}, errors.New("complete-entry required reference changed its source policy")
			}
			return read(ctx, tx, field)
		}); err != nil {
		return err
	}
	optional := make([]sourceform.SiviParentScalarAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.Column != "ProjectID" {
			optional = append(optional, assignment)
		}
	}
	if err := ApproveTwoPageEntryOptionalReferences(ctx, tx, original, optional, acknowledgements, read); err != nil {
		return err
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return err
	}
	return ctx.Err()
}
