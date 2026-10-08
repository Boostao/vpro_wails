package main

import "context"

const longVegetationCodeFeatureEnvironment = "VPRO_LONG_VEGETATION_CODE"

func longVegetationCodeReferences(ctx context.Context, source ProjectMetadataTable, options longVegetationOptions) (ProjectMetadataTable, longVegetationOptions, error) {
	if err := ctx.Err(); err != nil {
		return ProjectMetadataTable{}, longVegetationOptions{}, err
	}
	if !options.ShowSpeciesCode {
		return source, options, nil
	}
	columns, err := siteUnitTransferColumns(source, "Code", "EnglishName")
	if err != nil {
		return ProjectMetadataTable{}, longVegetationOptions{}, err
	}
	result := ProjectMetadataTable{
		Columns: append([]ProjectMetadataColumn{}, source.Columns...),
		Rows:    make([]ProjectMetadataRow, 0, len(source.Rows)),
	}
	for _, row := range source.Rows {
		if err := ctx.Err(); err != nil {
			return ProjectMetadataTable{}, longVegetationOptions{}, err
		}
		cells := make([]ProjectMetadataCell, len(row.Cells))
		for i, cell := range row.Cells {
			cells[i] = cloneSiteUnitCell(cell)
		}
		cells[columns["EnglishName"]] = cloneSiteUnitCell(row.Cells[columns["Code"]])
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	options.ShowEnglishName, options.ShowSpeciesCode = true, false
	return result, options, nil
}
