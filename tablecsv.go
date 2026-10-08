package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type TableCSVDescription struct {
	RowID string              `json:"rowId"`
	Value ProjectMetadataCell `json:"value"`
}

type TableCSVManifest struct {
	Version      int                     `json:"version"`
	Table        string                  `json:"table"`
	Columns      []ProjectMetadataColumn `json:"columns"`
	RowIDs       []string                `json:"rowIds"`
	Storage      [][]string              `json:"storage"`
	Descriptions []TableCSVDescription   `json:"descriptions"`
	SHA256       string                  `json:"sha256"`
}

type tableCSVDocument struct {
	Manifest TableCSVManifest
	Data     []byte
}

func tableCSVIdentity(value string) bool {
	integer, err := strconv.ParseInt(value, 10, 64)
	return err == nil && strconv.FormatInt(integer, 10) == value
}

func validateTableCSVManifest(manifest TableCSVManifest) error {
	if manifest.Version != 1 || manifest.Table == "" || !utf8.ValidString(manifest.Table) ||
		strings.ContainsRune(manifest.Table, 0) || manifest.Columns == nil || len(manifest.Columns) == 0 ||
		manifest.RowIDs == nil || manifest.Storage == nil || manifest.Descriptions == nil ||
		len(manifest.RowIDs) != len(manifest.Storage) {
		return errors.New("table CSV requires a complete version1 physical table manifest")
	}
	names := map[string]bool{}
	for _, column := range manifest.Columns {
		if column.Name == "" || names[column.Name] || !utf8.ValidString(column.Name) ||
			strings.ContainsAny(column.Name, "\x00\r") || !utf8.ValidString(column.DeclaredType) {
			return errors.New("table CSV column identity/type is unavailable, repeated or unsupported")
		}
		names[column.Name] = true
	}
	identities := map[string]bool{}
	for index, id := range manifest.RowIDs {
		if !tableCSVIdentity(id) || identities[id] || len(manifest.Storage[index]) != len(manifest.Columns) {
			return errors.New("table CSV row identity or complete storage tags are unavailable or repeated")
		}
		identities[id] = true
	}
	identities = map[string]bool{}
	for _, candidate := range manifest.Descriptions {
		if !tableCSVIdentity(candidate.RowID) || identities[candidate.RowID] {
			return errors.New("table CSV Description physical identities are unavailable or repeated")
		}
		identities[candidate.RowID] = true
		if _, err := metadataCellValue(candidate.Value); err != nil {
			return fmt.Errorf("table CSV Description %s: %w", candidate.RowID, err)
		}
	}
	return nil
}

func encodeTableCSVCell(cell ProjectMetadataCell) (string, error) {
	if _, err := metadataCellValue(cell); err != nil {
		return "", err
	}
	switch cell.Storage {
	case "null":
		return "", nil
	case "text":
		// JSON escapes preserve CRLF and prevent CSV readers repairing line endings.
		value, err := json.Marshal(*cell.Text)
		return string(value), err
	case "integer":
		return *cell.Integer, nil
	case "real":
		return strconv.FormatFloat(*cell.Real, 'g', -1, 64), nil
	case "blob":
		return *cell.BlobHex, nil
	}
	return "", errors.New("table CSV storage is unsupported")
}

