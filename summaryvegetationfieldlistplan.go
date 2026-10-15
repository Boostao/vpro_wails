package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

type summaryVegetationFieldListAction struct {
	Kind     string
	RowID    string
	Selected bool
}

type summaryVegetationFieldListAssignment struct {
	Before ProjectMetadataRow
	After  ProjectMetadataRow
}

type summaryVegetationFieldListPlan struct {
	Original    ProjectMetadataTable
	Proposed    ProjectMetadataTable
	Assignments []summaryVegetationFieldListAssignment
}

func validateSummaryVegetationFieldListWitness(ctx context.Context, table ProjectMetadataTable) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSummaryVegetationFieldListColumns(table); err != nil {
		return err
	}
	if table.Rows == nil {
		return errors.New("Summary field-list witness requires an explicit physical row array")
	}
	var last int64
	for i, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		id, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != row.RowID || i > 0 && id <= last {
			return errors.New("Summary field-list witness requires unique canonical physical rowids in physical order")
		}
		last = id
		if len(row.Cells) != len(summaryVegetationFieldListColumns) {
			return errors.New("Summary field-list witness requires all seven physical cells")
		}
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil {
				return fmt.Errorf("Summary field-list witness row %s: %w", row.RowID, err)
			}
		}
	}
	return nil
}

func cloneSummaryVegetationFieldListRow(row ProjectMetadataRow) ProjectMetadataRow {
	result := ProjectMetadataRow{RowID: row.RowID, Cells: make([]ProjectMetadataCell, len(row.Cells))}
	for i, cell := range row.Cells {
		result.Cells[i] = cloneSiteUnitCell(cell)
	}
	return result
}

func cloneSummaryVegetationFieldListTable(table ProjectMetadataTable) ProjectMetadataTable {
	result := ProjectMetadataTable{
		Columns: append([]ProjectMetadataColumn(nil), table.Columns...), Rows: make([]ProjectMetadataRow, len(table.Rows)),
	}
	for i, row := range table.Rows {
		result.Rows[i] = cloneSummaryVegetationFieldListRow(row)
	}
	return result
}

func planSummaryVegetationFieldList(ctx context.Context, current, expected ProjectMetadataTable, actions []summaryVegetationFieldListAction) (*summaryVegetationFieldListPlan, error) {
	for _, table := range []ProjectMetadataTable{current, expected} {
		if err := validateSummaryVegetationFieldListWitness(ctx, table); err != nil {
			return nil, err
		}
	}
	if !reflect.DeepEqual(current, expected) {
		return nil, errors.New("Summary field-list changed since review; reload before saving")
	}
	if actions == nil {
		return nil, errors.New("Summary field-list plan requires an explicit action array")
	}
	plan := &summaryVegetationFieldListPlan{
		Original: cloneSummaryVegetationFieldListTable(current), Proposed: cloneSummaryVegetationFieldListTable(current),
		Assignments: []summaryVegetationFieldListAssignment{},
	}
	for _, action := range actions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch action.Kind {
		case "all", "none":
			if action.RowID != "" || action.Selected != (action.Kind == "all") {
				return nil, errors.New("Summary field-list bulk actions require exact source selection and no row identity")
			}
		case "row":
			id, err := strconv.ParseInt(action.RowID, 10, 64)
			if err != nil || strconv.FormatInt(id, 10) != action.RowID {
				return nil, errors.New("Summary field-list row selection requires a canonical physical rowid")
			}
		default:
			return nil, fmt.Errorf("Summary field-list action %q is unavailable", action.Kind)
		}
		matched := action.Kind != "row"
		for i, original := range current.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if action.Kind == "row" && original.RowID != action.RowID {
				continue
			}
			if action.Kind == "row" {
				matched = true
				order, report := original.Cells[0], original.Cells[4]
				if order.Storage != "integer" || order.Integer == nil {
					return nil, errors.New("Summary field-list row checkbox is unavailable outside positive source Order")
				}
				value, err := strconv.ParseInt(*order.Integer, 10, 16)
				if err != nil || value <= 0 {
					return nil, errors.New("Summary field-list row checkbox is unavailable outside positive source Order")
				}
				if report.Storage != "integer" || report.Integer == nil || (*report.Integer != "-1" && *report.Integer != "0") {
					return nil, errors.New("Summary field-list row checkbox requires calibrated nonnull Boolean storage")
				}
			}
			value := "0"
			if action.Selected {
				value = "-1"
			}
			plan.Proposed.Rows[i].Cells[4] = ProjectMetadataCell{Storage: "integer", Integer: &value}
		}
		if !matched {
			return nil, errors.New("Summary field-list row checkbox identity is absent from the complete physical review")
		}
	}
	for i, row := range plan.Proposed.Rows {
		if !reflect.DeepEqual(row.Cells[4], current.Rows[i].Cells[4]) {
			plan.Assignments = append(plan.Assignments, summaryVegetationFieldListAssignment{
				Before: cloneSummaryVegetationFieldListRow(current.Rows[i]), After: cloneSummaryVegetationFieldListRow(row),
			})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return plan, nil
}
