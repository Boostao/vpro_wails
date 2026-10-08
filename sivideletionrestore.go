package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const siviDeletionRestorationTable = "__VPRO_SIVIDeletionRestorationHistory"
const siviDeletionRestorationSQL = `CREATE TABLE "__VPRO_SIVIDeletionRestorationHistory"(RequestID TEXT NOT NULL PRIMARY KEY,DeletionID TEXT NOT NULL UNIQUE,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`

type siviDeletionRestorationRequest struct {
	RequestID string             `json:"requestId"`
	ContextID string             `json:"contextId"`
	Project   string             `json:"project"`
	Plot      string             `json:"plot"`
	HistoryID string             `json:"historyId"`
	Action    AuditRestoreAction `json:"action"`
	Expected  string             `json:"expected"`
}

func (request *siviDeletionRestorationRequest) UnmarshalJSON(data []byte) error {
	required := []string{"requestId", "contextId", "project", "plot", "historyId", "action", "expected"}
	properties, err := sourceChildJSONObject(data, "SIVI deletion restoration", required, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "SIVI deletion restoration", required...); err != nil {
		return err
	}
	type plain siviDeletionRestorationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateSIVIDeletionRestorationRequest(siviDeletionRestorationRequest(decoded)); err != nil {
		return err
	}
	*request = siviDeletionRestorationRequest(decoded)
	return nil
}

func validateSIVIDeletionRestorationRequest(request siviDeletionRestorationRequest) error {
	if len(request.Expected) != sha256.Size*2 || request.Expected != strings.ToLower(request.Expected) {
		return errors.New("SIVI deletion restoration requires an exact lowercase SHA256 expected review token")
	}
	if _, err := hex.DecodeString(request.Expected); err != nil {
		return errors.New("SIVI deletion restoration requires an exact lowercase SHA256 expected review token")
	}
	if !siviDeletionUUID.MatchString(request.RequestID) || !siviDeletionUUID.MatchString(request.HistoryID) ||
		request.RequestID == request.HistoryID {
		return errors.New("SIVI deletion restoration requires separate lowercase UUIDv4 request and deletion history IDs")
	}
	if request.ContextID == "" {
		return errors.New("SIVI deletion restoration requires an explicit context")
	}
	if err := validateChildPhysicalText("SIVI deletion restoration context", request.ContextID, 100); err != nil {
		return err
	}
	if err := validateSIVIDeletionScope(request.Project, request.Plot, "SubVegA-SIVI", "0"); err != nil {
		return err
	}
	switch request.Action {
	case AuditRestoreCancel, AuditRestoreRetain, AuditRestorePrune:
		return nil
	default:
		return errors.New("SIVI deletion restoration requires exact cancel, retain or prune")
	}
}

type siviDeletionRestorationReview struct {
	ContextID    string                  `json:"contextId"`
	Project      string                  `json:"project"`
	Plot         string                  `json:"plot"`
	HistoryID    string                  `json:"historyId"`
	Expected     string                  `json:"expected"`
	Form         string                  `json:"form"`
	RowID        string                  `json:"rowId"`
	ID           int64                   `json:"id"`
	Columns      []ProjectMetadataColumn `json:"columns"`
	Original     ProjectMetadataRow      `json:"original"`
	Deletion     siviDeletionResult      `json:"deletion"`
	AuditColumns []ProjectMetadataColumn `json:"auditColumns"`
	AuditsBefore []ProjectMetadataRow    `json:"auditsBefore"`
}

