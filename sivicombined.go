package main

import (
	"context"
	"errors"
)

const siviCombinedHistoryTable = "__VPRO_SIVICombinedHistory"
const siviCombinedHistorySQL = `CREATE TABLE "__VPRO_SIVICombinedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviCombinedWritePolicy() siviVegetationWritePolicy {
	columns := append([]string{}, siviCoverWritePolicy().Columns...)
	columns = append(columns, siviHeightWritePolicy().Columns...)
	return siviVegetationWritePolicy{"combined", siviCombinedHistoryTable, siviCombinedHistorySQL, columns, planSIVICombinedEdits}
}

func planSIVICombinedEdits(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []siviHeightEdit) ([]siviHeightAssignment, error) {
	if ctx == nil {
		return nil, errors.New("SIVI combined planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI combined planning requires explicit edits")
	}
	heightColumns := map[string]bool{}
	for _, column := range siviHeightWritePolicy().Columns {
		heightColumns[column] = true
	}
	covers, heights := []siviHeightEdit{}, []siviHeightEdit{}
	for _, edit := range edits {
		if heightColumns[edit.Column] {
			heights = append(heights, edit)
		} else {
			covers = append(covers, edit)
		}
	}
	planned := map[[2]string]siviHeightAssignment{}
	for _, domain := range []struct {
		edits []siviHeightEdit
		plan  func(context.Context, string, bool, ProjectMetadataTable, []siviHeightEdit) ([]siviHeightAssignment, error)
	}{{covers, planSIVICoverEdits}, {heights, planSIVIHeightEdits}} {
		if len(domain.edits) == 0 {
			continue
		}
		assignments, err := domain.plan(ctx, plot, extended, veg, domain.edits)
		if err != nil {
			return nil, err
		}
		for _, assignment := range assignments {
			planned[[2]string{assignment.RowID, assignment.Column}] = assignment
		}
	}
	// Domain planners validate every draft (including noops); only changed cells
	// are merged back into the reviewed order for one transaction and history.
	assignments := []siviHeightAssignment{}
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if assignment, present := planned[[2]string{edit.RowID, edit.Column}]; present {
			assignments = append(assignments, assignment)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}
