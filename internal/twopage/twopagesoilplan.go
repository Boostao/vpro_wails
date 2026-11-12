package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"unicode/utf8"
)

type twoPageSoilEdit struct {
	ContextID string                       `json:"contextId"`
	Form      string                       `json:"form"`
	RowID     string                       `json:"rowId"`
	Column    string                       `json:"column"`
	Expected  vprocore.ProjectMetadataCell `json:"expected"`
	Value     vprocore.ProjectMetadataCell `json:"value"`
}

func (edit *twoPageSoilEdit) UnmarshalJSON(data []byte) error {
	required := []string{"contextId", "form", "rowId", "column", "expected", "value"}
	properties, err := sourceform.SourceChildJSONObject(data, "two-page soil edit", required, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, "two-page soil edit", required...); err != nil {
		return err
	}
	for _, name := range []string{"expected", "value"} {
		if err := sourceform.SourceChildCellJSON(properties[name], "two-page soil "+name); err != nil {
			return err
		}
	}
	type plain twoPageSoilEdit
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*edit = twoPageSoilEdit(decoded)
	return nil
}

type twoPageSoilAssignment struct {
	RowID, Column string
	ID            int64
	Before, After vprocore.ProjectMetadataCell
	Value         any
}

func validateTwoPageSoilValue(kind, column string, cell vprocore.ProjectMetadataCell) (any, error) {
	value, err := vprocore.MetadataCellValue(cell)
	if err != nil {
		return nil, fmt.Errorf("two-page soil %s.%s value: %w", kind, column, err)
	}
	if value != nil {
		switch {
		case vprocore.SoilChildSingleColumns[kind][column]:
			if cell.Storage != "real" {
				return nil, fmt.Errorf("%s.%s requires nullable real storage", kind, column)
			}
		case vprocore.SoilChildIntegerColumns[kind][column]:
			if cell.Storage != "integer" {
				return nil, fmt.Errorf("%s.%s requires nullable integer storage", kind, column)
			}
			number, err := strconv.ParseInt(*cell.Integer, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("%s.%s exceeds the physical Access Integer range (-32768 to 32767)", kind, column)
			}
			value = int(number)
		}
	}
	if err := vprocore.ValidateSoilChildField(kind, column, value); err != nil {
		return nil, err
	}
	return value, nil
}

// A plan proves draft consistency, not live context ownership or permission to write.
func planTwoPageSoilEdits(ctx context.Context, original *twoPageSoilOriginal, edits []twoPageSoilEdit) ([]twoPageSoilAssignment, error) {
	if ctx == nil || original == nil || original.ContextID == "" || original.Project == "" ||
		original.Plot == "" || !utf8.ValidString(original.ContextID) || !utf8.ValidString(original.Project) ||
		!utf8.ValidString(original.Plot) || len(edits) == 0 {
		return nil, errors.New("two-page soil planning requires an owned literal original and explicit edits")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var kind, source string
	switch original.Form {
	case "SoilHumusXL":
		kind, source = "Humus", "UsysHumus"
	case "SoilMineralXL":
		kind, source = "Mineral", "UsysMineral"
	default:
		return nil, fmt.Errorf("two-page soil planning does not support source form %q", original.Form)
	}
	if original.Table != original.Project+"_"+kind || original.RecordSource != source {
		return nil, errors.New("two-page soil original belongs to another physical source")
	}
	columns, err := vprocore.SiteUnitTransferColumns(original.Records, "ID", "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("two-page soil original schema: %w", err)
	}
	rows := make(map[string]vprocore.ProjectMetadataRow, len(original.Records.Rows))
	for _, row := range original.Records.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, cell := range row.Cells {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return nil, fmt.Errorf("two-page soil original row %s: %w", row.RowID, err)
			}
		}
		parent := row.Cells[columns["PlotNumber"]]
		if parent.Storage != "text" || parent.Text == nil || *parent.Text != original.Plot {
			return nil, errors.New("two-page soil original contains a row outside its exact literal parent")
		}
		rows[row.RowID] = row
	}
	allowed := make(map[string]bool, len(vprocore.ChildFields[kind]))
	for _, field := range vprocore.ChildFields[kind] {
		allowed[field.Column] = true
	}
	assignments := []twoPageSoilAssignment{}
	seen := map[[2]string]bool{}
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := [2]string{edit.RowID, edit.Column}
		row, present := rows[edit.RowID]
		column, available := columns[edit.Column]
		if edit.ContextID != original.ContextID || edit.Form != original.Form || !present ||
			!available || !allowed[edit.Column] || seen[key] {
			return nil, fmt.Errorf("two-page soil edit %s.%s is repeated or unavailable in its original owner", edit.RowID, edit.Column)
		}
		seen[key] = true
		identity := row.Cells[columns["ID"]]
		if identity.Storage != "integer" || identity.Integer == nil {
			return nil, errors.New("two-page soil edited row requires an original signed32 integer ID")
		}
		id, err := strconv.ParseInt(*identity.Integer, 10, 32)
		if err != nil {
			return nil, errors.New("two-page soil edited row requires an original signed32 integer ID")
		}
		for _, peer := range original.Records.Rows {
			if peer.RowID != row.RowID && sourceform.SiviIdentityCellMatches(peer.Cells[columns["ID"]], *identity.Integer) {
				return nil, errors.New("two-page soil edited ID belongs to another original physical row")
			}
		}
		before := row.Cells[column]
		if _, err := vprocore.MetadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("two-page soil expected value: %w", err)
		}
		if !reflect.DeepEqual(before, edit.Expected) {
			return nil, errors.New("two-page soil original value changed; cancel and reload")
		}
		if _, err := vprocore.MetadataCellValue(edit.Value); err != nil {
			return nil, fmt.Errorf("two-page soil draft value: %w", err)
		}
		if reflect.DeepEqual(before, edit.Value) {
			continue
		}
		value, err := validateTwoPageSoilValue(kind, edit.Column, edit.Value)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, twoPageSoilAssignment{edit.RowID, edit.Column, id,
			vprocore.CloneSiteUnitCell(before), vprocore.CloneSiteUnitCell(edit.Value), value})
	}
	return assignments, ctx.Err()
}
