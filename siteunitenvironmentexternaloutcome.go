package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
)

type externalSUEnvironmentCommitUncertainError struct{ error }

func (err *externalSUEnvironmentCommitUncertainError) Unwrap() error { return err.error }

type externalSUEnvironmentNotCommittedError struct{ error }

func (err *externalSUEnvironmentNotCommittedError) Unwrap() error { return err.error }

func externalSUEnvironmentProposal(source *externalSUEnvironmentSource, attempt *externalSUEnvironmentAttemptIdentity) ([]byte, error) {
	return json.Marshal(struct {
		Request   SiteUnitEnvironmentTransfer                `json:"request"`
		Selection desktopSelection                           `json:"selection"`
		Witness   map[string]map[string]ProjectMetadataTable `json:"witness"`
		User      string                                     `json:"user"`
		Paths     map[string]string                          `json:"paths"`
		Attempt   *externalSUEnvironmentAttemptIdentity      `json:"attempt,omitempty"`
	}{SiteUnitEnvironmentTransfer{source.Review, true}, source.Selection, source.Witness, source.User, source.Paths, attempt})
}

func validateExternalSUEnvironmentReceiptFile(source *externalSUEnvironmentSource) error {
	path, expected := source.Paths["project"], source.Files["project"]
	if path == "" || expected == nil {
		return errors.New("external SU/environment recovery lacks its original project path and opened identity")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("external SU/environment recovery project unavailable: %w", err)
	}
	info, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		return err
	}
	if !os.SameFile(expected, info) {
		return errors.New("external SU/environment recovery project was replaced; outcome remains unknown")
	}
	return nil
}

func lookupExternalSUEnvironmentReceipt(ctx context.Context, source *externalSUEnvironmentSource,
	identity externalSUEnvironmentAttemptIdentity) (result *SiteUnitEnvironmentResult, resultErr error) {
	if err := identity.validate(); err != nil {
		return nil, err
	}
	if err := validateExternalSUEnvironmentReceiptFile(source); err != nil {
		return nil, err
	}
	expected, err := externalSUEnvironmentProposal(source, &identity)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.Paths["project"], "ro")+"&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	tx, err := db.BeginTx(context.WithoutCancel(ctx), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() {
		err := tx.Rollback()
		if errors.Is(err, sql.ErrTxDone) {
			err = nil
		}
		resultErr = errors.Join(resultErr, err)
		if resultErr != nil {
			result = nil
		}
	}()
	var objects int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE=?`,
		siteUnitEnvironmentHistory).Scan(&objects); err != nil {
		return nil, err
	}
	if objects != 0 {
		if err := verifyTechnicalProvenance(ctx, tx, siteUnitEnvironmentHistory, siteUnitEnvironmentHistorySQL,
			"external SU/environment recovery"); err != nil {
			return nil, err
		}
		history, err := readSQLiteLiteralTextRows(ctx, tx, "main", siteUnitEnvironmentHistory,
			"Proposal", string(expected), "")
		if err != nil {
			return nil, err
		}
		if len(history.Rows) > 1 {
			return nil, errors.New("external SU/environment recovery found duplicate exact ownership history; outcome remains unknown")
		}
		if len(history.Rows) == 1 {
			columns, err := siteUnitTransferColumns(history, "ID", "Proposal")
			if err != nil {
				return nil, err
			}
			id := history.Rows[0].Cells[columns["ID"]]
			if id.Storage != "integer" || id.Integer == nil {
				return nil, errors.New("external SU/environment recovered history lacks its positive integer identity")
			}
			if value, err := strconv.ParseInt(*id.Integer, 10, 64); err != nil || value < 1 {
				return nil, errors.Join(err, errors.New("external SU/environment recovered history has an invalid identity"))
			}
			rows := make(map[string]bool)
			for _, change := range source.Review.Changes {
				rows[change.AdminRowID] = true
			}
			result = &SiteUnitEnvironmentResult{ChangedRows: len(rows), ChangedCells: len(source.Review.Changes), HistoryID: *id.Integer}
		}
	}
	if err := validateExternalSUEnvironmentReceiptFile(source); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("external SU/environment exact ownership history is absent; an uncertain commit remains unknown, never replay")
	}
	return result, nil
}
