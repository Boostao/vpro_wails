package main

import (
	"context"
	"errors"
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
	groups, err := projectSIVIVegetation(ctx, plot, extended, veg)
	if err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI height planning requires explicit edits")
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
		allowed := (edit.Form == groups[0].Form && (edit.Column == "HeightA" || edit.Column == "HeightB")) ||
			(edit.Form == groups[1].Form && edit.Column == "Height6")
		if !present || !allowed || seen[key] {
			return nil, fmt.Errorf("SIVI height edit %s.%s is repeated or unavailable in source form %q", edit.RowID, edit.Column, edit.Form)
		}
		seen[key] = true
		group := groups[0]
		if edit.Form == groups[1].Form {
			group = groups[1]
		}
		var before ProjectMetadataCell
		for i, column := range group.Columns {
			if column == edit.Column {
				before = row.Cells[i]
				break
			}
		}
		if _, err := metadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("SIVI height expected value: %w", err)
		}
		if !reflect.DeepEqual(before, edit.Expected) {
			return nil, errors.New("SIVI height source value changed; cancel and reload")
		}
		value, err := metadataCellValue(edit.Value)
		if err != nil {
			return nil, fmt.Errorf("SIVI height value: %w", err)
		}
		if reflect.DeepEqual(before, edit.Value) {
			continue
		}
		if edit.Column == "HeightB" {
			if err := validateChildPhysicalText("Veg.HeightB", value, 255); err != nil {
				return nil, err
			}
		} else {
			if edit.Value.Storage != "null" && edit.Value.Storage != "real" {
				return nil, fmt.Errorf("Veg.%s requires nullable numeric real storage", edit.Column)
			}
			if err := validateSingleRangeChange("Veg."+edit.Column, nil, edit.Value.Real); err != nil {
				return nil, err
			}
		}
		assignments = append(assignments, siviHeightAssignment{
			edit.RowID, edit.Column, cloneSiteUnitCell(before), cloneSiteUnitCell(edit.Value), value})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}
