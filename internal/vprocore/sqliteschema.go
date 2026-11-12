package vprocore

import (
	"context"
	"errors"
)

type SqliteSchemaObject struct {
	Type, Name, Table string
	SQL               *string
}

func ReadSQLiteSchemaObjects(ctx context.Context, db ProjectMetadataQueryer, alias, table string) (result []SqliteSchemaObject, resultErr error) {
	rows, err := db.QueryContext(ctx, `SELECT type,name,tbl_name,sql FROM `+QuoteHeaderIdentifier(alias)+
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
	result = []SqliteSchemaObject{}
	for rows.Next() {
		var object SqliteSchemaObject
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
