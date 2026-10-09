package main

import (
	"context"
	"errors"
	"fmt"
)

func profileMetadataColumns(ctx context.Context, db projectMetadataQueryer, role string) ([]ProjectMetadataColumn, error) {
	rows, err := db.QueryContext(ctx, `SELECT type FROM `+quoteHeaderIdentifier(role)+
		`.sqlite_master WHERE name COLLATE NOCASE='_table_metadata' AND type IN ('table','view')`)
	if err != nil {
		return nil, err
	}
	present := false
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		if present || kind != "table" {
			return nil, errors.Join(errors.New("profile description metadata must be an original physical table, not a view or ambiguous definition"), rows.Close())
		}
		present = true
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	columns, err := readSQLiteStorageColumns(ctx, db, role, "_table_metadata")
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, errors.New("present profile description metadata has no readable physical columns")
	}
	return columns, nil
}

func readProfileDescriptions(ctx context.Context, db projectMetadataQueryer, role, table string) (ProjectMetadataTable, error) {
	columns, err := profileMetadataColumns(ctx, db, role)
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	if columns == nil {
		return ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}, nil
	}
	if reason := profileMetadataReason(columns); reason != "" {
		return ProjectMetadataTable{}, fmt.Errorf("present profile description metadata is malformed: %s; no inferred repair", reason)
	}
	return readSQLiteStorageRows(ctx, db, role, "_table_metadata", "table_name", &table, "")
}
