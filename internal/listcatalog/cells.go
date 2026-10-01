// Package listcatalog preserves typed DAO list metadata at frozen fixture boundaries.
package listcatalog

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

type Cell struct {
	Name           string   `json:"Name"`
	DAOType        int      `json:"DAOType"`
	IsNull         bool     `json:"IsNull"`
	Value          *string  `json:"Value"`
	Utf16CodeUnits []uint16 `json:"Utf16CodeUnits"`
}

type Column struct {
	Name    string `json:"name"`
	DAOType int    `json:"daoType"`
	Size    int    `json:"size"`
}

type Choice struct {
	RowID         string   `json:"rowId"`
	Code          *string  `json:"code"`
	ListName      *string  `json:"listName"`
	ListFilter    *string  `json:"listFilter"`
	ItemOrder     *float64 `json:"itemOrder"`
	Description   *string  `json:"description"`
	FieldUsedIn   *string  `json:"fieldUsedIn"`
	ValidateLoops *string  `json:"validateLoops"`
	Validate      *bool    `json:"validate"`
	Note          *string  `json:"note"`
	Flag          *bool    `json:"flag"`
	Selectable    bool     `json:"selectable"`
	Diagnostic    string   `json:"diagnostic"`
}

func ExpectedSchema() []Column {
	return []Column{
		{"ListName", 10, 255}, {"ListFilter", 10, 255}, {"ItemOrder", 7, 8}, {"Item", 10, 255},
		{"ItemDescription", 10, 255}, {"FieldUsedIn", 10, 255}, {"ValidateLoops", 10, 255},
		{"Validate", 1, 1}, {"Note", 10, 255}, {"Flag", 1, 1},
	}
}

func DecodeCells(rowID string, cells []Cell) (Choice, error) {
	row := Choice{RowID: rowID}
	if len(cells) != 10 {
		return Choice{}, fmt.Errorf("row %s: expected ten DAO cells", rowID)
	}
	texts := map[string]**string{"ListName": &row.ListName, "ListFilter": &row.ListFilter,
		"Item": &row.Code, "ItemDescription": &row.Description, "FieldUsedIn": &row.FieldUsedIn,
		"ValidateLoops": &row.ValidateLoops, "Note": &row.Note}
	for i, field := range ExpectedSchema() {
		cell := cells[i]
		fail := func(message string) (Choice, error) {
			return Choice{}, fmt.Errorf("row %s/%s: %s", rowID, field.Name, message)
		}
		if cell.Name != field.Name || cell.DAOType != field.DAOType || cell.IsNull != (cell.Value == nil) {
			return fail("typed cell mismatch")
		}
		if cell.IsNull {
			if cell.Utf16CodeUnits != nil {
				return fail("NULL cell has UTF-16 data")
			}
			continue
		}
		if field.DAOType != 10 && cell.Utf16CodeUnits != nil {
			return fail("numeric/Boolean cell has UTF-16 data")
		}
		if dst, ok := texts[field.Name]; ok {
			units := utf16.Encode([]rune(*cell.Value))
			if !utf8.ValidString(*cell.Value) || len(units) > field.Size || !reflect.DeepEqual(units, cell.Utf16CodeUnits) {
				return fail("string/UTF-16 mismatch")
			}
			value := *cell.Value
			*dst = &value
		} else if field.Name == "ItemOrder" {
			value, err := strconv.ParseFloat(*cell.Value, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return fail("ItemOrder is not finite IEEE64")
			}
			row.ItemOrder = &value
		} else {
			value := false
			switch *cell.Value {
			case "True", "-1":
				value = true
			case "False", "0":
			default:
				return fail("not a DAO Boolean")
			}
			if field.Name == "Validate" {
				row.Validate = &value
			} else {
				row.Flag = &value
			}
		}
	}
	return row, nil
}

func OrderBytes(value *float64) []byte {
	if value == nil {
		return nil
	}
	result := make([]byte, 8)
	binary.BigEndian.PutUint64(result, math.Float64bits(*value))
	return result
}

// TypedHash hashes only the ten metadata cells, in caller-provided row order.
// Row identities and application selection policy are deliberately not metadata.
func TypedHash(rows []Choice) (string, error) {
	cells := make([][]any, 0, len(rows))
	for _, row := range rows {
		if row.ItemOrder != nil && (math.IsNaN(*row.ItemOrder) || math.IsInf(*row.ItemOrder, 0)) {
			return "", fmt.Errorf("row %s: nonfinite ItemOrder", row.RowID)
		}
		for _, value := range []*string{row.ListName, row.ListFilter, row.Code, row.Description, row.FieldUsedIn, row.ValidateLoops, row.Note} {
			if value != nil && (!utf8.ValidString(*value) || len(utf16.Encode([]rune(*value))) > 255) {
				return "", fmt.Errorf("row %s: invalid DAO text", row.RowID)
			}
		}
		cells = append(cells, []any{row.ListName, row.ListFilter, OrderBytes(row.ItemOrder),
			row.Code, row.Description, row.FieldUsedIn, row.ValidateLoops, row.Validate, row.Note, row.Flag})
	}
	data, err := json.Marshal(cells)
	if err != nil {
		return "", err
	}
	return checksum(data), nil
}

func checksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