type siviDeletionRestorationResult struct {
	RequestID       string                         `json:"requestId"`
	ContextID       string                         `json:"contextId"`
	Project         string                         `json:"project"`
	Plot            string                         `json:"plot"`
	HistoryID       string                         `json:"historyId"`
	Expected        string                         `json:"expected"`
	RestorationID   string                         `json:"restorationId"`
	Form            string                         `json:"form"`
	RowID           string                         `json:"rowId"`
	ID              int64                          `json:"id"`
	Action          AuditRestoreAction             `json:"action"`
	Actor           string                         `json:"actor"`
	AuditStrength   int                            `json:"auditStrength"`
	EditWhen        string                         `json:"editWhen"`
	Columns         []ProjectMetadataColumn        `json:"columns"`
	Original        ProjectMetadataRow             `json:"original"`
	Restored        *ProjectMetadataRow            `json:"restored"`
	Deletion        siviDeletionResult             `json:"deletion"`
	Request         siviDeletionRestorationRequest `json:"request"`
	AuditColumns    []ProjectMetadataColumn        `json:"auditColumns"`
	AuditsBefore    []ProjectMetadataRow           `json:"auditsBefore"`
	AuditsAfter     []ProjectMetadataRow           `json:"auditsAfter"`
	Cancelled       bool                           `json:"cancelled"`
	RestoredRows    int                            `json:"restoredRows"`
	PrunedAuditRows int                            `json:"prunedAuditRows"`
	DidCommit       bool                           `json:"didCommit"`
	Replayed        bool                           `json:"replayed"`
}

type siviDeletionRestorationHistory struct {
	Request       siviDeletionRestorationRequest
	Deletion      siviDeletionHistory
	Actor, When   string
	AuditStrength int
	AuditColumns  []ProjectMetadataColumn
	AuditsBefore  []ProjectMetadataRow
	AuditsAfter   []ProjectMetadataRow
	Restored      ProjectMetadataRow
}

func siviDeletionRestorationReceipt(history siviDeletionRestorationHistory, contextID string, committed bool) *siviDeletionRestorationResult {
	deletion := history.Deletion
	pruned := 0
	if history.Request.Action == AuditRestorePrune {
		pruned = len(history.AuditsBefore)
	}
	return &siviDeletionRestorationResult{
		RequestID: history.Request.RequestID, ContextID: contextID, Project: history.Request.Project,
		Plot: history.Request.Plot, HistoryID: history.Request.HistoryID, RestorationID: history.Request.RequestID,
		Expected: history.Request.Expected,
		Form:     deletion.Request.Form, RowID: deletion.Result.RowID, ID: deletion.Result.ID, Action: history.Request.Action,
		Actor: history.Actor, AuditStrength: history.AuditStrength, EditWhen: history.When,
		Columns: deletion.Columns, Original: deletion.Original, Restored: &history.Restored,
		Deletion: *siviDeletionReceipt(deletion, contextID, false), Request: history.Request,
		AuditColumns: history.AuditColumns, AuditsBefore: history.AuditsBefore, AuditsAfter: history.AuditsAfter,
		RestoredRows: 1, PrunedAuditRows: pruned, DidCommit: committed, Replayed: !committed,
	}
}

func siviDeletionRestorationExpected(deletion siviDeletionHistory) (string, error) {
	proposal, err := json.Marshal(deletion)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(proposal)
	return hex.EncodeToString(sum[:]), nil
}

// Deletion history remains immutable. Restoration's unique DeletionID is the
// consumption record, including when deletion emitted no source audits.
func readSIVIDeletionRestorationDeletions(ctx context.Context, tables map[string]ProjectMetadataTable) (map[string]siviDeletionHistory, error) {
	result := map[string]siviDeletionHistory{}
	table, present := tables[siviDeletionHistoryTable]
	if !present {
		return result, ctx.Err()
	}
	expected := []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
	if _, err := siteUnitTransferColumns(table, "RequestID", "Created", "Proposal"); err != nil || !reflect.DeepEqual(table.Columns, expected) {
		return nil, errors.Join(err, errors.New("SIVI deletion restoration original history schema differs"))
	}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("SIVI deletion history requires exact text provenance"))
			}
		}
		id, when, raw := *row.Cells[0].Text, *row.Cells[1].Text, *row.Cells[2].Text
		var history siviDeletionHistory
		if err := decodeProfileLifecycleJSON([]byte(raw), &history, "Request", "Result", "Actor", "When", "AuditStrength", "Columns", "Original", "Audits"); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != raw || id != history.Request.RequestID || when != history.When {
			return nil, errors.Join(err, errors.New("SIVI deletion history key/canonical provenance differs"))
		}
		if err := validateSIVIDeletionHistory(ctx, history); err != nil {
			return nil, err
		}
		if _, duplicate := result[id]; duplicate {
			return nil, errors.New("SIVI deletion history repeats its UUID")
		}
		result[id] = history
	}
	return result, ctx.Err()
}

