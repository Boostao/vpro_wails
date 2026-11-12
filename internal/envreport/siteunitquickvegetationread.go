package envreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

type siteUnitQuickVegetationInput struct {
	Environment SiteUnitSummaryPreview
	Quick       reportcore.SiteUnitQuickVegetation
	References  reportcore.VegetationLifeformReferences
	Lifeform    []siteUnitLifeformCoverUnit
	Tables      []vprocore.ProjectMetadataTable
}

func ReadSiteUnitQuickVegetationInput(s *vprocore.ContextService, ctx context.Context, contextID string, method int,
	hooks vprocore.PublicationReadSnapshotHooks) (siteUnitQuickVegetationInput, error) {
	if ctx == nil {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg requires a context")
	}
	if err := ctx.Err(); err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	if s == nil || s.Projects == nil || s.Projects.Sqlite == nil || s.Plots == nil {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg context service is unavailable")
	}
	if method != 1 && method != 2 {
		return siteUnitQuickVegetationInput{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (siteUnitQuickVegetationInput, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (siteUnitQuickVegetationInput, error) {
			return readSiteUnitQuickVegetationSource(ctx, owner, tx, contextID, method)
		})
	})
}

func readSiteUnitQuickVegetationSource(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx,
	contextID string, method int) (siteUnitQuickVegetationInput, error) {
	if owner.Selection.SU == "USysSuTableDynamic" {
		return siteUnitQuickVegetationInput{}, errors.New("Summary QuickVeg hierarchy/dynamic-break scope is unavailable")
	}
	environment, err := readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, method)
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	tables := append([]vprocore.ProjectMetadataTable{}, environment.Tables...)
	for _, source := range []struct{ role, table string }{
		{"project", owner.Selection.Project + "_Veg"},
		{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"},
	} {
		table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, source.role, source.table)
		if err != nil {
			return siteUnitQuickVegetationInput{}, err
		}
		tables = append(tables, table)
	}
	quick, err := reportcore.PrepareSiteUnitQuickVegetation(ctx, owner.Selection.Project, owner.Selection.SU, 2, tables[4], tables[2])
	if err != nil {
		return siteUnitQuickVegetationInput{}, err
	}
	references, err := reportcore.PrepareVegetationLifeformReferences(ctx, tables[5], tables[6])
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
