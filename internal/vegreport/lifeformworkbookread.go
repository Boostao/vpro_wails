package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type lifeformWorkbookInput struct {
	Lifeform   LifeformSummaryPreview
	Attributes SpeciesAttributeSummaryPreview
	Tables     []vprocore.ProjectMetadataTable
}

func ReadLifeformWorkbookInput(c *vprocore.ContextService, ctx context.Context, contextID string, hooks vprocore.PublicationReadSnapshotHooks) (lifeformWorkbookInput, error) {
	if ctx == nil {
		return lifeformWorkbookInput{}, errors.New("Lifeform workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return lifeformWorkbookInput{}, err
	}
	if c == nil || c.Projects == nil || c.Projects.Sqlite == nil || c.Plots == nil {
		return lifeformWorkbookInput{}, errors.New("Lifeform workbook context service is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return lifeformWorkbookInput{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, c, contextID, func(plots *vprocore.PlotService) (lifeformWorkbookInput, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (lifeformWorkbookInput, error) {
			return readLifeformWorkbookSource(ctx, owner, tx, contextID)
		})
	})
}

func readLifeformWorkbookSource(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID string) (lifeformWorkbookInput, error) {
	if owner.Selection.SU == "None" || owner.Selection.SU == "USysSuTableDynamic" {
		return lifeformWorkbookInput{}, errors.New("Lifeform workbook requires a selected normal SU; hierarchy/dynamic-break scope is unavailable")
	}
	tables := make([]vprocore.ProjectMetadataTable, 5)
	for i, source := range []struct{ role, table string }{
		{"project", owner.Selection.Project + "_Veg"},
		{"su", owner.Selection.SU + "_SU"},
		{"VLists", "USysAllSpecs"},
		{"VPro64", "LifeformCodes"},
		{"VLists", "USysSppAttributes"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return lifeformWorkbookInput{}, err
		}
		if count != 1 {
			return lifeformWorkbookInput{}, fmt.Errorf("Lifeform workbook requires original physical table %s.%s", source.role, source.table)
		}
		var err error
		tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return lifeformWorkbookInput{}, err
		}
	}
	lifeform, err := planLifeformSummary(ctx, owner.Selection.Project, owner.Selection.SU, tables[0], tables[1], tables[2], tables[3])
	if err != nil {
		return lifeformWorkbookInput{}, err
	}
	attributes, err := planSpeciesAttributeSummary(ctx, owner.Selection.Project, owner.Selection.SU, tables[0], tables[1], tables[4])
	if err != nil {
		return lifeformWorkbookInput{}, err
	}
	return lifeformWorkbookInput{
		Lifeform:   LifeformSummaryPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, lifeform},
		Attributes: SpeciesAttributeSummaryPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, attributes},
		Tables:     tables,
	}, nil
}
