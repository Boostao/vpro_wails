package main

import (
	"context"
	"database/sql"
	"errors"
)

type siteUnitDetailOwnedScope struct {
	ContextID   string
	ProjectPath string
	SUPath      string
	Scope       siteUnitDetailScope
}

func (s *ContextService) readSiteUnitDetailNormalSUScope(ctx context.Context, contextID string,
	maxJoinedRows int, hooks publicationReadSnapshotHooks) (siteUnitDetailOwnedScope, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (siteUnitDetailOwnedScope, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *sqliteContext, tx *sql.Tx) (siteUnitDetailOwnedScope, error) {
			if owner.selection.SU == "None" {
				return siteUnitDetailOwnedScope{}, errors.New("site unit detail scope requires an explicitly selected normal SU")
			}
			env, err := readPhysicalLocationTable(ctx, tx, "project", owner.selection.Project+"_Env")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			admin, err := readPhysicalLocationTable(ctx, tx, "project", owner.selection.Project+"_Admin")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			su, err := readPhysicalLocationTable(ctx, tx, "su", owner.selection.SU+"_SU")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			scope, err := prepareSiteUnitDetailScope(ctx, owner.selection.Project, owner.selection.SU,
				siteUnitDetailSelectedSU, maxJoinedRows, env, admin, su)
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			return siteUnitDetailOwnedScope{contextID, owner.selection.ProjectPath, owner.selection.SUPath, scope}, nil
		})
	})
}
