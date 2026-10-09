package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const siviDeletionHistoryTable = "__VPRO_SIVIDeletionHistory"

var siviDeletionUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type siviDeletionRequest struct {
	RequestID string                  `json:"requestId"`
	ContextID string                  `json:"contextId"`
	Project   string                  `json:"project"`
	Plot      string                  `json:"plot"`
	Form      string                  `json:"form"`
	Columns   []ProjectMetadataColumn `json:"columns"`
	Original  ProjectMetadataRow      `json:"original"`
}

func (request *siviDeletionRequest) UnmarshalJSON(data []byte) error {
	required := []string{"requestId", "contextId", "project", "plot", "form", "columns", "original"}
	properties, err := sourceChildJSONObject(data, "SIVI deletion", required, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "SIVI deletion", required...); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["columns"], func(raw json.RawMessage) error {
		column, err := sourceChildJSONObject(raw, "SIVI deletion column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(column, "SIVI deletion column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceChildRowJSON(properties["original"], "SIVI deletion original"); err != nil {
		return err
	}
	type plain siviDeletionRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateSIVIDeletionRequest(siviDeletionRequest(decoded)); err != nil {
		return err
	}
	*request = siviDeletionRequest(decoded)
	return nil
}

type siviDeletionResult struct {
	RequestID     string                  `json:"requestId"`
	ContextID     string                  `json:"contextId"`
	Project       string                  `json:"project"`
	Plot          string                  `json:"plot"`
	Form          string                  `json:"form"`
	RowID         string                  `json:"rowId"`
	ID            int64                   `json:"id"`
	HistoryID     string                  `json:"historyId"`
	Actor         string                  `json:"actor"`
	AuditStrength int                     `json:"auditStrength"`
	EditWhen      string                  `json:"editWhen"`
	Columns       []ProjectMetadataColumn `json:"columns"`
	Original      ProjectMetadataRow      `json:"original"`
	Request       siviDeletionRequest     `json:"request"`
	Audits        []AuditEntry            `json:"audits"`
	DidCommit     bool                    `json:"didCommit"`
	Replayed      bool                    `json:"replayed"`
}

type siviDeletionHistory struct {
	Request       siviDeletionRequest
	Result        siviCreationIdentity
	Actor, When   string
	AuditStrength int
	Columns       []ProjectMetadataColumn
	Original      ProjectMetadataRow
	Audits        []AuditEntry
}

func siviDeletionReceipt(history siviDeletionHistory, contextID string, didCommit bool) *siviDeletionResult {
	return &siviDeletionResult{
		RequestID: history.Result.RequestID, ContextID: contextID, Project: history.Result.Project,
		Plot: history.Result.Plot, Form: history.Request.Form, RowID: history.Result.RowID,
		ID: history.Result.ID, HistoryID: history.Result.HistoryID, Actor: history.Actor,
		AuditStrength: history.AuditStrength, EditWhen: history.When, Columns: history.Columns,
		Original: history.Original, Request: history.Request, Audits: history.Audits,
		DidCommit: didCommit, Replayed: !didCommit,
	}
}

func validateSIVIDeletionScope(project, plot, form, rowID string) error {
	if !projectNamePattern.MatchString(project) {
		return errors.New("SIVI deletion requires an exact project name")
	}
	if plot == "" {
		return errors.New("SIVI deletion requires an explicit parent")
	}
	if err := validateChildPhysicalText("SIVI deletion parent", plot, 7); err != nil {
		return err
	}
	switch form {
	case "SubVegA-SIVI", "SubVegA-SIVI_BC", "SubVegC-SIVI", "SubVegD-SIVI":
	default:
		return errors.New("unavailable SIVI deletion form")
	}
	id, err := strconv.ParseInt(rowID, 10, 64)
	if err != nil || strconv.FormatInt(id, 10) != rowID {
		return errors.New("SIVI deletion requires an exact signed64 physical rowId")
	}
	return nil
}

func validateSIVIDeletionRequest(request siviDeletionRequest) error {
	if !siviDeletionUUID.MatchString(request.RequestID) {
		return errors.New("SIVI deletion requires a stable lowercase UUIDv4 requestId")
	}
	if request.ContextID == "" {
		return errors.New("SIVI deletion requires an explicit contextId")
	}
	if err := validateChildPhysicalText("SIVI deletion context", request.ContextID, 100); err != nil {
		return err
	}
	_, err := validateSIVIDeletionOriginal(siviDeletionOriginal{request.ContextID, request.Project, request.Plot, request.Form, request.Columns, request.Original})
	return err
}

func validateSIVIDeletionOriginal(original siviDeletionOriginal) (int64, error) {
	if err := validateSIVIDeletionScope(original.Project, original.Plot, original.Form, original.Original.RowID); err != nil {
		return 0, err
	}
	table := ProjectMetadataTable{Columns: original.Columns, Rows: []ProjectMetadataRow{original.Original}}
	columns, err := siteUnitTransferColumns(table, siviCreationColumns...)
	if err != nil || len(columns) != len(siviCreationColumns) {
		return 0, errors.Join(err, errors.New("SIVI deletion original requires all44 exact typed cells"))
	}
	for i, cell := range original.Original.Cells {
		if _, err := siviDeletionAuditValue(original.Columns[i], cell); err != nil {
			return 0, err
		}
	}
	parent := original.Original.Cells[columns["PlotNumber"]]
	if parent.Storage != "text" || parent.Text == nil || *parent.Text != original.Plot {
		return 0, errors.New("SIVI deletion original belongs to another parent")
	}
	id := original.Original.Cells[columns["ID"]]
	if id.Storage != "integer" || id.Integer == nil {
		return 0, errors.New("SIVI deletion NULL or non-signed32 logical ID is unavailable; no identity repair")
	}
	number, err := strconv.ParseInt(*id.Integer, 10, 32)
	if err != nil {
		return 0, errors.New("SIVI deletion requires an existing signed32 logical ID")
	}
	return number, nil
}

func siviDeletionAuditValue(column ProjectMetadataColumn, cell ProjectMetadataCell) (any, error) {
	value, err := metadataCellValue(cell)
	if err != nil {
		return nil, err
	}
	if cell.Storage == "blob" {
		return nil, fmt.Errorf("SIVI deletion unsupported BLOB audit payload in %s", column.Name)
	}
	boolean := column.Name == "Flag" || strings.EqualFold(column.DeclaredType, "BOOLEAN") ||
		strings.EqualFold(column.DeclaredType, "BOOL") || strings.EqualFold(column.DeclaredType, "BIT")
	if boolean && value != nil {
		number, ok := value.(int64)
		if !ok {
			return nil, fmt.Errorf("SIVI deletion BOOLEAN %s requires integer storage", column.Name)
		}
		if number != 0 {
			return int64(-1), nil
		}
	}
	return value, nil
}

// Full-row deletion evidence is a desktop safety adaptation, not an Access
// deletion-audit or deleted-row restoration parity claim.
func validateSIVIDeletionHistory(ctx context.Context, history siviDeletionHistory) error {
	request := history.Request
	if err := validateSIVIDeletionRequest(request); err != nil {
		return err
	}
	id, err := validateSIVIDeletionOriginal(siviDeletionOriginal{request.ContextID, request.Project, request.Plot, request.Form, request.Columns, request.Original})
	if err != nil {
		return err
	}
	expectedIdentity := siviCreationIdentity{request.RequestID, request.Project, request.Plot, request.Original.RowID, request.RequestID, id}
	if history.Result != expectedIdentity || !reflect.DeepEqual(history.Columns, request.Columns) ||
		!reflect.DeepEqual(history.Original, request.Original) || history.Actor == "" ||
		history.AuditStrength < 0 || history.AuditStrength > 3 || history.Audits == nil {
		return errors.New("SIVI deletion history request/result/original authority differs")
	}
	if err := validateChildPhysicalText("SIVI deletion actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI deletion history time provenance differs")
	}
	groups, err := projectSIVIVegetation(ctx, request.Plot, request.Form == "SubVegA-SIVI",
		ProjectMetadataTable{Columns: request.Columns, Rows: []ProjectMetadataRow{request.Original}})
	if err != nil {
		return err
	}
	member := false
	for _, group := range groups {
		member = member || group.Form == request.Form && len(group.Rows) == 1
	}
	if !member {
		return errors.New("SIVI deletion durable original is not a source member")
	}
	expected := map[string]any{}
	for i, column := range request.Columns {
		value, err := siviDeletionAuditValue(column, request.Original.Cells[i])
		if err != nil {
			return err
		}
		if column.Name != "ID" && column.Name != "PlotNumber" && auditHeaderChange(value, nil, history.AuditStrength) {
			expected[column.Name] = value
		}
	}
	if len(expected) != len(history.Audits) {
		return errors.New("SIVI deletion history audit completeness differs")
	}
	seen := map[string]bool{}
	for _, audit := range history.Audits {
		value, present := expected[audit.EditField]
		rowID, err := strconv.ParseInt(audit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(rowID, 10) != audit.RowID || seen[audit.RowID] ||
			!present || audit.Project != request.Project || audit.PlotNumber != request.Plot ||
			audit.User != history.Actor || audit.EditWhen != history.When || audit.Table != "_Veg" ||
			audit.ID == nil || *audit.ID != id || audit.AfterEdit != nil || audit.Restore || audit.Flag ||
			!metadataAuditTextEqual(audit.BeforeEdit, value) {
			return errors.New("SIVI deletion history audit authority differs")
		}
		seen[audit.RowID] = true
		delete(expected, audit.EditField)
	}
	return ctx.Err()
}

func verifySIVIDeletionHistory(ctx context.Context, request siviDeletionRequest, history siviDeletionHistory, tables map[string]ProjectMetadataTable) error {
	if !sameSIVIDeletionRequest(request, history.Request) {
		return errors.New("SIVI deletion replay request differs")
	}
	veg := tables[request.Project+"_Veg"]
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil || !reflect.DeepEqual(veg.Columns, history.Columns) {
		return errors.Join(err, errors.New("SIVI deletion replay physical schema differs"))
	}
	for _, row := range veg.Rows {
		parent := row.Cells[columns["PlotNumber"]]
		if row.RowID == history.Result.RowID || parent.Text != nil && *parent.Text == request.Plot &&
			siviIdentityCellMatches(row.Cells[columns["ID"]], strconv.FormatInt(history.Result.ID, 10)) {
			return errors.New("SIVI deletion replay has a physical or parent/logical identity occupant")
		}
	}
	ledger := tables[siviIdentityLedger]
	ledgerColumns, err := siteUnitTransferColumns(ledger, "ChildTable", "ID")
	if err != nil || len(ledgerColumns) != 2 {
		return errors.Join(err, errors.New("SIVI deletion reservation schema differs"))
	}
	reserved := false
	for _, row := range ledger.Rows {
		table, id := row.Cells[ledgerColumns["ChildTable"]], row.Cells[ledgerColumns["ID"]]
		reserved = reserved || table.Storage == "text" && table.Text != nil &&
			*table.Text == quoteHeaderIdentifier(request.Project+"_Veg") && id.Storage == "integer" &&
			id.Integer != nil && *id.Integer == strconv.FormatInt(history.Result.ID, 10)
	}
	if !reserved {
		return errors.New("SIVI deletion replay lost its permanent identity reservation")
	}
	audit := tables[request.Project+"_Audit"]
	plan, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: audit.Columns}, history.Audits)
	if err != nil {
		return err
	}
	for _, expected := range plan.Rows {
		found := false
		for _, row := range audit.Rows {
			found = found || reflect.DeepEqual(row, expected)
		}
		if !found {
			return errors.New("SIVI deletion replay lost its exact source audit")
		}
	}
	return ctx.Err()
}

