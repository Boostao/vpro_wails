package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"strconv"
	"strings"
	"time"
)

const SiviDeletionHistoryTable = "__VPRO_SIVIDeletionHistory"

type SiviDeletionRequest struct {
	RequestID string                           `json:"requestId"`
	ContextID string                           `json:"contextId"`
	Project   string                           `json:"project"`
	Plot      string                           `json:"plot"`
	Form      string                           `json:"form"`
	Columns   []vprocore.ProjectMetadataColumn `json:"columns"`
	Original  vprocore.ProjectMetadataRow      `json:"original"`
}

func (request *SiviDeletionRequest) UnmarshalJSON(data []byte) error {
	required := []string{"requestId", "contextId", "project", "plot", "form", "columns", "original"}
	properties, err := SourceChildJSONObject(data, "SIVI deletion", required, nil)
	if err != nil {
		return err
	}
	if err := SourceChildJSONNonNull(properties, "SIVI deletion", required...); err != nil {
		return err
	}
	if err := SourceChildArrayJSON(properties["columns"], func(raw json.RawMessage) error {
		column, err := SourceChildJSONObject(raw, "SIVI deletion column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return SourceChildJSONNonNull(column, "SIVI deletion column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := SourceChildRowJSON(properties["original"], "SIVI deletion original"); err != nil {
		return err
	}
	type plain SiviDeletionRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := ValidateSIVIDeletionRequest(SiviDeletionRequest(decoded)); err != nil {
		return err
	}
	*request = SiviDeletionRequest(decoded)
	return nil
}

type SiviDeletionResult struct {
	RequestID     string                           `json:"requestId"`
	ContextID     string                           `json:"contextId"`
	Project       string                           `json:"project"`
	Plot          string                           `json:"plot"`
	Form          string                           `json:"form"`
	RowID         string                           `json:"rowId"`
	ID            int64                            `json:"id"`
	HistoryID     string                           `json:"historyId"`
	Actor         string                           `json:"actor"`
	AuditStrength int                              `json:"auditStrength"`
	EditWhen      string                           `json:"editWhen"`
	Columns       []vprocore.ProjectMetadataColumn `json:"columns"`
	Original      vprocore.ProjectMetadataRow      `json:"original"`
	Request       SiviDeletionRequest              `json:"request"`
	Audits        []vprocore.AuditEntry            `json:"audits"`
	DidCommit     bool                             `json:"didCommit"`
	Replayed      bool                             `json:"replayed"`
}

type SiviDeletionHistory struct {
	Request       SiviDeletionRequest
	Result        SiviCreationIdentity
	Actor, When   string
	AuditStrength int
	Columns       []vprocore.ProjectMetadataColumn
	Original      vprocore.ProjectMetadataRow
	Audits        []vprocore.AuditEntry
}

func SiviDeletionReceipt(history SiviDeletionHistory, contextID string, didCommit bool) *SiviDeletionResult {
	return &SiviDeletionResult{
		RequestID: history.Result.RequestID, ContextID: contextID, Project: history.Result.Project,
		Plot: history.Result.Plot, Form: history.Request.Form, RowID: history.Result.RowID,
		ID: history.Result.ID, HistoryID: history.Result.HistoryID, Actor: history.Actor,
		AuditStrength: history.AuditStrength, EditWhen: history.When, Columns: history.Columns,
		Original: history.Original, Request: history.Request, Audits: history.Audits,
		DidCommit: didCommit, Replayed: !didCommit,
	}
}

func ValidateSIVIDeletionScope(project, plot, form, rowID string) error {
	if !vprocore.ProjectNamePattern.MatchString(project) {
		return errors.New("SIVI deletion requires an exact project name")
	}
	if plot == "" {
		return errors.New("SIVI deletion requires an explicit parent")
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI deletion parent", plot, 7); err != nil {
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

func ValidateSIVIDeletionRequest(request SiviDeletionRequest) error {
	if !vprocore.SiviDeletionUUID.MatchString(request.RequestID) {
		return errors.New("SIVI deletion requires a stable lowercase UUIDv4 requestId")
	}
	if request.ContextID == "" {
		return errors.New("SIVI deletion requires an explicit contextId")
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI deletion context", request.ContextID, 100); err != nil {
		return err
	}
	_, err := ValidateSIVIDeletionOriginal(SiviDeletionOriginal{request.ContextID, request.Project, request.Plot, request.Form, request.Columns, request.Original})
	return err
}

func ValidateSIVIDeletionOriginal(original SiviDeletionOriginal) (int64, error) {
	if err := ValidateSIVIDeletionScope(original.Project, original.Plot, original.Form, original.Original.RowID); err != nil {
		return 0, err
	}
	table := vprocore.ProjectMetadataTable{Columns: original.Columns, Rows: []vprocore.ProjectMetadataRow{original.Original}}
	columns, err := vprocore.SiteUnitTransferColumns(table, SiviCreationColumns...)
	if err != nil || len(columns) != len(SiviCreationColumns) {
		return 0, errors.Join(err, errors.New("SIVI deletion original requires all44 exact typed cells"))
	}
	for i, cell := range original.Original.Cells {
		if _, err := SiviDeletionAuditValue(original.Columns[i], cell); err != nil {
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

func SiviDeletionAuditValue(column vprocore.ProjectMetadataColumn, cell vprocore.ProjectMetadataCell) (any, error) {
	value, err := vprocore.MetadataCellValue(cell)
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
func ValidateSIVIDeletionHistory(ctx context.Context, history SiviDeletionHistory) error {
	request := history.Request
	if err := ValidateSIVIDeletionRequest(request); err != nil {
		return err
	}
	id, err := ValidateSIVIDeletionOriginal(SiviDeletionOriginal{request.ContextID, request.Project, request.Plot, request.Form, request.Columns, request.Original})
	if err != nil {
		return err
	}
	expectedIdentity := SiviCreationIdentity{request.RequestID, request.Project, request.Plot, request.Original.RowID, request.RequestID, id}
	if history.Result != expectedIdentity || !reflect.DeepEqual(history.Columns, request.Columns) ||
		!reflect.DeepEqual(history.Original, request.Original) || history.Actor == "" ||
		history.AuditStrength < 0 || history.AuditStrength > 3 || history.Audits == nil {
		return errors.New("SIVI deletion history request/result/original authority differs")
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI deletion actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI deletion history time provenance differs")
	}
	groups, err := ProjectSIVIVegetation(ctx, request.Plot, request.Form == "SubVegA-SIVI",
		vprocore.ProjectMetadataTable{Columns: request.Columns, Rows: []vprocore.ProjectMetadataRow{request.Original}})
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
		value, err := SiviDeletionAuditValue(column, request.Original.Cells[i])
		if err != nil {
			return err
		}
		if column.Name != "ID" && column.Name != "PlotNumber" && vprocore.AuditHeaderChange(value, nil, history.AuditStrength) {
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
			!vprocore.MetadataAuditTextEqual(audit.BeforeEdit, value) {
			return errors.New("SIVI deletion history audit authority differs")
		}
		seen[audit.RowID] = true
		delete(expected, audit.EditField)
	}
	return ctx.Err()
}

func verifySIVIDeletionHistory(ctx context.Context, request SiviDeletionRequest, history SiviDeletionHistory, tables map[string]vprocore.ProjectMetadataTable) error {
	if !sameSIVIDeletionRequest(request, history.Request) {
		return errors.New("SIVI deletion replay request differs")
	}
	veg := tables[request.Project+"_Veg"]
	columns, err := vprocore.SiteUnitTransferColumns(veg, SiviCreationColumns...)
	if err != nil || !reflect.DeepEqual(veg.Columns, history.Columns) {
		return errors.Join(err, errors.New("SIVI deletion replay physical schema differs"))
	}
	for _, row := range veg.Rows {
		parent := row.Cells[columns["PlotNumber"]]
		if row.RowID == history.Result.RowID || parent.Text != nil && *parent.Text == request.Plot &&
			SiviIdentityCellMatches(row.Cells[columns["ID"]], strconv.FormatInt(history.Result.ID, 10)) {
			return errors.New("SIVI deletion replay has a physical or parent/logical identity occupant")
		}
	}
	ledger := tables[SiviIdentityLedger]
	ledgerColumns, err := vprocore.SiteUnitTransferColumns(ledger, "ChildTable", "ID")
	if err != nil || len(ledgerColumns) != 2 {
		return errors.Join(err, errors.New("SIVI deletion reservation schema differs"))
	}
	reserved := false
	for _, row := range ledger.Rows {
		table, id := row.Cells[ledgerColumns["ChildTable"]], row.Cells[ledgerColumns["ID"]]
		reserved = reserved || table.Storage == "text" && table.Text != nil &&
			*table.Text == vprocore.QuoteHeaderIdentifier(request.Project+"_Veg") && id.Storage == "integer" &&
			id.Integer != nil && *id.Integer == strconv.FormatInt(history.Result.ID, 10)
	}
	if !reserved {
		return errors.New("SIVI deletion replay lost its permanent identity reservation")
	}
	audit := tables[request.Project+"_Audit"]
	plan, err := AppendSIVIPlannedAudits(vprocore.ProjectMetadataTable{Columns: audit.Columns}, history.Audits)
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

func readSIVIDeletionReplay(ctx context.Context, request SiviDeletionRequest, tables map[string]vprocore.ProjectMetadataTable) (*SiviDeletionHistory, error) {
	table, present := tables[SiviDeletionHistoryTable]
	if !present {
		return nil, ctx.Err()
	}
	columns, err := vprocore.SiteUnitTransferColumns(table, "RequestID", "Created", "Proposal")
	if err != nil || !reflect.DeepEqual(table.Columns, []vprocore.ProjectMetadataColumn{
		{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"},
	}) {
		return nil, errors.Join(err, errors.New("SIVI deletion history schema differs"))
	}
	var matched *SiviDeletionHistory
	seen := map[string]bool{}
	for _, row := range table.Rows {
		id, created, proposal := row.Cells[columns["RequestID"]], row.Cells[columns["Created"]], row.Cells[columns["Proposal"]]
		for _, cell := range []vprocore.ProjectMetadataCell{id, created, proposal} {
			if _, err := vprocore.MetadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("SIVI deletion history requires exact text storage"))
			}
		}
		if seen[*id.Text] {
			return nil, errors.New("SIVI deletion history repeats a requestId")
		}
		seen[*id.Text] = true
		raw := []byte(*proposal.Text)
		if _, err := SourceChildJSONObject(raw, "SIVI deletion history",
			[]string{"Request", "Result", "Actor", "When", "AuditStrength", "Columns", "Original", "Audits"}, nil); err != nil {
			return nil, err
		}
		var history SiviDeletionHistory
		if err := json.Unmarshal(raw, &history); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != *proposal.Text || *created.Text != history.When || *id.Text != history.Request.RequestID {
			return nil, errors.Join(err, errors.New("SIVI deletion history key/canonical provenance differs"))
		}
		if err := ValidateSIVIDeletionHistory(ctx, history); err != nil {
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

func DeleteSIVIVegetation(s *vprocore.ContextService, ctx context.Context, contextID string, request SiviDeletionRequest) (*SiviDeletionResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateSIVIDeletionRequest(request); err != nil {
		return nil, err
	}
	return WithSIVIIdentityWriter(ctx, s, contextID, request.Plot, "deletion",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx) (*SiviDeletionResult, bool, error) {
			if request.Project != owner.Selection.Project {
				return nil, false, errors.New("SIVI deletion request belongs to another project")
			}
			before, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			replay, err := readSIVIDeletionReplay(ctx, request, before)
			if err != nil {
				return nil, false, err
			}
			if replay != nil {
				return SiviDeletionReceipt(*replay, contextID, false), false, nil
			}
			if request.ContextID != contextID {
				return nil, false, errors.New("SIVI deletion new request differs from the current context owner")
			}
			if plots.CurrentUser == "" || plots.AuditStrength < 0 || plots.AuditStrength > 3 {
				return nil, false, errors.New("SIVI deletion requires explicit valid audit provenance")
			}
			if err := vprocore.ValidateChildPhysicalText("SIVI deletion actor", plots.CurrentUser, 100); err != nil {
				return nil, false, err
			}
			veg := before[request.Project+"_Veg"]
			original, err := SiviDeletionSource(ctx, request.Project, request.Plot, request.Form, request.Original.RowID, veg)
			if err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(original.Columns, request.Columns) || !reflect.DeepEqual(original.Original, request.Original) {
				return nil, false, errors.New("SIVI deletion full typed original/schema changed; cancel and reload")
			}
			id, err := ValidateSIVIDeletionOriginal(*original)
			if err != nil {
				return nil, false, err
			}
			index, err := vprocore.SiteUnitTransferColumns(veg, "ID")
			if err != nil {
				return nil, false, err
			}
			baseline, ledger, err := reserveSIVIIdentityPlan(ctx, tx, request.Project, before,
				[]SiviHeightAssignment{{Before: request.Original.Cells[index["ID"]], After: vprocore.ProjectMetadataCell{Storage: "null"}}})
			if err != nil {
				return nil, false, err
			}
			deleted, err := tx.ExecContext(ctx, `DELETE FROM `+vprocore.QuoteHeaderIdentifier(request.Project+"_Veg")+
				` WHERE rowid=? AND PlotNumber COLLATE BINARY=?`, request.Original.RowID, request.Plot)
			if err != nil {
				return nil, false, err
			}
			if count, err := deleted.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI deletion did not delete exactly one physical row"))
			}
			when := time.Now().Format("2006-01-02 15:04:05")
			fields, values, nulls := []vprocore.ChildField{}, []any{}, []any{}
			for i, column := range veg.Columns {
				if column.Name == "ID" || column.Name == "PlotNumber" {
					continue
				}
				value, err := SiviDeletionAuditValue(column, request.Original.Cells[i])
				if err != nil {
					return nil, false, err
				}
				fields, values, nulls = append(fields, vprocore.ChildField{Column: column.Name}), append(values, value), append(nulls, nil)
			}
			audits, err := vprocore.AuditChildFieldsTracked(tx, request.Project, "Veg", request.Plot, id, fields, values, nulls,
				plots.CurrentUser, plots.AuditStrength, when)
			if err != nil {
				return nil, false, err
			}
			records := []vprocore.AuditEntry{}
			for _, audit := range audits {
				ids, err := vprocore.AuditRowIDs([]string{audit.RowID})
				if err != nil {
					return nil, false, err
				}
				selected, err := vprocore.SelectedAuditEntries(tx, request.Project, request.Plot, ids)
				if err != nil || len(selected) != 1 {
					return nil, false, errors.Join(err, errors.New("SIVI deletion audit identity differs"))
				}
				records = append(records, selected[0])
			}
			plannedVeg := vprocore.ProjectMetadataTable{Columns: veg.Columns, Rows: []vprocore.ProjectMetadataRow{}}
			for _, row := range veg.Rows {
				if row.RowID != request.Original.RowID {
					plannedVeg.Rows = append(plannedVeg.Rows, row)
				}
			}
			auditPlan, err := AppendSIVIPlannedAudits(before[request.Project+"_Audit"], records)
			if err != nil {
				return nil, false, err
			}
			expected := map[string]vprocore.ProjectMetadataTable{request.Project + "_Veg": plannedVeg, request.Project + "_Audit": auditPlan, SiviIdentityLedger: ledger}
			observed, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := vprocore.VerifySiteUnitTransferTables(baseline, observed, expected, ""); err != nil {
				return nil, false, err
			}
			history := SiviDeletionHistory{Request: request,
				Result: SiviCreationIdentity{request.RequestID, request.Project, request.Plot, request.Original.RowID, request.RequestID, id},
				Actor:  plots.CurrentUser, When: when, AuditStrength: plots.AuditStrength, Columns: veg.Columns, Original: request.Original, Audits: records}
			if err := ValidateSIVIDeletionHistory(ctx, history); err != nil {
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
			historyPlan := before[SiviDeletionHistoryTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []vprocore.ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]vprocore.ProjectMetadataRow{}, historyPlan.Rows...), vprocore.ProjectMetadataRow{
				RowID: strconv.FormatInt(rowID, 10), Cells: []vprocore.ProjectMetadataCell{SiviCreationText(request.RequestID), SiviCreationText(when), SiviCreationText(string(proposal))}})
			observed, err = vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := vprocore.VerifySiteUnitTransferTables(baseline, observed, expected, SiviDeletionHistoryTable); err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(observed[SiviDeletionHistoryTable], historyPlan) {
				return nil, false, errors.New("SIVI deletion durable history differs from its typed plan")
			}
			verified, err := readSIVIDeletionReplay(ctx, request, observed)
			if err != nil || verified == nil {
				return nil, false, errors.Join(err, errors.New("SIVI deletion durable receipt verification failed"))
			}
			return SiviDeletionReceipt(*verified, contextID, true), true, nil
		})
}
