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

const siviCreationUndoTable = "__VPRO_SIVICreationUndoHistory"
const siviCreationUndoSQL = `CREATE TABLE "__VPRO_SIVICreationUndoHistory"(RequestID TEXT NOT NULL PRIMARY KEY,CreationID TEXT NOT NULL UNIQUE,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`

type siviCreationUndoRequest struct {
	RequestID string             `json:"requestId"`
	ContextID string             `json:"contextId"`
	Project   string             `json:"project"`
	Plot      string             `json:"plot"`
	HistoryID string             `json:"historyId"`
	Action    AuditRestoreAction `json:"action"`
	Expected  string             `json:"expected"`
}

func (request *siviCreationUndoRequest) UnmarshalJSON(data []byte) error {
	keys := []string{"requestId", "contextId", "project", "plot", "historyId", "action", "expected"}
	properties, err := sourceChildJSONObject(data, "SIVI creation Undo", keys, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "SIVI creation Undo", keys...); err != nil {
		return err
	}
	type plain siviCreationUndoRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validateSIVICreationUndoRequest(siviCreationUndoRequest(decoded)); err != nil {
		return err
	}
	*request = siviCreationUndoRequest(decoded)
	return nil
}

func validateSIVICreationUndoRequest(request siviCreationUndoRequest) error {
	// The transport and scope rules are deliberately shared with typed undelete.
	return validateSIVIDeletionRestorationRequest(siviDeletionRestorationRequest(request))
}

type siviCreationUndoReview struct {
	ContextID    string                  `json:"contextId"`
	Project      string                  `json:"project"`
	Plot         string                  `json:"plot"`
	HistoryID    string                  `json:"historyId"`
	Expected     string                  `json:"expected"`
	Form         string                  `json:"form"`
	RowID        string                  `json:"rowId"`
	ID           int64                   `json:"id"`
	Columns      []ProjectMetadataColumn `json:"columns"`
	Committed    ProjectMetadataRow      `json:"committed"`
	Creation     siviCreationResult      `json:"creation"`
	AuditColumns []ProjectMetadataColumn `json:"auditColumns"`
	AuditsBefore []ProjectMetadataRow    `json:"auditsBefore"`
}

type siviCreationUndoResult struct {
	RequestID       string                  `json:"requestId"`
	ContextID       string                  `json:"contextId"`
	Project         string                  `json:"project"`
	Plot            string                  `json:"plot"`
	HistoryID       string                  `json:"historyId"`
	Expected        string                  `json:"expected"`
	UndoID          string                  `json:"undoId"`
	Form            string                  `json:"form"`
	RowID           string                  `json:"rowId"`
	ID              int64                   `json:"id"`
	Action          AuditRestoreAction      `json:"action"`
	Actor           string                  `json:"actor"`
	AuditStrength   int                     `json:"auditStrength"`
	EditWhen        string                  `json:"editWhen"`
	Columns         []ProjectMetadataColumn `json:"columns"`
	Committed       ProjectMetadataRow      `json:"committed"`
	Creation        siviCreationResult      `json:"creation"`
	Request         siviCreationUndoRequest `json:"request"`
	AuditColumns    []ProjectMetadataColumn `json:"auditColumns"`
	AuditsBefore    []ProjectMetadataRow    `json:"auditsBefore"`
	AuditsAfter     []ProjectMetadataRow    `json:"auditsAfter"`
	Cancelled       bool                    `json:"cancelled"`
	RemovedRows     int                     `json:"removedRows"`
	PrunedAuditRows int                     `json:"prunedAuditRows"`
	DidCommit       bool                    `json:"didCommit"`
	Replayed        bool                    `json:"replayed"`
}

type siviCreationUndoHistory struct {
	Request       siviCreationUndoRequest
	Creation      siviCreationHistory
	Actor, When   string
	AuditStrength int
	AuditColumns  []ProjectMetadataColumn
	AuditsBefore  []ProjectMetadataRow
	AuditsAfter   []ProjectMetadataRow
}

