package twopage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func PlanTwoPageEntryProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool) ([]sourceform.SiviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || len(edits) == 0 {
		return nil, errors.New("two-page entry planning requires a reviewed source original and explicit edits")
	}
	xl, err := twoPageXLFields(original.Form)
	if err != nil {
		return nil, err
	}
	var xlEdits, commonEdits, extraEdits []sourceform.SiviParentScalarEdit
	for _, edit := range edits {
		_, inXL := xl[edit.Column]
		_, inCommon := TwoPageParentCommonFields[edit.Column]
		_, inExtra := sourceform.TwoPageParentExtraFields[edit.Column]
		owners := 0
		for _, belongs := range []bool{inXL, inCommon, inExtra} {
			if belongs {
				owners++
			}
		}
		if owners != 1 {
			return nil, fmt.Errorf("two-page entry %s requires exactly one accepted field scope", edit.Column)
		}
		switch {
		case inXL:
			xlEdits = append(xlEdits, edit)
		case inCommon:
			commonEdits = append(commonEdits, edit)
		case inExtra:
			extraEdits = append(extraEdits, edit)
		}
	}
	assignments := []sourceform.SiviParentScalarAssignment{}
	for _, scope := range []struct {
		edits []sourceform.SiviParentScalarEdit
		plan  func(context.Context, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error)
	}{
		{xlEdits, func(ctx context.Context, parent *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
			return planTwoPageXLProjection(ctx, parent, edits, masterAllowed)
		}},
		{commonEdits, PlanTwoPageParentCommonProjection},
		{extraEdits, PlanTwoPageParentExtraProjection},
	} {
		if len(scope.edits) == 0 {
			continue
		}
		planned, err := scope.plan(ctx, original, scope.edits)
		if err != nil {
			return nil, fmt.Errorf("two-page complete entry planning: %w", err)
		}
		assignments = append(assignments, planned...)
	}
	return assignments, nil
}

