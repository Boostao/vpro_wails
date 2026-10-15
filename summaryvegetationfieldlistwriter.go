package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
)

const summaryFieldListReceiptTable = "__VPRO_SummaryFieldListReceipt"
const summaryFieldListReceiptSchema = `CREATE TABLE "__VPRO_SummaryFieldListReceipt" (RequestID TEXT NOT NULL PRIMARY KEY COLLATE BINARY, Version INTEGER NOT NULL CHECK(Version=1), Request TEXT NOT NULL, Plan TEXT NOT NULL)`

type summaryFieldListWriterOutcome struct {
	RequestID string
	Changed   bool
	Committed bool
	Replayed  bool
	Verified  bool
	Plan      *summaryVegetationFieldListPlan
}

type summaryFieldListWriterHooks struct {
	configure    func(*sql.Conn) error
	beforeCommit func() error
	afterClose   func() error
}

// This receipt is technical retry safety, not a source audit or a public Save API.
func commitSummaryVegetationFieldList(ctx context.Context, service *SummaryVegetationFieldListService,
	requestJSON string, hooks summaryFieldListWriterHooks) (summaryFieldListWriterOutcome, error) {
	if err := service.require(ctx, ""); err != nil {
		return summaryFieldListWriterOutcome{}, err
	}
	var request SummaryVegetationFieldListRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return summaryFieldListWriterOutcome{}, err
	}
	canonical, err := json.Marshal(request)
	if err != nil {
		return summaryFieldListWriterOutcome{}, err
	}
	return withContextPlotRequest(ctx, service.contexts, request.Review.ContextID,
		func(plots *PlotService) (outcome summaryFieldListWriterOutcome, resultErr error) {
			owner := plots.projects.sqlite
			if err := acquireMutexLease(ctx, &owner.mu); err != nil {
				return outcome, err
			}
			defer owner.mu.Unlock()
			actual := SummaryVegetationFieldListReview{
				ContextID: request.Review.ContextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, SourcePath: owner.attachments["VPro64"],
				Source: summaryVegetationFieldListSource, Physical: request.Review.Physical,
			}
			if owner.conn == nil || !reflect.DeepEqual(actual, request.Review) {
				return outcome, errors.New("Summary field-list request does not match actual owned context provenance")
			}
			files, err := openSummaryFieldListOwnedFiles(owner)
			if err != nil {
				return outcome, err
			}
			defer func() {
				for _, file := range files {
					resultErr = errors.Join(resultErr, file.Close())
				}
			}()
			db, err := sql.Open("sqlite3", "file::memory:?cache=private&_txlock=immediate&_busy_timeout=5000&_foreign_keys=on")
			if err != nil {
				return outcome, err
			}
			db.SetMaxOpenConns(1)
			defer func() {
				resultErr = errors.Join(resultErr, db.Close())
				if hooks.afterClose != nil {
					resultErr = errors.Join(resultErr, hooks.afterClose())
				}
			}()
			conn, err := db.Conn(ctx)
			if err != nil {
				return outcome, err
			}
			defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS VPro64`, sqliteFileURI(actual.SourcePath, "rw")); err != nil {
				return outcome, err
			}
			if err := checkSummaryFieldListOwnedFiles(owner, files); err != nil {
				return outcome, err
			}
			if hooks.configure != nil {
				if err := hooks.configure(conn); err != nil {
					return outcome, err
				}
			}
			// Unlike the pinned reader, this coordinator is disposable on cancellation.
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return outcome, err
			}
			finished := false
			defer func() {
				if !finished {
					rollbackErr := tx.Rollback()
					if errors.Is(rollbackErr, sql.ErrTxDone) && ctx.Err() != nil {
						rollbackErr = nil // database/sql already rolled back the cancelled disposable transaction.
					}
					resultErr = errors.Join(resultErr, rollbackErr)
				}
			}()
			if err := ctx.Err(); err != nil {
				return outcome, err
			}
			outcome.RequestID = request.RequestID
			recorded, exists, err := readSummaryFieldListReceipt(ctx, tx, request.RequestID, string(canonical))
			if err != nil {
				return outcome, err
			}
			if exists {
				outcome.Changed, outcome.Committed, outcome.Replayed, outcome.Plan = true, true, true, recorded
				if err := observeSummaryFieldListPlan(ctx, tx, recorded.Proposed); err != nil {
					return outcome, err
				}
				if err := checkSummaryFieldListOwnedFiles(owner, files); err != nil {
					return outcome, err
				}
				outcome.Verified = true
				return outcome, nil
			}
			plan, err := applySummaryVegetationFieldList(ctx, tx, request.Review.Physical, request.plannedActions())
			if err != nil {
				return outcome, err
			}
			if len(plan.Assignments) == 0 {
				if err := checkSummaryFieldListOwnedFiles(owner, files); err != nil {
					return outcome, err
				}
				outcome.Plan, outcome.Verified = plan, true
				return outcome, nil
			}
			planJSON, err := json.Marshal(plan)
			if err != nil {
				return outcome, err
			}
			present, err := summaryFieldListReceiptPresent(ctx, tx)
			if err != nil {
				return outcome, err
			}
			if !present {
				if _, err := tx.ExecContext(ctx, `CREATE TABLE VPro64.`+summaryFieldListReceiptSchema[len("CREATE TABLE "):]); err != nil {
					return outcome, err
				}
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO VPro64."__VPRO_SummaryFieldListReceipt" (RequestID,Version,Request,Plan) VALUES (?,1,?,?)`,
				request.RequestID, string(canonical), string(planJSON)); err != nil {
				return outcome, fmt.Errorf("Summary field-list technical receipt insertion failed: %w", err)
			}
			recorded, exists, err = readSummaryFieldListReceipt(ctx, tx, request.RequestID, string(canonical))
			if err != nil || !exists || !reflect.DeepEqual(recorded, plan) {
				return outcome, errors.Join(errors.New("Summary field-list inserted receipt differs from complete planned source"), err)
			}
			if err := observeSummaryFieldListPlan(ctx, tx, plan.Proposed); err != nil {
				return outcome, err
			}
			if hooks.beforeCommit != nil {
				if err := hooks.beforeCommit(); err != nil {
					return outcome, err
				}
			}
			if err := checkSummaryFieldListOwnedFiles(owner, files); err != nil {
				return outcome, err
			}
			if err := ctx.Err(); err != nil {
				return outcome, err
			}
			if err := tx.Commit(); err != nil {
				finished = true // database/sql has completed its transaction on a Commit error.
				return outcome, err
			}
			finished = true
			outcome.Changed, outcome.Committed, outcome.Verified, outcome.Plan = true, true, true, plan
			return outcome, nil
		})
}

