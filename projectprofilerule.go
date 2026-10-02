package main

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

type ProjectPlotProfileRuleDraft struct {
	RowID   string                  `json:"rowId"`
	Changes []ProjectMetadataChange `json:"changes"`
}

type profileRuleDraft = ProjectPlotProfileRuleDraft

type profileRuleAssignment struct {
	rowID  int64
	column string
	value  any
}

func prepareProfileRuleDrafts(original ProjectMetadataTable, drafts []profileRuleDraft) (ProjectMetadataTable, []profileRuleAssignment, error) {
	indices := map[string]int{}
	for index, column := range original.Columns {
		if strings.EqualFold(column.Name, "rowid") || strings.EqualFold(column.Name, "_rowid_") || strings.EqualFold(column.Name, "oid") {
			return ProjectMetadataTable{}, nil, errors.New("profile schema shadows physical row identity")
		}
		if _, present := indices[column.Name]; present {
			return ProjectMetadataTable{}, nil, errors.New("profile rule schema is ambiguous")
		}
		indices[column.Name] = index
	}
	for _, name := range []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"} {
		if _, present := indices[name]; !present {
			return ProjectMetadataTable{}, nil, fmt.Errorf("profile rule schema is missing %s", name)
		}
	}
	planned := ProjectMetadataTable{Columns: original.Columns, Rows: []ProjectMetadataRow{}}
	rows := map[string]int{}
	for index, row := range original.Rows {
		identity, err := strconv.ParseInt(row.RowID, 10, 64)
		_, duplicate := rows[row.RowID]
		if err != nil || strconv.FormatInt(identity, 10) != row.RowID || duplicate || len(row.Cells) != len(original.Columns) {
			return ProjectMetadataTable{}, nil, errors.New("profile draft requires complete distinct physical rows")
		}
		rows[row.RowID] = index
		planned.Rows = append(planned.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: append([]ProjectMetadataCell{}, row.Cells...)})
	}
	seen := map[string]bool{}
	assignments := []profileRuleAssignment{}
	for _, draft := range drafts {
		index, present := rows[draft.RowID]
		if !present || seen[draft.RowID] {
			return ProjectMetadataTable{}, nil, errors.New("profile draft repeats or invents a physical row")
		}
		seen[draft.RowID] = true
		fields := map[string]bool{}
		for _, change := range draft.Changes {
			column, present := indices[change.Column]
			if !present || !slices.Contains([]string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation"}, change.Column) || fields[change.Column] {
				return ProjectMetadataTable{}, nil, errors.New("profile draft repeats or changes an unavailable field")
			}
			fields[change.Column] = true
			before := original.Rows[index].Cells[column]
			if reflect.DeepEqual(before, change.Value) {
				continue
			}
			value, err := metadataCellValue(change.Value)
			if err != nil {
				return ProjectMetadataTable{}, nil, fmt.Errorf("profile.%s: %w", change.Column, err)
			}
			switch change.Column {
			case "Order":
				if value != nil {
					number, valid := value.(int64)
					if !valid || number < -32768 || number > 32767 {
						return ProjectMetadataTable{}, nil, errors.New("profile Order requires signed16 integer storage or explicit NULL")
					}
				}
			default:
				if value != nil {
					text, valid := value.(string)
					if !valid {
						return ProjectMetadataTable{}, nil, fmt.Errorf("profile.%s requires literal text or explicit NULL", change.Column)
					}
					if err := validateChildPhysicalText("profile."+change.Column, text, 255); err != nil {
						return ProjectMetadataTable{}, nil, err
					}
				}
			}
			rowID, _ := strconv.ParseInt(draft.RowID, 10, 64)
			assignments = append(assignments, profileRuleAssignment{rowID, change.Column, value})
			planned.Rows[index].Cells[column] = change.Value
		}
		table := planned.Rows[index].Cells[indices["Table"]]
		tableName := ""
		if table.Storage == "text" && table.Text != nil {
			tableName = strings.ToLower(*table.Text)
		}
		for _, name := range []string{"Layer", "Operator"} {
			cell := planned.Rows[index].Cells[indices[name]]
			if !fields[name] || reflect.DeepEqual(cell, original.Rows[index].Cells[indices[name]]) || cell.Storage == "null" {
				continue
			}
			choices := []string{"=", ">", "<"}
			if name == "Layer" {
				choices = []string{"SumAll", "SumA", "SumB", "Any", "1", "2", "3", "4", "5", "5a", "5b", "5c", "6", "7"}
			}
			if !slices.Contains([]string{"env", "veg", "lump"}, tableName) ||
				name == "Layer" && tableName == "env" {
				return ProjectMetadataTable{}, nil, fmt.Errorf("profile.%s has no source list for the selected Table", name)
			}
			if name == "Operator" && tableName == "env" {
				choices = append(choices, "Like", "Not Like")
			}
			if cell.Text == nil || !slices.Contains(choices, *cell.Text) {
				return ProjectMetadataTable{}, nil, fmt.Errorf("profile.%s requires an exact source LimitToList value", name)
			}
		}
		if fields["Table"] && !reflect.DeepEqual(table, original.Rows[index].Cells[indices["Table"]]) &&
			(tableName == "veg" || tableName == "lump") {
			expected := "Species"
			if tableName == "lump" {
				expected = "LumpCode"
			}
			field := planned.Rows[index].Cells[indices["Field"]]
			if field.Storage != "text" || field.Text == nil || *field.Text != expected {
				return ProjectMetadataTable{}, nil, errors.New("profile Table change requires its explicit source Field assignment; no dependent value was silently changed")
			}
		}
	}
	for _, assignment := range assignments {
		if assignment.column != "Order" || assignment.value == nil {
			continue
		}
		for _, row := range planned.Rows {
			if row.RowID == strconv.FormatInt(assignment.rowID, 10) {
				continue
			}
			cell := row.Cells[indices["Order"]]
			if cell.Storage == "integer" && cell.Integer != nil {
				value, err := strconv.ParseInt(*cell.Integer, 10, 64)
				if err == nil && value == assignment.value {
					return ProjectMetadataTable{}, nil, errors.New("changed profile Order collides with another physical rule")
				}
			}
		}
	}
	return planned, assignments, nil
}
