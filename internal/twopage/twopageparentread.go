package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
)

func ReadTwoPageParent(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string) (*sourceform.SiviParentProjection, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SiviParentProjection, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*sourceform.SiviParentProjection, error) {
			tables, err := sourceform.ReadSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			return ProjectTwoPageParent(ctx, contextID, owner.Selection.Project, plot, form, tables[0], tables[1])
		})
	})
}
