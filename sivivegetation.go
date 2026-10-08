package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type SIVIVegetationProjection struct {
	Form, Query string
	Columns     []string
	Rows        []ProjectMetadataRow
}

type siviVegetationProjection = SIVIVegetationProjection

func (projection *SIVIVegetationProjection) UnmarshalJSON(data []byte) error {
	type plain SIVIVegetationProjection
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "Form", "Query", "Columns", "Rows"); err != nil {
		return fmt.Errorf("SIVI original source transport: %w", err)
	}
	*projection = SIVIVegetationProjection(decoded)
	return nil
}

// This private read model preserves source query membership, not editor
// authorization. Historical aggregate-height cells remain typed and unmodified.
func projectSIVIVegetation(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable) ([]siviVegetationProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !utf8.ValidString(plot) || strings.ContainsRune(plot, 0) {
		return nil, errors.New("SIVI projection requires a literal valid plot identity")
	}
	a := []string{"ID", "PlotNumber", "Species", "Cover1", "Cover2", "Cover3", "TotalA", "HeightA",
		"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "HeightB", "Collected"}
	form := "SubVegA-SIVI_BC"
	if extended {
		form = "SubVegA-SIVI"
	}
	result := []siviVegetationProjection{
		{form, "USysVegA", a, []ProjectMetadataRow{}},
		{"SubVegC-SIVI", "USysVegC", []string{"ID", "PlotNumber", "Species", "Cover6", "Height6", "Collected"}, []ProjectMetadataRow{}},
		{"SubVegD-SIVI", "USysVegD", []string{"ID", "PlotNumber", "Species", "Cover7", "Cover8", "Cover9", "Collected"}, []ProjectMetadataRow{}},
	}
	predicates := [][]string{
		{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB"},
		{"Cover6"}, {"Cover7", "Cover8", "Cover9"},
	}
	required := []string{}
	for i, projection := range result {
		required = append(required, projection.Columns...)
		required = append(required, predicates[i]...)
	}
	columns, err := siteUnitTransferColumns(veg, required...)
	if err != nil {
		return nil, fmt.Errorf("SIVI physical vegetation schema: %w", err)
	}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		identity, err := vegetationReportIdentity(row.Cells[columns["PlotNumber"]])
		if err != nil {
			return nil, fmt.Errorf("SIVI row %s plot identity: %w", row.RowID, err)
		}
		if identity == nil || *identity != plot {
			continue
		}
		for i := range result {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			visible := false
			for _, column := range predicates[i] {
				if row.Cells[columns[column]].Storage != "null" {
					visible = true
				}
			}
			if !visible {
				continue
			}
			projected := ProjectMetadataRow{RowID: row.RowID, Cells: make([]ProjectMetadataCell, len(result[i].Columns))}
			for j, column := range result[i].Columns {
				projected.Cells[j] = cloneSiteUnitCell(row.Cells[columns[column]])
			}
			result[i].Rows = append(result[i].Rows, projected)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
