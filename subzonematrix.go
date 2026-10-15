package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/boostao/vpro-wails/internal/reports/subzonematrix"
)

type subzoneMatrixReview struct {
	ContextID         string
	Hierarchy         string
	HierarchyPath     string
	LabelsPath        string
	HierarchyOriginal ProjectMetadataTable
	LabelsOriginal    ProjectMetadataTable
	Matrix            subzonematrix.Matrix
}

func (s *ContextService) readSubzoneMatrix(ctx context.Context, contextID string) (subzoneMatrixReview, error) {
	if ctx == nil {
		return subzoneMatrixReview{}, errors.New("Subzone Matrix requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return subzoneMatrixReview{}, err
	}
	if s == nil || s.projects == nil || s.projects.sqlite == nil || s.plots == nil {
		return subzoneMatrixReview{}, errors.New("Subzone Matrix owned context is unavailable")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (subzoneMatrixReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{}, func(owner *sqliteContext, tx *sql.Tx) (subzoneMatrixReview, error) {
			return readSubzoneMatrixSnapshot(ctx, owner, tx, contextID)
		})
	})
}

func readSubzoneMatrixSnapshot(ctx context.Context, owner *sqliteContext, tx *sql.Tx, contextID string) (subzoneMatrixReview, error) {
	if owner.selection.Hierarchy == "None" {
		return subzoneMatrixReview{}, errors.New("Subzone Matrix requires a selected physical hierarchy")
	}
	remainingBytes, remainingCells := int64(subzonematrix.MaxInputBytes), int64(2000000)
	hierarchy, err := readSubzoneMatrixSource(ctx, tx, "hierarchy", owner.selection.Hierarchy+"_Hierarchy",
		&remainingBytes, &remainingCells, "ID", "Name", "Parent", "Level")
	if err != nil {
		return subzoneMatrixReview{}, fmt.Errorf("Subzone Matrix hierarchy: %w", err)
	}
	labels, err := readSubzoneMatrixSource(ctx, tx, "VPro64", "USysSubzoneMatrixLabels",
		&remainingBytes, &remainingCells, "MyUnit")
	if err != nil {
		return subzoneMatrixReview{}, fmt.Errorf("Subzone Matrix labels: %w", err)
	}
	rows, units, err := subzoneMatrixInputs(ctx, hierarchy, labels)
	if err != nil {
		return subzoneMatrixReview{}, err
	}
	matrix, err := subzonematrix.Plan(ctx, rows, units)
	if err != nil {
		return subzoneMatrixReview{}, err
	}
	return subzoneMatrixReview{
		ContextID: contextID, Hierarchy: owner.selection.Hierarchy,
		HierarchyPath: owner.selection.HierarchyPath, LabelsPath: owner.attachments["VPro64"],
		HierarchyOriginal: hierarchy, LabelsOriginal: labels, Matrix: matrix,
	}, nil
}

func readSubzoneMatrixSource(ctx context.Context, tx *sql.Tx, alias, table string,
	remainingBytes, remainingCells *int64, required ...string) (ProjectMetadataTable, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, "SELECT type FROM "+quoteHeaderIdentifier(alias)+
		".sqlite_master WHERE name COLLATE BINARY=?", table).Scan(&kind); err != nil {
		return ProjectMetadataTable{}, err
	}
	if kind != "table" {
		return ProjectMetadataTable{}, errors.New("requires an original physical table, not a view")
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, alias, table, "Subzone Matrix sources"); err != nil {
		return ProjectMetadataTable{}, err
	}
	columns, err := readSQLiteStorageColumns(ctx, tx, alias, table)
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	if len(columns) == 0 || len(columns) > 64 {
		return ProjectMetadataTable{}, errors.New("source requires 1..64 physical columns; no partial original")
	}
	if _, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: columns}, required...); err != nil {
		return ProjectMetadataTable{}, err
	}
	relation := quoteHeaderIdentifier(alias) + "." + quoteHeaderIdentifier(table)
	var count int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+relation).Scan(&count); err != nil {
		return ProjectMetadataTable{}, err
	}
	if count > subzonematrix.MaxInputRows || count*int64(len(columns)) > *remainingCells {
		return ProjectMetadataTable{}, errors.New("source exceeds physical row/complete-original cell bounds; no partial read")
	}
	lengths := make([]string, len(columns))
	for i, column := range columns {
		lengths[i] = "COALESCE(length(CAST(" + quoteHeaderIdentifier(column.Name) + " AS BLOB)),0)"
	}
	var size int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM("+strings.Join(lengths, "+")+"),0) FROM "+relation).Scan(&size); err != nil {
		return ProjectMetadataTable{}, err
	}
	if size > *remainingBytes {
		return ProjectMetadataTable{}, errors.New("source exceeds the 64MiB complete-original byte bound; no partial read")
	}
	*remainingBytes -= size
	*remainingCells -= count * int64(len(columns))
	return readSQLiteStorageRows(ctx, tx, alias, table, "", nil, "")
}