func validateSIVIDeletionRestorationHistory(ctx context.Context, history siviDeletionRestorationHistory, deletions map[string]siviDeletionHistory) error {
	request := history.Request
	if err := validateSIVIDeletionRestorationRequest(request); err != nil {
		return err
	}
	deletion, present := deletions[request.HistoryID]
	if !present || !reflect.DeepEqual(history.Deletion, deletion) || request.Project != deletion.Request.Project ||
		request.Plot != deletion.Request.Plot || !reflect.DeepEqual(history.Restored, deletion.Original) ||
		request.Action == AuditRestoreCancel || history.Actor == "" || history.AuditStrength < 0 || history.AuditStrength > 3 ||
		history.AuditsBefore == nil || history.AuditsAfter == nil {
		return errors.New("SIVI deletion restoration durable request/deletion/result authority differs")
	}
	expected, err := siviDeletionRestorationExpected(deletion)
	if err != nil || request.Expected != expected {
		return errors.Join(err, errors.New("SIVI deletion restoration durable review token differs from its full typed deletion evidence"))
	}
	if _, reused := deletions[request.RequestID]; reused {
		return errors.New("SIVI restoration request UUID already identifies a deletion")
	}
	if err := validateChildPhysicalText("SIVI deletion restoration actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI deletion restoration time provenance differs")
	}
	before := ProjectMetadataTable{Columns: history.AuditColumns, Rows: history.AuditsBefore}
	if _, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: history.AuditColumns}, nil); err != nil {
		return err
	}
	if _, err := siteUnitTransferColumns(before, "Restore"); err != nil || len(history.AuditsBefore) != len(deletion.Audits) {
		return errors.Join(err, errors.New("SIVI deletion restoration audit evidence is incomplete"))
	}
	for _, audit := range deletion.Audits {
		if err := verifySIVIPhysicalAudit(before, audit, request.Project); err != nil {
			return err
		}
	}
	planned, err := planSIVIDeletionRestorationAudits(before, deletion.Audits, request.Action)
	if err != nil || !reflect.DeepEqual(planned.Rows, history.AuditsAfter) {
		return errors.Join(err, errors.New("SIVI deletion restoration typed audit result differs"))
	}
	return ctx.Err()
}

