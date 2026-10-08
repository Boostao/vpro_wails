package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var vegetationLifeformReferenceColumns = []string{"Code", "ScientificName", "Lifeform", "EnglishName", "Codetype"}

type vegetationLifeformReferenceOrigin struct {
	Table string
	RowID string
}

type vegetationLifeformReferences struct {
	Table   ProjectMetadataTable
	Origins map[string][]vegetationLifeformReferenceOrigin
}

func vegetationLifeformInteger(cell ProjectMetadataCell) (*int, error) {
	if _, err := metadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage == "null" {
		return nil, nil
	}
	if cell.Storage != "integer" {
		return nil, errors.New("lifeform requires exact nullable INTEGER storage; no coercion")
	}
	value, err := strconv.ParseInt(*cell.Integer, 10, 16)
	if err != nil {
		return nil, errors.New("lifeform exceeds source INTEGER range")
	}
	result := int(value)
	return &result, nil
}

func vegetationLifeformReferenceSchema(ctx context.Context, table ProjectMetadataTable) (map[string]int, error) {
	return vegetationLifeformReferenceSourceSchema(ctx, table, "Lifeform")
}

func vegetationLifeformReferenceSourceSchema(ctx context.Context, table ProjectMetadataTable, lifeformColumn string) (map[string]int, error) {
	required := append([]string{}, vegetationLifeformReferenceColumns...)
	required[2] = lifeformColumn
	columns, err := siteUnitTransferColumns(table, required...)
	if err != nil {
		return nil, err
	}
	columns["Lifeform"] = columns[lifeformColumn]
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, name := range vegetationLifeformReferenceColumns {
			cell := row.Cells[columns[name]]
			var err error
			if name == "Lifeform" {
				_, err = vegetationLifeformInteger(cell)
			} else {
				_, err = vegetationReportIdentity(cell)
			}
			if err != nil {
				return nil, fmt.Errorf("lifeform reference row %s %s: %w", row.RowID, name, err)
			}
		}
	}
	return columns, ctx.Err()
}

// USysAllSpecies projects five columns and UNIONs whole rows, not species
// codes. NULL CodeType fails the source <> 's' predicate before either join.
func prepareVegetationLifeformReferences(ctx context.Context, master, personal ProjectMetadataTable) (vegetationLifeformReferences, error) {
	fail := func(err error) (vegetationLifeformReferences, error) { return vegetationLifeformReferences{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	result := vegetationLifeformReferences{Table: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
		Origins: map[string][]vegetationLifeformReferenceOrigin{}}
	for _, name := range vegetationLifeformReferenceColumns {
		kind := "TEXT"
		if name == "Lifeform" {
			kind = "INTEGER"
		}
		result.Table.Columns = append(result.Table.Columns, ProjectMetadataColumn{Name: name, DeclaredType: kind})
	}
	seen := map[string]string{}
	for _, source := range []struct {
		name  string
		table ProjectMetadataTable
	}{{"USysAllSpecs", master}, {"USysUserSpp", personal}} {
		lifeformColumn := "Lifeform"
		if source.name == "USysUserSpp" {
			canonical, original := false, false
			for _, column := range source.table.Columns {
				canonical = canonical || column.Name == "Lifeform"
				original = original || column.Name == "LifeForm"
			}
			if canonical && original {
				return fail(errors.New("USysUserSpp has ambiguous LifeForm/Lifeform columns"))
			}
			// Imported VUser retains LifeForm. Canonical fixture projections
			// retain Lifeform; no other spelling or source alias is accepted.
			if original {
				lifeformColumn = "LifeForm"
			}
		}
		columns, err := vegetationLifeformReferenceSourceSchema(ctx, source.table, lifeformColumn)
		if err != nil {
			return fail(err)
		}
		for _, row := range source.table.Rows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			codeType := row.Cells[columns["Codetype"]]
			if codeType.Text == nil || *codeType.Text == "s" || *codeType.Text == "S" {
				continue
			}
			cells := []ProjectMetadataCell{}
			for _, name := range vegetationLifeformReferenceColumns {
				cells = append(cells, cloneSiteUnitCell(row.Cells[columns[name]]))
			}
			key, err := json.Marshal(cells)
			if err != nil {
				return fail(err)
			}
			id := seen[string(key)]
			if id == "" {
				id = strconv.Itoa(len(result.Table.Rows) + 1)
				seen[string(key)] = id
				result.Table.Rows = append(result.Table.Rows, ProjectMetadataRow{RowID: id, Cells: cells})
			}
			result.Origins[id] = append(result.Origins[id], vegetationLifeformReferenceOrigin{source.name, row.RowID})
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
