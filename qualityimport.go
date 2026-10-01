package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

type qualityProvenance struct {
	Version           int               `json:"version"`
	Exporter          string            `json:"exporter"`
	SourceSHA256      string            `json:"sourceSha256"`
	SnapshotSHA256    string            `json:"snapshotSha256"`
	DatabaseSHA256    string            `json:"databaseSha256"`
	TypedCellsSHA256  string            `json:"typedCellsSha256"`
	Rows              int               `json:"rows"`
	Cells             int               `json:"cells"`
	SourceTable       string            `json:"sourceTable"`
	Filter            string            `json:"filter"`
	SQL               string            `json:"sql"`
	SourceSchema      []becSourceColumn `json:"sourceSchema"`
	ItemOrderEncoding string            `json:"itemOrderEncoding"`
}

type qualityNativeCell = listcatalog.Cell

type qualityNativeSnapshot struct {
	Source struct {
		Before string `json:"BeforeSHA256"`
		After  string `json:"AfterSHA256"`
	} `json:"source"`
	SourceTable string `json:"sourceTable"`
	Filter      string `json:"filter"`
	SQL         string `json:"sql"`
	Schema      struct {
		Fields []struct {
			Name string
			Type int
			Size int
		}
	} `json:"schema"`
	RowCount int                   `json:"rowCount"`
	Rows     [][]qualityNativeCell `json:"rowsInObservedNativeOrder"`
	ReadOnly bool                  `json:"sourceReadOnly"`
}

func decodeQualityJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("unexpected trailing quality JSON data")
	}
	return nil
}

func qualityExpectedSchema() []becSourceColumn {
	var result []becSourceColumn
	for _, column := range listcatalog.ExpectedSchema() {
		result = append(result, becSourceColumn{column.Name, column.DAOType, column.Size})
	}
	return result
}

const qualityOrderEncoding = "nullable IEEE754 binary64 big-endian 8-byte BLOB; native source ordinal order"

func validateQualityProvenance(p qualityProvenance) error {
	for _, hash := range []string{p.SourceSHA256, p.SnapshotSHA256, p.DatabaseSHA256, p.TypedCellsSHA256} {
		if decoded, err := hex.DecodeString(hash); err != nil || len(decoded) != 32 || strings.ToLower(hash) != hash {
			return errors.New("quality provenance contains an invalid checksum")
		}
	}
	if p.Version != 1 || p.Exporter != "native-access-dao-plot-quality-site-v1" ||
		p.Rows != 5 || p.Cells != 50 || p.SourceTable != "USysTableOfLists" ||
		p.Filter != `ListName="PlotQualitySite"` ||
		p.SQL != `SELECT * FROM USysTableOfLists WHERE ListName="PlotQualitySite" ORDER BY ItemOrder` ||
		!reflect.DeepEqual(p.SourceSchema, qualityExpectedSchema()) || p.ItemOrderEncoding != qualityOrderEncoding {
		return errors.New("quality provenance source/schema mismatch")
	}
	return nil
}

// The bounded boundary consumes only the frozen five-row DAO fixture, never an Access file.
func qualitySnapshotRows(data []byte, p qualityProvenance) ([]PlotQualityChoice, error) {
	if err := validateQualityProvenance(p); err != nil {
		return nil, err
	}
	if becHash(data) != p.SnapshotSHA256 {
		return nil, errors.New("quality snapshot checksum mismatch")
	}
	var snapshot qualityNativeSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	if !snapshot.ReadOnly || snapshot.SourceTable != p.SourceTable || snapshot.Filter != p.Filter || snapshot.SQL != p.SQL ||
		strings.ToLower(snapshot.Source.Before) != p.SourceSHA256 || snapshot.Source.Before != snapshot.Source.After ||
		snapshot.RowCount != p.Rows || len(snapshot.Rows) != p.Rows || len(snapshot.Schema.Fields) != 10 {
		return nil, errors.New("quality native snapshot provenance mismatch")
	}
	for i, field := range snapshot.Schema.Fields {
		if field.Name != p.SourceSchema[i].Name || field.Type != p.SourceSchema[i].DAOType || field.Size != p.SourceSchema[i].Size {
			return nil, errors.New("quality native source schema mismatch")
		}
	}
	result := make([]PlotQualityChoice, 0, p.Rows)
	for ordinal, cells := range snapshot.Rows {
		row, err := listcatalog.DecodeCells(strconv.Itoa(ordinal+1), cells)
		if err != nil {
			return nil, fmt.Errorf("quality native cells: %w", err)
		}
		result = append(result, PlotQualityChoice(row))
	}
	return result, nil
}

