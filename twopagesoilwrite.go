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
	"unicode/utf8"
)

const twoPageSoilHistoryTable = "__VPRO_TwoPageSoilHistory"

type twoPageSoilRequest struct {
	RequestID string               `json:"requestId"`
	Original  *twoPageSoilOriginal `json:"original"`
	Edits     []twoPageSoilEdit    `json:"edits"`
	Horizon   *string              `json:"horizon,omitempty"`
}

func (request *twoPageSoilRequest) UnmarshalJSON(data []byte) error {
	properties, err := sourceChildJSONObject(data, "two-page soil request", []string{"requestId", "original", "edits"}, []string{"horizon"})
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "two-page soil request", "requestId", "original", "edits"); err != nil {
		return err
	}
	if _, present := properties["horizon"]; present {
		if err := sourceChildJSONNonNull(properties, "soil creation request", "horizon"); err != nil {
			return err
		}
	}
	original, err := sourceChildJSONObject(properties["original"], "two-page soil original",
		[]string{"contextId", "project", "plot", "form", "recordSource", "table", "records"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(original, "two-page soil original",
		"contextId", "project", "plot", "form", "recordSource", "table", "records"); err != nil {
		return err
	}
	records, err := sourceChildJSONObject(original["records"], "two-page soil records", []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildArrayJSON(records["columns"], func(raw json.RawMessage) error {
		column, err := sourceChildJSONObject(raw, "two-page soil column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(column, "two-page soil column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceChildRowsJSON(records["rows"], "two-page soil original"); err != nil {
		return err
	}
	type plain twoPageSoilRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !siviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("two-page soil request requires a stable lowercase UUIDv4")
	}
	if err := validateTwoPageSoilRequestPlan(context.Background(), twoPageSoilRequest(decoded)); err != nil {
		return err
	}
	*request = twoPageSoilRequest(decoded)
	return nil
}

type twoPageSoilHistory struct {
	Request       twoPageSoilRequest
	Committed     ProjectMetadataTable
	Actor         string
	AuditStrength int
	EditWhen      string
	Audits        []AuditEntry
	CreatedRowID  *string `json:"CreatedRowID,omitempty"`
}

type twoPageSoilWriteResult struct {
	History   twoPageSoilHistory `json:"history"`
	DidCommit bool               `json:"didCommit"`
	Replayed  bool               `json:"replayed"`
}

func twoPageSoilPlannedTable(original ProjectMetadataTable, assignments []twoPageSoilAssignment) ProjectMetadataTable {
	planned := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn{}, original.Columns...), Rows: []ProjectMetadataRow{}}
	for _, row := range original.Rows {
		copy := ProjectMetadataRow{RowID: row.RowID, Cells: make([]ProjectMetadataCell, len(row.Cells))}
		for i, cell := range row.Cells {
			copy.Cells[i] = cloneSiteUnitCell(cell)
			for _, assignment := range assignments {
				if row.RowID == assignment.RowID && original.Columns[i].Name == assignment.Column {
					copy.Cells[i] = cloneSiteUnitCell(assignment.After)
				}
			}
		}
		planned.Rows = append(planned.Rows, copy)
	}
	return planned
}

func validateTwoPageSoilHistory(ctx context.Context, history twoPageSoilHistory) error {
	if history.Request.Horizon != nil {
		ordinary, err := twoPageSoilCreationEditHistory(ctx, history)
		if err != nil {
			return err
		}
		return validateTwoPageSoilHistory(ctx, ordinary)
	}
	if history.CreatedRowID != nil {
		return errors.New("existing-row soil history cannot contain creation authority")
	}
	plan, err := planTwoPageSoilEdits(ctx, history.Request.Original, history.Request.Edits)
	if err != nil || len(plan) == 0 || !siviDeletionUUID.MatchString(history.Request.RequestID) {
		return errors.Join(err, errors.New("two-page soil history requires a complete changed source plan"))
	}
	if history.Actor == "" || history.AuditStrength < 0 || history.AuditStrength > 3 {
		return errors.New("two-page soil history has unavailable audit provenance")
	}
	if err := validateChildPhysicalText("two-page soil actor", history.Actor, 100); err != nil {
		return err
	}
	if _, err := time.Parse("2006-01-02 15:04:05", history.EditWhen); err != nil {
		return fmt.Errorf("two-page soil history time: %w", err)
	}
	if !reflect.DeepEqual(history.Committed, twoPageSoilPlannedTable(history.Request.Original.Records, plan)) {
		return errors.New("two-page soil history changed unrelated source cells")
	}
	audited := 0
	seen := map[string]bool{}
	for _, assignment := range plan {
		before, err := metadataCellValue(assignment.Before)
		if err != nil {
			return err
		}
		after, err := metadataCellValue(assignment.After)
		if err != nil {
			return err
		}
		if !auditHeaderChange(before, after, history.AuditStrength) {
			continue
		}
		if audited >= len(history.Audits) {
			return errors.New("two-page soil history lost an audited change")
		}
		audit := history.Audits[audited]
		audited++
		rowID, err := strconv.ParseInt(audit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(rowID, 10) != audit.RowID || seen[audit.RowID] {
			return errors.New("two-page soil history requires distinct canonical audit rowids")
		}
		seen[audit.RowID] = true
		original := history.Request.Original
		kind := "Humus"
		if original.Form == "SoilMineralXL" {
			kind = "Mineral"
		}
		if audit.Project != original.Project || audit.PlotNumber != original.Plot || audit.User != history.Actor ||
			audit.Table != "_"+kind || audit.EditField != assignment.Column || audit.EditWhen != history.EditWhen ||
			audit.ID == nil || *audit.ID != assignment.ID || audit.Restore || audit.Flag ||
			!reflect.DeepEqual(otherNullable(audit.BeforeEdit), headerAuditValue(before)) ||
			!reflect.DeepEqual(otherNullable(audit.AfterEdit), headerAuditValue(after)) {
			return errors.New("two-page soil history audit differs from its source assignment")
		}
	}
	if audited != len(history.Audits) {
		return errors.New("two-page soil history contains unplanned audits")
	}
	return ctx.Err()
}

func readTwoPageSoilReceipt(ctx context.Context, request twoPageSoilRequest, table ProjectMetadataTable) (*twoPageSoilHistory, error) {
	if table.Columns == nil {
		return nil, ctx.Err()
	}
	columns, err := siteUnitTransferColumns(table, "RequestID", "Created", "Proposal")
	want := []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
	if err != nil || !reflect.DeepEqual(table.Columns, want) {
		return nil, errors.Join(err, errors.New("two-page soil receipt schema differs"))
	}
	var matched *twoPageSoilHistory
	seen := map[string]bool{}
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, when, proposal := row.Cells[columns["RequestID"]], row.Cells[columns["Created"]], row.Cells[columns["Proposal"]]
		for _, cell := range []ProjectMetadataCell{id, when, proposal} {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("two-page soil receipt requires canonical TEXT"))
			}
		}
		if seen[*id.Text] {
			return nil, errors.New("two-page soil receipt repeats a request")
		}
		seen[*id.Text] = true
		raw := []byte(*proposal.Text)
		properties, err := sourceChildJSONObject(raw, "two-page soil history",
			[]string{"Request", "Committed", "Actor", "AuditStrength", "EditWhen", "Audits"}, []string{"CreatedRowID"})
		if err != nil {
			return nil, err
		}
		if _, present := properties["CreatedRowID"]; present {
			if err := sourceChildJSONNonNull(properties, "soil creation history", "CreatedRowID"); err != nil {
				return nil, err
			}
		}
		var history twoPageSoilHistory
		if err := json.Unmarshal(raw, &history); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != *proposal.Text || *id.Text != history.Request.RequestID || *when.Text != history.EditWhen {
			return nil, errors.Join(err, errors.New("two-page soil receipt differs from canonical provenance"))
		}
		if err := validateTwoPageSoilHistory(ctx, history); err != nil {
			return nil, err
		}
		if *id.Text == request.RequestID {
			if !reflect.DeepEqual(history.Request, request) {
				return nil, errors.New("two-page soil request UUID belongs to another exact source plan")
			}
			matched = &history
		}
	}
	return matched, ctx.Err()
}

