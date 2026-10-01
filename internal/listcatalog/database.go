package listcatalog

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
)

const schemaSQL = `
CREATE TABLE SiteCodeChoices(
 RowID TEXT NOT NULL, ListName TEXT NOT NULL, ListFilter TEXT, ItemOrder BLOB,
 Item TEXT, ItemDescription TEXT, FieldUsedIn TEXT, ValidateLoops TEXT,
 Validate INTEGER CHECK(Validate IS NULL OR (typeof(Validate)='integer' AND Validate IN (0,1))), Note TEXT,
 Flag INTEGER CHECK(Flag IS NULL OR (typeof(Flag)='integer' AND Flag IN (0,1))),
 PRIMARY KEY(ListName,RowID),
 CHECK(ItemOrder IS NULL OR (typeof(ItemOrder)='blob' AND length(ItemOrder)=8)));
CREATE TABLE SiteCodeProvenance(
 Version INTEGER NOT NULL, Exporter TEXT NOT NULL, SourceSHA256 TEXT NOT NULL,
 SnapshotSHA256 TEXT NOT NULL, TypedCellsSHA256 TEXT NOT NULL);
`

func ImportSnapshot(db *sql.DB, data []byte, p Provenance) error {
	rows, err := SnapshotRows(data, p)
	if err != nil {
		return err
	}
	return ImportRows(db, rows, p, SiteProfile())
}

func ImportRows(db *sql.DB, rows []Choice, p Provenance, profile Profile) error {
	if err := ValidateProvenanceFor(p, profile); err != nil {
		return err
	}
	hash, err := TypedHash(rows)
	if err != nil {
		return err
	}
	if hash != p.TypedCellsSHA256 {
		return errors.New("site code snapshot typed-cell checksum mismatch")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// No IF NOT EXISTS: either catalogue table collision refuses the import.
	if _, err := tx.Exec(schemaSQL); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := tx.Exec(`INSERT INTO SiteCodeChoices VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			row.RowID, row.ListName, row.ListFilter, OrderBytes(row.ItemOrder), row.Code,
			row.Description, row.FieldUsedIn, row.ValidateLoops, row.Validate, row.Note, row.Flag); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO SiteCodeProvenance VALUES (?,?,?,?,?)`,
		p.Version, p.Exporter, p.SourceSHA256, p.SnapshotSHA256, p.TypedCellsSHA256); err != nil {
		return err
	}
	if err := ValidateDatabaseFor(tx, p, profile); err != nil {
		return err
	}
	return tx.Commit()
}

func ReadChoices(db interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}, list string) ([]Choice, error) {
	return ReadChoicesFor(db, list, SiteProfile())
}

func ReadChoicesFor(db interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}, list string, profile Profile) ([]Choice, error) {
	supported := list == ""
	for _, definition := range profile.Lists {
		supported = supported || list == definition.Name
	}
	if !supported {
		return nil, fmt.Errorf("unsupported site code list %q", list)
	}
	query := `SELECT RowID,ListName,ListFilter,ItemOrder,Item,ItemDescription,
 FieldUsedIn,ValidateLoops,Validate,Note,Flag,typeof(ItemOrder),typeof(Validate),typeof(Flag)
 FROM SiteCodeChoices`
	var args []any
	if list != "" {
		query += ` WHERE ListName=?`
		args = append(args, list)
	}
	query += ` ORDER BY ListName COLLATE BINARY,CAST(RowID AS INTEGER)`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Choice{}
	for rows.Next() {
		var row Choice
		var order []byte
		var validate, flag sql.NullInt64
		var orderType, validateType, flagType string
		if err := rows.Scan(&row.RowID, &row.ListName, &row.ListFilter, &order, &row.Code,
			&row.Description, &row.FieldUsedIn, &row.ValidateLoops, &validate, &row.Note, &flag,
			&orderType, &validateType, &flagType); err != nil {
			return nil, err
		}
		if orderType != "null" {
			if orderType != "blob" || len(order) != 8 {
				return nil, fmt.Errorf("row %s: ItemOrder is not exact IEEE64 BLOB", row.RowID)
			}
			value := math.Float64frombits(binary.BigEndian.Uint64(order))
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("row %s: nonfinite ItemOrder", row.RowID)
			}
			row.ItemOrder = &value
		}
		for _, field := range []struct {
			name string
			src  sql.NullInt64
			kind string
			dst  **bool
		}{{"Validate", validate, validateType, &row.Validate}, {"Flag", flag, flagType, &row.Flag}} {
			if !field.src.Valid {
				continue
			}
			if field.kind != "integer" || (field.src.Int64 != 0 && field.src.Int64 != 1) {
				return nil, fmt.Errorf("row %s: %s is not normalized Boolean", row.RowID, field.name)
			}
			value := field.src.Int64 == 1
			*field.dst = &value
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func ValidateDatabase(db interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}, p Provenance) error {
	return ValidateDatabaseFor(db, p, SiteProfile())
}

func ValidateDatabaseFor(db interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}, p Provenance, profile Profile) error {
	if err := ValidateProvenanceFor(p, profile); err != nil {
		return err
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM SiteCodeProvenance`).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return errors.New("site code database provenance is missing/ambiguous")
	}
	var stored Provenance
	if err := db.QueryRow(`SELECT Version,Exporter,SourceSHA256,SnapshotSHA256,TypedCellsSHA256 FROM SiteCodeProvenance`).
		Scan(&stored.Version, &stored.Exporter, &stored.SourceSHA256, &stored.SnapshotSHA256, &stored.TypedCellsSHA256); err != nil {
		return err
	}
	if stored.Version != p.Version || stored.Exporter != p.Exporter || stored.SourceSHA256 != p.SourceSHA256 ||
		stored.SnapshotSHA256 != p.SnapshotSHA256 || stored.TypedCellsSHA256 != p.TypedCellsSHA256 {
		return errors.New("site code database provenance mismatch")
	}
	rows, err := ReadChoicesFor(db, "", profile)
	if err != nil {
		return err
	}
	if len(rows) != p.Rows {
		return errors.New("site code database row count mismatch")
	}
	index := 0
	for _, list := range profile.Lists {
		for ordinal := 1; ordinal <= list.Rows; ordinal++ {
			row := rows[index]
			index++
			if row.ListName == nil || *row.ListName != list.Name || row.RowID != strconv.Itoa(ordinal) {
				return errors.New("site code source ordinal identity mismatch")
			}
		}
	}
	hash, err := TypedHash(rows)
	if err != nil {
		return err
	}
	if hash != p.TypedCellsSHA256 {
		return errors.New("site code database typed-cell checksum mismatch")
	}
	return nil
}