func openSummaryFieldListOwnedFiles(owner *sqliteContext) (map[string]*os.File, error) {
	if owner.attachmentInfo["VPro64"] == nil || len(owner.attachments) != len(owner.attachmentInfo) {
		return nil, errors.New("Summary field-list requires complete opened attachment identities")
	}
	for role := range owner.attachments {
		if owner.attachmentInfo[role] == nil {
			return nil, errors.New("Summary field-list attachment identity is absent")
		}
	}
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	files := make(map[string]*os.File, len(owner.attachments))
	for role, path := range owner.attachments {
		file, err := os.Open(path)
		if err == nil {
			files[role] = file
		}
		if err != nil {
			for _, opened := range files {
				err = errors.Join(err, opened.Close())
			}
			return nil, err
		}
	}
	if err := checkSummaryFieldListOwnedFiles(owner, files); err != nil {
		for _, opened := range files {
			err = errors.Join(err, opened.Close())
		}
		return nil, err
	}
	return files, nil
}

func checkSummaryFieldListOwnedFiles(owner *sqliteContext, files map[string]*os.File) error {
	if err := profileOwnedFiles(owner); err != nil {
		return err
	}
	for role, file := range files {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		if !os.SameFile(info, owner.attachmentInfo[role]) {
			return fmt.Errorf("Summary field-list %s opened identity changed", role)
		}
		if role != "VPro64" && os.SameFile(info, owner.attachmentInfo["VPro64"]) {
			return fmt.Errorf("Summary field-list VPro64 is also attached as %s; shared-role writes refused", role)
		}
	}
	return nil
}

