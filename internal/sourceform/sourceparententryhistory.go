package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

func VerifySourceParentAuditedSubset(audited, planned []SiviParentScalarAssignment, label string) error {
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

func VerifySourceParentEntryPlan(ctx context.Context, original, committed *SiviParentProjection,
	planned []SiviParentScalarAssignment, projectParent SourceParentProjector,
	equal func(*SiviParentProjection, *SiviParentProjection) bool, label string) error {
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{
		{RowID: original.Rows[0].Env.RowID, Cells: append([]vprocore.ProjectMetadataCell{}, original.Rows[0].Env.Cells...)},
	}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{
		{RowID: original.Rows[0].Admin.RowID, Cells: append([]vprocore.ProjectMetadataCell{}, original.Rows[0].Admin.Cells...)},
	}}
	for _, assignment := range planned {
		table := &env
		if assignment.Table == original.AdminTable {
			table = &admin
		}
		columns, err := vprocore.SiteUnitTransferColumns(*table, assignment.Column)
		if err != nil {
			return err
		}
		table.Rows[0].Cells[columns[assignment.Column]] = vprocore.CloneSiteUnitCell(assignment.After)
	}
	rebuilt, err := projectParent(ctx, original.ContextID, original.Project, original.Plot, env, admin)
	if err != nil || !equal(rebuilt, committed) {
		return errors.Join(err, fmt.Errorf("%s history changed unrelated physical parent values", label))
	}
	return nil
}

func SourceParentProjectionStorageEqual(left, right *SiviParentProjection) bool {
	if !reflect.DeepEqual(left, right) {
		return false
	}
	if left == nil {
		return true
	}
	for i, row := range left.Rows {
		for _, pair := range [][2]vprocore.ProjectMetadataRow{{row.Env, right.Rows[i].Env}, {row.Admin, right.Rows[i].Admin}} {
			for j, cell := range pair[0].Cells {
				if !vprocore.MetadataCellEqual(cell, pair[1].Cells[j]) {
					return false
				}
			}
		}
	}
	return true
}
