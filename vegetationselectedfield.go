package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const longVegetationSelectedFieldFeatureEnvironment = "VPRO_LONG_VEGETATION_SELECTED_FIELD"

type longVegetationSelectedFieldProjection struct {
	Kind   string
	values map[string]ProjectMetadataCell
}

func longVegetationSelectedFieldColumn(columns []ProjectMetadataColumn, field string) (int, string, error) {
	if field == "" || !utf8.ValidString(field) || strings.ContainsRune(field, 0) {
		return -1, "", errors.New("Long Vegetation selected field requires an explicit nonempty physical identifier without NUL")
	}
	// The source concatenates an unquoted identifier, not an arbitrary physical name.
	switch strings.ToLower(field) {
	case "yearofchange", "changedescription", "apcfamily", "familycode", "oldcode", "code", "splcode", "treecode",
		"scientificname", "lifeform", "authority", "englishname_old", "englishname", "common_name_pb",
		"notes_on_names", "shortenedguidename", "native", "codetype", "province", "id", "taxonlevel",
		"isattribute", "report", "splcode_calc", "codelength", "combinedenglishname",
		"redbluelist", "wetland_ind", "weedstatus", "alttext", "altlong", "altbool", "altdouble", "literal":
	default:
		return -1, "", fmt.Errorf("Long Vegetation selected field %q has an unverified raw source identifier; preference storage was not changed", field)
	}
	index := -1
	identifierKey := func(value string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + ('a' - 'A')
			}
			return r
		}, value)
	}
	for i, column := range columns {
		if identifierKey(column.Name) == identifierKey(field) {
			if index != -1 {
				return -1, "", errors.New("Long Vegetation selected physical field is ambiguous; no identifier was repaired")
			}
			index = i
		}
	}
	if index == -1 {
		return -1, "", fmt.Errorf("Long Vegetation selected physical field %q is unavailable in owned VLists.USysAllSpecs", field)
	}
	kind := ""
	switch columns[index].DeclaredType {
	case "TEXT":
		kind = "text"
	case "INTEGER":
		kind = "integer"
	case "BOOLEAN":
		kind = "boolean"
	case "REAL":
		kind = "real"
	default:
		return -1, "", fmt.Errorf("Long Vegetation selected field %q has unverified physical type %q", field, columns[index].DeclaredType)
	}
	return index, kind, nil
}

func longVegetationSelectedSpeciesKey(cell ProjectMetadataCell) (string, error) {
	value, err := vegetationReportIdentity(cell)
	if err != nil || value == nil {
		return "null", err
	}
	var key strings.Builder
	for _, char := range *value {
		if char == 0 {
			return "", errors.New("Long Vegetation selected field has an unverified NUL effective species identity")
		}
		if char > 127 {
			return "", errors.New("Long Vegetation selected field has an unverified non-ASCII effective species identity")
		}
		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			key.WriteRune(char)
		}
	}
	return "text:" + key.String(), nil
}

func longVegetationSelectedScalarValue(kind string, cell ProjectMetadataCell) (any, error) {
	if kind != "text" && kind != "integer" && kind != "boolean" && kind != "real" {
		return nil, errors.New("Long Vegetation selected scalar has an unverified type")
	}
	value, err := metadataCellValue(cell)
	if err != nil || value == nil {
		return value, err
	}
	switch kind {
	case "text":
		text, ok := value.(string)
		if !ok || strings.ContainsRune(text, 0) {
			return nil, errors.New("selected scalar requires original Unicode text without NUL")
		}
	case "integer", "boolean":
		number, ok := value.(int64)
		if !ok || kind == "integer" && (number < math.MinInt32 || number > math.MaxInt32) ||
			kind == "boolean" && number != -1 && number != 0 {
			return nil, errors.New("selected scalar requires native Long or Boolean -1/0 storage without conversion")
		}
	case "real":
		if _, ok := value.(float64); !ok {
			return nil, errors.New("selected scalar requires original DOUBLE storage without conversion")
		}
	}
	return value, nil
}

func projectLongVegetationSelectedField(ctx context.Context, source ProjectMetadataTable, field string) (ProjectMetadataTable, *longVegetationSelectedFieldProjection, error) {
	fail := func(err error) (ProjectMetadataTable, *longVegetationSelectedFieldProjection, error) {
		return ProjectMetadataTable{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	columns, err := siteUnitTransferColumns(source, "Code", "EnglishName", "ScientificName")
	if err != nil {
		return fail(err)
	}
	index, kind, err := longVegetationSelectedFieldColumn(source.Columns, field)
	if err != nil {
		return fail(err)
	}
	projection := &longVegetationSelectedFieldProjection{Kind: kind, values: map[string]ProjectMetadataCell{}}
	result := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn{}, source.Columns...), Rows: []ProjectMetadataRow{}}
	for _, row := range source.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		cell := row.Cells[index]
		value, err := longVegetationSelectedScalarValue(kind, cell)
		if err != nil {
			return fail(fmt.Errorf("Long Vegetation selected field %q row %s: %w", field, row.RowID, err))
		}
		token := ""
		if value != nil {
			switch kind {
			case "text":
				text := *cell.Text
				token = "text:" + hex.EncodeToString([]byte(text))
			case "integer", "boolean":
				token = kind + ":" + *cell.Integer
			case "real":
				number := *cell.Real
				bits := math.Float64bits(number)
				if number == 0 {
					bits = 0
				}
				token = "real:" + strconv.FormatUint(bits, 16)
				if previous, present := projection.values[token]; present &&
					math.Float64bits(*previous.Real) != math.Float64bits(number) {
					return fail(errors.New("Long Vegetation selected DOUBLE has unverified signed-zero grouping representatives"))
				}
			}
			projection.values[token] = cloneSiteUnitCell(cell)
		}
		cells := make([]ProjectMetadataCell, len(row.Cells))
		for i, original := range row.Cells {
			cells[i] = cloneSiteUnitCell(original)
		}
		if value == nil {
			cells[columns["EnglishName"]] = ProjectMetadataCell{Storage: "null"}
		} else {
			cells[columns["EnglishName"]] = ProjectMetadataCell{Storage: "text", Text: &token}
		}
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result, projection, nil
}

