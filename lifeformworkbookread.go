package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type lifeformWorkbookInput struct {
	Lifeform   LifeformSummaryPreview
	Attributes SpeciesAttributeSummaryPreview
	Tables     []ProjectMetadataTable
}

func (c *ContextService) readLifeformWorkbookInput(ctx context.Context, contextID string, hooks publicationReadSnapshotHooks) (lifeformWorkbookInput, error) {
	if ctx == nil {
		return lifeformWorkbookInput{}, errors.New("Lifeform workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return lifeformWorkbookInput{}, err
	}
	if c == nil || c.projects == nil || c.projects.sqlite == nil || c.plots == nil {
		return lifeformWorkbookInput{}, errors.New("Lifeform workbook context service is unavailable")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return lifeformWorkbookInput{}, err
	}
	return withContextPlotRequest(ctx, c, contextID, func(plots *PlotService) (lifeformWorkbookInput, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *sqliteContext, tx *sql.Tx) (lifeformWorkbookInput, error) {
			return readLifeformWorkbookSource(ctx, owner, tx, contextID)
		})
	})
}

func readLifeformWorkbookSource(ctx context.Context, owner *sqliteContext, tx *sql.Tx, contextID string) (lifeformWorkbookInput, error) {
	if owner.selection.SU == "None" || owner.selection.SU == "USysSuTableDynamic" {
		return lifeformWorkbookInput{}, errors.New("Lifeform workbook requires a selected normal SU; hierarchy/dynamic-break scope is unavailable")
	}
	tables := make([]ProjectMetadataTable, 5)
	for i, source := range []struct{ role, table string }{
		{"project", owner.selection.Project + "_Veg"},
		{"su", owner.selection.SU + "_SU"},
		{"VLists", "USysAllSpecs"},
		{"VPro64", "LifeformCodes"},
		{"VLists", "USysSppAttributes"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return lifeformWorkbookInput{}, err
		}
		if count != 1 {
			return lifeformWorkbookInput{}, fmt.Errorf("Lifeform workbook requires original physical table %s.%s", source.role, source.table)
		}
		var err error
		tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return lifeformWorkbookInput{}, err
		}
	}
	lifeform, err := planLifeformSummary(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], tables[2], tables[3])
	if err != nil {
		return lifeformWorkbookInput{}, err
	}
	attributes, err := planSpeciesAttributeSummary(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], tables[4])
	if err != nil {
		return lifeformWorkbookInput{}, err
	}
	return lifeformWorkbookInput{
		Lifeform:   LifeformSummaryPreview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, lifeform},
		Attributes: SpeciesAttributeSummaryPreview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, attributes},
		Tables:     tables,
	}, nil
}
