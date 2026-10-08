package main

import (
	"context"
	"fmt"
	"reflect"
)

type SIVIHeightEdit struct {
	RowID    string              `json:"rowId"`
	Form     string              `json:"form"`
	Column   string              `json:"column"`
	Expected ProjectMetadataCell `json:"expected"`
	Value    ProjectMetadataCell `json:"value"`
}

type siviHeightEdit = SIVIHeightEdit

func (edit *siviHeightEdit) UnmarshalJSON(data []byte) error {
	type plain siviHeightEdit
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "rowId", "form", "column", "expected", "value"); err != nil {
		return fmt.Errorf("SIVI height draft transport: %w", err)
	}
	*edit = siviHeightEdit(decoded)
	return nil
}

type siviHeightAssignment struct {
	RowID, Column string
	Before, After ProjectMetadataCell
	Value         any
}

// Planning does not authorize a write or enable the unfinished SIVI editor.
func planSIVIHeightEdits(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []siviHeightEdit) ([]siviHeightAssignment, error) {
	return planSIVIChildEdits(ctx, plot, extended, veg, edits, "height",
		func(group int, column string) bool {
			return (group == 0 && (column == "HeightA" || column == "HeightB")) ||
				(group == 1 && column == "Height6")
		}, validateSIVIHeightCell)
}

func validateSIVIHeightCell(column string, cell ProjectMetadataCell) error {
	if column == "HeightB" {
		value, err := metadataCellValue(cell)
		if err != nil {
			return err
		}
		return validateChildPhysicalText("Veg.HeightB", value, 255)
	}
	return validateSIVIChildSingle(column, cell)
}

func validateSIVIChildSingle(column string, cell ProjectMetadataCell) error {
	if cell.Storage != "null" && cell.Storage != "real" {
		return fmt.Errorf("Veg.%s requires nullable numeric real storage", column)
	}
	return validateSingleRangeChange("Veg."+column, nil, cell.Real)
}

func planSIVIChildEdits(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []siviHeightEdit, kind string, allowed func(int, string) bool, validate func(string, ProjectMetadataCell) error) ([]siviHeightAssignment, error) {
	groups, err := projectSIVIVegetation(ctx, plot, extended, veg)
	if err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("SIVI %s planning requires explicit edits", kind)
	}
	visible := map[string]map[string]ProjectMetadataRow{}
	for _, group := range groups {
		rows := map[string]ProjectMetadataRow{}
		for _, row := range group.Rows {
			rows[row.RowID] = row
		}
		visible[group.Form] = rows
	}
	assignments := []siviHeightAssignment{}
	seen := map[[2]string]bool{}
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := [2]string{edit.RowID, edit.Column}
		row, present := visible[edit.Form][edit.RowID]
		groupIndex := -1
		for i, group := range groups {
			if group.Form == edit.Form {
				groupIndex = i
				break
			}
		}
		if !present || groupIndex < 0 || !allowed(groupIndex, edit.Column) || seen[key] {
			return nil, fmt.Errorf("SIVI %s edit %s.%s is repeated or unavailable in source form %q", kind, edit.RowID, edit.Column, edit.Form)
		}
		seen[key] = true
		group := groups[groupIndex]
		var before ProjectMetadataCell
		for i, column := range group.Columns {
			if column == edit.Column {
				before = row.Cells[i]
				break
			}
		}
		if _, err := metadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("SIVI %s expected value: %w", kind, err)
		}
		if !reflect.DeepEqual(before, edit.Expected) {
			return nil, fmt.Errorf("SIVI %s source value changed; cancel and reload", kind)
		}
		value, err := metadataCellValue(edit.Value)
		if err != nil {
			return nil, fmt.Errorf("SIVI %s value: %w", kind, err)
		}
		if reflect.DeepEqual(before, edit.Value) {
			continue
		}
		if err := validate(edit.Column, edit.Value); err != nil {
			return nil, err
		}
		assignments = append(assignments, siviHeightAssignment{
			edit.RowID, edit.Column, cloneSiteUnitCell(before), cloneSiteUnitCell(edit.Value), value})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}