func subzoneMatrixInputs(ctx context.Context, hierarchy, labels ProjectMetadataTable) ([]subzonematrix.HierarchyRow, []*string, error) {
	indices, err := siteUnitTransferColumns(hierarchy, "ID", "Name", "Parent", "Level")
	if err != nil {
		return nil, nil, fmt.Errorf("Subzone Matrix hierarchy schema: %w", err)
	}
	labelIndices, err := siteUnitTransferColumns(labels, "MyUnit")
	if err != nil {
		return nil, nil, fmt.Errorf("Subzone Matrix labels schema: %w", err)
	}
	integer := func(cell ProjectMetadataCell, nullable bool, minimum, maximum int64) (*int64, error) {
		value, err := metadataCellValue(cell)
		if err != nil {
			return nil, err
		}
		if value == nil && nullable {
			return nil, nil
		}
		number, ok := value.(int64)
		if !ok || number < minimum || number > maximum {
			return nil, errors.New("requires original source-width INTEGER storage; no coercion or widening")
		}
		return &number, nil
	}
	text := func(cell ProjectMetadataCell) (*string, error) {
		value, err := metadataCellValue(cell)
		if err != nil || value == nil {
			return nil, err
		}
		literal, ok := value.(string)
		if !ok {
			return nil, errors.New("requires original TEXT/NULL storage; no coercion")
		}
		return &literal, nil
	}
	rows := make([]subzonematrix.HierarchyRow, 0, len(hierarchy.Rows))
	for _, row := range hierarchy.Rows {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		id, err := integer(row.Cells[indices["ID"]], false, math.MinInt32, math.MaxInt32)
		if err != nil {
			return nil, nil, fmt.Errorf("Subzone Matrix ID row %s: %w", row.RowID, err)
		}
		parent, err := integer(row.Cells[indices["Parent"]], true, math.MinInt32, math.MaxInt32)
		if err != nil {
			return nil, nil, fmt.Errorf("Subzone Matrix Parent row %s: %w", row.RowID, err)
		}
		level, err := integer(row.Cells[indices["Level"]], true, math.MinInt16, math.MaxInt16)
		if err != nil {
			return nil, nil, fmt.Errorf("Subzone Matrix Level row %s: %w", row.RowID, err)
		}
		name, err := text(row.Cells[indices["Name"]])
		if err != nil {
			return nil, nil, fmt.Errorf("Subzone Matrix Name row %s: %w", row.RowID, err)
		}
		rows = append(rows, subzonematrix.HierarchyRow{ID: *id, Name: name, Parent: parent, Level: level})
	}
	units := make([]*string, 0, len(labels.Rows))
	for _, row := range labels.Rows {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		unit, err := text(row.Cells[labelIndices["MyUnit"]])
		if err != nil {
			return nil, nil, fmt.Errorf("Subzone Matrix MyUnit row %s: %w", row.RowID, err)
		}
		units = append(units, unit)
	}
	return rows, units, nil
}