func twoPageEntryHistoryAssignments(ctx context.Context, form string, masterAllowed bool,
	event sourceform.SiviParentHistory, project, plot string) ([]sourceform.SiviParentScalarAssignment, error) {
	if event.Original == nil || event.Committed == nil || event.Original.Form != form || event.Committed.Form != form {
		return nil, errors.New("complete-entry history belongs to another exact source form")
	}
	projectParent, err := TwoPageParentProjector(form)
	if err != nil {
		return nil, err
	}
	original, err := sourceform.ValidateSourceParentHistoryOriginal(ctx, event, project, plot, projectParent)
	if err != nil {
		return nil, err
	}
	edits := make([]sourceform.SiviParentScalarEdit, 0, len(event.Changes))
	for _, change := range event.Changes {
		edits = append(edits, sourceform.SiviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
	}
	audited, err := PlanTwoPageEntryProjection(ctx, original, edits, masterAllowed)
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
	planned, err := PlanTwoPageEntryProjection(ctx, original, event.EntryPlan, true)
	if err != nil || len(planned) != len(event.EntryPlan) {
		return nil, errors.Join(err, errors.New("complete-entry history has a repeated, unchanged or malformed full source plan"))
	}
	if err := sourceform.VerifySourceParentAuditedSubset(audited, planned, "complete-entry"); err != nil {
		return nil, err
	}
	var projectChange *sourceform.SiviParentHistoryChange
	for _, assignment := range planned {
		if assignment.Column == "ProjectID" {
			projectChange = &sourceform.SiviParentHistoryChange{Table: assignment.Table, RowID: assignment.RowID, Column: assignment.Column,
				Before: assignment.Before, After: assignment.After}
		}
	}
	if projectChange == nil {
		if event.ProjectAssignment != nil {
			return nil, errors.New("complete-entry history has unused ProjectID source provenance")
		}
	} else {
		projectPlan, err := sourceform.ValidateSourceProjectAssignmentHistory(ctx, original, *projectChange, event.ProjectAssignment, "form:"+form+"/ProjectID")
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
	if err := sourceform.VerifySourceParentEntryPlan(ctx, original, event.Committed, planned, projectParent,
		func(left, right *sourceform.SiviParentProjection) bool { return reflect.DeepEqual(left, right) }, "complete-entry"); err != nil {
		return nil, err
	}
	return audited, nil
}

type TwoPageEntryFieldPolicy struct {
	Column  string `json:"column"`
	Owner   string `json:"owner"`
	Kind    string `json:"kind"`
	Maximum int    `json:"maximum"`
}

func TwoPageEntryFieldPolicies(form string) ([]TwoPageEntryFieldPolicy, error) {
	bindings, err := twoPageParentBindings(form)
	if err != nil {
		return nil, err
	}
	xl, err := twoPageXLFields(form)
	if err != nil {
		return nil, err
	}
	commonTextBounds := map[string]int{"SV_PolygonNumber": 25, "SV_CanopyComposition": 50,
		"SV_RootZoneTexture": 100, "SV_AhorizonType": 5, "PlotType": 10}
	result := []TwoPageEntryFieldPolicy{}
	seen := map[string]bool{}
	for _, binding := range bindings {
		column := binding.Binding
		if seen[column] || column == "PlotNumber" {
			continue
		}
		seen[column] = true
		policy := TwoPageEntryFieldPolicy{Column: column}
		switch {
		case xl[column].kind != "":
			field := xl[column]
			policy.Owner, policy.Kind, policy.Maximum = field.owner, field.kind, field.maximum
		case TwoPageParentCommonFields[column].Domain != "":
			field := TwoPageParentCommonFields[column]
			policy.Owner, policy.Kind = field.Owner, field.Domain
			switch field.Domain {
			case "scalar":
				policy.Kind = "single"
				if column == "SV_FloodPlain" {
					policy.Kind = "boolean"
				}
			case "text", "categorical", "plot-type":
				policy.Kind, policy.Maximum = field.Domain, commonTextBounds[column]
				if policy.Maximum == 0 {
					return nil, fmt.Errorf("complete-entry %s has no reviewed common text bound", column)
				}
			}
		case sourceform.TwoPageParentExtraFields[column].Domain != "":
			field := sourceform.TwoPageParentExtraFields[column]
			policy.Owner, policy.Kind, policy.Maximum = field.Owner, field.Domain, field.Maximum
		default:
			return nil, fmt.Errorf("complete-entry %s has no reviewed editable domain", column)
		}
		result = append(result, policy)
	}
	expected := 117
	if form == "FS882-8x6XL-CHARS" {
		expected = 119
	}
	if len(result) != expected {
		return nil, errors.New("complete-entry editable policy cohort changed")
	}
	return result, nil
}

func PlanTwoPageEntryProjectAssignment(ctx context.Context, original *sourceform.SiviParentProjection,
	choices sourceform.SIVIProjectChoices, selection sourceform.SiviProjectSelection) (*sourceform.SiviProjectAssignmentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("complete-entry ProjectID selection requires one owned source original")
	}
	projectParent, err := TwoPageParentProjector(original.Form)
	if err != nil {
		return nil, err
	}
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
	rebuilt, err := projectParent(ctx, original.ContextID, original.Project, original.Plot, env, admin)
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return nil, errors.Join(err, errors.New("complete-entry ProjectID original differs from its exact source projection"))
	}
	return sourceform.PlanSourceProjectAssignment(ctx, "form:"+original.Form+"/ProjectID", original.ContextID, original.Project,
		original.Plot, env, admin, choices, selection)
}

// Mandatory membership supplements, but never substitutes for, complete entry approval.
func WriteTwoPageEntryWithRequiredReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool,
	readers sourceform.SiviParentSharedReferenceReaders, approve TwoPageEntryApproval) (*sourceform.SiviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("complete-entry mandatory references require separate full reference and role approval")
	}
	references := &sourceform.SIVIParentSharedService{References: readers}
	lifecycle := TwoPageEntryLifecycle{
		Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
			owner := plots.Projects.Sqlite
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return nil, err
			}
			path, present := owner.Attachments["VLists"]
			if !present || path == "" {
				return nil, errors.New("complete-entry required references need the configured owned VLists family")
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS two_page_entry_refs`, sqlutil.SqliteFileURI(path, "ro")); err != nil {
				return nil, fmt.Errorf("complete-entry reference attachment: %w", err)
			}
			return func() {}, nil
		},
		Verify: func(plots *vprocore.PlotService, _ *sql.Tx) error {
			return vprocore.ProfileOwnedFiles(plots.Projects.Sqlite)
		},
	}
	return WriteTwoPageEntryWithLifecycle(s, ctx, contextID, plot, form, original, edits, masterAllowed,
		func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			read := func(ctx context.Context, tx *sql.Tx, field sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
				return references.ReadReference(ctx, tx, "two_page_entry_refs", field, vprocore.ProjectMetadataCell{Storage: "null"})
			}
			if err := ApproveTwoPageEntryRequiredReferences(ctx, tx, observed, assignments, read); err != nil {
				return err
			}
			return approve(tx, observed, assignments)
		}, lifecycle)
}
