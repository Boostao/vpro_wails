package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func openListCatalogue(dataDir, filename, label string, database, provenanceJSON []byte, profile listcatalog.Profile) (*sql.DB, string, listcatalog.Provenance, error) {
	var p listcatalog.Provenance
	if dataDir == "" {
		var err error
		dataDir, err = userDataDir()
		if err != nil {
			return nil, "", p, err
		}
	}
	p, err := listcatalog.DecodeProvenanceFor(provenanceJSON, profile)
	if err != nil {
		return nil, "", p, fmt.Errorf("%s catalogue provenance: %w", label, err)
	}
	if becHash(database) != p.DatabaseSHA256 {
		return nil, "", p, fmt.Errorf("embedded %s catalogue checksum mismatch", label)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, "", p, err
	}
	target := filepath.Join(dataDir, filename)
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		_, writeErr := file.Write(database)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return nil, "", p, errors.Join(writeErr, closeErr, os.Remove(target))
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, "", p, err
	}
	if err := checkListCatalogueChecksum(target, p, label); err != nil {
		return nil, "", p, err
	}
	db, err := openReadOnly(target)
	if err != nil {
		return nil, "", p, err
	}
	if err := listcatalog.ValidateDatabaseFor(db, p, profile); err != nil {
		return nil, "", p, errors.Join(err, db.Close())
	}
	return db, target, p, nil
}

func checkListCatalogueChecksum(path string, p listcatalog.Provenance, label string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s catalogue: %w", label, err)
	}
	if becHash(data) != p.DatabaseSHA256 {
		return fmt.Errorf("existing %s catalogue checksum mismatch; file was not replaced", label)
	}
	return nil
}