const qualitySchemaSQL = `
CREATE TABLE PlotQualityChoices(
 RowID TEXT PRIMARY KEY NOT NULL, ListName TEXT, ListFilter TEXT, ItemOrder BLOB,
 Item TEXT, ItemDescription TEXT, FieldUsedIn TEXT, ValidateLoops TEXT,
 Validate BOOLEAN CHECK(Validate IS NULL OR Validate IN (0,1)), Note TEXT,
 Flag BOOLEAN CHECK(Flag IS NULL OR Flag IN (0,1)),
 CHECK(ItemOrder IS NULL OR (typeof(ItemOrder)='blob' AND length(ItemOrder)=8)));
CREATE TABLE QualityProvenance(
 Version INTEGER NOT NULL, Exporter TEXT NOT NULL, SourceSHA256 TEXT NOT NULL,
 SnapshotSHA256 TEXT NOT NULL, TypedCellsSHA256 TEXT NOT NULL);
`

func qualityOrderBytes(value *float64) []byte {
	return listcatalog.OrderBytes(value)
}

func qualityTypedHash(rows []PlotQualityChoice) (string, error) {
	choices := make([]listcatalog.Choice, 0, len(rows))
	for _, row := range rows {
		choices = append(choices, listcatalog.Choice(row))
	}
	return listcatalog.TypedHash(choices)
}

func importQualitySnapshot(db *sql.DB, data []byte, p qualityProvenance) error {
	rows, err := qualitySnapshotRows(data, p)
	if err != nil {
		return err
	}
	hash, err := qualityTypedHash(rows)
	if err != nil {
		return err
	}
	if hash != p.TypedCellsSHA256 {
		return errors.New("quality snapshot typed-cell checksum mismatch")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(qualitySchemaSQL); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := tx.Exec(`INSERT INTO PlotQualityChoices VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			row.RowID, row.ListName, row.ListFilter, qualityOrderBytes(row.ItemOrder), row.Code,
			row.Description, row.FieldUsedIn, row.ValidateLoops, row.Validate, row.Note, row.Flag); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO QualityProvenance VALUES (?,?,?,?,?)`,
		p.Version, p.Exporter, p.SourceSHA256, p.SnapshotSHA256, p.TypedCellsSHA256); err != nil {
		return err
	}
	if err := validateQualityDatabase(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func validateQualityDatabase(db headerDB, p qualityProvenance) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM QualityProvenance`).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return errors.New("quality database provenance is missing/ambiguous")
	}
	var stored qualityProvenance
	if err := db.QueryRow(`SELECT Version,Exporter,SourceSHA256,SnapshotSHA256,TypedCellsSHA256 FROM QualityProvenance`).
		Scan(&stored.Version, &stored.Exporter, &stored.SourceSHA256, &stored.SnapshotSHA256, &stored.TypedCellsSHA256); err != nil {
		return err
	}
	if stored.Version != p.Version || stored.Exporter != p.Exporter || stored.SourceSHA256 != p.SourceSHA256 ||
		stored.SnapshotSHA256 != p.SnapshotSHA256 || stored.TypedCellsSHA256 != p.TypedCellsSHA256 {
		return errors.New("quality database provenance mismatch")
	}
	rows, err := qualityChoices(db)
	if err != nil {
		return err
	}
	if len(rows) != p.Rows {
		return errors.New("quality database row count mismatch")
	}
	for i, row := range rows {
		if row.RowID != strconv.Itoa(i+1) {
			return errors.New("quality source ordinal identity mismatch")
		}
	}
	hash, err := qualityTypedHash(rows)
	if err != nil {
		return err
	}
	if hash != p.TypedCellsSHA256 {
		return errors.New("quality database typed-cell checksum mismatch")
	}
	return nil
}
