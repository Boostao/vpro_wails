package main

import (
	"context"
	"database/sql"
)

type ownedGoogleEarthLocations struct {
	ContextID, ProjectPath, SUPath string
	Report                         googleEarthLocations
}

func readGoogleEarthLocationsSnapshot(ctx context.Context, contextID, descriptionField string, owner *sqliteContext, tx *sql.Tx) (*ownedGoogleEarthLocations, error) {
	env, err := readPhysicalLocationTable(ctx, tx, "project", owner.selection.Project+"_Env")
	if err != nil {
		return nil, err
	}
	selected := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
	if owner.selection.SU != "None" {
		selected, err = readPhysicalLocationTable(ctx, tx, "su", owner.selection.SU+"_SU")
		if err != nil {
			return nil, err
		}
	}
	report, err := planGoogleEarthLocations(ctx, owner.selection.Project, owner.selection.SU, descriptionField, env, selected)
	if err != nil {
		return nil, err
	}
	return &ownedGoogleEarthLocations{ContextID: contextID, ProjectPath: owner.selection.ProjectPath,
		SUPath: owner.selection.SUPath, Report: *report}, nil
}

func (s *ContextService) readGoogleEarthLocations(ctx context.Context, contextID, descriptionField string) (*ownedGoogleEarthLocations, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*ownedGoogleEarthLocations, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*ownedGoogleEarthLocations, error) {
			return readGoogleEarthLocationsSnapshot(ctx, contextID, descriptionField, owner, tx)
		})
	})
}
