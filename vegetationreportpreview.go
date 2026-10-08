package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *ContextService) previewLongVegetationLayers(ctx context.Context, contextID string) (vegetationLayerReport, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (vegetationLayerReport, error) {
		options, err := loadLongVegetationOptions(plots)
		if err != nil {
			return vegetationLayerReport{}, err
		}
		if err := s.checkLongVegetationGrouping(options); err != nil {
			return vegetationLayerReport{}, err
		}
		return readLongVegetationLayers(ctx, plots, options)
	})
}

func readLongVegetationLayers(ctx context.Context, plots *PlotService, options longVegetationOptions) (result vegetationLayerReport, resultErr error) {
	owner := plots.projects.sqlite
	if owner.selection.SU == "None" {
		return result, errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback")
	}
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		return result, err
	}
	defer owner.mu.Unlock()
	if err := profileOwnedFiles(owner); err != nil {
		return result, err
	}
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = vegetationLayerReport{}
		}
	}()
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
			return result, err
		}
		if count != 1 {
			return result, fmt.Errorf("Long Vegetation requires original physical table %s.%s", source.role, source.table)
		}
		tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return result, err
		}
	}
	var prepared VegetationReportPreparation
	var quality vegetationQualitySelection
	if options.Quality == nil {
		prepared, err = prepareLongVegetation(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], tables[3])
	} else {
		prepared, quality, err = prepareQualityLongVegetation(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[3], tables[5], tables[6], tables[7], *options.Quality)
	}
	if err != nil {
		return result, err
	}
	if options.Quality != nil && len(quality.Occurrences) == 0 {
		return result, errors.New("Sorry, no plots. Please adjust your plot quality standards or use a different site unit table")
	}
	reportSpecies, reportOptions, err := longVegetationCodeReferences(ctx, tables[2], options)
	if err != nil {
		return result, err
	}
	if options.LifeformGrouping {
		references, referenceErr := prepareVegetationLifeformReferences(ctx, tables[2], tables[personalIndex])
		if referenceErr != nil {
			return result, referenceErr
		}
		result, err = planLongVegetationLifeforms(ctx, prepared, reportSpecies, references.Table, tables[3], reportOptions)
	} else if options.StrataGrouping {
		result, err = planLongVegetationStrata(ctx, prepared, reportSpecies, tables[3], reportOptions)
	} else {
		result, err = planLongVegetationLayers(ctx, prepared, reportSpecies, tables[3], reportOptions)
	}
	if err != nil {
		return result, err
	}
	if options.Quality != nil {
		result.Quality = &quality
	}
	if err := addLongVegetationUnitNames(ctx, &result, tables[4]); err != nil {
		return result, err
	}
	if err := profileOwnedFiles(owner); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
