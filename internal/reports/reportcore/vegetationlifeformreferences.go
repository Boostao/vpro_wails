package reportcore

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var VegetationLifeformReferenceColumns = []string{"Code", "ScientificName", "Lifeform", "EnglishName", "Codetype"}

type VegetationLifeformReferenceOrigin struct {
	Table string
	RowID string
}

type VegetationLifeformReferences struct {
	Table   vprocore.ProjectMetadataTable
	Origins map[string][]VegetationLifeformReferenceOrigin
}

func AppendVegetationLifeformUnionRow(result *VegetationLifeformReferences, seen map[string]string,
	cells []vprocore.ProjectMetadataCell, origin VegetationLifeformReferenceOrigin) error {
	key, err := json.Marshal(cells)
	if err != nil {
		return err
	}
	id := seen[string(key)]
	if id == "" {
		id = strconv.Itoa(len(result.Table.Rows) + 1)
		seen[string(key)] = id
		result.Table.Rows = append(result.Table.Rows, vprocore.ProjectMetadataRow{RowID: id, Cells: cells})
	}
	result.Origins[id] = append(result.Origins[id], origin)
	return nil
}

func VegetationLifeformReferenceSchema(ctx context.Context, table vprocore.ProjectMetadataTable) (map[string]int, error) {
	return vegetationLifeformReferenceSourceSchema(ctx, table, "Lifeform")
}

func vegetationLifeformReferenceSourceSchema(ctx context.Context, table vprocore.ProjectMetadataTable, lifeformColumn string) (map[string]int, error) {
	return VegetationLifeformReferenceSourceSchemaWithInteger(ctx, table, lifeformColumn, vprocore.VegetationLifeformInteger)
}

func VegetationLifeformReferenceSourceSchemaWithInteger(ctx context.Context, table vprocore.ProjectMetadataTable, lifeformColumn string,
	readInteger func(vprocore.ProjectMetadataCell) (*int, error)) (map[string]int, error) {
	required := append([]string{}, VegetationLifeformReferenceColumns...)
	required[2] = lifeformColumn
	columns, err := vprocore.SiteUnitTransferColumns(table, required...)
	if err != nil {
		return nil, err
	}
	columns["Lifeform"] = columns[lifeformColumn]
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, name := range VegetationLifeformReferenceColumns {
			cell := row.Cells[columns[name]]
			var err error
			if name == "Lifeform" {
				_, err = readInteger(cell)
			} else {
				_, err = vprocore.VegetationReportIdentity(cell)
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
func PrepareVegetationLifeformReferences(ctx context.Context, master, personal vprocore.ProjectMetadataTable) (VegetationLifeformReferences, error) {
	fail := func(err error) (VegetationLifeformReferences, error) { return VegetationLifeformReferences{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	result := VegetationLifeformReferences{Table: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
		Origins: map[string][]VegetationLifeformReferenceOrigin{}}
	for _, name := range VegetationLifeformReferenceColumns {
		kind := "TEXT"
		if name == "Lifeform" {
			kind = "INTEGER"
		}
		result.Table.Columns = append(result.Table.Columns, vprocore.ProjectMetadataColumn{Name: name, DeclaredType: kind})
	}
	seen := map[string]string{}
	for _, source := range []struct {
		name  string
		table vprocore.ProjectMetadataTable
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
			cells := []vprocore.ProjectMetadataCell{}
			for _, name := range VegetationLifeformReferenceColumns {
				cells = append(cells, vprocore.CloneSiteUnitCell(row.Cells[columns[name]]))
			}
			if err := AppendVegetationLifeformUnionRow(&result, seen, cells,
				VegetationLifeformReferenceOrigin{source.name, row.RowID}); err != nil {
				return fail(err)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
