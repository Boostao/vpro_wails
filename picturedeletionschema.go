package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func pictureRestorationCollisionCount(ctx context.Context, tx *sql.Tx, rowID, id int64) (int, error) {
	var collisions int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tblVPics WHERE rowid=? OR ID=? OR
		(typeof(ID) IN ('text','real','blob') AND CAST(ID AS NUMERIC)=?)`, rowID, id, id).Scan(&collisions)
	return collisions, err
}

func pictureDeletionWriterSchema(ctx context.Context, tx *sql.Tx) error {
	if err := pictureMetadataWriterSchema(ctx, tx); err != nil {
		return err
	}
	var definition string
	if err := tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name COLLATE BINARY='tblVPics'`).Scan(&definition); err != nil {
		return err
	}
	upper := strings.ToUpper(definition)
	if strings.Contains(upper, "VIRTUAL TABLE") || strings.Contains(upper, "WITHOUT ROWID") {
		return errors.New("picture deletion requires an ordinary rowid table")
	}
	var incoming int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master AS source
		JOIN pragma_foreign_key_list(source.name) AS dependency
		WHERE source.type='table' AND dependency."table" COLLATE NOCASE='tblVPics'`).Scan(&incoming); err != nil {
		return err
	}
	if incoming != 0 {
		return errors.New("picture deletion refuses unreviewed incoming foreign keys")
	}
	return nil
}