func readSIVIDeletionRestorations(ctx context.Context, tables map[string]ProjectMetadataTable, deletions map[string]siviDeletionHistory) (map[string]siviDeletionRestorationHistory, error) {
	result := map[string]siviDeletionRestorationHistory{}
	table, present := tables[siviDeletionRestorationTable]
	if !present {
		return result, ctx.Err()
	}
	expected := []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "DeletionID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
	if _, err := siteUnitTransferColumns(table, "RequestID", "DeletionID", "Created", "Proposal"); err != nil || !reflect.DeepEqual(table.Columns, expected) {
		return nil, errors.Join(err, errors.New("SIVI deletion restoration history schema differs"))
	}
	schema := tables["sqlite_master"]
	schemaColumns, err := siteUnitTransferColumns(schema, "name", "type", "tbl_name", "sql")
	if err != nil {
		return nil, err
	}
	definition := false
	for _, row := range schema.Rows {
		name, kind, owner, sqlText := row.Cells[schemaColumns["name"]], row.Cells[schemaColumns["type"]], row.Cells[schemaColumns["tbl_name"]], row.Cells[schemaColumns["sql"]]
		if owner.Text == nil || !strings.EqualFold(*owner.Text, siviDeletionRestorationTable) {
			continue
		}
		if kind.Text != nil && *kind.Text == "table" && name.Text != nil && *name.Text == siviDeletionRestorationTable &&
			sqlText.Text != nil && strings.Join(strings.Fields(*sqlText.Text), " ") == strings.Join(strings.Fields(siviDeletionRestorationSQL), " ") {
			definition = true
		} else if kind.Text == nil || *kind.Text != "index" || sqlText.Storage != "null" {
			return nil, errors.New("SIVI deletion restoration provenance has an unreviewed definition/index/trigger")
		}
	}
	if !definition {
		return nil, errors.New("SIVI deletion restoration unique immutable provenance schema differs")
	}
	consumed := map[string]bool{}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("SIVI deletion restoration history requires exact text storage"))
			}
		}
		id, deletionID, when, raw := *row.Cells[0].Text, *row.Cells[1].Text, *row.Cells[2].Text, *row.Cells[3].Text
		var history siviDeletionRestorationHistory
		if err := decodeProfileLifecycleJSON([]byte(raw), &history, "Request", "Deletion", "Actor", "When", "AuditStrength", "AuditColumns", "AuditsBefore", "AuditsAfter", "Restored"); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != raw || id != history.Request.RequestID || deletionID != history.Request.HistoryID || when != history.When {
			return nil, errors.Join(err, errors.New("SIVI deletion restoration history key/canonical provenance differs"))
		}
		if err := validateSIVIDeletionRestorationHistory(ctx, history, deletions); err != nil {
			return nil, err
		}
		if _, duplicate := result[id]; duplicate || consumed[deletionID] {
			return nil, errors.New("SIVI deletion restoration repeats a request or consumes a deletion twice")
		}
		result[id], consumed[deletionID] = history, true
	}
	return result, ctx.Err()
}

