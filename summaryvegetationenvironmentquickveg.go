package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
)

func prepareSummaryEnvironmentalQuickVeg(ctx context.Context, project, suName string,
	veg, su ProjectMetadataTable) (ProjectMetadataTable, error) {
	fail := func(err error) (ProjectMetadataTable, error) {
		return ProjectMetadataTable{}, fmt.Errorf("Summary environmental raw QuickVeg: %w", err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	names := []string{"PlotNumber", "Species"}
	for layer := 1; layer <= 9; layer++ {
		names = append(names, "Cover"+strconv.Itoa(layer))
	}
	columns, err := siteUnitTransferColumns(veg, names...)
	if err != nil {
		return fail(err)
	}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		for _, name := range []string{"PlotNumber", "Species"} {
			cell := row.Cells[columns[name]]
			limit := 8
			if name == "PlotNumber" {
				limit = 7
			}
			if err := lifeformSummaryText(cell, limit); err != nil {
				return fail(fmt.Errorf("Veg row %s %s: %w", row.RowID, name, err))
			}
			if cell.Storage != "text" || cell.Text == nil || *cell.Text == "" {
				return fail(fmt.Errorf("Veg row %s %s requires original nonempty TEXT", row.RowID, name))
			}
		}
		for _, name := range names[2:] {
			number, err := vegetationReportNumber(row.Cells[columns[name]])
			if err != nil {
				return fail(fmt.Errorf("Veg row %s %s: %w", row.RowID, name, err))
			}
			if number == nil {
				continue
			}
			single, err := lifeformSummarySingle(number)
			if err != nil {
				return fail(fmt.Errorf("Veg row %s %s: %w", row.RowID, name, err))
			}
			if new(big.Rat).SetFloat64(single).Cmp(number) != 0 {
				return fail(fmt.Errorf("Veg row %s %s requires exactly assigned finite SINGLE", row.RowID, name))
			}
		}
	}
	prepared, err := prepareSiteUnitQuickVegetation(ctx, project, suName, 1, veg, su)
	if err != nil {
		return fail(err)
	}
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Plotnumber", DeclaredType: "TEXT"}, {Name: "Species", DeclaredType: "TEXT"}, {Name: "Cover", DeclaredType: "REAL"},
	}, Rows: make([]ProjectMetadataRow, 0, len(prepared.Entries))}
	for i, entry := range prepared.Entries {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, cover := entry.PlotNumber, entry.Cover
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: []ProjectMetadataCell{
			{Storage: "text", Text: &plot}, cloneSiteUnitCell(entry.Species), {Storage: "real", Real: &cover},
		}})
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
