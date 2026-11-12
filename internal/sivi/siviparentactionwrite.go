package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

const siviParentActionHistoryTable = "__VPRO_SIVIParentActionHistory"
const siviParentActionHistorySQL = `CREATE TABLE "__VPRO_SIVIParentActionHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviParentActionHistory = sourceform.SiviParentHistoryDomain{
	Table: siviParentActionHistoryTable, Schema: siviParentActionHistorySQL, Label: "SIVI parent action", Assignments: siviParentActionHistoryAssignments,
}

type siviParentActionWriteResult struct {
	sourceform.SiviParentWriteResult
	SourceRefreshRequired bool
}

func planSIVIParentActionEdits(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentActionEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("SIVI parent action writing requires one physical pair and explicit source actions")
	}
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
	return sourceform.PlanSIVIParentActions(ctx, original.ContextID, original.Project, original.Plot, env, admin, edits)
}

func WriteSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot string, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentActionEdit) (*siviParentActionWriteResult, error) {
	refresh := false
	result, err := sourceform.WriteSIVIParentPlanned(s, ctx, contextID, plot, original, siviParentActionHistory, func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
		assignments, err := planSIVIParentActionEdits(ctx, observed, edits)
		if err != nil {
			return nil, err
		}
		if err := sourceform.ValidateSourceParentBooleanAssignments(assignments); err != nil {
			return nil, fmt.Errorf("SIVI source action: %w", err)
		}
		sources, err := sourceform.SiviParentActionSources()
		if err != nil {
			return nil, err
		}
		for _, edit := range edits {
			refresh = refresh || sources[edit.ControlID].Binding == "PlotType"
		}
		return assignments, nil
	})
	if err != nil {
		return nil, err
	}
	return &siviParentActionWriteResult{*result, refresh}, nil
}

func siviParentActionHistoryAssignments(ctx context.Context, event sourceform.SiviParentHistory, project, plot string) ([]sourceform.SiviParentScalarAssignment, error) {
	original, err := sourceform.ValidateSIVIParentHistoryOriginal(ctx, event, project, plot)
	if err != nil {
		return nil, err
	}
	sources, err := sourceform.SiviParentActionSources()
	if err != nil {
		return nil, err
	}
	edits := make([]sourceform.SiviParentActionEdit, 0, len(event.Changes))
	for _, change := range event.Changes {
		control := ""
		binding := ""
		for id, source := range sources {
			if source.Binding == change.Column {
				control, binding = id, source.Binding
			}
		}
		if control == "" {
			return nil, errors.New("typed SIVI action history contains an unavailable direct or identity target")
		}
		options := []*int{}
		if binding == "SpeciesListComplete" {
			options = append(options, nil)
		}
		max := 5
		if binding == "SpeciesListComplete" {
			max = 2
		}
		for value := 1; value <= max; value++ {
			option := value
			options = append(options, &option)
		}
		matched := false
		for _, option := range options {
			edit := sourceform.SiviParentActionEdit{ContextID: original.ContextID, ControlID: control, Table: change.Table, RowID: change.RowID, Expected: change.Before, Option: option}
			planned, err := planSIVIParentActionEdits(ctx, original, []sourceform.SiviParentActionEdit{edit})
			if err != nil {
				return nil, err
			}
			if len(planned) == 1 && reflect.DeepEqual(planned[0].After, change.After) {
				edits = append(edits, edit)
				matched = true
				break
			}
		}
		if !matched {
			return nil, errors.New("typed SIVI action history contains an unchanged or unverified source value")
		}
	}
	assignments, err := planSIVIParentActionEdits(ctx, original, edits)
	if err != nil || len(assignments) != len(event.Changes) {
		return nil, errors.Join(err, errors.New("typed SIVI action history contains repeated or unchanged assignments"))
	}
	return assignments, nil
}

func CtxRestoreSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	return sourceform.RestoreSIVIParentHistory(s, ctx, contextID, plot, historyID, action, siviParentActionHistory)
}
