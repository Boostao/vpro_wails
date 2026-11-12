package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

const siviCombinedHistoryTable = "__VPRO_SIVICombinedHistory"
const siviCombinedHistorySQL = `CREATE TABLE "__VPRO_SIVICombinedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviCombinedWritePolicy() sourceform.SiviVegetationWritePolicy {
	columns := append([]string{}, sourceform.SiviCoverWritePolicy().Columns...)
	columns = append(columns, sourceform.SiviHeightWritePolicy().Columns...)
	return sourceform.SiviVegetationWritePolicy{Kind: "combined", HistoryTable: siviCombinedHistoryTable, HistorySQL: siviCombinedHistorySQL, Columns: columns, Plan: planSIVICombinedEdits}
}

func planSIVICombinedEdits(ctx context.Context, plot string, extended bool, veg vprocore.ProjectMetadataTable, edits []sourceform.SiviHeightEdit) ([]sourceform.SiviHeightAssignment, error) {
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
	for _, column := range sourceform.SiviHeightWritePolicy().Columns {
		heightColumns[column] = true
	}
	covers, heights := []sourceform.SiviHeightEdit{}, []sourceform.SiviHeightEdit{}
	for _, edit := range edits {
		if heightColumns[edit.Column] {
			heights = append(heights, edit)
		} else {
			covers = append(covers, edit)
		}
	}
	planned := map[[2]string]sourceform.SiviHeightAssignment{}
	for _, domain := range []struct {
		edits []sourceform.SiviHeightEdit
		plan  func(context.Context, string, bool, vprocore.ProjectMetadataTable, []sourceform.SiviHeightEdit) ([]sourceform.SiviHeightAssignment, error)
	}{{covers, sourceform.PlanSIVICoverEdits}, {heights, sourceform.PlanSIVIHeightEdits}} {
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
	assignments := []sourceform.SiviHeightAssignment{}
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