func siviCreationUndoExpected(creation siviCreationHistory) (string, error) {
	raw, err := json.Marshal(creation)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// This validates immutable facts, not the accepted creation receipt's current
// eligibility. In particular, it does not require today's row or source audits.
func validateSIVICreationUndoCreation(ctx context.Context, history siviCreationHistory) error {
	request := history.Request
	if request.RequestID == "" || request.ContextID == "" ||
		history.Result != (siviCreationIdentity{request.RequestID, request.Project, request.Plot, history.Committed.RowID, request.RequestID, history.Result.ID}) ||
		history.Result.ID <= 0 || history.Result.ID > 2147483647 || history.Audits == nil ||
		history.Actor == "" || history.AuditStrength < 0 || history.AuditStrength > 3 {
		return errors.New("SIVI creation Undo immutable creation identity/provenance differs")
	}
	if err := validateChildPhysicalText("SIVI creation context", request.ContextID, 100); err != nil {
		return err
	}
	if err := validateChildPhysicalText("SIVI creation request", request.RequestID, 100); err != nil {
		return err
	}
	if _, err := validateSIVIDeletionOriginal(siviDeletionOriginal{ContextID: request.ContextID, Project: request.Project,
		Plot: request.Plot, Form: request.Form, Columns: history.Columns, Original: history.Committed}); err != nil {
		return err
	}
	veg := ProjectMetadataTable{Columns: history.Columns, Rows: []ProjectMetadataRow{history.Committed}}
	auditColumns := []ProjectMetadataColumn{
		{Name: "Project", DeclaredType: "TEXT"}, {Name: "User", DeclaredType: "TEXT"}, {Name: "PlotNumber", DeclaredType: "TEXT"},
		{Name: "Table", DeclaredType: "TEXT"}, {Name: "EditField", DeclaredType: "TEXT"}, {Name: "EditWhen", DeclaredType: "DATETIME"},
		{Name: "BeforeEdit", DeclaredType: "MEMO"}, {Name: "AfterEdit", DeclaredType: "MEMO"},
		{Name: "Restore", DeclaredType: "BOOLEAN"}, {Name: "Flag", DeclaredType: "BOOLEAN"}, {Name: "ID", DeclaredType: "LONG"},
	}
	audit, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: auditColumns}, history.Audits)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, row := range audit.Rows {
		id, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != row.RowID || seen[row.RowID] {
			return errors.New("SIVI creation Undo source audit identity repeats or is malformed")
		}
		seen[row.RowID] = true
	}
	ledger := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "ChildTable", DeclaredType: "TEXT"}, {Name: "ID", DeclaredType: "INTEGER"}},
		Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{
			siviCreationText(quoteHeaderIdentifier(request.Project + "_Veg")), siviCreationInteger(strconv.FormatInt(history.Result.ID, 10)),
		}}},
	}
	if len(request.Covers) != 0 {
		return verifySIVICreationHistory(ctx, request, history, map[string]ProjectMetadataTable{
			request.Project + "_Veg": veg, request.Project + "_Audit": audit, siviIdentityLedger: ledger,
		})
	}
	// An all-NULL historical creation must itself own complete canonical all44
	// evidence. Clearing covers after a normal creation is NOT this authority.
	// No USysVeg membership or CleanVegPlot deletion is fabricated.
	if err := validateChildPhysicalText("SIVI creation Species", request.Species, 8); err != nil || request.Species == "" {
		return errors.Join(err, errors.New("SIVI creation Undo historical species differs"))
	}
	if request.Decision != nil && request.Decision.Kind == "" {
		return errors.New("SIVI creation Undo historical decision is not explicit")
	}
	edit := request.speciesEdit()
	if err := validateVegetationSpeciesDecision((VegetationCreationRequest{Form: edit.Form, Species: edit.Value,
		Decision: edit.Decision, Entered: edit.Entered, Selected: edit.Selected}).speciesUpdate()); err != nil {
		return err
	}
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil || len(columns) != 44 {
		return errors.Join(err, errors.New("SIVI creation Undo historical schema is not complete44"))
	}
	original := ProjectMetadataRow{Cells: make([]ProjectMetadataCell, 44)}
	for i := range original.Cells {
		original.Cells[i] = ProjectMetadataCell{Storage: "null"}
	}
	original.Cells[columns["Flag"]] = siviCreationInteger("0")
	committed := ProjectMetadataRow{RowID: history.Result.RowID, Cells: append([]ProjectMetadataCell{}, original.Cells...)}
	committed.Cells[columns["PlotNumber"]] = siviCreationText(request.Plot)
	committed.Cells[columns["Species"]] = siviCreationText(request.Species)
	committed.Cells[columns["ID"]] = siviCreationInteger(strconv.FormatInt(history.Result.ID, 10))
	if !reflect.DeepEqual(original, history.Original) || !reflect.DeepEqual(committed, history.Committed) {
		return errors.New("SIVI creation Undo historical NULL creation is not its complete typed buffer")
	}
	if err := validateChildPhysicalText("SIVI creation actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI creation Undo historical timestamp differs")
	}
	expectedSpecies := auditHeaderChange(nil, request.Species, history.AuditStrength)
	if len(history.Audits) != boolSIVICreationUndoCount(expectedSpecies) {
		return errors.New("SIVI creation Undo historical source audits are incomplete")
	}
	for _, record := range history.Audits {
		if record.Project != request.Project || record.PlotNumber != request.Plot || record.Table != "_Veg" ||
			record.User != history.Actor || record.EditWhen != history.When || record.EditField != "Species" ||
			record.ID == nil || *record.ID != history.Result.ID || record.BeforeEdit != nil ||
			!metadataAuditTextEqual(record.AfterEdit, request.Species) || record.Restore || record.Flag {
			return errors.New("SIVI creation Undo historical source audit authority differs")
		}
	}
	return ctx.Err()
}

func boolSIVICreationUndoCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func readSIVICreationUndoCreations(ctx context.Context, tables map[string]ProjectMetadataTable) (map[string]siviCreationHistory, error) {
	result := map[string]siviCreationHistory{}
	table, present := tables[siviCreationHistoryTable]
	if !present {
		return result, ctx.Err()
	}
	expected := []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
	if _, err := siteUnitTransferColumns(table, "RequestID", "Created", "Proposal"); err != nil || !reflect.DeepEqual(table.Columns, expected) {
		return nil, errors.Join(err, errors.New("SIVI creation Undo creation history schema differs"))
	}
	schema := tables["sqlite_master"]
	index, err := siteUnitTransferColumns(schema, "name", "type", "tbl_name", "sql")
	if err != nil {
		return nil, err
	}
	defined := false
	const definition = `CREATE TABLE "__VPRO_SIVICreationHistory" ( RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
	for _, row := range schema.Rows {
		name, kind, owner, sqlText := row.Cells[index["name"]], row.Cells[index["type"]], row.Cells[index["tbl_name"]], row.Cells[index["sql"]]
		if owner.Text == nil || !strings.EqualFold(*owner.Text, siviCreationHistoryTable) {
			continue
		}
		if name.Text != nil && *name.Text == siviCreationHistoryTable && kind.Text != nil && *kind.Text == "table" &&
			sqlText.Text != nil && strings.Join(strings.Fields(*sqlText.Text), " ") == definition {
			defined = true
		} else if kind.Text == nil || *kind.Text != "index" || sqlText.Storage != "null" {
			return nil, errors.New("SIVI creation Undo creation history has unreviewed schema/index/trigger")
		}
	}
	if !defined {
		return nil, errors.New("SIVI creation Undo creation history lacks immutable keyed provenance schema")
	}
	logicalIDs := map[string]bool{}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("SIVI creation Undo creation history storage differs"))
			}
		}
		var history siviCreationHistory
		raw := *row.Cells[2].Text
		if err := decodeProfileLifecycleJSON([]byte(raw), &history, "Request", "Result", "Actor", "When", "AuditStrength", "Columns", "Original", "Committed", "Audits"); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != raw || *row.Cells[0].Text != history.Request.RequestID || *row.Cells[1].Text != history.When {
			return nil, errors.Join(err, errors.New("SIVI creation Undo canonical history key differs"))
		}
		if err := validateSIVICreationUndoCreation(ctx, history); err != nil {
			return nil, err
		}
		if _, duplicate := result[history.Request.RequestID]; duplicate {
			return nil, errors.New("SIVI creation Undo creation history UUID is ambiguous")
		}
		logicalID := history.Request.Project + "\x00" + strconv.FormatInt(history.Result.ID, 10)
		if logicalIDs[logicalID] {
			return nil, errors.New("SIVI creation Undo multiple creation events own the same permanent logical ID")
		}
		logicalIDs[logicalID] = true
		result[history.Request.RequestID] = history
	}
	return result, ctx.Err()
}

func validateSIVICreationUndoHistory(ctx context.Context, history siviCreationUndoHistory, creations map[string]siviCreationHistory) error {
	request := history.Request
	if err := validateSIVICreationUndoRequest(request); err != nil {
		return err
	}
	creation, present := creations[request.HistoryID]
	expected, err := siviCreationUndoExpected(creation)
	if err != nil || !present || !reflect.DeepEqual(creation, history.Creation) || expected != request.Expected ||
		request.Project != creation.Request.Project || request.Plot != creation.Request.Plot ||
		request.Action == AuditRestoreCancel || history.Actor == "" || history.AuditStrength < 0 || history.AuditStrength > 3 ||
		history.AuditsBefore == nil || history.AuditsAfter == nil {
		return errors.Join(err, errors.New("SIVI creation Undo immutable authority/evidence differs"))
	}
	if _, reused := creations[request.RequestID]; reused {
		return errors.New("SIVI creation Undo request UUID already identifies a creation")
	}
	if err := validateChildPhysicalText("SIVI creation Undo actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI creation Undo timestamp differs")
	}
	before := ProjectMetadataTable{Columns: history.AuditColumns, Rows: history.AuditsBefore}
	if _, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: history.AuditColumns}, nil); err != nil {
		return err
	}
	if len(before.Rows) != len(creation.Audits) {
		return errors.New("SIVI creation Undo source audit evidence is incomplete")
	}
	for _, record := range creation.Audits {
		if err := verifySIVIPhysicalAudit(before, record, request.Project); err != nil {
			return err
		}
	}
	planned, err := planSIVIDeletionRestorationAudits(before, creation.Audits, request.Action)
	if err != nil || !reflect.DeepEqual(planned.Rows, history.AuditsAfter) {
		return errors.Join(err, errors.New("SIVI creation Undo typed retain/prune plan differs"))
	}
	return ctx.Err()
}

func readSIVICreationUndos(ctx context.Context, tables map[string]ProjectMetadataTable, creations map[string]siviCreationHistory) (map[string]siviCreationUndoHistory, error) {
	result := map[string]siviCreationUndoHistory{}
	table, present := tables[siviCreationUndoTable]
	if !present {
		return result, ctx.Err()
	}
	columns := []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "CreationID", DeclaredType: "TEXT"},
		{Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
	if _, err := siteUnitTransferColumns(table, "RequestID", "CreationID", "Created", "Proposal"); err != nil || !reflect.DeepEqual(columns, table.Columns) {
		return nil, errors.Join(err, errors.New("SIVI creation Undo history schema differs"))
	}
	schema := tables["sqlite_master"]
	index, err := siteUnitTransferColumns(schema, "name", "type", "tbl_name", "sql")
	if err != nil {
		return nil, err
	}
	defined := false
	for _, row := range schema.Rows {
		name, kind, owner, definition := row.Cells[index["name"]], row.Cells[index["type"]], row.Cells[index["tbl_name"]], row.Cells[index["sql"]]
		if owner.Text == nil || !strings.EqualFold(*owner.Text, siviCreationUndoTable) {
			continue
		}
		if kind.Text != nil && *kind.Text == "table" && name.Text != nil && *name.Text == siviCreationUndoTable &&
			definition.Text != nil && strings.Join(strings.Fields(*definition.Text), " ") == strings.Join(strings.Fields(siviCreationUndoSQL), " ") {
			defined = true
		} else if kind.Text == nil || *kind.Text != "index" || definition.Storage != "null" {
			return nil, errors.New("SIVI creation Undo history has an unreviewed index/trigger/definition")
		}
	}
	if !defined {
		return nil, errors.New("SIVI creation Undo history lacks unique immutable consumption schema")
	}
	consumed := map[string]bool{}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("SIVI creation Undo history storage differs"))
			}
		}
		var history siviCreationUndoHistory
		raw := *row.Cells[3].Text
		if err := decodeProfileLifecycleJSON([]byte(raw), &history, "Request", "Creation", "Actor", "When", "AuditStrength", "AuditColumns", "AuditsBefore", "AuditsAfter"); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != raw || *row.Cells[0].Text != history.Request.RequestID ||
			*row.Cells[1].Text != history.Request.HistoryID || *row.Cells[2].Text != history.When {
			return nil, errors.Join(err, errors.New("SIVI creation Undo history canonical key differs"))
		}
		if err := validateSIVICreationUndoHistory(ctx, history, creations); err != nil {
			return nil, err
		}
		if _, duplicate := result[history.Request.RequestID]; duplicate || consumed[history.Request.HistoryID] {
			return nil, errors.New("SIVI creation Undo history repeats a request or consumes a creation twice")
		}
		result[history.Request.RequestID], consumed[history.Request.HistoryID] = history, true
	}
	return result, ctx.Err()
}

func verifySIVICreationUndoIdentity(ctx context.Context, creation siviCreationHistory, tables map[string]ProjectMetadataTable, removed bool) error {
	// Reuse exact raw identity, signed32 occupancy and permanent reservation checks.
	deletion := siviDeletionHistory{Request: siviDeletionRequest{Project: creation.Request.Project},
		Result: creation.Result, Columns: creation.Columns, Original: creation.Committed}
	return verifySIVIDeletionRestorationIdentity(ctx, deletion, tables, !removed)
}

func siviCreationUndoReviewFromTables(ctx context.Context, contextID, project, plot, historyID string, tables map[string]ProjectMetadataTable) (*siviCreationUndoReview, siviCreationHistory, error) {
	creations, err := readSIVICreationUndoCreations(ctx, tables)
	if err != nil {
		return nil, siviCreationHistory{}, err
	}
	undos, err := readSIVICreationUndos(ctx, tables, creations)
	if err != nil {
		return nil, siviCreationHistory{}, err
	}
	creation, present := creations[historyID]
	if !present || creation.Request.Project != project || creation.Request.Plot != plot {
		return nil, siviCreationHistory{}, errors.New("SIVI creation Undo history missing or owned by another literal project/parent")
	}
	for _, undo := range undos {
		if undo.Request.HistoryID == historyID {
			return nil, siviCreationHistory{}, errors.New("SIVI creation was already consumed by an Undo")
		}
	}
	if err := verifySIVICreationUndoIdentity(ctx, creation, tables, false); err != nil {
		return nil, siviCreationHistory{}, err
	}
	audit := tables[project+"_Audit"]
	if _, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: audit.Columns}, nil); err != nil {
		return nil, siviCreationHistory{}, err
	}
	rows := []ProjectMetadataRow{}
	for _, record := range creation.Audits {
		if err := verifySIVIPhysicalAudit(audit, record, project); err != nil {
			return nil, siviCreationHistory{}, err
		}
		// Exact physical row ownership is necessary but not sufficient: a copied
		// source audit at another rowID is ambiguous creation authority.
		matches := 0
		for _, candidate := range audit.Rows {
			candidate.RowID = record.RowID
			if verifySIVIPhysicalAudit(ProjectMetadataTable{Columns: audit.Columns, Rows: []ProjectMetadataRow{candidate}}, record, project) == nil {
				matches++
			}
		}
		if matches != 1 {
			return nil, siviCreationHistory{}, errors.New("SIVI creation Undo source audit authority has a duplicate occupant")
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
	expected, err := siviCreationUndoExpected(creation)
	return &siviCreationUndoReview{ContextID: contextID, Project: project, Plot: plot, HistoryID: historyID,
		Expected: expected, Form: creation.Request.Form, RowID: creation.Result.RowID, ID: creation.Result.ID,
		Columns: creation.Columns, Committed: creation.Committed, Creation: *siviCreationReceipt(creation, contextID, false),
		AuditColumns: audit.Columns, AuditsBefore: rows}, creation, err
}

func siviCreationUndoReceipt(history siviCreationUndoHistory, contextID string, committed bool) *siviCreationUndoResult {
	creation := history.Creation
	pruned := 0
	if history.Request.Action == AuditRestorePrune {
		pruned = len(history.AuditsBefore)
	}
	return &siviCreationUndoResult{RequestID: history.Request.RequestID, ContextID: contextID, Project: history.Request.Project,
		Plot: history.Request.Plot, HistoryID: history.Request.HistoryID, Expected: history.Request.Expected, UndoID: history.Request.RequestID,
		Form: creation.Request.Form, RowID: creation.Result.RowID, ID: creation.Result.ID, Action: history.Request.Action,
		Actor: history.Actor, AuditStrength: history.AuditStrength, EditWhen: history.When, Columns: creation.Columns,
		Committed: creation.Committed, Creation: *siviCreationReceipt(creation, contextID, false), Request: history.Request,
		AuditColumns: history.AuditColumns, AuditsBefore: history.AuditsBefore, AuditsAfter: history.AuditsAfter,
		RemovedRows: 1, PrunedAuditRows: pruned, DidCommit: committed, Replayed: !committed}
}

func verifySIVICreationUndoReplay(ctx context.Context, request siviCreationUndoRequest, history siviCreationUndoHistory, tables map[string]ProjectMetadataTable) error {
	caller, durable := request, history.Request
	caller.ContextID, durable.ContextID = "", ""
	if caller != durable {
		return errors.New("SIVI creation Undo receipt request differs")
	}
	if err := verifySIVICreationUndoIdentity(ctx, history.Creation, tables, true); err != nil {
		return err
	}
	audit := tables[request.Project+"_Audit"]
	if !reflect.DeepEqual(audit.Columns, history.AuditColumns) {
		return errors.New("SIVI creation Undo receipt audit schema changed")
	}
	after := map[string]ProjectMetadataRow{}
	for _, row := range history.AuditsAfter {
		after[row.RowID] = row
	}
	for _, original := range history.AuditsBefore {
		expected, retained := after[original.RowID]
		found := false
		for _, actual := range audit.Rows {
			if actual.RowID == original.RowID {
				if !retained || !reflect.DeepEqual(actual, expected) {
					return errors.New("SIVI creation Undo retained/pruned source audit changed or rowID was reused")
				}
				found = true
			}
		}
		if found != retained {
			return errors.New("SIVI creation Undo retained source audit disappeared")
		}
	}
	return ctx.Err()
}

func readSIVICreationUndoTables(ctx context.Context, tx *sql.Tx, project string) (map[string]ProjectMetadataTable, error) {
	tables, err := readSIVIDeletionRestorationTables(ctx, tx, project)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{siviCreationHistoryTable, siviCreationUndoTable} {
		present := false
		for _, row := range tables["sqlite_master"].Rows {
			present = present || row.Cells[0].Text != nil && *row.Cells[0].Text == "table" && row.Cells[1].Text != nil && *row.Cells[1].Text == name
		}
		if present {
			table, err := readSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
			if err != nil {
				return nil, err
			}
			tables[name] = table
		}
	}
	return tables, nil
}

func (s *ContextService) reviewSIVICreationUndo(ctx context.Context, contextID, plot, historyID string) (*siviCreationUndoReview, error) {
	if s == nil || ctx == nil || !siviDeletionUUID.MatchString(historyID) {
		return nil, errors.New("SIVI creation Undo review requires an owned context and lowercase creation UUID")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviCreationUndoReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviCreationUndoReview, error) {
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, owner.selection.Project, plot); err != nil {
				return nil, err
			}
			tables, err := readSIVICreationUndoTables(ctx, tx, owner.selection.Project)
			if err != nil {
				return nil, err
			}
			review, _, err := siviCreationUndoReviewFromTables(ctx, contextID, owner.selection.Project, plot, historyID, tables)
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

func (s *ContextService) lookupSIVICreationUndoReceipt(ctx context.Context, contextID string, request siviCreationUndoRequest) (*siviCreationUndoResult, error) {
	if s == nil || ctx == nil {
		return nil, errors.New("SIVI creation Undo lookup requires an owned context")
	}
	if err := validateSIVICreationUndoRequest(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviCreationUndoResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviCreationUndoResult, error) {
			if request.Project != owner.selection.Project {
				return nil, errors.New("SIVI creation Undo lookup belongs to another project")
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, request.Project, request.Plot); err != nil {
				return nil, err
			}
			tables, err := readSIVICreationUndoTables(ctx, tx, request.Project)
			if err != nil {
				return nil, err
			}
			creations, err := readSIVICreationUndoCreations(ctx, tables)
			if err != nil {
				return nil, err
			}
			undos, err := readSIVICreationUndos(ctx, tables, creations)
			if err != nil {
				return nil, err
			}
			history, present := undos[request.RequestID]
			if !present {
				return nil, nil
			}
			if err := verifySIVICreationUndoReplay(ctx, request, history, tables); err != nil {
				return nil, err
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			return siviCreationUndoReceipt(history, contextID, false), nil
		})
	})
}

// Exact owned creation Undo is a desktop safety adaptation. It never inherits
// Access's absent-row/Cover skips or destructive whole-plot CleanVegPlot.
func (s *ContextService) undoSIVICreation(ctx context.Context, contextID string, request siviCreationUndoRequest) (*siviCreationUndoResult, error) {
	if s == nil || ctx == nil {
		return nil, errors.New("SIVI creation Undo requires an owned context")
	}
	if err := validateSIVICreationUndoRequest(request); err != nil {
		return nil, err
	}
	if request.Action == AuditRestoreCancel {
		if contextID != request.ContextID {
			return nil, errors.New("SIVI creation Undo cancel differs from current owner")
		}
		review, err := s.reviewSIVICreationUndo(ctx, contextID, request.Plot, request.HistoryID)
		if err != nil {
			return nil, err
		}
		if review.Project != request.Project || review.Expected != request.Expected {
			return nil, errors.New("SIVI creation Undo cancellation evidence/scope changed")
		}
		return &siviCreationUndoResult{RequestID: request.RequestID, ContextID: contextID, Project: request.Project,
			Plot: request.Plot, HistoryID: request.HistoryID, Expected: request.Expected, Form: review.Form, RowID: review.RowID,
			ID: review.ID, Action: request.Action, Columns: review.Columns, Committed: review.Committed,
			Creation: review.Creation, Request: request, AuditColumns: review.AuditColumns, AuditsBefore: review.AuditsBefore,
			AuditsAfter: review.AuditsBefore, Cancelled: true}, nil
	}
	return withSIVIIdentityWriter(ctx, s, contextID, request.Plot, "creation Undo",
		func(plots *PlotService, owner *sqliteContext, tx *sql.Tx) (*siviCreationUndoResult, bool, error) {
			if request.Project != owner.selection.Project {
				return nil, false, errors.New("SIVI creation Undo belongs to another project")
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			creations, err := readSIVICreationUndoCreations(ctx, before)
			if err != nil {
				return nil, false, err
			}
			undos, err := readSIVICreationUndos(ctx, before, creations)
			if err != nil {
				return nil, false, err
			}
			if history, present := undos[request.RequestID]; present {
				if err := verifySIVICreationUndoReplay(ctx, request, history, before); err != nil {
					return nil, false, err
				}
				return siviCreationUndoReceipt(history, contextID, false), false, nil
			}
			if request.ContextID != contextID {
				return nil, false, errors.New("SIVI creation Undo new request differs from current owner")
			}
			if _, reused := creations[request.RequestID]; reused {
				return nil, false, errors.New("SIVI creation Undo request UUID already identifies a creation")
			}
			review, creation, err := siviCreationUndoReviewFromTables(ctx, contextID, request.Project, request.Plot, request.HistoryID, before)
			if err != nil {
				return nil, false, err
			}
			if review.Expected != request.Expected {
				return nil, false, errors.New("SIVI creation Undo reviewed immutable evidence changed; reload")
			}
			if plots.currentUser == "" || plots.auditStrength < 0 || plots.auditStrength > 3 {
				return nil, false, errors.New("SIVI creation Undo requires explicit actor/audit strength")
			}
			if err := validateChildPhysicalText("SIVI creation Undo actor", plots.currentUser, 100); err != nil {
				return nil, false, err
			}
			removed, err := tx.ExecContext(ctx, `DELETE FROM `+quoteHeaderIdentifier(request.Project+"_Veg")+` WHERE rowid=?`, review.RowID)
			if err != nil {
				return nil, false, err
			}
			if count, err := removed.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI creation Undo did not remove exactly one physical row"))
			}
			auditPlan, err := planSIVIDeletionRestorationAudits(before[request.Project+"_Audit"], creation.Audits, request.Action)
			if err != nil {
				return nil, false, err
			}
			for _, audit := range creation.Audits {
				query := `UPDATE ` + quoteHeaderIdentifier(request.Project+"_Audit") + ` SET Restore=-1 WHERE rowid=?`
				if request.Action == AuditRestorePrune {
					query = `DELETE FROM ` + quoteHeaderIdentifier(request.Project+"_Audit") + ` WHERE rowid=?`
				}
				changed, err := tx.ExecContext(ctx, query, audit.RowID)
				if err != nil {
					return nil, false, err
				}
				if count, err := changed.RowsAffected(); err != nil || count != 1 {
					return nil, false, errors.Join(err, errors.New("SIVI creation Undo did not retain/prune exactly one source audit"))
				}
			}
			vegPlan := ProjectMetadataTable{Columns: review.Columns, Rows: []ProjectMetadataRow{}}
			for _, row := range before[request.Project+"_Veg"].Rows {
				if row.RowID != review.RowID {
					vegPlan.Rows = append(vegPlan.Rows, row)
				}
			}
			expected := map[string]ProjectMetadataTable{request.Project + "_Veg": vegPlan, request.Project + "_Audit": auditPlan}
			observed, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(before, observed, expected, ""); err != nil {
				return nil, false, err
			}
			evidence, err := planSIVIDeletionRestorationAudits(ProjectMetadataTable{Columns: review.AuditColumns, Rows: review.AuditsBefore}, creation.Audits, request.Action)
			if err != nil {
				return nil, false, err
			}
			history := siviCreationUndoHistory{Request: request, Creation: creation, Actor: plots.currentUser,
				When: time.Now().Format("2006-01-02 15:04:05"), AuditStrength: plots.auditStrength,
				AuditColumns: review.AuditColumns, AuditsBefore: review.AuditsBefore, AuditsAfter: evidence.Rows}
			if err := validateSIVICreationUndoHistory(ctx, history, creations); err != nil {
				return nil, false, err
			}
			if err := verifySIVICreationUndoReplay(ctx, request, history, observed); err != nil {
				return nil, false, err
			}
			raw, err := json.Marshal(history)
			if err != nil {
				return nil, false, err
			}
			if _, present := before[siviCreationUndoTable]; !present {
				if _, err := tx.ExecContext(ctx, siviCreationUndoSQL); err != nil {
					return nil, false, err
				}
			}
			entry, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_SIVICreationUndoHistory"(RequestID,CreationID,Created,Proposal) VALUES(?,?,?,?)`,
				request.RequestID, request.HistoryID, history.When, string(raw))
			if err != nil {
				return nil, false, err
			}
			if count, err := entry.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI creation Undo did not record exactly one consumption"))
			}
			rowID, err := entry.LastInsertId()
			if err != nil {
				return nil, false, err
			}
			historyPlan := before[siviCreationUndoTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "CreationID", DeclaredType: "TEXT"},
					{Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]ProjectMetadataRow{}, historyPlan.Rows...), ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10),
				Cells: []ProjectMetadataCell{siviCreationText(request.RequestID), siviCreationText(request.HistoryID), siviCreationText(history.When), siviCreationText(string(raw))}})
			observed, err = environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(before, observed, expected, siviCreationUndoTable); err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(observed[siviCreationUndoTable], historyPlan) {
				return nil, false, errors.New("SIVI creation Undo immutable typed history differs")
			}
			// Re-read original evidence too: coherent trigger rewrites must not
			// substitute a different valid creation after the reviewed SHA check.
			currentCreations, err := readSIVICreationUndoCreations(ctx, observed)
			if err != nil || !reflect.DeepEqual(currentCreations, creations) {
				return nil, false, errors.Join(err, errors.New("SIVI creation Undo immutable evidence drifted in transaction"))
			}
			verified, err := readSIVICreationUndos(ctx, observed, currentCreations)
			if err != nil {
				return nil, false, err
			}
			durable, present := verified[request.RequestID]
			if !present || !reflect.DeepEqual(durable, history) {
				return nil, false, fmt.Errorf("SIVI creation Undo durable receipt missing or differs")
			}
			return siviCreationUndoReceipt(durable, contextID, true), true, nil
		})
}