func planSIVIDeletionRestorationAudits(table ProjectMetadataTable, records []AuditEntry, action AuditRestoreAction) (ProjectMetadataTable, error) {
	columns, err := siteUnitTransferColumns(table, "Restore")
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	owned := map[string]bool{}
	for _, record := range records {
		owned[record.RowID] = true
	}
	result := ProjectMetadataTable{Columns: table.Columns, Rows: []ProjectMetadataRow{}}
	for _, row := range table.Rows {
		if owned[row.RowID] {
			if action == AuditRestorePrune {
				continue
			}
			row.Cells = append([]ProjectMetadataCell{}, row.Cells...)
			row.Cells[columns["Restore"]] = siviCreationInteger("-1")
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

func verifySIVIDeletionRestorationIdentity(ctx context.Context, deletion siviDeletionHistory, tables map[string]ProjectMetadataTable, restored bool) error {
	veg := tables[deletion.Request.Project+"_Veg"]
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil || !reflect.DeepEqual(veg.Columns, deletion.Columns) {
		return errors.Join(err, errors.New("SIVI deletion restoration all44 physical schema changed"))
	}
	occupants, physical := 0, false
	for _, row := range veg.Rows {
		if row.RowID == deletion.Result.RowID {
			if !restored || !reflect.DeepEqual(row, deletion.Original) {
				return errors.New("SIVI deletion restoration physical identity occupied or restored row changed")
			}
			physical = true
		}
		if siviIdentityCellMatches(row.Cells[columns["ID"]], strconv.FormatInt(deletion.Result.ID, 10)) {
			occupants++
		}
	}
	if (!restored && occupants != 0) || (restored && (!physical || occupants != 1)) {
		return errors.New("SIVI deletion restoration requires a vacant logical ID or its sole exact restored occupant")
	}
	ledger := tables[siviIdentityLedger]
	if _, err := siteUnitTransferColumns(ledger, "ChildTable", "ID"); err != nil ||
		!reflect.DeepEqual(ledger.Columns, []ProjectMetadataColumn{{Name: "ChildTable", DeclaredType: "TEXT"}, {Name: "ID", DeclaredType: "INTEGER"}}) {
		return errors.Join(err, errors.New("SIVI deletion restoration permanent reservation schema differs"))
	}
	reserved := 0
	for _, row := range ledger.Rows {
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil {
				return err
			}
		}
		if row.Cells[0].Text != nil && *row.Cells[0].Text == quoteHeaderIdentifier(deletion.Request.Project+"_Veg") &&
			row.Cells[1].Integer != nil && *row.Cells[1].Integer == strconv.FormatInt(deletion.Result.ID, 10) {
			reserved++
		}
	}
	if reserved != 1 {
		return errors.New("SIVI deletion restoration lost its exact permanent reservation")
	}
	return ctx.Err()
}

func siviDeletionRestorationReviewFromTables(ctx context.Context, contextID, project, plot, historyID string, tables map[string]ProjectMetadataTable) (*siviDeletionRestorationReview, siviDeletionHistory, error) {
	deletions, err := readSIVIDeletionRestorationDeletions(ctx, tables)
	if err != nil {
		return nil, siviDeletionHistory{}, err
	}
	restorations, err := readSIVIDeletionRestorations(ctx, tables, deletions)
	if err != nil {
		return nil, siviDeletionHistory{}, err
	}
	deletion, present := deletions[historyID]
	if !present || deletion.Request.Project != project || deletion.Request.Plot != plot {
		return nil, siviDeletionHistory{}, errors.New("SIVI deletion restoration history is missing or belongs to another project/parent")
	}
	for _, history := range restorations {
		if history.Request.HistoryID == historyID {
			return nil, siviDeletionHistory{}, errors.New("SIVI deletion history was already consumed by a restoration")
		}
	}
	if err := verifySIVIDeletionRestorationIdentity(ctx, deletion, tables, false); err != nil {
		return nil, siviDeletionHistory{}, err
	}
	audit := tables[project+"_Audit"]
	if _, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: audit.Columns}, nil); err != nil {
		return nil, siviDeletionHistory{}, err
	}
	rows := []ProjectMetadataRow{}
	for _, record := range deletion.Audits {
		if err := verifySIVIPhysicalAudit(audit, record, project); err != nil {
			return nil, siviDeletionHistory{}, err
		}
		for _, row := range audit.Rows {
			if row.RowID == record.RowID {
				rows = append(rows, row)
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, _ := strconv.ParseInt(rows[i].RowID, 10, 64)
		b, _ := strconv.ParseInt(rows[j].RowID, 10, 64)
		return a < b
	})
	expected, err := siviDeletionRestorationExpected(deletion)
	if err != nil {
		return nil, siviDeletionHistory{}, err
	}
	return &siviDeletionRestorationReview{ContextID: contextID, Project: project, Plot: plot, HistoryID: historyID,
		Expected: expected,
		Form:     deletion.Request.Form, RowID: deletion.Result.RowID, ID: deletion.Result.ID,
		Columns: deletion.Columns, Original: deletion.Original, Deletion: *siviDeletionReceipt(deletion, contextID, false),
		AuditColumns: audit.Columns, AuditsBefore: rows}, deletion, nil
}

func verifySIVIDeletionRestorationReplay(ctx context.Context, request siviDeletionRestorationRequest, history siviDeletionRestorationHistory, tables map[string]ProjectMetadataTable) error {
	durable, caller := history.Request, request
	durable.ContextID, caller.ContextID = "", ""
	if durable != caller {
		return errors.New("SIVI deletion restoration replay request differs")
	}
	if err := verifySIVIDeletionRestorationIdentity(ctx, history.Deletion, tables, true); err != nil {
		return err
	}
	audit := tables[request.Project+"_Audit"]
	if !reflect.DeepEqual(audit.Columns, history.AuditColumns) {
		return errors.New("SIVI deletion restoration replay source audit schema changed")
	}
	after := map[string]ProjectMetadataRow{}
	for _, row := range history.AuditsAfter {
		after[row.RowID] = row
	}
	for _, original := range history.AuditsBefore {
		found := false
		expected, retained := after[original.RowID]
		for _, actual := range audit.Rows {
			if actual.RowID == original.RowID {
				if !retained || !reflect.DeepEqual(actual, expected) {
					return errors.New("SIVI deletion restoration replay retained/pruned audit changed")
				}
				found = true
			}
		}
		if found != retained {
			return errors.New("SIVI deletion restoration replay lost its retained audit")
		}
	}
	return ctx.Err()
}

func readSIVIDeletionRestorationTables(ctx context.Context, tx *sql.Tx, project string) (map[string]ProjectMetadataTable, error) {
	schema, err := readSQLiteStorageRows(ctx, tx, "project", "sqlite_master", "", nil, "")
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, row := range schema.Rows {
		if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" && row.Cells[1].Text != nil {
			present[*row.Cells[1].Text] = true
		}
	}
	tables := map[string]ProjectMetadataTable{"sqlite_master": schema}
	for _, name := range []string{siviDeletionHistoryTable, siviDeletionRestorationTable, siviIdentityLedger, project + "_Veg", project + "_Audit"} {
		if !present[name] {
			continue
		}
		table, err := readSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
		if err != nil {
			return nil, err
		}
		tables[name] = table
	}
	return tables, nil
}

