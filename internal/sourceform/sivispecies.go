package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

const siviSpeciesHistoryTable = "__VPRO_SIVISpeciesHistory"
const siviSpeciesHistorySQL = `CREATE TABLE "__VPRO_SIVISpeciesHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviSpeciesWritePolicy() SiviVegetationWritePolicy {
	return SiviVegetationWritePolicy{"Species", siviSpeciesHistoryTable, siviSpeciesHistorySQL,
		[]string{"Species"}, planSIVISpeciesCells}
}

func speciesSIVIDrafts(ctx context.Context, edits []SIVISpeciesEdit) ([]SiviHeightEdit, error) {
	if ctx == nil {
		return nil, errors.New("SIVI Species planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI Species planning requires explicit edits")
	}
	drafts := make([]SiviHeightEdit, len(edits))
	for i, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := strconv.ParseInt(edit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != edit.RowID {
			return nil, errors.New("SIVI Species requires the original signed64 physical row identity")
		}
		if _, err := vprocore.MetadataCellValue(edit.Expected); err != nil {
			return nil, err
		}
		value, err := vprocore.ProjectMetadataCellInternal("text", []byte(edit.Value))
		if err != nil {
			return nil, err
		}
		drafts[i] = SiviHeightEdit{edit.RowID, edit.Form, "Species", vprocore.CloneSiteUnitCell(edit.Expected), value}
	}
	return drafts, ctx.Err()
}

func planSIVISpeciesCells(ctx context.Context, plot string, extended bool, veg vprocore.ProjectMetadataTable, edits []SiviHeightEdit) ([]SiviHeightAssignment, error) {
	return PlanSIVIChildEdits(ctx, plot, extended, veg, edits, "Species",
		func(group int, column string) bool { return group >= 0 && group < 3 && column == "Species" },
		func(_ string, cell vprocore.ProjectMetadataCell) error {
			if cell.Storage != "text" || cell.Text == nil {
				return errors.New("new Species requires nonempty literal text")
			}
			return vprocore.ValidateVegetationSpeciesLookup(*cell.Text)
		})
}

var SiviSpeciesMasterColumns = []string{"Code", "ScientificName", "Lifeform", "EnglishName", "Codetype", "OldCode"}
var SiviSpeciesPersonalColumns = []string{"Code", "ScientificName", "LifeForm", "EnglishName", "Codetype"}

func ProjectSIVISpeciesDefinitions(ctx context.Context, source vprocore.ProjectMetadataTable, personal bool) (vprocore.ProjectMetadataTable, error) {
	return ProjectPhysicalSpeciesDefinitions(ctx, source, personal, func(cell vprocore.ProjectMetadataCell) error {
		_, err := vprocore.VegetationLifeformInteger(cell)
		return err
	})
}

// Only identifier recasing is allowed. Physical definitions, including duplicate
// codes, NULLs and empty metadata, retain their original typed cells and rowids.
func ProjectPhysicalSpeciesDefinitions(ctx context.Context, source vprocore.ProjectMetadataTable, personal bool,
	validateLifeform func(vprocore.ProjectMetadataCell) error) (vprocore.ProjectMetadataTable, error) {
	if ctx == nil || validateLifeform == nil {
		return vprocore.ProjectMetadataTable{}, errors.New("Species references require a context and explicit Lifeform policy")
	}
	if _, err := vprocore.SiteUnitTransferColumns(source); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	required := SiviSpeciesMasterColumns
	if personal {
		required = SiviSpeciesPersonalColumns
	}
	result := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	indices := make([]int, len(required))
	for i, name := range required {
		found := -1
		for j, column := range source.Columns {
			if siviSpeciesASCIIEqual(column.Name, name) {
				if found >= 0 {
					return vprocore.ProjectMetadataTable{}, fmt.Errorf("ambiguous Species reference column %s", name)
				}
				found = j
			}
		}
		if found < 0 {
			return vprocore.ProjectMetadataTable{}, fmt.Errorf("missing Species reference column %s", name)
		}
		indices[i] = found
		result.Columns = append(result.Columns, source.Columns[found])
	}
	for _, row := range source.Rows {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, err
		}
		projected := vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: make([]vprocore.ProjectMetadataCell, len(indices))}
		for i, index := range indices {
			cell := row.Cells[index]
			var err error
			if i == 2 {
				err = validateLifeform(cell)
			} else {
				_, err = vprocore.VegetationReportIdentity(cell)
			}
			if err != nil {
				return vprocore.ProjectMetadataTable{}, fmt.Errorf("Species definition %s.%s: %w", row.RowID, required[i], err)
			}
			projected.Cells[i] = vprocore.CloneSiteUnitCell(cell)
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

func ValidateSIVISpeciesMembership(ctx context.Context, edit SIVISpeciesEdit, references SIVISpeciesReferences) error {
	update := vprocore.VegetationSpeciesUpdate{Form: edit.Form, Value: edit.Value, Decision: edit.Decision, Entered: edit.Entered, Selected: edit.Selected}
	if err := vprocore.ValidateVegetationSpeciesDecision(update); err != nil {
		return err
	}
	if err := vprocore.ValidateVegetationSpeciesLookup(edit.Value); err != nil {
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
	for _, table := range []vprocore.ProjectMetadataTable{references.Master, references.Personal} {
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			code, kind := row.Cells[0].Text, row.Cells[4].Text
			lifeform, err := vprocore.VegetationLifeformInteger(row.Cells[2])
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

func validateSIVISpeciesAssignments(ctx context.Context, assignments []SiviHeightAssignment, edits []SIVISpeciesEdit, references SIVISpeciesReferences) error {
	for _, assignment := range assignments {
		found := false
		for _, edit := range edits {
			if edit.RowID == assignment.RowID {
				found = true
				if !reflect.DeepEqual(assignment.Before, edit.Expected) || assignment.After.Text == nil || *assignment.After.Text != edit.Value {
					return errors.New("Species assignment differs from its reviewed draft")
				}
				if err := ValidateSIVISpeciesMembership(ctx, edit, references); err != nil {
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
