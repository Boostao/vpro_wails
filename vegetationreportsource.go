package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type longVegetationReportSource struct {
	Report vegetationLayerReport
	Tables []ProjectMetadataTable
}

func readLongVegetationReportSource(ctx context.Context, owner *sqliteContext, tx *sql.Tx, options longVegetationOptions) (longVegetationReportSource, error) {
	fail := func(err error) (longVegetationReportSource, error) { return longVegetationReportSource{}, err }
	if owner.selection.SU == "None" {
		return fail(errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback"))
	}
	sources := []struct{ role, table string }{
		{"project", owner.selection.Project + "_Veg"},
		{"su", owner.selection.SU + "_SU"},
		{"VLists", "USysAllSpecs"},
		{"VPro64", "LayerCode"},
		{"VLists", "MasterSiteUnitList"},
	}
	if options.Quality != nil {
		sources = append(sources, struct{ role, table string }{"project", owner.selection.Project + "_Env"},
			struct{ role, table string }{"project", owner.selection.Project + "_Admin"},
			struct{ role, table string }{"VLists", "USysTableOfLists"})
	}
	personalIndex := -1
	if options.LifeformGrouping {
		personalIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"VUser", "USysUserSpp"})
	}
	tables := make([]ProjectMetadataTable, len(sources))
	for i, source := range sources {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(fmt.Errorf("Long Vegetation requires original physical table %s.%s", source.role, source.table))
		}
		var err error
		tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return fail(err)
		}
	}
	var prepared VegetationReportPreparation
	var quality vegetationQualitySelection
	var err error
	if options.Quality == nil {
		prepared, err = prepareLongVegetation(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], tables[3])
	} else {
		prepared, quality, err = prepareQualityLongVegetation(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[3], tables[5], tables[6], tables[7], *options.Quality)
	}
	if err != nil {
		return fail(err)
	}
	if options.Quality != nil && len(quality.Occurrences) == 0 {
		return fail(errors.New("Sorry, no plots. Please adjust your plot quality standards or use a different site unit table"))
	}
	reportSpecies, reportOptions, err := longVegetationCodeReferences(ctx, tables[2], options)
	if err != nil {
		return fail(err)
	}
	var report vegetationLayerReport
	if options.LifeformGrouping {
		references, referenceErr := prepareVegetationLifeformReferences(ctx, tables[2], tables[personalIndex])
		if referenceErr != nil {
			return fail(referenceErr)
		}
		report, err = planLongVegetationLifeforms(ctx, prepared, reportSpecies, references.Table, tables[3], reportOptions)
	} else if options.StrataGrouping {
		report, err = planLongVegetationStrata(ctx, prepared, reportSpecies, tables[3], reportOptions)
	} else {
		report, err = planLongVegetationLayers(ctx, prepared, reportSpecies, tables[3], reportOptions)
	}
	if err != nil {
		return fail(err)
	}
	if options.Quality != nil {
		report.Quality = &quality
	}
	if err := addLongVegetationUnitNames(ctx, &report, tables[4]); err != nil {
		return fail(err)
	}
	return longVegetationReportSource{report, tables}, nil
}
