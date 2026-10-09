package listcatalog

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type daoNativeCell struct {
	CLRType                *string  `json:"CLRType"`
	DAOType                int      `json:"DAOType"`
	IEEE754LittleEndianHex *string  `json:"IEEE754LittleEndianHex"`
	IsNull                 bool     `json:"IsNull"`
	Name                   string   `json:"Name"`
	Utf16CodeUnits         []uint16 `json:"Utf16CodeUnits"`
	Value                  *string  `json:"Value"`
}

type daoSnapshotSchema struct {
	Name    string
	Connect string
	Fields  []struct {
		Name string
		Type int
		Size int
	}
}

type daoSnapshotCatalogue struct {
	SQL    string
	Count  int
	SHA256 string
	Rows   []string
}

func readDAOGzip(data []byte, target any) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	raw, readErr := io.ReadAll(io.LimitReader(reader, 1024*1024+1))
	if err := errors.Join(readErr, reader.Close()); err != nil {
		return err
	}
	if len(raw) > 1024*1024 {
		return errors.New("DAO snapshot exceeds frozen boundary")
	}
	return json.Unmarshal(raw, target)
}

func validateDAOSchema(schema daoSnapshotSchema, p Provenance) error {
	if schema.Name != p.SourceTable || schema.Connect != "" || len(schema.Fields) != len(p.SourceSchema) {
		return errors.New("DAO native source schema mismatch")
	}
	for i, field := range schema.Fields {
		want := p.SourceSchema[i]
		if field.Name != want.Name || field.Type != want.DAOType || field.Size != want.Size {
			return errors.New("DAO native source schema mismatch")
		}
	}
	return nil
}

func decodeDAOCatalogue(catalogue daoSnapshotCatalogue, choices []Choice, list ListDefinition) ([]Choice, [][]daoNativeCell, error) {
	if len(choices) != list.Rows {
		return nil, nil, fmt.Errorf("DAO %s choice count mismatch", list.Name)
	}
	rows, native, err := decodeDAORows(catalogue, list)
	if err != nil {
		return nil, nil, err
	}
	for i, row := range rows {
		if choices[i].RowID != row.RowID {
			return nil, nil, errors.New("DAO source-ordinal mismatch")
		}
		wantHash, err := TypedHash([]Choice{choices[i]})
		if err != nil {
			return nil, nil, err
		}
		gotHash, err := TypedHash([]Choice{row})
		if err != nil || wantHash != gotHash {
			return nil, nil, errors.New("DAO13-field conversion mismatch")
		}
	}
	return rows, native, nil
}

func decodeDAORows(catalogue daoSnapshotCatalogue, list ListDefinition) ([]Choice, [][]daoNativeCell, error) {
	if catalogue.SQL != DAOSourceSQL(list.Name) || catalogue.Count != list.Rows ||
		len(catalogue.Rows) != list.Rows {
		return nil, nil, fmt.Errorf("DAO %s query/casing/count mismatch", list.Name)
	}
	if checksum([]byte(strings.Join(catalogue.Rows, "\n"))) != catalogue.SHA256 {
		return nil, nil, errors.New("DAO raw-row checksum mismatch")
	}
	rows := make([]Choice, 0, list.Rows)
	allCells := make([][]daoNativeCell, 0, list.Rows)
	for ordinal, encoded := range catalogue.Rows {
		var native []daoNativeCell
		if err := json.Unmarshal([]byte(encoded), &native); err != nil {
			return nil, nil, err
		}
		cells := make([]Cell, len(native))
		for i, cell := range native {
			cells[i] = Cell{cell.Name, cell.DAOType, cell.IsNull, cell.Value, cell.Utf16CodeUnits}
			if err := validateDAOScalar(cell); err != nil {
				return nil, nil, err
			}
		}
		rowID := strconv.Itoa(ordinal + 1)
		row, err := DecodeCells(rowID, cells)
		if err != nil {
			return nil, nil, err
		}
		if row.ListName == nil || *row.ListName != list.Name {
			return nil, nil, errors.New("DAO member/source-ordinal mismatch")
		}
		if !native[2].IsNull {
			bits, _ := hex.DecodeString(*native[2].IEEE754LittleEndianHex)
			value := math.Float64frombits(binary.LittleEndian.Uint64(bits))
			row.ItemOrder = &value
		}
		rows = append(rows, row)
		allCells = append(allCells, native)
	}
	return rows, allCells, nil
}

func validateDAOScalar(cell daoNativeCell) error {
	if cell.IsNull {
		if cell.CLRType != nil || cell.IEEE754LittleEndianHex != nil {
			return errors.New("DAO NULL scalar has native payload")
		}
		return nil
	}
	if cell.Value == nil || cell.CLRType == nil {
		return errors.New("DAO native scalar metadata missing")
	}
	expected := map[int]string{10: "System.String", 7: "System.Double", 1: "System.Boolean"}
	if expected[cell.DAOType] != *cell.CLRType {
		return errors.New("DAO native CLR/type mismatch")
	}
	if cell.DAOType != 7 {
		if cell.IEEE754LittleEndianHex != nil {
			return errors.New("DAO nonnumeric scalar has double bits")
		}
		return nil
	}
	if cell.IEEE754LittleEndianHex == nil {
		return errors.New("DAO double bits missing")
	}
	bits, err := hex.DecodeString(*cell.IEEE754LittleEndianHex)
	if err != nil || len(bits) != 8 {
		return errors.New("DAO double bits invalid")
	}
	value := math.Float64frombits(binary.LittleEndian.Uint64(bits))
	text, err := strconv.ParseFloat(*cell.Value, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || text != value {
		return errors.New("DAO double text/bits mismatch")
	}
	return nil
}

// Reproduce the captured Python sorted-key/ASCII JSON digest, including CLR and exact native double bits.
func daoNativeHash(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	data := bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))
	var ascii bytes.Buffer
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return "", errors.New("DAO native JSON invalid UTF8")
		}
		if r < 128 {
			ascii.WriteByte(byte(r))
		} else {
			for _, unit := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&ascii, `\u%04x`, unit)
			}
		}
		data = data[size:]
	}
	return checksum(ascii.Bytes()), nil
}
