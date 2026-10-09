package main

import (
	"context"
	"errors"
)

type sqliteSchemaObject struct {
	Type, Name, Table string
	SQL               *string
}

func readSQLiteSchemaObjects(ctx context.Context, db projectMetadataQueryer, alias, table string) (result []sqliteSchemaObject, resultErr error) {
	rows, err := db.QueryContext(ctx, `SELECT type,name,tbl_name,sql FROM `+quoteHeaderIdentifier(alias)+
		`.sqlite_master WHERE tbl_name COLLATE BINARY=? ORDER BY type COLLATE BINARY,name COLLATE BINARY`, table)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, rows.Close())
		if resultErr != nil {
			result = nil
		}
	}()
	result = []sqliteSchemaObject{}
	for rows.Next() {
		var object sqliteSchemaObject
		if err := rows.Scan(&object.Type, &object.Name, &object.Table, &object.SQL); err != nil {
			return nil, err
		}
		result = append(result, object)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
