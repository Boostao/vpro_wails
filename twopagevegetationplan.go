package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

type twoPageVegetationEdit struct {
	ContextID string              `json:"contextId"`
	Form      string              `json:"form"`
	RowID     string              `json:"rowId"`
	Column    string              `json:"column"`
	Expected  ProjectMetadataCell `json:"expected"`
	Value     ProjectMetadataCell `json:"value"`
}

func (edit *twoPageVegetationEdit) UnmarshalJSON(data []byte) error {
	required := []string{"contextId", "form", "rowId", "column", "expected", "value"}
	properties, err := sourceChildJSONObject(data, "two-page vegetation edit", required, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "two-page vegetation edit", required...); err != nil {
		return err
	}
	for _, name := range []string{"expected", "value"} {
		if err := sourceChildCellJSON(properties[name], "two-page vegetation "+name); err != nil {
			return err
		}
	}
	type plain twoPageVegetationEdit
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*edit = twoPageVegetationEdit(decoded)
	return nil
}

type twoPageVegetationAssignment struct {
	RowID, Column string
	ID            int64
	Before, After ProjectMetadataCell
	Value         any
}

func planTwoPageVegetationEdits(ctx context.Context, original *twoPageVegetationOriginal, edits []twoPageVegetationEdit) ([]twoPageVegetationAssignment, error) {
	return planTwoPageVegetationTypedEdits(ctx, original, edits, true)
}

func planTwoPageVegetationHistoricalEdits(ctx context.Context, original *twoPageVegetationOriginal, edits []twoPageVegetationEdit) ([]twoPageVegetationAssignment, error) {
	return planTwoPageVegetationTypedEdits(ctx, original, edits, false)
}

func planTwoPageVegetationTypedEdits(ctx context.Context, original *twoPageVegetationOriginal, edits []twoPageVegetationEdit, canonicalSingle bool) ([]twoPageVegetationAssignment, error) {
	return planTwoPageVegetationQualifiedEdits(ctx, original, edits, canonicalSingle, nil)
}

