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
	if options.SelectedUnitFields != nil && options.NoneUnits {
		return fail(errors.New("Long Vegetation cannot combine None and selected-fields report units"))
	}
	sources := []struct{ role, table string }{
		{"project", owner.selection.Project + "_Veg"},
		{"su", owner.selection.SU + "_SU"},
		{"VLists", "USysAllSpecs"},
		{"VPro64", "LayerCode"},
		{"VLists", "MasterSiteUnitList"},
	}
	if options.Quality != nil || options.NoneUnits || options.SelectedUnitFields != nil {
		sources = append(sources, struct{ role, table string }{"project", owner.selection.Project + "_Env"})
	}
	if options.Quality != nil || options.SelectedUnitFields != nil {
		sources = append(sources, struct{ role, table string }{"project", owner.selection.Project + "_Admin"})
	}
	if options.Quality != nil {
		sources = append(sources, struct{ role, table string }{"VLists", "USysTableOfLists"})
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
	var noneUnits []vegetationNoneUnitOccurrence
	var selectedUnits []vegetationSelectedUnitOccurrence
	var err error
	if options.SelectedUnitFields != nil {
		var qualified *vegetationQualitySelection
		if options.Quality != nil {
			quality, err = planLongVegetationQuality(ctx, tables[1], tables[5], tables[6], tables[7], *options.Quality)
			if err != nil {
				return fail(err)
			}
			qualified = &quality
		}
		prepared, selectedUnits, err = prepareLongVegetationSelectedUnits(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[3], tables[5], tables[6], options.SelectedUnitFields, qualified)
		if err == nil && len(selectedUnits) == 0 {
			err = errors.New("Long Vegetation selected report units have no derived memberships; no all-project fallback")
		}
	} else if options.NoneUnits {
		var qualified *vegetationQualitySelection
		if options.Quality != nil {
			quality, err = planLongVegetationQuality(ctx, tables[1], tables[5], tables[6], tables[7], *options.Quality)
			if err != nil {
				return fail(err)
			}
			qualified = &quality
		}
		prepared, noneUnits, err = prepareLongVegetationNoneUnits(ctx, owner.selection.Project, owner.selection.SU,
			tables[0], tables[1], tables[3], tables[5], qualified)
		if err == nil && len(noneUnits) == 0 {
			err = errors.New("Long Vegetation None report units have no joined Env/current-list plots; no all-project fallback")
		}
	} else if options.Quality == nil {
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
	var personal ProjectMetadataTable
	if personalIndex >= 0 {
		personal = tables[personalIndex]
	}
	report, err := planLongVegetationPreparedSource(ctx, prepared, tables[2], personal, tables[3], options)
	if err != nil {
		return fail(err)
	}
	if options.Quality != nil {
		report.Quality = &quality
		if options.NoneUnits || options.SelectedUnitFields != nil {
			if err := addLongVegetationQualityMultiplicity(ctx, &report, quality); err != nil {
				return fail(err)
			}
		}
	}
	report.NoneUnits = noneUnits
	report.SelectedUnits = selectedUnits
	if err := addLongVegetationUnitNames(ctx, &report, tables[4]); err != nil {
		return fail(err)
	}
	return longVegetationReportSource{report, tables}, nil
}

func planLongVegetationPreparedSource(ctx context.Context, prepared VegetationReportPreparation,
	species, personal, layers ProjectMetadataTable, options longVegetationOptions) (vegetationLayerReport, error) {
	reportSpecies, reportOptions, err := longVegetationCodeReferences(ctx, species, options)
	if err != nil {
		return vegetationLayerReport{}, err
	}
	var selectedField *longVegetationSelectedFieldProjection
	if options.SelectedField != "" {
		reportSpecies, selectedField, err = projectLongVegetationSelectedField(ctx, species, options.SelectedField)
		if err != nil {
			return vegetationLayerReport{}, err
		}
		reportOptions.ShowEnglishName = true
		reportOptions.selectedFieldGrouping, err = selectedField.newGroupingGuard()
		if err != nil {
			return vegetationLayerReport{}, err
		}
	}
	var report vegetationLayerReport
	if options.LifeformGrouping {
		references, referenceErr := prepareVegetationLifeformReferences(ctx, species, personal)
		if referenceErr != nil {
			return vegetationLayerReport{}, referenceErr
		}
		report, err = planLongVegetationLifeforms(ctx, prepared, reportSpecies, references.Table, layers, reportOptions)
	} else if options.StrataGrouping {
		report, err = planLongVegetationStrata(ctx, prepared, reportSpecies, layers, reportOptions)
	} else {
		report, err = planLongVegetationLayers(ctx, prepared, reportSpecies, layers, reportOptions)
	}
	if err != nil {
		return vegetationLayerReport{}, err
	}
	if selectedField != nil {
		if err := selectedField.restore(ctx, &report); err != nil {
			return vegetationLayerReport{}, err
		}
		if err := selectedField.validateGrouping(ctx, &report); err != nil {
			return vegetationLayerReport{}, err
		}
		report.SelectedFieldType = selectedField.Kind
	}
	return report, nil
}

func addLongVegetationQualityMultiplicity(ctx context.Context, report *vegetationLayerReport, quality vegetationQualitySelection) error {
	counts := map[string]int{}
	for _, occurrence := range quality.Occurrences {
		if err := ctx.Err(); err != nil {
			return err
		}
		counts[occurrence.Membership.RowID]++
	}
	seen := map[string]bool{}
	for _, occurrence := range quality.Occurrences {
		if err := ctx.Err(); err != nil {
			return err
		}
		id := occurrence.Membership.RowID
		if counts[id] > 1 && !seen[id] {
			report.Diagnostics = append(report.Diagnostics, vegetationLayerDiagnostic{"quality_reference_multiplicity", id, counts[id]})
			seen[id] = true
		}
	}
	return ctx.Err()
}
