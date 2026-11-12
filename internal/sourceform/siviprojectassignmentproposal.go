package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"fmt"
)

type siviProjectAssignmentProposal struct {
	Original *SiviParentProjection
	Plan     *SiviProjectAssignmentPlan
}

func readSIVIProjectAssignmentSnapshot(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID, plot string, source int) (*SIVIProjectAssignmentOriginal, error) {
	original, err := readSIVIParentSnapshot(ctx, owner, tx, contextID, plot)
	if err != nil {
		return nil, err
	}
	choices, err := readSIVIProjectChoicesSnapshot(ctx, owner, tx, contextID, source)
	if err != nil {
		return nil, err
	}
	available, diagnostic, err := SiviProjectAssignmentAvailability(ctx, tx, source)
	if err != nil {
		return nil, err
	}
	return &SIVIProjectAssignmentOriginal{
		Original: original, Choices: choices, AssignmentAvailable: available, AssignmentDiagnostic: diagnostic,
	}, nil
}

func PrepareSIVIProjectAssignment(s *vprocore.ContextService, ctx context.Context, contextID, plot string, selection SiviProjectSelection) (*siviProjectAssignmentProposal, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviProjectAssignmentProposal, error) {
		values, err := plots.Projects.Preferences.Snapshot()
		if err != nil {
			return nil, err
		}
		source, err := vprocore.ConfigInt(values, "Current", "ProjectIdSource", 1, 2)
		if err != nil {
			return nil, fmt.Errorf("SIVI ProjectID assignment source unavailable: %w", err)
		}
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*siviProjectAssignmentProposal, error) {
			snapshot, err := readSIVIProjectAssignmentSnapshot(ctx, owner, tx, contextID, plot, source)
			if err != nil {
				return nil, err
			}
			original, choices := snapshot.Original, snapshot.Choices
			env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns}
			admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns}
			for _, pair := range original.Rows {
				env.Rows = append(env.Rows, pair.Env)
				admin.Rows = append(admin.Rows, pair.Admin)
			}
			plan, err := planSIVIProjectAssignment(ctx, contextID, owner.Selection.Project, plot, env, admin, *choices, selection)
			if err != nil {
				return nil, err
			}
			return &siviProjectAssignmentProposal{Original: original, Plan: plan}, nil
		})
	})
}
