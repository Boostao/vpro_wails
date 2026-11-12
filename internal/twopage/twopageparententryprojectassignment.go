package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"reflect"
)

func PlanTwoPageEntryProjectAssignment(ctx context.Context, original *sourceform.SiviParentProjection,
	choices sourceform.SIVIProjectChoices, selection sourceform.SiviProjectSelection) (*sourceform.SiviProjectAssignmentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("complete-entry ProjectID selection requires one owned source original")
	}
	projectParent, err := TwoPageParentProjector(original.Form)
	if err != nil {
		return nil, err
	}
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
	rebuilt, err := projectParent(ctx, original.ContextID, original.Project, original.Plot, env, admin)
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return nil, errors.Join(err, errors.New("complete-entry ProjectID original differs from its exact source projection"))
	}
	return sourceform.PlanSourceProjectAssignment(ctx, "form:"+original.Form+"/ProjectID", original.ContextID, original.Project,
		original.Plot, env, admin, choices, selection)
}
