package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "context"

const LongVegetationCodeFeatureEnvironment = "VPRO_LONG_VEGETATION_CODE"

func longVegetationCodeReferences(ctx context.Context, source vprocore.ProjectMetadataTable, options longVegetationOptions) (vprocore.ProjectMetadataTable, longVegetationOptions, error) {
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataTable{}, longVegetationOptions{}, err
	}
	if !options.ShowSpeciesCode {
		return source, options, nil
	}
	columns, err := vprocore.SiteUnitTransferColumns(source, "Code", "EnglishName")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, longVegetationOptions{}, err
	}
	result := vprocore.ProjectMetadataTable{
		Columns: append([]vprocore.ProjectMetadataColumn{}, source.Columns...),
		Rows:    make([]vprocore.ProjectMetadataRow, 0, len(source.Rows)),
	}
	for _, row := range source.Rows {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, longVegetationOptions{}, err
		}
		cells := make([]vprocore.ProjectMetadataCell, len(row.Cells))
		for i, cell := range row.Cells {
			cells[i] = vprocore.CloneSiteUnitCell(cell)
		}
		cells[columns["EnglishName"]] = vprocore.CloneSiteUnitCell(row.Cells[columns["Code"]])
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	options.ShowEnglishName, options.ShowSpeciesCode = true, false
	return result, options, nil
}
