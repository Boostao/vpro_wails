package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

type siteUnitSummaryWorkbookInput struct {
	Preview SiteUnitSummaryPreview
	Tables  []vprocore.ProjectMetadataTable
}

func ReadSiteUnitSummaryWorkbookInput(s *vprocore.ContextService, ctx context.Context, contextID string, method int,
	hooks vprocore.PublicationReadSnapshotHooks) (siteUnitSummaryWorkbookInput, error) {
	if ctx == nil {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return siteUnitSummaryWorkbookInput{}, err
	}
	if s == nil || s.Projects == nil || s.Projects.Sqlite == nil || s.Plots == nil {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment workbook context service is unavailable")
	}
	if method != 1 && method != 2 {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return siteUnitSummaryWorkbookInput{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (siteUnitSummaryWorkbookInput, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (siteUnitSummaryWorkbookInput, error) {
			return readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, method)
		})
	})
}

func readSiteUnitSummaryWorkbookSource(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx,
	contextID string, method int) (siteUnitSummaryWorkbookInput, error) {
	if owner.Selection.SU == "None" {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment requires an explicitly selected normal SU")
	}
	tables := make([]vprocore.ProjectMetadataTable, 4)
	for i, source := range []struct{ role, table string }{
		{"project", owner.Selection.Project + "_Env"}, {"project", owner.Selection.Project + "_Admin"},
		{"su", owner.Selection.SU + "_SU"}, {"VLists", "MasterSiteUnitList"},
	} {
		table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, source.role, source.table)
		if err != nil {
			return siteUnitSummaryWorkbookInput{}, err
		}
		tables[i] = table
	}
	report, err := planSiteUnitSummary(ctx, owner.Selection.Project, owner.Selection.SU, method, 1000000,
		tables[0], tables[1], tables[2], tables[3])
	if err != nil {
		return siteUnitSummaryWorkbookInput{}, err
	}
	return siteUnitSummaryWorkbookInput{
		Preview: SiteUnitSummaryPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, report},
		Tables:  tables,
	}, nil
}
