package main

import (
	"context"
	"database/sql"
	"errors"
)

type siteUnitSummaryWorkbookInput struct {
	Preview SiteUnitSummaryPreview
	Tables  []ProjectMetadataTable
}

func (s *ContextService) readSiteUnitSummaryWorkbookInput(ctx context.Context, contextID string, method int,
	hooks publicationReadSnapshotHooks) (siteUnitSummaryWorkbookInput, error) {
	if ctx == nil {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return siteUnitSummaryWorkbookInput{}, err
	}
	if s == nil || s.projects == nil || s.projects.sqlite == nil || s.plots == nil {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment workbook context service is unavailable")
	}
	if method != 1 && method != 2 {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return siteUnitSummaryWorkbookInput{}, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (siteUnitSummaryWorkbookInput, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *sqliteContext, tx *sql.Tx) (siteUnitSummaryWorkbookInput, error) {
			return readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, method)
		})
	})
}

func readSiteUnitSummaryWorkbookSource(ctx context.Context, owner *sqliteContext, tx *sql.Tx,
	contextID string, method int) (siteUnitSummaryWorkbookInput, error) {
	if owner.selection.SU == "None" {
		return siteUnitSummaryWorkbookInput{}, errors.New("Summary Environment requires an explicitly selected normal SU")
	}
	tables := make([]ProjectMetadataTable, 4)
	for i, source := range []struct{ role, table string }{
		{"project", owner.selection.Project + "_Env"}, {"project", owner.selection.Project + "_Admin"},
		{"su", owner.selection.SU + "_SU"}, {"VLists", "MasterSiteUnitList"},
	} {
		table, err := readPhysicalLocationTable(ctx, tx, source.role, source.table)
		if err != nil {
			return siteUnitSummaryWorkbookInput{}, err
		}
		tables[i] = table
	}
	report, err := planSiteUnitSummary(ctx, owner.selection.Project, owner.selection.SU, method, 1000000,
		tables[0], tables[1], tables[2], tables[3])
	if err != nil {
		return siteUnitSummaryWorkbookInput{}, err
	}
	return siteUnitSummaryWorkbookInput{
		Preview: SiteUnitSummaryPreview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, report},
		Tables:  tables,
	}, nil
}
