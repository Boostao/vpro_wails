package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

func ReadSiteUnitDetailNormalSUScope(s *vprocore.ContextService, ctx context.Context, contextID string,
	maxJoinedRows int, hooks vprocore.PublicationReadSnapshotHooks) (siteUnitDetailOwnedScope, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (siteUnitDetailOwnedScope, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (siteUnitDetailOwnedScope, error) {
			if owner.Selection.SU == "None" {
				return siteUnitDetailOwnedScope{}, errors.New("site unit detail scope requires an explicitly selected normal SU")
			}
			env, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			admin, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Admin")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			su, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "su", owner.Selection.SU+"_SU")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			scope, err := prepareSiteUnitDetailScope(ctx, owner.Selection.Project, owner.Selection.SU,
				siteUnitDetailSelectedSU, maxJoinedRows, env, admin, su)
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			return siteUnitDetailOwnedScope{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, scope}, nil
		})
	})
}
