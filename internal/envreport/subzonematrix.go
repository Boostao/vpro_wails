package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	HierarchyOriginal vprocore.ProjectMetadataTable
	LabelsOriginal    vprocore.ProjectMetadataTable
	Matrix            subzonematrix.Matrix
}

func ReadSubzoneMatrix(s *vprocore.ContextService, ctx context.Context, contextID string) (subzoneMatrixReview, error) {
	if ctx == nil {
		return subzoneMatrixReview{}, errors.New("Subzone Matrix requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return subzoneMatrixReview{}, err
	}
	if s == nil || s.Projects == nil || s.Projects.Sqlite == nil || s.Plots == nil {
		return subzoneMatrixReview{}, errors.New("Subzone Matrix owned context is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (subzoneMatrixReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (subzoneMatrixReview, error) {
			return readSubzoneMatrixSnapshot(ctx, owner, tx, contextID)
		})
	})
}

func readSubzoneMatrixSnapshot(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID string) (subzoneMatrixReview, error) {
	if owner.Selection.Hierarchy == "None" {
		return subzoneMatrixReview{}, errors.New("Subzone Matrix requires a selected physical hierarchy")
	}
	remainingBytes, remainingCells := int64(subzonematrix.MaxInputBytes), int64(2000000)
	hierarchy, err := readSubzoneMatrixSource(ctx, tx, "hierarchy", owner.Selection.Hierarchy+"_Hierarchy",
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
		ContextID: contextID, Hierarchy: owner.Selection.Hierarchy,
		HierarchyPath: owner.Selection.HierarchyPath, LabelsPath: owner.Attachments["VPro64"],
		HierarchyOriginal: hierarchy, LabelsOriginal: labels, Matrix: matrix,
	}, nil
}

func readSubzoneMatrixSource(ctx context.Context, tx *sql.Tx, alias, table string,
	remainingBytes, remainingCells *int64, required ...string) (vprocore.ProjectMetadataTable, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, "SELECT type FROM "+vprocore.QuoteHeaderIdentifier(alias)+
		".sqlite_master WHERE name COLLATE BINARY=?", table).Scan(&kind); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	if kind != "table" {
		return vprocore.ProjectMetadataTable{}, errors.New("requires an original physical table, not a view")
	}
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, alias, table, "Subzone Matrix sources"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	columns, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, alias, table)
	if err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	if len(columns) == 0 || len(columns) > 64 {
		return vprocore.ProjectMetadataTable{}, errors.New("source requires 1..64 physical columns; no partial original")
	}
	if _, err := vprocore.SiteUnitTransferColumns(vprocore.ProjectMetadataTable{Columns: columns}, required...); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	relation := vprocore.QuoteHeaderIdentifier(alias) + "." + vprocore.QuoteHeaderIdentifier(table)
	var count int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+relation).Scan(&count); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	if count > subzonematrix.MaxInputRows || count*int64(len(columns)) > *remainingCells {
		return vprocore.ProjectMetadataTable{}, errors.New("source exceeds physical row/complete-original cell bounds; no partial read")
	}
	lengths := make([]string, len(columns))
	for i, column := range columns {
		lengths[i] = "COALESCE(length(CAST(" + vprocore.QuoteHeaderIdentifier(column.Name) + " AS BLOB)),0)"
	}
	var size int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM("+strings.Join(lengths, "+")+"),0) FROM "+relation).Scan(&size); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	if size > *remainingBytes {
		return vprocore.ProjectMetadataTable{}, errors.New("source exceeds the 64MiB complete-original byte bound; no partial read")
	}
	*remainingBytes -= size
	*remainingCells -= count * int64(len(columns))
	return vprocore.ReadSQLiteStorageRows(ctx, tx, alias, table, "", nil, "")
}

func subzoneMatrixInputs(ctx context.Context, hierarchy, labels vprocore.ProjectMetadataTable) ([]subzonematrix.HierarchyRow, []*string, error) {
	indices, err := vprocore.SiteUnitTransferColumns(hierarchy, "ID", "Name", "Parent", "Level")
	if err != nil {
		return nil, nil, fmt.Errorf("Subzone Matrix hierarchy schema: %w", err)
	}
	labelIndices, err := vprocore.SiteUnitTransferColumns(labels, "MyUnit")
	if err != nil {
		return nil, nil, fmt.Errorf("Subzone Matrix labels schema: %w", err)
	}
	integer := func(cell vprocore.ProjectMetadataCell, nullable bool, minimum, maximum int64) (*int64, error) {
		value, err := vprocore.MetadataCellValue(cell)
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
	text := func(cell vprocore.ProjectMetadataCell) (*string, error) {
		value, err := vprocore.MetadataCellValue(cell)
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