func (s *ContextService) reviewSIVIDeletionRestoration(ctx context.Context, contextID, plot, historyID string) (*siviDeletionRestorationReview, error) {
	if ctx == nil || s == nil || !siviDeletionUUID.MatchString(historyID) {
		return nil, errors.New("SIVI deletion restoration review requires an owned context and exact deletion UUID")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviDeletionRestorationReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviDeletionRestorationReview, error) {
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, owner.selection.Project, plot); err != nil {
				return nil, err
			}
			tables, err := readSIVIDeletionRestorationTables(ctx, tx, owner.selection.Project)
			if err != nil {
				return nil, err
			}
			review, _, err := siviDeletionRestorationReviewFromTables(ctx, contextID, owner.selection.Project, plot, historyID, tables)
			if err != nil {
				return nil, err
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			return review, siviHeightContextParent(ctx, owner, plot)
		})
	})
}

func (s *ContextService) lookupSIVIDeletionRestorationReceipt(ctx context.Context, contextID string, request siviDeletionRestorationRequest) (*siviDeletionRestorationResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion restoration receipt requires an owned context")
	}
	if err := validateSIVIDeletionRestorationRequest(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviDeletionRestorationResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviDeletionRestorationResult, error) {
			if request.Project != owner.selection.Project {
				return nil, errors.New("SIVI deletion restoration receipt belongs to another project")
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, request.Project, request.Plot); err != nil {
				return nil, err
			}
			tables, err := readSIVIDeletionRestorationTables(ctx, tx, request.Project)
			if err != nil {
				return nil, err
			}
			if _, present := tables[siviDeletionRestorationTable]; !present {
				return nil, nil
			}
			deletions, err := readSIVIDeletionRestorationDeletions(ctx, tables)
			if err != nil {
				return nil, err
			}
			restorations, err := readSIVIDeletionRestorations(ctx, tables, deletions)
			if err != nil {
				return nil, err
			}
			history, matched := restorations[request.RequestID]
			if !matched {
				return nil, nil
			}
			if err := verifySIVIDeletionRestorationReplay(ctx, request, history, tables); err != nil {
				return nil, err
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			return siviDeletionRestorationReceipt(history, contextID, false), nil
		})
	})
}

