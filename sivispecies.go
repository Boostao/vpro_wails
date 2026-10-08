package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

const siviSpeciesHistoryTable = "__VPRO_SIVISpeciesHistory"
const siviSpeciesHistorySQL = `CREATE TABLE "__VPRO_SIVISpeciesHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviSpeciesWritePolicy() siviVegetationWritePolicy {
	return siviVegetationWritePolicy{"Species", siviSpeciesHistoryTable, siviSpeciesHistorySQL,
		[]string{"Species"}, planSIVISpeciesCells}
}

func speciesSIVIDrafts(ctx context.Context, edits []SIVISpeciesEdit) ([]siviHeightEdit, error) {
	if ctx == nil {
		return nil, errors.New("SIVI Species planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI Species planning requires explicit edits")
	}
	drafts := make([]siviHeightEdit, len(edits))
	for i, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := strconv.ParseInt(edit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != edit.RowID {
			return nil, errors.New("SIVI Species requires the original signed64 physical row identity")
		}
		if _, err := metadataCellValue(edit.Expected); err != nil {
			return nil, err
		}
		value, err := projectMetadataCell("text", []byte(edit.Value))
		if err != nil {
			return nil, err
		}
		drafts[i] = siviHeightEdit{edit.RowID, edit.Form, "Species", cloneSiteUnitCell(edit.Expected), value}
	}
	return drafts, ctx.Err()
}

func planSIVISpeciesCells(ctx context.Context, plot string, extended bool, veg ProjectMetadataTable, edits []siviHeightEdit) ([]siviHeightAssignment, error) {
	return planSIVIChildEdits(ctx, plot, extended, veg, edits, "Species",
		func(group int, column string) bool { return group >= 0 && group < 3 && column == "Species" },
		func(_ string, cell ProjectMetadataCell) error {
			if cell.Storage != "text" || cell.Text == nil {
				return errors.New("new Species requires nonempty literal text")
			}
			return validateVegetationSpeciesLookup(*cell.Text)
		})
}

var siviSpeciesMasterColumns = []string{"Code", "ScientificName", "Lifeform", "EnglishName", "Codetype", "OldCode"}
var siviSpeciesPersonalColumns = []string{"Code", "ScientificName", "LifeForm", "EnglishName", "Codetype"}

// Only identifier recasing is allowed. Physical definitions, including duplicate
// codes, NULLs and empty metadata, retain their original typed cells and rowids.
func projectSIVISpeciesDefinitions(ctx context.Context, source ProjectMetadataTable, personal bool) (ProjectMetadataTable, error) {
	if ctx == nil {
		return ProjectMetadataTable{}, errors.New("Species references require a context")
	}
	if _, err := siteUnitTransferColumns(source); err != nil {
		return ProjectMetadataTable{}, err
	}
	required := siviSpeciesMasterColumns
	if personal {
		required = siviSpeciesPersonalColumns
	}
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
	indices := make([]int, len(required))
	for i, name := range required {
		found := -1
		for j, column := range source.Columns {
			if siviSpeciesASCIIEqual(column.Name, name) {
				if found >= 0 {
					return ProjectMetadataTable{}, fmt.Errorf("ambiguous Species reference column %s", name)
				}
				found = j
			}
		}
		if found < 0 {
			return ProjectMetadataTable{}, fmt.Errorf("missing Species reference column %s", name)
		}
		indices[i] = found
		result.Columns = append(result.Columns, source.Columns[found])
	}
	for _, row := range source.Rows {
		if err := ctx.Err(); err != nil {
			return ProjectMetadataTable{}, err
		}
		projected := ProjectMetadataRow{RowID: row.RowID, Cells: make([]ProjectMetadataCell, len(indices))}
		for i, index := range indices {
			cell := row.Cells[index]
			var err error
			if i == 2 {
				_, err = vegetationLifeformInteger(cell)
			} else {
				_, err = vegetationReportIdentity(cell)
			}
			if err != nil {
				return ProjectMetadataTable{}, fmt.Errorf("Species definition %s.%s: %w", row.RowID, required[i], err)
			}
			projected.Cells[i] = cloneSiteUnitCell(cell)
		}
		result.Rows = append(result.Rows, projected)
	}
	return result, ctx.Err()
}

func siviSpeciesASCIIEqual(a, b string) bool {
	fold := func(value string) string {
		raw := []byte(value)
		for i, c := range raw {
			if c >= 'A' && c <= 'Z' {
				raw[i] = c + ('a' - 'A')
			}
		}
		return string(raw)
	}
	return fold(a) == fold(b)
}

func validateSIVISpeciesMembership(ctx context.Context, edit SIVISpeciesEdit, references SIVISpeciesReferences) error {
	update := VegetationSpeciesUpdate{Form: edit.Form, Value: edit.Value, Decision: edit.Decision, Entered: edit.Entered, Selected: edit.Selected}
	if err := validateVegetationSpeciesDecision(update); err != nil {
		return err
	}
	if err := validateVegetationSpeciesLookup(edit.Value); err != nil {
		return err
	}
	var lifeforms map[int]bool
	switch edit.Form {
	case "SubVegA-SIVI_BC", "SubVegA-SIVI":
		lifeforms = map[int]bool{1: true, 2: true, 3: true, 4: true}
	case "SubVegC-SIVI":
		lifeforms = map[int]bool{5: true, 6: true, 7: true, 8: true, 12: true}
	case "SubVegD-SIVI":
		lifeforms = map[int]bool{1: true, 2: true, 9: true, 10: true, 11: true}
	default:
		return errors.New("unavailable SIVI Species form")
	}
	hasAlias, replacement := false, false
	if edit.Decision != "" {
		for _, row := range references.Master.Rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			if row.Cells[5].Text != nil && row.Cells[0].Text != nil &&
				siviSpeciesASCIIEqual(*row.Cells[5].Text, *edit.Entered) {
				hasAlias = true
				replacement = replacement || edit.Selected != nil && *row.Cells[0].Text == *edit.Selected
			}
		}
		switch edit.Decision {
		case "keep":
			if hasAlias {
				return ctx.Err()
			}
		case "replace":
			if replacement {
				return ctx.Err()
			}
		case "user":
			if hasAlias {
				return errors.New("old-code definitions take precedence over personal Species decisions")
			}
			for _, row := range references.Personal.Rows {
				if err := ctx.Err(); err != nil {
					return err
				}
				if row.Cells[0].Text != nil && siviSpeciesASCIIEqual(*row.Cells[0].Text, *edit.Entered) &&
					*row.Cells[0].Text == *edit.Selected {
					return nil
				}
			}
		}
		return errors.New("Species decision definition changed; review the references again")
	}
	for _, table := range []ProjectMetadataTable{references.Master, references.Personal} {
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			code, kind := row.Cells[0].Text, row.Cells[4].Text
			lifeform, err := vegetationLifeformInteger(row.Cells[2])
			if err != nil {
				return err
			}
			if code != nil && *code == edit.Value && kind != nil &&
				(siviSpeciesASCIIEqual(*kind, "u") || siviSpeciesASCIIEqual(*kind, "x")) &&
				lifeform != nil && lifeforms[*lifeform] {
				return nil
			}
		}
	}
	return errors.New("Species is not listed in the caller's source form")
}

func validateSIVISpeciesAssignments(ctx context.Context, assignments []siviHeightAssignment, edits []SIVISpeciesEdit, references SIVISpeciesReferences) error {
	for _, assignment := range assignments {
		found := false
		for _, edit := range edits {
			if edit.RowID == assignment.RowID {
				found = true
				if !reflect.DeepEqual(assignment.Before, edit.Expected) || assignment.After.Text == nil || *assignment.After.Text != edit.Value {
					return errors.New("Species assignment differs from its reviewed draft")
				}
				if err := validateSIVISpeciesMembership(ctx, edit, references); err != nil {
					return err
				}
				break
			}
		}
		if !found {
			return errors.New("Species assignment has no reviewed source form")
		}
	}
	return ctx.Err()
}
