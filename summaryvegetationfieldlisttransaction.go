package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
)

// The caller owns the transaction and must roll back any error, including history.
func applySummaryVegetationFieldList(ctx context.Context, tx *sql.Tx, expected ProjectMetadataTable,
	actions []summaryVegetationFieldListAction) (*summaryVegetationFieldListPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("Summary field-list assignments require a caller-owned transaction")
	}
	current, err := readSummaryVegetationFieldList(ctx, tx)
	if err != nil {
		return nil, err
	}
	plan, err := planSummaryVegetationFieldList(ctx, current, expected, actions)
	if err != nil {
		return nil, err
	}
	for _, assignment := range plan.Assignments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, err := metadataCellValue(assignment.After.Cells[4])
		if err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE VPro64.USysShortVegSummaryFieldList SET Report=? WHERE rowid=?`,
			value, assignment.Before.RowID)
		if err != nil {
			return nil, fmt.Errorf("Summary field-list row %s Report assignment failed: %w", assignment.Before.RowID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return nil, errors.Join(fmt.Errorf("Summary field-list row %s assignment expected one physical row, found %d",
				assignment.Before.RowID, affected), err)
		}
	}
	observed, err := readSummaryVegetationFieldList(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(observed, plan.Proposed) {
		return nil, errors.New("Summary field-list committed plan observation differs; roll back the transaction before retrying")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return plan, nil
}
