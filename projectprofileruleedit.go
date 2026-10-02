package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
)

type ProjectPlotProfileEdit struct {
	OriginalRules ProjectMetadataTable          `json:"originalRules"`
	Drafts        []ProjectPlotProfileRuleDraft `json:"drafts"`
}

func (request *ProjectPlotProfileEdit) UnmarshalJSON(data []byte) error {
	type plain ProjectPlotProfileEdit
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "originalRules", "drafts"); err != nil {
		return fmt.Errorf("profile draft: %w", err)
	}
	for _, draft := range decoded.Drafts {
		if draft.RowID == "" || draft.Changes == nil {
			return errors.New("profile draft requires explicit physical row identity and changes")
		}
	}
	*request = ProjectPlotProfileEdit(decoded)
	return nil
}

func (s *ContextService) SaveProjectPlotProfile(ctx context.Context, contextID string, request ProjectPlotProfileEdit) error {
	_, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		if err := plots.requireContextEdit(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, plots.projects.sqlite.saveProfileRuleDrafts(ctx, request.OriginalRules, request.Drafts, plots.currentUser)
	})
	return err
}

func sameProfileStorageRows(a, b ProjectMetadataTable) bool {
	if !reflect.DeepEqual(a.Columns, b.Columns) || len(a.Rows) != len(b.Rows) {
		return false
	}
	rows := map[string]ProjectMetadataRow{}
	for _, row := range a.Rows {
		if _, duplicate := rows[row.RowID]; duplicate {
			return false
		}
		rows[row.RowID] = row
	}
	for _, row := range b.Rows {
		prior, present := rows[row.RowID]
		if !present || !reflect.DeepEqual(prior, row) {
			return false
		}
		delete(rows, row.RowID)
	}
	return len(rows) == 0
}

type profileRulesCommittedError struct{ cause error }

func (err *profileRulesCommittedError) Error() string {
	return fmt.Sprintf("Profile rule changes committed, but writer cleanup failed; reload before any retry: %v", err.cause)
}

func (err *profileRulesCommittedError) Unwrap() error { return err.cause }

func (c *sqliteContext) saveProfileRuleDrafts(ctx context.Context, original ProjectMetadataTable, drafts []profileRuleDraft, user string) error {
	planned, assignments, err := prepareProfileRuleDrafts(original, drafts)
	if err != nil {
		return err
	}
	return c.mutateProfileRules(ctx, original, user, func(tx *sql.Tx, table string, fresh ProjectMetadataTable) (*ProjectMetadataTable, error) {
		if len(assignments) == 0 {
			return nil, nil
		}
		for _, assignment := range assignments {
			result, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(table)+` SET `+
				quoteHeaderIdentifier(assignment.column)+`=? WHERE rowid=?`, assignment.value, assignment.rowID)
			if err != nil {
				return nil, err
			}
			if count, err := result.RowsAffected(); err != nil || count != 1 {
				return nil, errors.Join(fmt.Errorf("profile mutation expected one physical row, found %d", count), err)
			}
		}
		return &planned, nil
	}, nil)
}

func (c *sqliteContext) mutateProfileRules(ctx context.Context, original ProjectMetadataTable, user string,
	mutation func(*sql.Tx, string, ProjectMetadataTable) (*ProjectMetadataTable, error),
	finalCheck func(*sql.Tx) error) error {
	if _, _, err := prepareProfileRuleDrafts(original, nil); err != nil {
		return err
	}
	if err := validateChildPhysicalText("Profile edit user", user, 255); err != nil {
		return err
	}
	committed := false
	err := c.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
		}()
		tableName := c.selection.Project + "_Profile"
		fresh, err := readSQLiteStorageRows(ctx, tx, "main", tableName, "", nil, "Order")
		if err != nil {
			return err
		}
		if !sameProfileStorageRows(original, fresh) {
			return errors.New("profile rules/schema/counts changed since review; reload before saving")
		}
		planned, err := mutation(tx, tableName, fresh)
		if err != nil {
			return err
		}
		if planned == nil {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ProfileHistory"(
			"ID" INTEGER PRIMARY KEY AUTOINCREMENT,
			"ProfileTable" TEXT NOT NULL,
			"User" TEXT NOT NULL,
			"EditWhen" TEXT NOT NULL,
			"BeforeRules" TEXT NOT NULL,
			"AfterRules" TEXT NOT NULL)`); err != nil {
			return err
		}
		if err := validateProfileHistorySchema(ctx, tx); err != nil {
			return err
		}
		observed, err := readSQLiteStorageRows(ctx, tx, "main", tableName, "", nil, "Order")
		if err != nil {
			return err
		}
		if !sameProfileStorageRows(*planned, observed) {
			return errors.New("profile final stored rules differ from the complete plan; changes/history rolled back")
		}
		before, err := json.Marshal(original)
		if err != nil {
			return err
		}
		after, err := json.Marshal(observed)
		if err != nil {
			return err
		}
		editWhen := time.Now().Format("2006-01-02 15:04:05")
		history, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ProfileHistory"
			("ProfileTable","User","EditWhen","BeforeRules","AfterRules") VALUES(?,?,?,?,?)`,
			tableName, user, editWhen, string(before), string(after))
		if err != nil {
			return err
		}
		if count, err := history.RowsAffected(); err != nil || count != 1 {
			return errors.Join(fmt.Errorf("profile history expected one inserted event, found %d", count), err)
		}
		historyID, err := history.LastInsertId()
		if err != nil {
			return err
		}
		var storedTable, storedUser, storedWhen, storedBefore, storedAfter string
		if err := tx.QueryRowContext(ctx, `SELECT ProfileTable,User,EditWhen,BeforeRules,AfterRules
			FROM "__VPRO_ProfileHistory" WHERE ID=?`, historyID).
			Scan(&storedTable, &storedUser, &storedWhen, &storedBefore, &storedAfter); err != nil {
			return err
		}
		if storedTable != tableName || storedUser != user || storedWhen != editWhen ||
			storedBefore != string(before) || storedAfter != string(after) {
			return errors.New("profile stored history differs from its typed plan; mutation/history rolled back")
		}
		final, err := readSQLiteStorageRows(ctx, tx, "main", tableName, "", nil, "Order")
		if err != nil {
			return err
		}

		if !sameProfileStorageRows(*planned, final) {
			return errors.New("profile history insertion changed observed rules; mutation/history rolled back")
		}
		if finalCheck != nil {
			if err := finalCheck(tx); err != nil {
				return err
			}
		}
		if err := c.validateMetadataWriterFiles(); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil && committed {
		return &profileRulesCommittedError{cause: err}
	}
	return err
}

func validateProfileHistorySchema(ctx context.Context, tx *sql.Tx) (resultErr error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info("__VPRO_ProfileHistory")`)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	expected := []string{"ID", "ProfileTable", "User", "EditWhen", "BeforeRules", "AfterRules"}
	index := 0
	for rows.Next() {
		var cid, required, primary int
		var name, declared string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &declared, &required, &defaultValue, &primary); err != nil {
			return err
		}
		wantType, wantRequired, wantPrimary := "TEXT", 1, 0
		if index == 0 {
			wantType, wantRequired, wantPrimary = "INTEGER", 0, 1
		}
		if index >= len(expected) || cid != index || name != expected[index] || declared != wantType ||
			required != wantRequired || primary != wantPrimary || defaultValue.Valid {
			return errors.New("profile technical history schema is incompatible; no existing history was overwritten")
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if index != len(expected) {
		return errors.New("profile technical history schema is incomplete")
	}
	return nil
}
