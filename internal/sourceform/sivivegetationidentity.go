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
	"strings"
)

// New identities are deliberately collision-free; Access's nonunique ID index
// is not treated as permission to invalidate other desktop editors' identities.
func planSIVIIdentityEdits(ctx context.Context, plot string, extended bool, veg vprocore.ProjectMetadataTable, edits []SiviHeightEdit, reserved map[string]bool) ([]SiviHeightAssignment, error) {
	assignments, err := PlanSIVIChildEdits(ctx, plot, extended, veg, edits, "identity",
		func(_ int, column string) bool { return column == "ID" }, ValidateSIVIIdentityCell)
	if err != nil {
		return nil, err
	}
	columns, err := vprocore.SiteUnitTransferColumns(veg, "ID")
	if err != nil {
		return nil, err
	}
	claimed := map[string]bool{}
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := ValidateSIVIIdentityCell("ID", assignment.Before); err != nil {
			return nil, fmt.Errorf("historical SIVI ID is read-only until independently repaired: %w", err)
		}
		if assignment.After.Storage == "null" {
			continue
		}
		id := *assignment.After.Integer
		if reserved[id] || claimed[id] {
			return nil, errors.New("SIVI ID is deleted, reserved or repeated in this edit; no identity is reassigned")
		}
		for _, row := range veg.Rows {
			if row.RowID != assignment.RowID && SiviIdentityCellMatches(row.Cells[columns["ID"]], id) {
				return nil, errors.New("SIVI ID already belongs to another physical row; identity swaps are unavailable")
			}
		}
		claimed[id] = true
	}
	return assignments, ctx.Err()
}

func ValidateSIVIIdentityCell(_ string, cell vprocore.ProjectMetadataCell) error {
	if _, err := vprocore.MetadataCellValue(cell); err != nil {
		return err
	}
	if cell.Storage == "null" {
		return nil
	}
	if cell.Storage != "integer" || cell.Integer == nil {
		return errors.New("Veg.ID requires nullable signed32 integer storage")
	}
	value, err := strconv.ParseInt(*cell.Integer, 10, 32)
	if err != nil || strconv.FormatInt(value, 10) != *cell.Integer {
		return errors.New("Veg.ID requires an exact signed32 integer")
	}
	return nil
}

func SiviIdentityCellMatches(cell vprocore.ProjectMetadataCell, id string) bool {
	if cell.Storage == "integer" {
		return cell.Integer != nil && *cell.Integer == id
	}
	value, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		return false
	}
	if cell.Storage == "real" {
		return cell.Real != nil && *cell.Real == float64(value)
	}
	if cell.Storage == "text" && cell.Text != nil {
		number, err := strconv.ParseFloat(strings.TrimSpace(*cell.Text), 64)
		return err == nil && number == float64(value)
	}
	return false
}

func siviIdentityReservedIDs(project string, tables map[string]vprocore.ProjectMetadataTable) (map[string]bool, error) {
	return SourceChildIdentityReservedIDs(project, "Veg", "SIVI identity", tables)
}

func SourceChildIdentityReservedIDs(project, kind, operation string, tables map[string]vprocore.ProjectMetadataTable) (map[string]bool, error) {
	reserved := map[string]bool{}
	for _, source := range []struct{ table, ownerColumn string }{
		{"__VPRO_ChildIdentity", "ChildTable"}, {project + "_Audit", "Table"},
	} {
		table, present := tables[source.table]
		if !present && source.table == "__VPRO_ChildIdentity" {
			continue
		}
		columns, err := vprocore.SiteUnitTransferColumns(table, source.ownerColumn, "ID")
		if err != nil {
			return nil, err
		}
		for _, row := range table.Rows {
			owner := row.Cells[columns[source.ownerColumn]]
			if owner.Storage != "text" || owner.Text == nil {
				return nil, fmt.Errorf("%s reservations contain unsupported table-owner storage", operation)
			}
			matches := *owner.Text == vprocore.QuoteHeaderIdentifier(project+"_"+kind)
			if source.ownerColumn == "Table" {
				matches = strings.EqualFold(*owner.Text, "_"+kind) || strings.EqualFold(*owner.Text, project+"_"+kind)
			}
			if !matches {
				continue
			}
			id := row.Cells[columns["ID"]]
			if id.Storage == "null" && source.ownerColumn == "Table" {
				continue
			}
			if err := ValidateSIVIIdentityCell("ID", id); err != nil || id.Integer == nil {
				return nil, errors.Join(err, fmt.Errorf("%s reservation is not an exact signed32 ID", operation))
			}
			reserved[*id.Integer] = true
		}
	}
	return reserved, nil
}

type siviIdentityOccupant = vprocore.ProjectMetadataRow

func siviIdentityReturnOccupants(veg vprocore.ProjectMetadataTable, assignments []SiviHeightAssignment) ([]siviIdentityOccupant, error) {
	columns, err := vprocore.SiteUnitTransferColumns(veg, "ID")
	if err != nil {
		return nil, err
	}
	owned, returning := map[string]bool{}, map[string]bool{}
	for _, assignment := range assignments {
		owned[assignment.RowID] = true
		if assignment.Before.Storage == "integer" && assignment.Before.Integer != nil {
			returning[*assignment.Before.Integer] = true
		}
	}
	occupants := []siviIdentityOccupant{}
	for _, row := range veg.Rows {
		if owned[row.RowID] {
			continue
		}
		cell := row.Cells[columns["ID"]]
		for id := range returning {
			if SiviIdentityCellMatches(cell, id) {
				peer := vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: make([]vprocore.ProjectMetadataCell, len(row.Cells))}
				for i, cell := range row.Cells {
					peer.Cells[i] = vprocore.CloneSiteUnitCell(cell)
				}
				occupants = append(occupants, peer)
				break
			}
		}
	}
	return occupants, nil
}

func applySIVIIdentityPlan(veg vprocore.ProjectMetadataTable, assignments []SiviHeightAssignment) (vprocore.ProjectMetadataTable, error) {
	columns, err := vprocore.SiteUnitTransferColumns(veg, "ID")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	planned := vprocore.ProjectMetadataTable{Columns: veg.Columns, Rows: append([]vprocore.ProjectMetadataRow{}, veg.Rows...)}
	for _, assignment := range assignments {
		found := false
		for i, row := range planned.Rows {
			if row.RowID != assignment.RowID {
				continue
			}
			if assignment.Column != "ID" || !reflect.DeepEqual(row.Cells[columns["ID"]], assignment.Before) {
				return vprocore.ProjectMetadataTable{}, errors.New("SIVI identity plan differs from its complete original row")
			}
			planned.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
			planned.Rows[i].Cells[columns["ID"]] = vprocore.CloneSiteUnitCell(assignment.After)
			found = true
			break
		}
		if !found {
			return vprocore.ProjectMetadataTable{}, errors.New("SIVI identity plan lost its physical row")
		}
	}
	return planned, nil
}
