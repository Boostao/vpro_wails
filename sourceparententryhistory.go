package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

func verifySourceParentAuditedSubset(audited, planned []siviParentScalarAssignment, label string) error {
	for _, assignment := range audited {
		found := false
		for _, source := range planned {
			found = found || reflect.DeepEqual(assignment, source)
		}
		if !found {
			return fmt.Errorf("%s audited change differs from the full committed source plan", label)
		}
	}
	return nil
}

func verifySourceParentEntryPlan(ctx context.Context, original, committed *siviParentProjection,
	planned []siviParentScalarAssignment, projectParent sourceParentProjector,
	equal func(*siviParentProjection, *siviParentProjection) bool, label string) error {
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{
		{RowID: original.Rows[0].Env.RowID, Cells: append([]ProjectMetadataCell{}, original.Rows[0].Env.Cells...)},
	}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{
		{RowID: original.Rows[0].Admin.RowID, Cells: append([]ProjectMetadataCell{}, original.Rows[0].Admin.Cells...)},
	}}
	for _, assignment := range planned {
		table := &env
		if assignment.Table == original.AdminTable {
			table = &admin
		}
		columns, err := siteUnitTransferColumns(*table, assignment.Column)
		if err != nil {
			return err
		}
		table.Rows[0].Cells[columns[assignment.Column]] = cloneSiteUnitCell(assignment.After)
	}
	rebuilt, err := projectParent(ctx, original.ContextID, original.Project, original.Plot, env, admin)
	if err != nil || !equal(rebuilt, committed) {
		return errors.Join(err, fmt.Errorf("%s history changed unrelated physical parent values", label))
	}
	return nil
}

func sourceParentProjectionStorageEqual(left, right *siviParentProjection) bool {
	if !reflect.DeepEqual(left, right) {
		return false
	}
	if left == nil {
		return true
	}
	for i, row := range left.Rows {
		for _, pair := range [][2]ProjectMetadataRow{{row.Env, right.Rows[i].Env}, {row.Admin, right.Rows[i].Admin}} {
			for j, cell := range pair[0].Cells {
				if !metadataCellEqual(cell, pair[1].Cells[j]) {
					return false
				}
			}
		}
	}
	return true
}
