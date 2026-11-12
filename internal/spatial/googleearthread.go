package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
)

type ownedGoogleEarthLocations struct {
	ContextID, ProjectPath, SUPath string
	Report                         googleEarthLocations
}

func readGoogleEarthLocationsSnapshot(ctx context.Context, contextID, descriptionField string, owner *vprocore.SqliteContext, tx *sql.Tx) (*ownedGoogleEarthLocations, error) {
	env, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
	if err != nil {
		return nil, err
	}
	selected := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	if owner.Selection.SU != "None" {
		selected, err = vprocore.ReadPhysicalLocationTable(ctx, tx, "su", owner.Selection.SU+"_SU")
		if err != nil {
			return nil, err
		}
	}
	report, err := planGoogleEarthLocations(ctx, owner.Selection.Project, owner.Selection.SU, descriptionField, env, selected)
	if err != nil {
		return nil, err
	}
	return &ownedGoogleEarthLocations{ContextID: contextID, ProjectPath: owner.Selection.ProjectPath,
		SUPath: owner.Selection.SUPath, Report: *report}, nil
}

func ReadGoogleEarthLocations(s *vprocore.ContextService, ctx context.Context, contextID, descriptionField string) (*ownedGoogleEarthLocations, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*ownedGoogleEarthLocations, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*ownedGoogleEarthLocations, error) {
			return readGoogleEarthLocationsSnapshot(ctx, contextID, descriptionField, owner, tx)
		})
	})
}
