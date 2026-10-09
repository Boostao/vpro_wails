package main

import (
	"context"
	"errors"
	"reflect"
)

func planTwoPageEntryProjectAssignment(ctx context.Context, original *siviParentProjection,
	choices SIVIProjectChoices, selection siviProjectSelection) (*siviProjectAssignmentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("complete-entry ProjectID selection requires one owned source original")
	}
	projectParent, err := twoPageParentProjector(original.Form)
	if err != nil {
		return nil, err
	}
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
	rebuilt, err := projectParent(ctx, original.ContextID, original.Project, original.Plot, env, admin)
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return nil, errors.Join(err, errors.New("complete-entry ProjectID original differs from its exact source projection"))
	}
	return planSourceProjectAssignment(ctx, "form:"+original.Form+"/ProjectID", original.ContextID, original.Project,
		original.Plot, env, admin, choices, selection)
}