func planTwoPageVegetationQualifiedEdits(ctx context.Context, original *twoPageVegetationOriginal, edits []twoPageVegetationEdit,
	canonicalSingle bool, speciesReferences *twoPageVegetationSpeciesReferences) ([]twoPageVegetationAssignment, error) {
	if ctx == nil || original == nil || original.ContextID == "" || original.Project == "" ||
		original.Plot == "" || !utf8.ValidString(original.ContextID) || !utf8.ValidString(original.Project) ||
		!utf8.ValidString(original.Plot) || len(edits) == 0 {
		return nil, errors.New("two-page vegetation planning requires an owned literal original and explicit edits")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	source, covers, err := twoPageVegetationSource(original.Form)
	if err != nil {
		return nil, err
	}
	if original.Table != original.Project+"_Veg" || original.RecordSource != source {
		return nil, errors.New("two-page vegetation original belongs to another physical source")
	}
	if speciesReferences != nil && (speciesReferences.ContextID != original.ContextID ||
		speciesReferences.Project != original.Project || speciesReferences.Plot != original.Plot ||
		speciesReferences.Form != original.Form || original.Form != "SubVegAXL_BC" && original.Form != "SubVegAXL") {
		return nil, errors.New("two-page Species list references belong to another exact original A/CHARS owner")
	}
	columns, err := siteUnitTransferColumns(original.Records, "ID", "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("two-page vegetation original schema: %w", err)
	}
	for i, column := range original.Records.Columns {
		if !utf8.ValidString(column.Name) || !utf8.ValidString(column.DeclaredType) {
			return nil, errors.New("two-page vegetation original schema contains malformed Unicode")
		}
		for _, peer := range original.Records.Columns[:i] {
			if strings.EqualFold(column.Name, peer.Name) {
				return nil, errors.New("two-page vegetation original schema contains ambiguous column identities")
			}
		}
	}
	rows := make(map[string]ProjectMetadataRow, len(original.Records.Rows))
	for _, row := range original.Records.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil {
				return nil, fmt.Errorf("two-page vegetation original row %s: %w", row.RowID, err)
			}
		}
		parent := row.Cells[columns["PlotNumber"]]
		if parent.Storage != "text" || parent.Text == nil || *parent.Text != original.Plot {
			return nil, errors.New("two-page vegetation original contains a row outside its exact literal parent")
		}
		rows[row.RowID] = row
	}
	sourceRows, err := twoPageVegetationSourceRows(original.Records, source, covers)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(sourceRows, original.SourceRowIDs) {
		return nil, errors.New("two-page vegetation original source membership changed; cancel and reload")
	}
	visible := make(map[string]bool, len(sourceRows))
	for _, rowID := range sourceRows {
		visible[rowID] = true
	}
	if original.Form == "SubVegAXL_BC" {
		covers = covers[:7]
	}
	allowed := map[string]bool{"Collected": true}
	if speciesReferences != nil {
		allowed["Species"] = true
	}
	for _, cover := range covers {
		allowed[cover] = true
	}
	assignments := []twoPageVegetationAssignment{}
	seen := map[[2]string]bool{}
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := [2]string{edit.RowID, edit.Column}
		row, present := rows[edit.RowID]
		column, available := columns[edit.Column]
		if !available {
			for name, index := range columns {
				if strings.EqualFold(name, edit.Column) {
					column, available = index, true
					break
				}
			}
		}
		if edit.ContextID != original.ContextID || edit.Form != original.Form || !present || !visible[edit.RowID] ||
			!available || !allowed[edit.Column] || seen[key] {
			return nil, fmt.Errorf("two-page vegetation edit %s.%s is repeated or unavailable in its original source owner; Species callbacks remain unavailable", edit.RowID, edit.Column)
		}
		seen[key] = true
		identity := row.Cells[columns["ID"]]
		if identity.Storage != "integer" || identity.Integer == nil {
			return nil, errors.New("two-page vegetation edited row requires an original signed32 integer ID")
		}
		id, err := strconv.ParseInt(*identity.Integer, 10, 32)
		if err != nil {
			return nil, errors.New("two-page vegetation edited row requires an original signed32 integer ID")
		}
		before := row.Cells[column]
		if _, err := metadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("two-page vegetation expected value: %w", err)
		}
		if !twoPageVegetationCellEqual(before, edit.Expected) {
			return nil, errors.New("two-page vegetation original value changed; cancel and reload")
		}
		value, err := metadataCellValue(edit.Value)
		if err != nil {
			return nil, fmt.Errorf("two-page vegetation draft value: %w", err)
		}
		if twoPageVegetationCellEqual(before, edit.Value) {
			continue
		}
		if edit.Value.Real != nil && *edit.Value.Real == 0 && math.Signbit(*edit.Value.Real) {
			return nil, errors.New("two-page vegetation new negative-zero values require lossless source/storage calibration")
		}
		if edit.Column == "Species" {
			if edit.Value.Storage != "text" || edit.Value.Text == nil {
				err = errors.New("two-page Species list assignment requires non-NULL literal text")
			} else {
				_, err = qualifyTwoPageVegetationSpeciesListSelection(ctx, speciesReferences, *edit.Value.Text)
			}
		} else if edit.Column == "Collected" {
			err = validateChildPhysicalText("Veg.Collected", value, 1)
		} else {
			err = validateSIVIChildSingle(edit.Column, edit.Value)
			if err == nil && edit.Value.Real != nil {
				cover := *edit.Value.Real
				if canonicalSingle {
					cover = float64(float32(cover))
				}
				if cover >= 100 {
					err = errors.New("source cover validation requires a value below 100 or NULL (<100 Or Is Null)")
				} else if canonicalSingle && cover == 0 && math.Signbit(cover) {
					err = errors.New("new negative-zero values after source SINGLE conversion require lossless source/storage calibration")
				} else if canonicalSingle && math.Float64bits(cover) != math.Float64bits(*edit.Value.Real) {
					err = errors.New("typed REAL assignment requires the exact canonical source SINGLE value; raw decimal input must be prepared before transport")
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("two-page vegetation draft %s.%s: %w", edit.RowID, edit.Column, err)
		}
		assignments = append(assignments, twoPageVegetationAssignment{edit.RowID, edit.Column, id,
			cloneSiteUnitCell(before), cloneSiteUnitCell(edit.Value), value})
	}
	if speciesReferences != nil {
		if err := requireTwoPageVegetationSpeciesListCovers(original, assignments); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}

func twoPageVegetationCellEqual(left, right ProjectMetadataCell) bool {
	return metadataCellEqual(left, right)
}

func twoPageVegetationTableEqual(left, right ProjectMetadataTable) bool {
	if !reflect.DeepEqual(left, right) {
		return false
	}
	for i, row := range left.Rows {
		for j, cell := range row.Cells {
			if !twoPageVegetationCellEqual(cell, right.Rows[i].Cells[j]) {
				return false
			}
		}
	}
	return true
}
