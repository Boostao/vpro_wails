package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

const summaryVegetationFieldListTable = "USysShortVegSummaryFieldList"

var summaryVegetationFieldListColumns = []string{
	"Order", "Source", "Label", "Item", "Report", "FunctionName", "FieldNumber",
}

type summaryVegetationFieldListIndex struct {
	Visible  []string
	Selected []string
}

func readSummaryVegetationFieldList(ctx context.Context, tx *sql.Tx) (ProjectMetadataTable, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM VPro64.sqlite_master WHERE name COLLATE BINARY=?`,
		summaryVegetationFieldListTable).Scan(&kind); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("Summary environmental field-list physical table unavailable: %w", err)
	}
	if kind != "table" {
		return ProjectMetadataTable{}, errors.New("Summary environmental field-list requires its physical VPro64 table")
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, "VPro64", summaryVegetationFieldListTable, "Summary environmental field-list"); err != nil {
		return ProjectMetadataTable{}, err
	}
	table, err := readSQLiteStorageRows(ctx, tx, "VPro64", summaryVegetationFieldListTable, "", nil, "")
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("Summary environmental field-list physical read failed: %w", err)
	}
	if err := validateSummaryVegetationFieldListColumns(table); err != nil {
		return ProjectMetadataTable{}, err
	}
	return table, nil
}

func validateSummaryVegetationFieldListColumns(table ProjectMetadataTable) error {
	if len(table.Columns) != len(summaryVegetationFieldListColumns) {
		return errors.New("Summary environmental field-list requires the exact seven source columns")
	}
	for i, name := range summaryVegetationFieldListColumns {
		if table.Columns[i].Name != name {
			return errors.New("Summary environmental field-list requires the exact source column order and names")
		}
	}
	return nil
}

func indexSummaryVegetationFieldList(ctx context.Context, table ProjectMetadataTable) (summaryVegetationFieldListIndex, error) {
	result := summaryVegetationFieldListIndex{Visible: []string{}, Selected: []string{}}
	if err := ctx.Err(); err != nil {
		return summaryVegetationFieldListIndex{}, err
	}
	if err := validateSummaryVegetationFieldListColumns(table); err != nil {
		return summaryVegetationFieldListIndex{}, err
	}
	type entry struct {
		rowID    string
		order    *int64
		selected bool
	}
	entries := make([]entry, 0, len(table.Rows))
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return summaryVegetationFieldListIndex{}, err
		}
		if len(row.Cells) != len(summaryVegetationFieldListColumns) {
			return summaryVegetationFieldListIndex{}, errors.New("Summary environmental field-list row does not contain all seven source cells")
		}
		item := entry{rowID: row.RowID}
		order, report := row.Cells[0], row.Cells[4]
		if order.Storage != "null" {
			if order.Storage != "integer" || order.Integer == nil {
				return summaryVegetationFieldListIndex{}, fmt.Errorf("Summary environmental field-list row %s Order is unavailable without coercion", row.RowID)
			}
			value, err := strconv.ParseInt(*order.Integer, 10, 16)
			if err != nil {
				return summaryVegetationFieldListIndex{}, fmt.Errorf("Summary environmental field-list row %s Order is outside source INTEGER storage: %w", row.RowID, err)
			}
			item.order = &value
		}
		if report.Storage != "null" {
			if report.Storage != "integer" || report.Integer == nil || (*report.Integer != "-1" && *report.Integer != "0") {
				return summaryVegetationFieldListIndex{}, fmt.Errorf("Summary environmental field-list row %s Report is unavailable without Boolean normalization", row.RowID)
			}
			item.selected = *report.Integer == "-1"
		}
		entries = append(entries, item)
	}
	// Physical reads are rowid-ordered; source equal-Order ties have no defined order.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].order == nil {
			return entries[j].order != nil
		}
		return entries[j].order != nil && *entries[i].order < *entries[j].order
	})
	for _, item := range entries {
		if item.order != nil && *item.order > 0 {
			result.Visible = append(result.Visible, item.rowID)
		}
		if item.selected {
			result.Selected = append(result.Selected, item.rowID)
		}
	}
	if err := ctx.Err(); err != nil {
		return summaryVegetationFieldListIndex{}, err
	}
	return result, nil
}
