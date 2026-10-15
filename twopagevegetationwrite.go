package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

func twoPageVegetationHistoryObject(ctx context.Context, tx *sql.Tx, alias string) (bool, error) {
	var kind, name string
	err := tx.QueryRowContext(ctx, `SELECT type,name FROM `+quoteHeaderIdentifier(alias)+
		`.sqlite_master WHERE name COLLATE NOCASE=?`, twoPageVegetationHistoryTable).Scan(&kind, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if kind != "table" || name != twoPageVegetationHistoryTable {
		return false, errors.New("two-page vegetation receipt requires its exact original physical history table")
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, alias, twoPageVegetationHistoryTable, "two-page vegetation receipt"); err != nil {
		return false, err
	}
	return true, nil
}

func (s *ContextService) lookupTwoPageVegetationReceipt(ctx context.Context, contextID string, request twoPageVegetationRequest) (*twoPageVegetationWriteResult, error) {
	detached, err := detachTwoPageVegetationRequest(ctx, s, request)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageVegetationWriteResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageVegetationWriteResult, error) {
			if owner.selection.Project != detached.Original.Project {
				return nil, errors.New("two-page vegetation receipt belongs to another current project")
			}
			var selected bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv
				WHERE typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB))`,
				detached.Original.Plot).Scan(&selected); err != nil {
				return nil, fmt.Errorf("two-page vegetation receipt selected parent: %w", err)
			}
			if !selected {
				return nil, errors.New("two-page vegetation receipt parent is outside the selected context")
			}
			present, err := twoPageVegetationHistoryObject(ctx, tx, "project")
			if err != nil || !present {
				return nil, err
			}
			table, err := readSQLiteStorageRows(ctx, tx, "project", twoPageVegetationHistoryTable, "", nil, "")
			if err != nil {
				return nil, err
			}
			history, err := readTwoPageVegetationReceipt(ctx, detached, table)
			if err != nil || history == nil {
				return nil, err
			}
			return &twoPageVegetationWriteResult{History: *history, Replayed: true}, nil
		})
	})
}

func (s *ContextService) writeTwoPageVegetation(ctx context.Context, contextID string, request twoPageVegetationRequest) (*twoPageVegetationWriteResult, error) {
	return s.writeTwoPageVegetationObserved(ctx, contextID, request, nil, nil)
}

func (s *ContextService) writeTwoPageVegetationObserved(ctx context.Context, contextID string, request twoPageVegetationRequest,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*twoPageVegetationWriteResult, error) {
	detached, err := detachTwoPageVegetationRequest(ctx, s, request)
	if err != nil {
		return nil, err
	}
	request = detached
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageVegetationWriteResult, error) {
		if err := plots.requireContextEdit(); err != nil {
			return nil, err
		}
		owner := plots.projects.sqlite
		var result *twoPageVegetationWriteResult
		committed := false
		err := owner.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			original := request.Original
			if original.Project != owner.selection.Project {
				return errors.New("two-page vegetation request belongs to another current project")
			}
			if err := prepareTwoPageVegetationSpeciesListWriter(ctx, conn, owner, request); err != nil {
				return err
			}
			suAlias, err := siviParentWriterSU(ctx, conn, owner)
			if err != nil {
				return fmt.Errorf("two-page vegetation selected SU attachment: %w", err)
			}
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}()
			if suAlias == "sivi_su" {
				if err := requireRollbackJournalReadLock(ctx, tx, suAlias, "two-page vegetation selected SU observations"); err != nil {
					return err
				}
			}
			if err := twoPageChildWriterSelectedParent(ctx, tx, owner, suAlias, original.Plot, "vegetation"); err != nil {
				return err
			}
			for _, table := range []string{original.Project + "_Env", original.Project + "_Admin", original.Table} {
				var physical bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.sqlite_master
					WHERE type='table' AND name COLLATE BINARY=?)`, table).Scan(&physical); err != nil {
					return err
				}
				if !physical {
					return errors.New("two-page vegetation mutation requires original physical project tables")
				}
				if err := validateSIVIPhysicalSchema(ctx, tx, "main", table, "two-page vegetation mutation"); err != nil {
					return err
				}
			}
			if _, err := twoPageVegetationHistoryObject(ctx, tx, "main"); err != nil {
				return err
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			replay, err := readTwoPageVegetationReceipt(ctx, request, before[twoPageVegetationHistoryTable])
			if err != nil {
				return err
			}
			if replay != nil {
				result = &twoPageVegetationWriteResult{History: *replay, Replayed: true}
				return nil
			}
			if original.ContextID != contextID || plots.currentUser == "" || plots.auditStrength < 0 || plots.auditStrength > 3 {
				return errors.New("two-page vegetation new request requires current context and explicit valid audit provenance")
			}
			for _, parent := range []struct{ table, column string }{{original.Project + "_Env", "PlotNumber"}, {original.Project + "_Admin", "Plot"}} {
				table, present := before[parent.table]
				if !present {
					return errors.New("two-page vegetation original parent requires a physical project table")
				}
				columns, err := siteUnitTransferColumns(table, parent.column)
				if err != nil {
					return err
				}
				count := 0
				for _, row := range table.Rows {
					cell := row.Cells[columns[parent.column]]
					if cell.Storage == "text" && cell.Text != nil && *cell.Text == original.Plot {
						count++
					}
				}
				if count != 1 {
					return errors.New("two-page vegetation mutation requires one exact physical parent")
				}
			}
			veg, present := before[original.Table]
			if !present {
				return errors.New("two-page vegetation mutation requires a physical vegetation table")
			}
			fresh, err := readSQLiteLiteralTextRows(ctx, tx, "main", original.Table, "PlotNumber", original.Plot, "")
			if err != nil || !twoPageVegetationTableEqual(fresh, original.Records) {
				return errors.Join(err, errors.New("two-page vegetation complete original/schema changed; cancel and reload"))
			}
			if err := observeTwoPageVegetationSpeciesListWriter(ctx, tx, request); err != nil {
				return err
			}
			plan, err := planTwoPageVegetationRequest(ctx, request, true)
			if err != nil || len(plan) == 0 {
				return errors.Join(err, errors.New("two-page vegetation mutation requires a nonempty changed source plan"))
			}
			history := twoPageVegetationHistory{Request: request, Committed: twoPageVegetationPlannedTable(original.Records, plan),
				Actor: plots.currentUser, AuditStrength: plots.auditStrength,
				EditWhen: time.Now().Format("2006-01-02 15:04:05"), Audits: []AuditEntry{}}
			for _, assignment := range plan {
				updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(original.Table)+` SET `+quoteHeaderIdentifier(assignment.Column)+
					`=? WHERE rowid=? AND typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB)`,
					assignment.Value, assignment.RowID, original.Plot)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("two-page vegetation update did not affect exactly one original physical row"))
				}
				oldValue, err := metadataCellValue(assignment.Before)
				if err != nil {
					return err
				}
				newValue, err := metadataCellValue(assignment.After)
				if err != nil {
					return err
				}
				audits, err := auditChildFieldsTracked(tx, original.Project, "Veg", original.Plot, assignment.ID,
					[]childField{{column: assignment.Column}}, []any{oldValue}, []any{newValue},
					history.Actor, history.AuditStrength, history.EditWhen)
				if err != nil {
					return err
				}
				for _, audit := range audits {
					ids, err := auditRowIDs([]string{audit.RowID})
					if err != nil {
						return err
					}
					records, err := selectedAuditEntries(tx, original.Project, original.Plot, ids)
					if err != nil || len(records) != 1 {
						return errors.Join(err, errors.New("two-page vegetation audit identity differs"))
					}
					history.Audits = append(history.Audits, records[0])
				}
			}
			auditPlan, err := appendSIVIPlannedAudits(before[original.Project+"_Audit"], history.Audits)
			if err != nil {
				return err
			}
			expected := map[string]ProjectMetadataTable{original.Table: twoPageVegetationPlannedTable(veg, plan), original.Project + "_Audit": auditPlan}
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("two-page vegetation transaction differs from its complete plan: %w", err)
			}
			if !twoPageVegetationTableEqual(after[original.Table], expected[original.Table]) {
				return errors.New("two-page vegetation transaction changed exact REAL bits")
			}
			if err := validateTwoPageVegetationHistory(ctx, history); err != nil {
				return err
			}
			proposal, err := json.Marshal(history)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_TwoPageVegetationHistory"(
				RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
				return err
			}
			inserted, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_TwoPageVegetationHistory" VALUES(?,?,?)`,
				request.RequestID, history.EditWhen, string(proposal))
			if err != nil {
				return err
			}
			if count, err := inserted.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("two-page vegetation history did not append exactly one request"))
			}
			rowID, err := inserted.LastInsertId()
			if err != nil {
				return err
			}
			historyPlan := before[twoPageVegetationHistoryTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]ProjectMetadataRow{}, historyPlan.Rows...), ProjectMetadataRow{
				RowID: strconv.FormatInt(rowID, 10), Cells: []ProjectMetadataCell{
					siviCreationText(request.RequestID), siviCreationText(history.EditWhen), siviCreationText(string(proposal))}})
			after, err = environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(before, after, expected, twoPageVegetationHistoryTable); err != nil {
				return fmt.Errorf("two-page vegetation provenance transaction differs: %w", err)
			}
			if !reflect.DeepEqual(after[twoPageVegetationHistoryTable], historyPlan) {
				return errors.New("two-page vegetation durable history differs from its complete plan")
			}
			if !twoPageVegetationTableEqual(after[original.Table], expected[original.Table]) {
				return errors.New("two-page vegetation provenance transaction changed exact REAL bits")
			}
			verified, err := readTwoPageVegetationReceipt(ctx, request, after[twoPageVegetationHistoryTable])
			if err != nil || verified == nil {
				return errors.Join(err, errors.New("two-page vegetation durable receipt verification failed"))
			}
			if beforeCommit != nil {
				if err := beforeCommit(tx); err != nil {
					return err
				}
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return err
			}
			if err := twoPageChildWriterSelectedParent(ctx, tx, owner, suAlias, original.Plot, "vegetation"); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			committed = true
			result = &twoPageVegetationWriteResult{History: *verified, DidCommit: true}
			if afterCommit != nil {
				return afterCommit(result)
			}
			return nil
		})
		if err != nil {
			if committed {
				return result, fmt.Errorf("two-page vegetation edit committed but completion failed; recover the retained request before retrying: %w", err)
			}
			return nil, errors.Join(err, ctx.Err())
		}
		return result, nil
	})
}