func encodeTableCSV(ctx context.Context, name string, table ProjectMetadataTable, descriptions []TableCSVDescription) (tableCSVDocument, error) {
	var zero tableCSVDocument
	if table.Rows == nil {
		return zero, errors.New("table CSV requires explicit physical rows, including an empty list")
	}
	manifest := TableCSVManifest{Version: 1, Table: name, Columns: table.Columns,
		RowIDs: []string{}, Storage: [][]string{}, Descriptions: descriptions}
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if len(row.Cells) != len(table.Columns) {
			return zero, errors.New("table CSV row must contain every physical column")
		}
		tags := make([]string, len(row.Cells))
		for index, cell := range row.Cells {
			tags[index] = cell.Storage
		}
		manifest.RowIDs = append(manifest.RowIDs, row.RowID)
		manifest.Storage = append(manifest.Storage, tags)
	}
	if err := validateTableCSVManifest(manifest); err != nil {
		return zero, err
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	header := make([]string, len(table.Columns))
	for index, column := range table.Columns {
		header[index] = column.Name
	}
	if err := writer.Write(header); err != nil {
		return zero, err
	}
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		values := make([]string, len(row.Cells))
		for index, cell := range row.Cells {
			value, err := encodeTableCSVCell(cell)
			if err != nil {
				return zero, fmt.Errorf("table CSV row %s column %s: %w", row.RowID, table.Columns[index].Name, err)
			}
			values[index] = value
		}
		if len(values) == 1 && values[0] == "" {
			// A bare empty record is skipped by CSV readers; quote its sole field.
			writer.Flush()
			if err := writer.Error(); err != nil {
				return zero, err
			}
			output.WriteString("\"\"\n")
		} else {
			if err := writer.Write(values); err != nil {
				return zero, err
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	data := append([]byte(nil), output.Bytes()...)
	hash := sha256.Sum256(data)
	manifest.SHA256 = hex.EncodeToString(hash[:])
	// Detach nested Description cell pointers as well as schema/storage slices.
	raw, err := json.Marshal(manifest)
	if err != nil {
		return zero, err
	}
	var detached TableCSVManifest
	if err := json.Unmarshal(raw, &detached); err != nil {
		return zero, err
	}
	return tableCSVDocument{Manifest: detached, Data: data}, nil
}

func decodeTableCSVCell(storage, raw string) (ProjectMetadataCell, error) {
	cell := ProjectMetadataCell{Storage: storage}
	switch storage {
	case "null":
		if raw != "" {
			return cell, errors.New("NULL cell must have an empty CSV value")
		}
	case "text":
		if err := validateMetadataDraftJSON([]byte(raw)); err != nil {
			return cell, err
		}
		if err := json.Unmarshal([]byte(raw), &cell.Text); err != nil {
			return cell, err
		}
	case "integer":
		cell.Integer = &raw
	case "real":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return cell, err
		}
		cell.Real = &value
	case "blob":
		cell.BlobHex = &raw
	default:
		return cell, errors.New("table CSV storage tag is unsupported")
	}
	if _, err := metadataCellValue(cell); err != nil {
		return cell, err
	}
	return cell, nil
}

func decodeTableCSV(ctx context.Context, document tableCSVDocument) (ProjectMetadataTable, error) {
	var zero ProjectMetadataTable
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	manifest := document.Manifest
	if err := validateTableCSVManifest(manifest); err != nil {
		return zero, err
	}
	hash := sha256.Sum256(document.Data)
	if manifest.SHA256 != hex.EncodeToString(hash[:]) {
		return zero, errors.New("table CSV checksum differs from the complete manifest")
	}
	if !utf8.Valid(document.Data) {
		return zero, errors.New("table CSV contains malformed UTF-8; no input was repaired")
	}
	reader := csv.NewReader(bytes.NewReader(document.Data))
	reader.FieldsPerRecord = len(manifest.Columns)
	header, err := reader.Read()
	if err != nil {
		return zero, err
	}
	for index, column := range manifest.Columns {
		if header[index] != column.Name {
			return zero, errors.New("table CSV header differs from the physical manifest")
		}
	}
	result := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn(nil), manifest.Columns...), Rows: []ProjectMetadataRow{}}
	for index, id := range manifest.RowIDs {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		values, err := reader.Read()
		if err != nil {
			return zero, fmt.Errorf("table CSV row %s: %w", id, err)
		}
		row := ProjectMetadataRow{RowID: id, Cells: make([]ProjectMetadataCell, len(values))}
		for column, raw := range values {
			value, err := decodeTableCSVCell(manifest.Storage[index][column], raw)
			if err != nil {
				return zero, fmt.Errorf("table CSV row %s column %s: %w", id, manifest.Columns[column].Name, err)
			}
			row.Cells[column] = value
		}
		result.Rows = append(result.Rows, row)
	}
	if _, err := reader.Read(); err != io.EOF {
		return zero, errors.Join(errors.New("table CSV has extra records or malformed trailing data"), err)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return result, nil
}
