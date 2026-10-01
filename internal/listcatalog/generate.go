package listcatalog

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func GenerateDatabase(database, provenancePath string, rows []Choice, p Provenance, profile Profile) (err error) {
	provenance, err := os.OpenFile(provenancePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("reserve provenance output (not overwritten): %w", err)
	}
	provenanceClosed, databaseOwned, complete := false, false, false
	var db *sql.DB
	defer func() {
		if db != nil {
			err = errors.Join(err, db.Close())
		}
		if !provenanceClosed {
			err = errors.Join(err, provenance.Close())
		}
		if !complete {
			err = errors.Join(err, os.Remove(provenancePath))
			if databaseOwned {
				err = errors.Join(err, os.Remove(database))
			}
		}
	}()
	reserved, err := os.OpenFile(database, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("reserve database output (not overwritten): %w", err)
	}
	databaseOwned = true
	if err := reserved.Close(); err != nil {
		return err
	}
	absolute, err := filepath.Abs(database)
	if err != nil {
		return err
	}
	db, err = sql.Open("sqlite3", absolute)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if err := ImportRows(db, rows, p, profile); err != nil {
		return err
	}
	if err := ValidateDatabaseFor(db, p, profile); err != nil {
		return err
	}
	err = db.Close()
	db = nil
	if err != nil {
		return err
	}
	data, err := os.ReadFile(database)
	if err != nil {
		return err
	}
	p.DatabaseSHA256 = checksum(data)
	encoded, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if _, err := DecodeProvenanceFor(encoded, profile); err != nil {
		return err
	}
	if _, err := provenance.Write(encoded); err != nil {
		return err
	}
	if err := provenance.Sync(); err != nil {
		return err
	}
	err = provenance.Close()
	provenanceClosed = true
	if err != nil {
		return err
	}
	complete = true
	return nil
}