func readSIVIDeletionReplay(ctx context.Context, request siviDeletionRequest, tables map[string]ProjectMetadataTable) (*siviDeletionHistory, error) {
	table, present := tables[siviDeletionHistoryTable]
	if !present {
		return nil, ctx.Err()
	}
	columns, err := siteUnitTransferColumns(table, "RequestID", "Created", "Proposal")
	if err != nil || !reflect.DeepEqual(table.Columns, []ProjectMetadataColumn{
		{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"},
	}) {
		return nil, errors.Join(err, errors.New("SIVI deletion history schema differs"))
	}
	var matched *siviDeletionHistory
	seen := map[string]bool{}
	for _, row := range table.Rows {
		id, created, proposal := row.Cells[columns["RequestID"]], row.Cells[columns["Created"]], row.Cells[columns["Proposal"]]
		for _, cell := range []ProjectMetadataCell{id, created, proposal} {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("SIVI deletion history requires exact text storage"))
			}
		}
		if seen[*id.Text] {
			return nil, errors.New("SIVI deletion history repeats a requestId")
		}
		seen[*id.Text] = true
		raw := []byte(*proposal.Text)
		if _, err := sourceChildJSONObject(raw, "SIVI deletion history",
			[]string{"Request", "Result", "Actor", "When", "AuditStrength", "Columns", "Original", "Audits"}, nil); err != nil {
			return nil, err
		}
		var history siviDeletionHistory
		if err := json.Unmarshal(raw, &history); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != *proposal.Text || *created.Text != history.When || *id.Text != history.Request.RequestID {
			return nil, errors.Join(err, errors.New("SIVI deletion history key/canonical provenance differs"))
		}
		if err := validateSIVIDeletionHistory(ctx, history); err != nil {
			return nil, err
		}
		if *id.Text != request.RequestID {
			continue
		}
		if history.Request.Project != request.Project {
			return nil, errors.New("SIVI deletion requestId belongs to another project")
		}
		if err := verifySIVIDeletionHistory(ctx, request, history, tables); err != nil {
			return nil, err
		}
		matched = &history
	}
	return matched, ctx.Err()
}

