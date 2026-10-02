package main

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

func reserveProfileIdentities(ctx context.Context, tx *sql.Tx, table string, current ProjectMetadataTable) (map[int64]bool, error) {
	ids := map[int64]bool{}
	add := func(snapshot ProjectMetadataTable) error {
		if _, _, err := prepareProfileRuleDrafts(snapshot, nil); err != nil {
			return err
		}
		for _, row := range snapshot.Rows {
			id, err := strconv.ParseInt(row.RowID, 10, 64)
			if err != nil {
				return err
			}
			ids[id] = true
		}
		return nil
	}
	if err := add(current); err != nil {
		return nil, err
	}
	var historyExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master
		WHERE name COLLATE NOCASE='__VPRO_ProfileHistory')`).Scan(&historyExists); err != nil {
		return nil, err
	}
	if historyExists {
		if err := validateProfileHistorySchema(ctx, tx); err != nil {
			return nil, err
		}
		rows, err := tx.QueryContext(ctx, `SELECT BeforeRules,AfterRules FROM "__VPRO_ProfileHistory" WHERE ProfileTable COLLATE BINARY=?`, table)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var before, after string
			if err := rows.Scan(&before, &after); err != nil {
				rows.Close()
				return nil, err
			}
			for _, raw := range []string{before, after} {
				var snapshot ProjectMetadataTable
				if err := decodeProfileLifecycleJSON([]byte(raw), &snapshot, "columns", "rows"); err != nil {
					rows.Close()
					return nil, err
				}
				if err := add(snapshot); err != nil {
					rows.Close()
					return nil, err
				}
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ProfileIdentity"(
		ProfileTable TEXT NOT NULL,RuleRowID INTEGER NOT NULL,PRIMARY KEY(ProfileTable,RuleRowID))`); err != nil {
		return nil, err
	}
	if err := validateProfileIdentitySchema(ctx, tx); err != nil {
		return nil, err
	}
	expected, err := readSQLiteStorageRows(ctx, tx, "main", "__VPRO_ProfileIdentity", "", nil, "")
	if err != nil {
		return nil, err
	}
	prior := map[int64]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT typeof(RuleRowID),RuleRowID FROM "__VPRO_ProfileIdentity"
		WHERE ProfileTable COLLATE BINARY=?`, table)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var storage string
		var id int64
		if err := rows.Scan(&storage, &id); err != nil {
			rows.Close()
			return nil, err
		}
		if storage != "integer" {
			rows.Close()
			return nil, errors.New("profile reservation requires literal signed64 integer storage")
		}
		ids[id] = true
		prior[id] = true
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	for id := range ids {
		if !prior[id] {
			if err := addProfileReservation(ctx, tx, table, id, &expected); err != nil {
				return nil, err
			}
		}
		var reserved bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM "__VPRO_ProfileIdentity"
			WHERE ProfileTable COLLATE BINARY=? AND RuleRowID=?)`, table, id).Scan(&reserved); err != nil {
			return nil, err
		}
		if !reserved {
			return nil, errors.New("profile physical identity reservation was ignored; mutation/history rolled back")
		}
	}
	if err := verifyProfileReservations(ctx, tx, expected); err != nil {
		return nil, err
	}
	return ids, nil
}

func addProfileReservation(ctx context.Context, tx *sql.Tx, table string, id int64, expected *ProjectMetadataTable) error {
	result, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ProfileIdentity"(ProfileTable,RuleRowID) VALUES(?,?)`, table, id)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return errors.Join(errors.New("profile reservation expected one inserted physical identity"), err)
	}
	physicalID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	literalID := strconv.FormatInt(id, 10)
	expected.Rows = append(expected.Rows, ProjectMetadataRow{RowID: strconv.FormatInt(physicalID, 10),
		Cells: []ProjectMetadataCell{{Storage: "text", Text: &table}, {Storage: "integer", Integer: &literalID}}})
	return nil
}

func validateProfileIdentitySchema(ctx context.Context, tx *sql.Tx) (resultErr error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info("__VPRO_ProfileIdentity")`)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	index := 0
	for rows.Next() {
		var cid, required, primary int
		var name, declared string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &declared, &required, &defaultValue, &primary); err != nil {
			return err
		}
		wantName, wantType := "ProfileTable", "TEXT"
		if index == 1 {
			wantName, wantType = "RuleRowID", "INTEGER"
		}
		if index > 1 || cid != index || name != wantName || declared != wantType ||
			required != 1 || primary != index+1 || defaultValue.Valid {
			return errors.New("profile physical identity reservation schema is incompatible")
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if index != 2 {
		return errors.New("profile physical identity reservation schema is incomplete")
	}
	return nil
}
