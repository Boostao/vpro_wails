package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var siviCHARSParentSpeciesAction = sync.OnceValues(func() (string, error) {
	layout, err := siviCHARSParentSourceLayout()
	if err != nil {
		return "", err
	}
	control := ""
	start, end := 0, 0
	for _, field := range layout.Forms[0].Fields {
		if field.ControlName != "optSpeciesListComplete" {
			continue
		}
		if control != "" || field.ControlID != "form:frmSIVIsite-CHARS/optSpeciesListComplete" ||
			field.Type != "OptionGroup" || field.Binding != "" || !field.ReadOnly || field.Implementation != "unmapped" {
			return "", errors.New("CHARS Species action requires its exact unbound source option group")
		}
		resolved := 0
		for _, event := range field.Events {
			if event.Property == "AfterUpdate" && event.Binding == "[Event Procedure]" &&
				event.Procedure == "optSpeciesListComplete_AfterUpdate" && event.Resolved {
				resolved++
			}
		}
		if resolved != 1 {
			return "", errors.New("CHARS Species action requires its resolved original AfterUpdate")
		}
		control = field.ControlID
		start, end = field.Line, field.EndLine
	}
	if control == "" {
		return "", errors.New("CHARS Species source action is missing")
	}
	options := 0
	for _, field := range layout.Forms[0].Fields {
		if field.Type != "OptionButton" || field.Line <= start || field.EndLine >= end {
			continue
		}
		expected := map[string]string{"Option618": "1", "Option620": "2"}[field.ControlName]
		caption := map[string]string{"1": "Comp.", "2": "Part."}[expected]
		value, present := field.Properties["OptionValue"]
		if expected == "" || !present || value.Value != expected || field.Caption != caption {
			return "", errors.New("CHARS Species action lost its original Comp./Part. option values")
		}
		options++
	}
	if options != 2 {
		return "", errors.New("CHARS Species action requires both original option buttons")
	}
	return control, nil
})

func planSIVICHARSParentActions(ctx context.Context, original *siviParentProjection,
	actions []siviParentActionEdit) ([]siviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || original.Form != siviCHARSParentForm || len(original.Rows) != 1 || len(actions) == 0 {
		return nil, errors.New("CHARS source actions require one owned physical pair and explicit options")
	}
	control, err := siviCHARSParentSpeciesAction()
	if err != nil {
		return nil, err
	}
	edits := make([]siviParentScalarEdit, 0, len(actions))
	for _, action := range actions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if action.ControlID != control || action.Option == nil || *action.Option != 1 && *action.Option != 2 {
			return nil, errors.New("CHARS Species action requires its source control and selected Comp./Part. option1/2")
		}
		value := "0"
		if *action.Option == 1 {
			value = "-1"
		}
		edits = append(edits, siviParentScalarEdit{action.ContextID, action.Table, action.RowID,
			"SpeciesListComplete", action.Expected, ProjectMetadataCell{Storage: "integer", Integer: &value}})
	}
	return planProjectedParentCells(ctx, original.ContextID, original.Project, original,
		edits, map[string]string{"SpeciesListComplete": "Env"}, map[string]bool{"SpeciesListComplete": true},
		func(_ string, value ProjectMetadataCell) error {
			return validateSIVIParentScalar("SV_FloodPlain", value)
		})
}

func planSIVICHARSParentProposal(ctx context.Context, original *siviParentProjection,
	edits []siviParentScalarEdit, actions []siviParentActionEdit) ([]siviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 && len(actions) == 0 {
		return nil, errors.New("CHARS entry requires explicit direct edits or qualified source actions")
	}
	var assignments []siviParentScalarAssignment
	if len(edits) != 0 {
		planned, err := planSIVICHARSParentWritableEntry(ctx, original, edits)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, planned...)
	}
	if len(actions) != 0 {
		planned, err := planSIVICHARSParentActions(ctx, original, actions)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, planned...)
	}
	return assignments, nil
}

func planSIVICHARSParentHistoryPlan(ctx context.Context, original *siviParentProjection,
	plan []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	direct := make([]siviParentScalarEdit, 0, len(plan))
	var actions []siviParentActionEdit
	for _, edit := range plan {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if edit.Column != "SpeciesListComplete" {
			direct = append(direct, edit)
			continue
		}
		control, err := siviCHARSParentSpeciesAction()
		if err != nil {
			return nil, err
		}
		if _, err := metadataCellValue(edit.Value); err != nil {
			return nil, fmt.Errorf("CHARS Species history storage: %w", err)
		}
		if edit.Value.Storage != "integer" || edit.Value.Integer == nil ||
			*edit.Value.Integer != "-1" && *edit.Value.Integer != "0" {
			return nil, fmt.Errorf("CHARS Species history requires selected Comp./Part. storage, not %s", edit.Value.Storage)
		}
		option := 2
		if *edit.Value.Integer == "-1" {
			option = 1
		}
		actions = append(actions, siviParentActionEdit{edit.ContextID, control, edit.Table, edit.RowID, edit.Expected, &option})
	}
	return planSIVICHARSParentProposal(ctx, original, direct, actions)
}