func twoPageSoilSelectedParent(ctx context.Context, owner *sqliteContext, plot string) error {
	var selected bool
	if err := owner.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv AS e
		WHERE typeof(e.PlotNumber)='text' AND CAST(e.PlotNumber AS BLOB)=CAST(? AS BLOB))`, plot).Scan(&selected); err != nil {
		return fmt.Errorf("two-page soil selected parent: %w", err)
	}
	if !selected {
		return errors.New("two-page soil parent is outside the selected context")
	}
	return nil
}

func twoPageSoilWriterSelectedParent(ctx context.Context, tx *sql.Tx, owner *sqliteContext, suAlias, plot string) error {
	return twoPageChildWriterSelectedParent(ctx, tx, owner, suAlias, plot, "soil")
}

func (s *ContextService) lookupTwoPageSoilReceipt(ctx context.Context, contextID string, request twoPageSoilRequest) (*twoPageSoilWriteResult, error) {
	detached, err := detachTwoPageSoilRequest(ctx, s, request)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageSoilWriteResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageSoilWriteResult, error) {
			if owner.selection.Project != detached.Original.Project {
				return nil, errors.New("two-page soil receipt belongs to another current project")
			}
			if err := twoPageSoilSelectedParent(ctx, owner, detached.Original.Plot); err != nil {
				return nil, err
			}
			var kind, name string
			err := tx.QueryRowContext(ctx, `SELECT type,name FROM project.sqlite_master WHERE name COLLATE NOCASE=?`,
				twoPageSoilHistoryTable).Scan(&kind, &name)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ctx.Err()
			}
			if err != nil {
				return nil, err
			}
			if kind != "table" || name != twoPageSoilHistoryTable {
				return nil, errors.New("two-page soil receipt requires its exact original physical history table")
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", twoPageSoilHistoryTable, "two-page soil receipt"); err != nil {
				return nil, err
			}
			table, err := readSQLiteStorageRows(ctx, tx, "project", twoPageSoilHistoryTable, "", nil, "")
			if err != nil {
				return nil, err
			}
			history, err := readTwoPageSoilReceipt(ctx, detached, table)
			if err != nil || history == nil {
				return nil, err
			}
			return &twoPageSoilWriteResult{History: *history, Replayed: true}, nil
		})
	})
}

func detachTwoPageSoilRequest(ctx context.Context, service *ContextService, request twoPageSoilRequest) (twoPageSoilRequest, error) {
	if ctx == nil || service == nil {
		return twoPageSoilRequest{}, errors.New("two-page soil request requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return twoPageSoilRequest{}, err
	}
	if !siviDeletionUUID.MatchString(request.RequestID) {
		return twoPageSoilRequest{}, errors.New("two-page soil request requires a stable lowercase UUIDv4")
	}
	if err := validateTwoPageSoilRequestPlan(ctx, request); err != nil {
		return twoPageSoilRequest{}, err
	}
	for _, column := range request.Original.Records.Columns {
		if !utf8.ValidString(column.Name) || !utf8.ValidString(column.DeclaredType) {
			return twoPageSoilRequest{}, errors.New("two-page soil request schema requires valid Unicode")
		}
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return twoPageSoilRequest{}, err
	}
	var detached twoPageSoilRequest
	if err := json.Unmarshal(raw, &detached); err != nil {
		return twoPageSoilRequest{}, err
	}
	return detached, nil
}

func (s *ContextService) writeTwoPageSoil(ctx context.Context, contextID string, request twoPageSoilRequest) (*twoPageSoilWriteResult, error) {
	return s.writeTwoPageSoilObserved(ctx, contextID, request, nil, nil)
}

func (s *ContextService) writeTwoPageSoilObserved(ctx context.Context, contextID string, request twoPageSoilRequest,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageSoilWriteResult) error) (*twoPageSoilWriteResult, error) {
	detached, err := detachTwoPageSoilRequest(ctx, s, request)
	if err != nil {
		return nil, err
	}
	request = detached
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageSoilWriteResult, error) {
		if err := plots.requireContextEdit(); err != nil {
			return nil, err
		}
		owner := plots.projects.sqlite
		var result *twoPageSoilWriteResult
		committed := false
		err := owner.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			original := request.Original
			if original.Project != owner.selection.Project {
				return errors.New("two-page soil request belongs to another current project")
			}
			suAlias, err := siviParentWriterSU(ctx, conn, owner)
			if err != nil {
				return fmt.Errorf("two-page soil selected SU attachment: %w", err)
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
				if err := requireRollbackJournalReadLock(ctx, tx, suAlias, "two-page soil selected SU observations"); err != nil {
					return err
				}
			}
			if err := twoPageSoilWriterSelectedParent(ctx, tx, owner, suAlias, original.Plot); err != nil {
				return err
			}
			for _, table := range []string{original.Project + "_Env", original.Project + "_Admin", original.Table} {
				var physical bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&physical); err != nil {
					return err
				}
				if !physical {
					return errors.New("two-page soil mutation requires original physical project tables")
				}
				if err := validateSIVIPhysicalSchema(ctx, tx, "main", table, "two-page soil mutation"); err != nil {
					return err
				}
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if _, present := before[twoPageSoilHistoryTable]; present {
				if err := validateSIVIPhysicalSchema(ctx, tx, "main", twoPageSoilHistoryTable, "two-page soil receipt"); err != nil {
					return err
				}
			}
			replay, err := readTwoPageSoilReceipt(ctx, request, before[twoPageSoilHistoryTable])
			if err != nil {
				return err
			}
			if replay != nil {
				result = &twoPageSoilWriteResult{History: *replay, Replayed: true}
				return nil
			}
			if original.ContextID != contextID || plots.currentUser == "" || plots.auditStrength < 0 || plots.auditStrength > 3 {
				return errors.New("two-page soil new request requires current context and explicit valid audit provenance")
			}
			for _, parent := range []struct{ table, column string }{{original.Project + "_Env", "PlotNumber"}, {original.Project + "_Admin", "Plot"}} {
				table, present := before[parent.table]
				if !present {
					return errors.New("two-page soil original parent requires a physical project table")
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
					return errors.New("two-page soil mutation requires one exact physical parent")
				}
			}
			soil, present := before[original.Table]
			if !present {
				return errors.New("two-page soil mutation requires a physical soil table")
			}
			fresh, err := readSQLiteLiteralTextRows(ctx, tx, "main", original.Table, "PlotNumber", original.Plot, "")
			if err != nil || !reflect.DeepEqual(fresh, original.Records) {
				return errors.Join(err, errors.New("two-page soil complete original/schema changed; cancel and reload"))
			}
			edits := request.Edits
			var creation *twoPageSoilCreationPreparation
			if request.Horizon != nil {
				creation, err = prepareTwoPageSoilCreation(ctx, tx, request, before)
				if err != nil {
					return err
				}
				before, original, soil, edits = creation.Baseline, creation.Original, creation.Soil, creation.Edits
			}
			plan, err := planTwoPageSoilEdits(ctx, original, edits)
			if err != nil || len(plan) == 0 {
				return errors.Join(err, errors.New("two-page soil mutation requires a nonempty changed source plan"))
			}
			columns, err := siteUnitTransferColumns(soil, "ID", "PlotNumber")
			if err != nil {
				return err
			}
			history := twoPageSoilHistory{Request: request, Committed: twoPageSoilPlannedTable(original.Records, plan),
				Actor: plots.currentUser, AuditStrength: plots.auditStrength,
				EditWhen: time.Now().Format("2006-01-02 15:04:05"), Audits: []AuditEntry{}}
			if creation != nil {
				history.CreatedRowID = &creation.RowID
			}
			kind := "Humus"
			if original.Form == "SoilMineralXL" {
				kind = "Mineral"
			}
			for _, assignment := range plan {
				for _, peer := range soil.Rows {
					if peer.RowID != assignment.RowID && siviIdentityCellMatches(peer.Cells[columns["ID"]], strconv.FormatInt(assignment.ID, 10)) {
						return errors.New("two-page soil edited ID belongs to another global physical row")
					}
				}
				updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(original.Table)+` SET `+quoteHeaderIdentifier(assignment.Column)+
					`=? WHERE rowid=? AND typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB)`,
					assignment.Value, assignment.RowID, original.Plot)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("two-page soil update did not affect exactly one original physical row"))
				}
				oldValue, err := metadataCellValue(assignment.Before)
				if err != nil {
					return err
				}
				newValue, err := metadataCellValue(assignment.After)
				if err != nil {
					return err
				}
				audits, err := auditChildFieldsTracked(tx, original.Project, kind, original.Plot, assignment.ID,
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
						return errors.Join(err, errors.New("two-page soil audit identity differs"))
					}
					history.Audits = append(history.Audits, records[0])
				}
			}
			auditPlan, err := appendSIVIPlannedAudits(before[original.Project+"_Audit"], history.Audits)
			if err != nil {
				return err
			}
			expected := map[string]ProjectMetadataTable{original.Table: twoPageSoilPlannedTable(soil, plan), original.Project + "_Audit": auditPlan}
			if creation != nil {
				expected[siviIdentityLedger] = creation.Ledger
			}
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("two-page soil transaction differs from its complete plan: %w", err)
			}
			if err := validateTwoPageSoilHistory(ctx, history); err != nil {
				return err
			}
			proposal, err := json.Marshal(history)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_TwoPageSoilHistory" (
				RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
				return err
			}
			inserted, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_TwoPageSoilHistory" VALUES(?,?,?)`,
				request.RequestID, history.EditWhen, string(proposal))
			if err != nil {
				return err
			}
			if count, err := inserted.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("two-page soil history did not append exactly one request"))
			}
			rowID, err := inserted.LastInsertId()
			if err != nil {
				return err
			}
			historyPlan := before[twoPageSoilHistoryTable]
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
			if err := verifySiteUnitTransferTables(before, after, expected, twoPageSoilHistoryTable); err != nil {
				return fmt.Errorf("two-page soil provenance transaction differs: %w", err)
			}
			if !reflect.DeepEqual(after[twoPageSoilHistoryTable], historyPlan) {
				return errors.New("two-page soil durable history differs from its complete plan")
			}
			verified, err := readTwoPageSoilReceipt(ctx, request, after[twoPageSoilHistoryTable])
			if err != nil || verified == nil {
				return errors.Join(err, errors.New("two-page soil durable receipt verification failed"))
			}
			if beforeCommit != nil {
				if err := beforeCommit(tx); err != nil {
					return err
				}
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return err
			}
			if err := twoPageSoilWriterSelectedParent(ctx, tx, owner, suAlias, original.Plot); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			committed = true
			result = &twoPageSoilWriteResult{History: *verified, DidCommit: true}
			if afterCommit != nil {
				return afterCommit(result)
			}
			return nil
		})
		if err != nil {
			if committed {
				return result, fmt.Errorf("two-page soil edit committed but completion failed; recover the retained request before retrying: %w", err)
			}
			return nil, errors.Join(err, ctx.Err())
		}
		return result, nil
	})
}
