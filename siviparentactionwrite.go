package main

import (
	"context"
	"errors"
	"reflect"
)

const siviParentActionHistoryTable = "__VPRO_SIVIParentActionHistory"
const siviParentActionHistorySQL = `CREATE TABLE "__VPRO_SIVIParentActionHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviParentActionHistory = siviParentHistoryDomain{
	siviParentActionHistoryTable, siviParentActionHistorySQL, "SIVI parent action", siviParentActionHistoryAssignments,
}

type siviParentActionWriteResult struct {
	siviParentWriteResult
	SourceRefreshRequired bool
}

func planSIVIParentActionEdits(ctx context.Context, original *siviParentProjection, edits []siviParentActionEdit) ([]siviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("SIVI parent action writing requires one physical pair and explicit source actions")
	}
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
	return planSIVIParentActions(ctx, original.ContextID, original.Project, original.Plot, env, admin, edits)
}

func (s *ContextService) writeSIVIParentActions(ctx context.Context, contextID, plot string, original *siviParentProjection, edits []siviParentActionEdit) (*siviParentActionWriteResult, error) {
	refresh := false
	result, err := s.writeSIVIParentPlanned(ctx, contextID, plot, original, siviParentActionHistory, func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
		assignments, err := planSIVIParentActionEdits(ctx, observed, edits)
		if err != nil {
			return nil, err
		}
		sources, err := siviParentActionSources()
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

func siviParentActionHistoryAssignments(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
	original, err := validateSIVIParentHistoryOriginal(ctx, event, project, plot)
	if err != nil {
		return nil, err
	}
	sources, err := siviParentActionSources()
	if err != nil {
		return nil, err
	}
	edits := make([]siviParentActionEdit, 0, len(event.Changes))
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
			edit := siviParentActionEdit{original.ContextID, control, change.Table, change.RowID, change.Before, option}
			planned, err := planSIVIParentActionEdits(ctx, original, []siviParentActionEdit{edit})
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

func (s *ContextService) restoreSIVIParentActions(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return s.restoreSIVIParentHistory(ctx, contextID, plot, historyID, action, siviParentActionHistory)
}
