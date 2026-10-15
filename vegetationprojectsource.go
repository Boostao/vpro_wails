package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// A nil field list means the original Env-only source3, not an empty source2 draft.
func readLongVegetationProjectSource(ctx context.Context, owner *sqliteContext, tx *sql.Tx,
	fields []string, options longVegetationOptions) (longVegetationReportSource, error) {
	if ctx == nil || owner == nil || tx == nil {
		return longVegetationReportSource{}, errors.New("project report source requires an owned physical read snapshot")
	}
	if err := ctx.Err(); err != nil {
		return longVegetationReportSource{}, err
	}
	if options.NoneUnits || options.SelectedUnitFields != nil {
		return longVegetationReportSource{}, errors.New("project report source cannot combine selected-list unit options")
	}
	if fields == nil && owner.selection.SU != "None" {
		return longVegetationReportSource{}, errors.New("project Env-only source3 requires the explicit original None SU context")
	}
	if options.Quality != nil && (fields == nil || owner.selection.SU == "None") {
		return longVegetationReportSource{}, errors.New("project quality requires actual original selected-SU authority")
	}
	sources := []struct{ role, table string }{
		{"project", owner.selection.Project + "_Veg"},
		{"project", owner.selection.Project + "_Env"},
		{"VLists", "USysAllSpecs"},
		{"VPro64", "LayerCode"},
		{"VLists", "MasterSiteUnitList"},
	}
	adminIndex, suIndex, qualityIndex, personalIndex := -1, -1, -1, -1
	if fields != nil {
		adminIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"project", owner.selection.Project + "_Admin"})
		if owner.selection.SU != "None" {
			suIndex = len(sources)
			sources = append(sources, struct{ role, table string }{"su", owner.selection.SU + "_SU"})
		}
	}
	if options.Quality != nil {
		qualityIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"VLists", "USysTableOfLists"})
	}
	if options.LifeformGrouping {
		personalIndex = len(sources)
		sources = append(sources, struct{ role, table string }{"VUser", "USysUserSpp"})
	}
	tables := make([]ProjectMetadataTable, len(sources))
	for i, source := range sources {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return longVegetationReportSource{}, err
		}
		if count != 1 {
			return longVegetationReportSource{}, fmt.Errorf("project report requires original physical table %s.%s", source.role, source.table)
		}
		var err error
		tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return longVegetationReportSource{}, err
		}
	}
	var project vegetationProjectPreparation
	var quality *vegetationQualitySelection
	var err error
	if fields == nil {
		project, err = prepareLongVegetationProjectNoneUnits(ctx, owner.selection.Project, tables[0], tables[3], tables[1])
	} else {
		var su ProjectMetadataTable
		if suIndex >= 0 {
			su = tables[suIndex]
		}
		if options.Quality != nil {
			qualified, qualityErr := planLongVegetationQuality(ctx, su, tables[1], tables[adminIndex], tables[qualityIndex], *options.Quality)
			if qualityErr != nil {
				return longVegetationReportSource{}, qualityErr
			}
			quality = &qualified
		}
		project, err = prepareLongVegetationProjectSelectedUnits(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[3], tables[1], tables[adminIndex], su, fields, quality)
	}
	if err != nil {
		return longVegetationReportSource{}, err
	}
	if quality != nil && len(quality.Occurrences) == 0 {
		return longVegetationReportSource{}, errors.New("Sorry, no plots. Please adjust your plot quality standards or use a different site unit table")
	}
	prepared, err := projectReportPreparation(ctx, project)
	if err != nil {
		return longVegetationReportSource{}, err
	}
	if len(prepared.projectUnitInputs) == 0 {
		return longVegetationReportSource{}, errors.New("project report has no original derived unit tuples; empty-unit output is unavailable")
	}
	var personal ProjectMetadataTable
	if personalIndex >= 0 {
		personal = tables[personalIndex]
	}
	report, err := planLongVegetationPreparedSource(ctx, prepared, tables[2], personal, tables[3], options)
	if err != nil {
		return longVegetationReportSource{}, err
	}
	if quality != nil {
		report.Quality = quality
		if err := addLongVegetationQualityMultiplicity(ctx, &report, *quality); err != nil {
			return longVegetationReportSource{}, err
		}
	}
	if err := addLongVegetationUnitNames(ctx, &report, tables[4]); err != nil {
		return longVegetationReportSource{}, err
	}
	return longVegetationReportSource{report, tables}, nil
}
