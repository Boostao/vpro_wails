package main

import (
	"context"
	"database/sql"
	"fmt"
)

type siviProjectAssignmentProposal struct {
	Original *siviParentProjection
	Plan     *siviProjectAssignmentPlan
}

func readSIVIProjectAssignmentSnapshot(ctx context.Context, owner *sqliteContext, tx *sql.Tx, contextID, plot string, source int) (*SIVIProjectAssignmentOriginal, error) {
	original, err := readSIVIParentSnapshot(ctx, owner, tx, contextID, plot)
	if err != nil {
		return nil, err
	}
	choices, err := readSIVIProjectChoicesSnapshot(ctx, owner, tx, contextID, source)
	if err != nil {
		return nil, err
	}
	available, diagnostic, err := siviProjectAssignmentAvailability(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	return &SIVIProjectAssignmentOriginal{
		Original: original, Choices: choices, AssignmentAvailable: available, AssignmentDiagnostic: diagnostic,
	}, nil
}

func (s *ContextService) prepareSIVIProjectAssignment(ctx context.Context, contextID, plot string, selection siviProjectSelection) (*siviProjectAssignmentProposal, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviProjectAssignmentProposal, error) {
		values, err := plots.projects.preferences.snapshot()
		if err != nil {
			return nil, err
		}
		source, err := configInt(values, "Current", "ProjectIdSource", 1, 2)
		if err != nil {
			return nil, fmt.Errorf("SIVI ProjectID assignment source unavailable: %w", err)
		}
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviProjectAssignmentProposal, error) {
			snapshot, err := readSIVIProjectAssignmentSnapshot(ctx, owner, tx, contextID, plot, source)
			if err != nil {
				return nil, err
			}
			original, choices := snapshot.Original, snapshot.Choices
			env := ProjectMetadataTable{Columns: original.EnvColumns}
			admin := ProjectMetadataTable{Columns: original.AdminColumns}
			for _, pair := range original.Rows {
				env.Rows = append(env.Rows, pair.Env)
				admin.Rows = append(admin.Rows, pair.Admin)
			}
			plan, err := planSIVIProjectAssignment(ctx, contextID, owner.selection.Project, plot, env, admin, *choices, selection)
			if err != nil {
				return nil, err
			}
			return &siviProjectAssignmentProposal{Original: original, Plan: plan}, nil
		})
	})
}
