package main

import (
	"context"
	"database/sql"
	"errors"
)

type siteUnitQuickVegetationInput struct {
	Environment SiteUnitSummaryPreview
	Quick       siteUnitQuickVegetation
	References  vegetationLifeformReferences
	Lifeform    []siteUnitLifeformCoverUnit
	Tables      []ProjectMetadataTable
}

func (s *ContextService) readSiteUnitQuickVegetationInput(ctx context.Context, contextID string, method int,
	hooks publicationReadSnapshotHooks) (siteUnitQuickVegetationInput, error) {
	if ctx == nil {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg requires a context")
	}
	if err := ctx.Err(); err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	if s == nil || s.projects == nil || s.projects.sqlite == nil || s.plots == nil {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg context service is unavailable")
	}
	if method != 1 && method != 2 {
		return siteUnitQuickVegetationInput{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (siteUnitQuickVegetationInput, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *sqliteContext, tx *sql.Tx) (siteUnitQuickVegetationInput, error) {
			return readSiteUnitQuickVegetationSource(ctx, owner, tx, contextID, method)
		})
	})
}

func readSiteUnitQuickVegetationSource(ctx context.Context, owner *sqliteContext, tx *sql.Tx,
	contextID string, method int) (siteUnitQuickVegetationInput, error) {
	if owner.selection.SU == "USysSuTableDynamic" {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg hierarchy/dynamic-break scope is unavailable")
	}
	environment, err := readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, method)
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	tables := append([]ProjectMetadataTable{}, environment.Tables...)
	for _, source := range []struct{ role, table string }{
		{"project", owner.selection.Project + "_Veg"},
		{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"},
	} {
		table, err := readPhysicalLocationTable(ctx, tx, source.role, source.table)
		if err != nil {
			return siteUnitQuickVegetationInput{}, err
		}
		tables = append(tables, table)
	}
	quick, err := prepareSiteUnitQuickVegetation(ctx, owner.selection.Project, owner.selection.SU, 2, tables[4], tables[2])
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	references, err := prepareVegetationLifeformReferences(ctx, tables[5], tables[6])
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	input := siteUnitQuickVegetationInput{
		Environment: environment.Preview, Quick: quick, References: references, Tables: tables,
	}
	input.Lifeform, err = planSiteUnitLifeformCovers(ctx, input)
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	return input, nil
}
