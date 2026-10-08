package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type SIVIParentCellEdit struct {
	ContextID string              `json:"contextId"`
	Table     string              `json:"table"`
	RowID     string              `json:"rowId"`
	Column    string              `json:"column"`
	Expected  ProjectMetadataCell `json:"expected"`
	Value     ProjectMetadataCell `json:"value"`
}

type siviParentScalarEdit = SIVIParentCellEdit

type siviParentScalarAssignment struct {
	ContextID, Table, RowID, Column string
	Before, After                   ProjectMetadataCell
	Value                           any
}

var siviParentScalarOwners = map[string]string{
	"SV_StandHeight":        "Env",
	"SV_AhorizonDepth":      "Env",
	"SV_GleyingMottlingCM":  "Env",
	"SV_PercentCoarseFrags": "Env",
	"SV_SoilDepth":          "Env",
	"StrataCoverTotal":      "Admin",
	"SV_FloodPlain":         "Env",
}

// Planning preserves the labelled literal membership adaptation. It does not
// authorize a write or resolve the still-unverified full parent lifecycle.
func planSIVIParentScalars(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	return planSIVIParentCells(ctx, contextID, project, plot, env, admin, edits, siviParentScalarOwners, validateSIVIParentScalar)
}

func validateSIVIParentScalar(column string, value ProjectMetadataCell) error {
	if column == "SV_FloodPlain" {
		if value.Storage != "null" &&
			(value.Storage != "integer" || value.Integer == nil || (*value.Integer != "0" && *value.Integer != "-1")) {
			return errors.New("SIVI floodplain requires explicit NULL or Access BOOLEAN0/-1 storage")
		}
		return nil
	}
	if value.Storage != "null" && (value.Storage != "real" || value.Real == nil) {
		return fmt.Errorf("SIVI %s requires nullable real storage", column)
	}
	return validateSingleRangeChange(column, nil, value.Real)
}

func planSIVIParentCells(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit, owners map[string]string, validate func(string, ProjectMetadataCell) error) ([]siviParentScalarAssignment, error) {
	return planSIVIParentSourceCells(ctx, contextID, project, plot, env, admin, edits, owners, nil, validate)
}

func planSIVIParentSourceCells(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit, owners map[string]string, implicit map[string]bool, validate func(string, ProjectMetadataCell) error) ([]siviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID == "" || project == "" || len(edits) == 0 {
		return nil, errors.New("SIVI parent planning requires owned context/project and explicit edits")
	}
	parent, err := projectSIVIParent(ctx, contextID, project, plot, env, admin)
	if err != nil {
		return nil, err
	}
	return planProjectedParentCells(ctx, contextID, project, parent, edits, owners, implicit, validate)
}

func planProjectedParentCells(ctx context.Context, contextID, project string, parent *siviParentProjection, edits []siviParentScalarEdit, owners map[string]string, implicit map[string]bool, validate func(string, ProjectMetadataCell) error) ([]siviParentScalarAssignment, error) {
	if len(parent.Rows) != 1 {
		return nil, errors.New("SIVI parent planning requires one unambiguous physical Env/Admin pair")
	}
	bindings := map[string]siviParentBinding{}
	for _, binding := range parent.Bindings {
		bindings[binding.Binding] = binding
	}
	assignments := []siviParentScalarAssignment{}
	seen := map[[2]string]bool{}
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		suffix, allowed := owners[edit.Column]
		binding, bound := bindings[edit.Column]
		key := [2]string{edit.Table, edit.Column}
		if edit.ContextID != contextID || !allowed || !bound || (binding.Implicit && !implicit[edit.Column]) ||
			edit.Table != project+"_"+suffix || binding.Table != edit.Table || seen[key] {
			return nil, fmt.Errorf("SIVI scalar target %s.%s is stale, repeated or outside the source scope", edit.Table, edit.Column)
		}
		seen[key] = true
		row := parent.Rows[0].Env
		if suffix == "Admin" {
			row = parent.Rows[0].Admin
		}
		if edit.RowID != row.RowID {
			return nil, errors.New("SIVI scalar edit requires the exact reviewed physical parent row")
		}
		before := row.Cells[binding.Column]
		if _, err := metadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("SIVI scalar expected value: %w", err)
		}
		if !reflect.DeepEqual(before, edit.Expected) {
			return nil, errors.New("SIVI scalar source value changed; reload before planning")
		}
		value, err := metadataCellValue(edit.Value)
		if err != nil {
			return nil, fmt.Errorf("SIVI scalar proposed value: %w", err)
		}
		if reflect.DeepEqual(before, edit.Value) {
			continue
		}
		if err := validate(edit.Column, edit.Value); err != nil {
			return nil, err
		}
		assignments = append(assignments, siviParentScalarAssignment{
			ContextID: contextID, Table: edit.Table, RowID: row.RowID, Column: edit.Column,
			Before: cloneSiteUnitCell(before), After: cloneSiteUnitCell(edit.Value), Value: value,
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}

func planSIVIParentOptions(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	owners := map[string]string{"SV_StandAgeEstMeas": "Env", "SV_StandHeightEstMeas": "Env"}
	return planSIVIParentCells(ctx, contextID, project, plot, env, admin, edits, owners, validateSIVIParentOption)
}

func validateSIVIParentOption(column string, value ProjectMetadataCell) error {
	if value.Storage != "null" && (value.Storage != "text" || value.Text == nil || (*value.Text != "1" && *value.Text != "2")) {
		return fmt.Errorf("SIVI %s requires explicit NULL or source option TEXT1/2", column)
	}
	return nil
}