type longVegetationSelectedGroupingGuard struct {
	projection *longVegetationSelectedFieldProjection
	texts      map[string]string
	identities map[string]string
}

func (p *longVegetationSelectedFieldProjection) newGroupingGuard() (*longVegetationSelectedGroupingGuard, error) {
	if p == nil {
		return nil, errors.New("Long Vegetation selected grouping requires its explicit projection")
	}
	if _, err := longVegetationSelectedScalarValue(p.Kind, ProjectMetadataCell{Storage: "null"}); err != nil {
		return nil, err
	}
	return &longVegetationSelectedGroupingGuard{p, map[string]string{}, map[string]string{}}, nil
}

func (g *longVegetationSelectedGroupingGuard) observeProjected(ctx context.Context, species, cell ProjectMetadataCell) error {
	if g == nil || g.projection == nil {
		return errors.New("Long Vegetation selected grouping requires its owned projection")
	}
	if _, err := metadataCellValue(cell); err != nil {
		return err
	}
	if cell.Storage != "null" {
		if cell.Storage != "text" || cell.Text == nil {
			return errors.New("Long Vegetation selected grouping lost its explicit token")
		}
		original, present := g.projection.values[*cell.Text]
		if !present {
			return errors.New("Long Vegetation selected grouping introduced an unowned token")
		}
		cell = original
	}
	return g.observeTyped(ctx, species, cell)
}

func (g *longVegetationSelectedGroupingGuard) observeTyped(ctx context.Context, species, cell ProjectMetadataCell) error {
	if ctx == nil || g == nil || g.projection == nil {
		return errors.New("Long Vegetation selected grouping requires its context and owned projection")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := longVegetationSelectedSpeciesKey(species)
	if err != nil {
		return err
	}
	identity := vegetationTextKey(species)
	// This over-inclusive refusal key never groups or rewrites source values.
	if previous, present := g.identities[key]; present && previous != identity {
		return fmt.Errorf("Long Vegetation selected field has unverified native effective species representatives %q and %q; no separate byte groups were invented", previous, identity)
	}
	g.identities[key] = identity
	value, err := longVegetationSelectedScalarValue(g.projection.Kind, cell)
	if err != nil {
		return err
	}
	if g.projection.Kind == "text" && value != nil {
		text := *cell.Text
		if previous, present := g.texts[identity]; present && previous != text {
			return fmt.Errorf("Long Vegetation selected field has unverified native collation representatives for effective species %q; multiple literal TEXT values remain unavailable", identity)
		}
		g.texts[identity] = text
	}
	return nil
}

func (p *longVegetationSelectedFieldProjection) validateGrouping(ctx context.Context, report *vegetationLayerReport) error {
	if ctx == nil || p == nil || report == nil {
		return errors.New("Long Vegetation selected grouping validation requires its explicit context, projection and report")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	guard, err := p.newGroupingGuard()
	if err != nil {
		return err
	}
	for _, unit := range report.Units {
		for _, row := range unit.Rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			for _, cell := range []ProjectMetadataCell{row.EnglishName, row.MatchedName} {
				if err := guard.observeTyped(ctx, row.Species, cell); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *longVegetationSelectedFieldProjection) restore(ctx context.Context, report *vegetationLayerReport) error {
	if p == nil || report == nil {
		return errors.New("Long Vegetation selected scalar restoration requires its explicit projection and report")
	}
	for i := range report.Units {
		for j := range report.Units[i].Rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			row := &report.Units[i].Rows[j]
			for _, cell := range []*ProjectMetadataCell{&row.EnglishName, &row.MatchedName} {
				if _, err := metadataCellValue(*cell); err != nil {
					return err
				}
				if cell.Storage == "null" {
					continue
				}
				if cell.Text == nil {
					return errors.New("Long Vegetation selected scalar grouping lost its explicit token")
				}
				original, present := p.values[*cell.Text]
				if !present {
					return errors.New("Long Vegetation selected scalar grouping introduced an unowned token")
				}
				*cell = cloneSiteUnitCell(original)
			}
		}
	}
	return nil
}