// This exact-row undelete is a desktop adaptation, not V7mdlAudit restoration.
func (s *ContextService) restoreSIVIDeletion(ctx context.Context, contextID string, request siviDeletionRestorationRequest) (*siviDeletionRestorationResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion restoration requires an owned context")
	}
	if err := validateSIVIDeletionRestorationRequest(request); err != nil {
		return nil, err
	}
	if request.Action == AuditRestoreCancel {
		if request.ContextID != contextID {
			return nil, errors.New("SIVI deletion restoration cancellation differs from the current owner")
		}
		review, err := s.reviewSIVIDeletionRestoration(ctx, contextID, request.Plot, request.HistoryID)
		if err != nil {
			return nil, err
		}
		if review.Project != request.Project {
			return nil, errors.New("SIVI deletion restoration cancellation belongs to another project")
		}
		if request.Expected != review.Expected {
			return nil, errors.New("SIVI deletion restoration reviewed evidence changed; cancel and reload")
		}
		return &siviDeletionRestorationResult{RequestID: request.RequestID, ContextID: contextID, Project: request.Project,
			Plot: request.Plot, HistoryID: request.HistoryID, Form: review.Form, RowID: review.RowID, ID: review.ID,
			Expected: request.Expected,
			Action:   request.Action, Columns: review.Columns, Original: review.Original, Deletion: review.Deletion,
			Request: request, AuditColumns: review.AuditColumns, AuditsBefore: review.AuditsBefore,
			AuditsAfter: review.AuditsBefore, Cancelled: true}, nil
	}
	return withSIVIIdentityWriter(ctx, s, contextID, request.Plot, "deletion restoration",
		func(plots *PlotService, owner *sqliteContext, tx *sql.Tx) (*siviDeletionRestorationResult, bool, error) {
			if request.Project != owner.selection.Project {
				return nil, false, errors.New("SIVI deletion restoration belongs to another project")
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			deletions, err := readSIVIDeletionRestorationDeletions(ctx, before)
			if err != nil {
				return nil, false, err
			}
			restorations, err := readSIVIDeletionRestorations(ctx, before, deletions)
			if err != nil {
				return nil, false, err
			}
			if replay, matched := restorations[request.RequestID]; matched {
				if err := verifySIVIDeletionRestorationReplay(ctx, request, replay, before); err != nil {
					return nil, false, err
				}
				return siviDeletionRestorationReceipt(replay, contextID, false), false, nil
			}
			if request.ContextID != contextID {
				return nil, false, errors.New("SIVI deletion restoration new request differs from the current owner")
			}
			if _, reused := deletions[request.RequestID]; reused {
				return nil, false, errors.New("SIVI restoration request UUID already identifies a deletion")
			}
			review, deletion, err := siviDeletionRestorationReviewFromTables(ctx, contextID, request.Project, request.Plot, request.HistoryID, before)
			if err != nil {
				return nil, false, err
			}
			if request.Expected != review.Expected {
				return nil, false, errors.New("SIVI deletion restoration reviewed evidence changed; cancel and reload before restoring")
			}
			if plots.currentUser == "" || plots.auditStrength < 0 || plots.auditStrength > 3 {
				return nil, false, errors.New("SIVI deletion restoration requires valid explicit provenance")
			}
			if err := validateChildPhysicalText("SIVI deletion restoration actor", plots.currentUser, 100); err != nil {
				return nil, false, err
			}
			names, marks, values := []string{"rowid"}, []string{"?"}, []any{review.RowID}
			for i, column := range review.Columns {
				value, err := metadataCellValue(review.Original.Cells[i])
				if err != nil {
					return nil, false, err
				}
				names, marks, values = append(names, quoteHeaderIdentifier(column.Name)), append(marks, "?"), append(values, value)
			}
			inserted, err := tx.ExecContext(ctx, `INSERT INTO `+quoteHeaderIdentifier(request.Project+"_Veg")+
				` (`+strings.Join(names, ",")+`) VALUES (`+strings.Join(marks, ",")+`)`, values...)
			if err != nil {
				return nil, false, err
			}
			if count, err := inserted.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI deletion restoration did not reinsert exactly one physical row"))
			}
			auditPlan, err := planSIVIDeletionRestorationAudits(before[request.Project+"_Audit"], deletion.Audits, request.Action)
			if err != nil {
				return nil, false, err
			}
			for _, audit := range deletion.Audits {
				query := `UPDATE ` + quoteHeaderIdentifier(request.Project+"_Audit") + ` SET Restore=-1 WHERE rowid=?`
				if request.Action == AuditRestorePrune {
					query = `DELETE FROM ` + quoteHeaderIdentifier(request.Project+"_Audit") + ` WHERE rowid=?`
				}
				changed, err := tx.ExecContext(ctx, query, audit.RowID)
				if err != nil {
					return nil, false, err
				}
				if count, err := changed.RowsAffected(); err != nil || count != 1 {
					return nil, false, errors.Join(err, errors.New("SIVI deletion restoration did not retain/prune exactly one source audit"))
				}
			}
			vegPlan := before[request.Project+"_Veg"]
			vegPlan.Rows = append(append([]ProjectMetadataRow{}, vegPlan.Rows...), review.Original)
			sort.Slice(vegPlan.Rows, func(i, j int) bool {
				a, _ := strconv.ParseInt(vegPlan.Rows[i].RowID, 10, 64)
				b, _ := strconv.ParseInt(vegPlan.Rows[j].RowID, 10, 64)
				return a < b
			})
			expected := map[string]ProjectMetadataTable{request.Project + "_Veg": vegPlan, request.Project + "_Audit": auditPlan}
			observed, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(before, observed, expected, ""); err != nil {
				return nil, false, err
			}
			evidence, err := planSIVIDeletionRestorationAudits(ProjectMetadataTable{Columns: review.AuditColumns, Rows: review.AuditsBefore}, deletion.Audits, request.Action)
			if err != nil {
				return nil, false, err
			}
			history := siviDeletionRestorationHistory{Request: request, Deletion: deletion, Actor: plots.currentUser,
				When: time.Now().Format("2006-01-02 15:04:05"), AuditStrength: plots.auditStrength,
				AuditColumns: review.AuditColumns, AuditsBefore: review.AuditsBefore, AuditsAfter: evidence.Rows, Restored: review.Original}
			if err := validateSIVIDeletionRestorationHistory(ctx, history, deletions); err != nil {
				return nil, false, err
			}
			if err := verifySIVIDeletionRestorationReplay(ctx, request, history, observed); err != nil {
				return nil, false, err
			}
			raw, err := json.Marshal(history)
			if err != nil {
				return nil, false, err
			}
			if _, present := before[siviDeletionRestorationTable]; !present {
				if _, err := tx.ExecContext(ctx, siviDeletionRestorationSQL); err != nil {
					return nil, false, err
				}
			}
			entry, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_SIVIDeletionRestorationHistory"(RequestID,DeletionID,Created,Proposal) VALUES(?,?,?,?)`,
				request.RequestID, request.HistoryID, history.When, string(raw))
			if err != nil {
				return nil, false, err
			}
			if count, err := entry.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI deletion restoration did not record exactly one consumption"))
			}
			rowID, err := entry.LastInsertId()
			if err != nil {
				return nil, false, err
			}
			historyPlan := before[siviDeletionRestorationTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "DeletionID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]ProjectMetadataRow{}, historyPlan.Rows...), ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10),
				Cells: []ProjectMetadataCell{siviCreationText(request.RequestID), siviCreationText(request.HistoryID), siviCreationText(history.When), siviCreationText(string(raw))}})
			observed, err = environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(before, observed, expected, siviDeletionRestorationTable); err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(observed[siviDeletionRestorationTable], historyPlan) {
				return nil, false, errors.New("SIVI deletion restoration immutable typed history differs")
			}
			verified, err := readSIVIDeletionRestorations(ctx, observed, deletions)
			if err != nil {
				return nil, false, err
			}
			durable, present := verified[request.RequestID]
			if !present || !reflect.DeepEqual(durable, history) {
				return nil, false, fmt.Errorf("SIVI deletion restoration durable receipt is missing or differs")
			}
			return siviDeletionRestorationReceipt(durable, contextID, true), true, nil
		})
}