func (s *ContextService) deleteSIVIVegetation(ctx context.Context, contextID string, request siviDeletionRequest) (*siviDeletionResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateSIVIDeletionRequest(request); err != nil {
		return nil, err
	}
	return withSIVIIdentityWriter(ctx, s, contextID, request.Plot, "deletion",
		func(plots *PlotService, owner *sqliteContext, tx *sql.Tx) (*siviDeletionResult, bool, error) {
			if request.Project != owner.selection.Project {
				return nil, false, errors.New("SIVI deletion request belongs to another project")
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			replay, err := readSIVIDeletionReplay(ctx, request, before)
			if err != nil {
				return nil, false, err
			}
			if replay != nil {
				return siviDeletionReceipt(*replay, contextID, false), false, nil
			}
			if request.ContextID != contextID {
				return nil, false, errors.New("SIVI deletion new request differs from the current context owner")
			}
			if plots.currentUser == "" || plots.auditStrength < 0 || plots.auditStrength > 3 {
				return nil, false, errors.New("SIVI deletion requires explicit valid audit provenance")
			}
			if err := validateChildPhysicalText("SIVI deletion actor", plots.currentUser, 100); err != nil {
				return nil, false, err
			}
			veg := before[request.Project+"_Veg"]
			original, err := siviDeletionSource(ctx, request.Project, request.Plot, request.Form, request.Original.RowID, veg)
			if err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(original.Columns, request.Columns) || !reflect.DeepEqual(original.Original, request.Original) {
				return nil, false, errors.New("SIVI deletion full typed original/schema changed; cancel and reload")
			}
			id, err := validateSIVIDeletionOriginal(*original)
			if err != nil {
				return nil, false, err
			}
			index, err := siteUnitTransferColumns(veg, "ID")
			if err != nil {
				return nil, false, err
			}
			baseline, ledger, err := reserveSIVIIdentityPlan(ctx, tx, request.Project, before,
				[]siviHeightAssignment{{Before: request.Original.Cells[index["ID"]], After: ProjectMetadataCell{Storage: "null"}}})
			if err != nil {
				return nil, false, err
			}
			deleted, err := tx.ExecContext(ctx, `DELETE FROM `+quoteHeaderIdentifier(request.Project+"_Veg")+
				` WHERE rowid=? AND PlotNumber COLLATE BINARY=?`, request.Original.RowID, request.Plot)
			if err != nil {
				return nil, false, err
			}
			if count, err := deleted.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI deletion did not delete exactly one physical row"))
			}
			when := time.Now().Format("2006-01-02 15:04:05")
			fields, values, nulls := []childField{}, []any{}, []any{}
			for i, column := range veg.Columns {
				if column.Name == "ID" || column.Name == "PlotNumber" {
					continue
				}
				value, err := siviDeletionAuditValue(column, request.Original.Cells[i])
				if err != nil {
					return nil, false, err
				}
				fields, values, nulls = append(fields, childField{column: column.Name}), append(values, value), append(nulls, nil)
			}
			audits, err := auditChildFieldsTracked(tx, request.Project, "Veg", request.Plot, id, fields, values, nulls,
				plots.currentUser, plots.auditStrength, when)
			if err != nil {
				return nil, false, err
			}
			records := []AuditEntry{}
			for _, audit := range audits {
				ids, err := auditRowIDs([]string{audit.RowID})
				if err != nil {
					return nil, false, err
				}
				selected, err := selectedAuditEntries(tx, request.Project, request.Plot, ids)
				if err != nil || len(selected) != 1 {
					return nil, false, errors.Join(err, errors.New("SIVI deletion audit identity differs"))
				}
				records = append(records, selected[0])
			}
			plannedVeg := ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{}}
			for _, row := range veg.Rows {
				if row.RowID != request.Original.RowID {
					plannedVeg.Rows = append(plannedVeg.Rows, row)
				}
			}
			auditPlan, err := appendSIVIPlannedAudits(before[request.Project+"_Audit"], records)
			if err != nil {
				return nil, false, err
			}
			expected := map[string]ProjectMetadataTable{request.Project + "_Veg": plannedVeg, request.Project + "_Audit": auditPlan, siviIdentityLedger: ledger}
			observed, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(baseline, observed, expected, ""); err != nil {
				return nil, false, err
			}
			history := siviDeletionHistory{Request: request,
				Result: siviCreationIdentity{request.RequestID, request.Project, request.Plot, request.Original.RowID, request.RequestID, id},
				Actor:  plots.currentUser, When: when, AuditStrength: plots.auditStrength, Columns: veg.Columns, Original: request.Original, Audits: records}
			if err := validateSIVIDeletionHistory(ctx, history); err != nil {
				return nil, false, err
			}
			if err := verifySIVIDeletionHistory(ctx, request, history, observed); err != nil {
				return nil, false, err
			}
			proposal, err := json.Marshal(history)
			if err != nil {
				return nil, false, err
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_SIVIDeletionHistory" (
				RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
				return nil, false, err
			}
			entry, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_SIVIDeletionHistory"(RequestID,Created,Proposal) VALUES(?,?,?)`,
				request.RequestID, when, string(proposal))
			if err != nil {
				return nil, false, err
			}
			if count, err := entry.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI deletion history did not insert exactly one request"))
			}
			rowID, err := entry.LastInsertId()
			if err != nil {
				return nil, false, err
			}
			historyPlan := before[siviDeletionHistoryTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]ProjectMetadataRow{}, historyPlan.Rows...), ProjectMetadataRow{
				RowID: strconv.FormatInt(rowID, 10), Cells: []ProjectMetadataCell{siviCreationText(request.RequestID), siviCreationText(when), siviCreationText(string(proposal))}})
			observed, err = environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(baseline, observed, expected, siviDeletionHistoryTable); err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(observed[siviDeletionHistoryTable], historyPlan) {
				return nil, false, errors.New("SIVI deletion durable history differs from its typed plan")
			}
			verified, err := readSIVIDeletionReplay(ctx, request, observed)
			if err != nil || verified == nil {
				return nil, false, errors.Join(err, errors.New("SIVI deletion durable receipt verification failed"))
			}
			return siviDeletionReceipt(*verified, contextID, true), true, nil
		})
}