func summaryFieldListReceiptPresent(ctx context.Context, tx *sql.Tx) (bool, error) {
	var kind, schema string
	err := tx.QueryRowContext(ctx, `SELECT type,sql FROM VPro64.sqlite_master WHERE name COLLATE BINARY=?`,
		summaryFieldListReceiptTable).Scan(&kind, &schema)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if kind != "table" || schema != summaryFieldListReceiptSchema {
		return false, errors.New("Summary field-list technical receipt schema is unavailable or altered")
	}
	return true, nil
}

func readSummaryFieldListReceipt(ctx context.Context, tx *sql.Tx, id, canonical string) (*summaryVegetationFieldListPlan, bool, error) {
	present, err := summaryFieldListReceiptPresent(ctx, tx)
	if err != nil || !present {
		return nil, false, err
	}
	var matches int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM VPro64."__VPRO_SummaryFieldListReceipt"
		WHERE CAST(RequestID AS BLOB)=CAST(? AS BLOB)`, id).Scan(&matches); err != nil {
		return nil, false, err
	}
	if matches == 0 {
		return nil, false, nil
	}
	if matches != 1 {
		return nil, false, errors.New("Summary field-list technical receipt UUID has ambiguous physical storage")
	}
	var version int64
	var storedRequest, storedPlan, versionType, requestType, planType, idType string
	err = tx.QueryRowContext(ctx, `SELECT Version,Request,Plan,typeof(Version),typeof(Request),typeof(Plan),typeof(RequestID)
		FROM VPro64."__VPRO_SummaryFieldListReceipt" WHERE CAST(RequestID AS BLOB)=CAST(? AS BLOB)`,
		id).Scan(&version, &storedRequest, &storedPlan, &versionType, &requestType, &planType, &idType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if version != 1 || versionType != "integer" || requestType != "text" || planType != "text" || idType != "text" {
		return nil, false, errors.New("Summary field-list technical receipt requires exact typed version/request/plan")
	}
	var request SummaryVegetationFieldListRequest
	if err := json.Unmarshal([]byte(storedRequest), &request); err != nil {
		return nil, false, fmt.Errorf("Summary field-list persisted request invalid: %w", err)
	}
	normalized, err := json.Marshal(request)
	if err != nil || string(normalized) != storedRequest || request.RequestID != id {
		return nil, false, errors.Join(errors.New("Summary field-list persisted request is not canonical"), err)
	}
	if storedRequest != canonical {
		return nil, false, errors.New("Summary field-list stable UUID collision with a different canonical request")
	}
	plan, err := planSummaryVegetationFieldList(ctx, request.Review.Physical, request.Review.Physical, request.plannedActions())
	if err != nil {
		return nil, false, err
	}
	encoded, err := json.Marshal(plan)
	if err != nil || string(encoded) != storedPlan || len(plan.Assignments) == 0 {
		return nil, false, errors.Join(errors.New("Summary field-list persisted full source plan is invalid"), err)
	}
	return plan, true, nil
}

func observeSummaryFieldListPlan(ctx context.Context, tx *sql.Tx, proposed ProjectMetadataTable) error {
	current, err := readSummaryVegetationFieldList(ctx, tx)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, proposed) {
		return errors.New("Summary field-list current complete source differs from recorded committed plan")
	}
	return ctx.Err()
}
