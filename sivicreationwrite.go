package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const siviCreationHistoryTable = "__VPRO_SIVICreationHistory"

type siviCreationCover struct {
	Column string              `json:"column"`
	Value  ProjectMetadataCell `json:"value"`
}

type siviCreationDecision struct {
	Kind     string  `json:"kind"`
	Entered  string  `json:"entered"`
	Selected *string `json:"selected,omitempty"`
}

type siviCreationRequest struct {
	RequestID string                `json:"requestId"`
	ContextID string                `json:"contextId"`
	Project   string                `json:"project"`
	Plot      string                `json:"plot"`
	Form      string                `json:"form"`
	Species   string                `json:"species"`
	Decision  *siviCreationDecision `json:"decision,omitempty"`
	Covers    []siviCreationCover   `json:"covers"`
}

func (request *siviCreationRequest) UnmarshalJSON(data []byte) error {
	required := []string{"requestId", "contextId", "project", "plot", "form", "species", "covers"}
	properties, err := sourceChildJSONObject(data, "SIVI creation", required, []string{"decision"})
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "SIVI creation", append(required, "decision")...); err != nil {
		return err
	}
	if raw, present := properties["decision"]; present {
		decision, err := sourceChildJSONObject(raw, "SIVI creation decision", []string{"kind", "entered"}, []string{"selected"})
		if err != nil {
			return err
		}
		if err := sourceChildJSONNonNull(decision, "SIVI creation decision", "kind", "entered", "selected"); err != nil {
			return err
		}
	}
	if err := sourceChildArrayJSON(properties["covers"], func(raw json.RawMessage) error {
		cover, err := sourceChildJSONObject(raw, "SIVI creation cover", []string{"column", "value"}, nil)
		if err != nil {
			return err
		}
		if err := sourceChildJSONNonNull(cover, "SIVI creation cover", "column", "value"); err != nil {
			return err
		}
		return sourceChildCellJSON(cover["value"], "SIVI creation cover")
	}); err != nil {
		return err
	}
	type plain siviCreationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = siviCreationRequest(decoded)
	return nil
}

type siviCreationResult struct {
	RequestID     string                  `json:"requestId"`
	ContextID     string                  `json:"contextId"`
	Project       string                  `json:"project"`
	Plot          string                  `json:"plot"`
	Form          string                  `json:"form"`
	RowID         string                  `json:"rowId"`
	HistoryID     string                  `json:"historyId"`
	ID            int64                   `json:"id"`
	Actor         string                  `json:"actor"`
	AuditStrength int                     `json:"auditStrength"`
	EditWhen      string                  `json:"editWhen"`
	Columns       []ProjectMetadataColumn `json:"columns"`
	Original      ProjectMetadataRow      `json:"original"`
	Committed     ProjectMetadataRow      `json:"committed"`
	Covers        []siviCreationCover     `json:"covers"`
	Request       siviCreationRequest     `json:"request"`
	DidCommit     bool                    `json:"didCommit"`
	Replayed      bool                    `json:"replayed"`
}

type siviCreationIdentity struct {
	RequestID, Project, Plot, RowID, HistoryID string
	ID                                         int64
}

type siviCreationHistory struct {
	Request       siviCreationRequest
	Result        siviCreationIdentity
	Actor, When   string
	AuditStrength int
	Columns       []ProjectMetadataColumn
	Original      ProjectMetadataRow
	Committed     ProjectMetadataRow
	Audits        []AuditEntry
}

func siviCreationReceipt(history siviCreationHistory, contextID string, didCommit bool) *siviCreationResult {
	// didCommit describes this invocation; a verified lookup/retry only acknowledges the durable commit.
	return &siviCreationResult{
		RequestID: history.Result.RequestID, ContextID: contextID, Project: history.Result.Project,
		Plot: history.Result.Plot, Form: history.Request.Form, RowID: history.Result.RowID,
		HistoryID: history.Result.HistoryID, ID: history.Result.ID, Actor: history.Actor,
		AuditStrength: history.AuditStrength, EditWhen: history.When, Columns: history.Columns,
		Original: history.Original, Committed: history.Committed, Covers: history.Request.Covers,
		Request: history.Request, DidCommit: didCommit, Replayed: !didCommit,
	}
}

var siviCreationColumns = []string{
	"PlotNumber", "Species", "Layer", "Cover1", "Height1", "Cover2", "Height2", "Cover3", "Height3",
	"TotalA", "HeightA", "Cover4", "Height4", "Cover5", "Height5", "Cover5a", "Height5a",
	"Cover5b", "Height5b", "Cover5c", "Height5c", "TotalB", "HeightB", "Cover6", "Height6",
	"Cover7", "Cover8", "Cover9", "Cover10", "Collected", "Flag", "ID", "LL", "AF", "DC",
	"UT", "VI", "PV", "PG", "FFA", "Cultural1", "Cultural2", "Other1", "Other2",
}

func siviCreationInteger(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "integer", Integer: &value}
}

func siviCreationText(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "text", Text: &value}
}

func (request siviCreationRequest) speciesEdit() SIVISpeciesEdit {
	edit := SIVISpeciesEdit{Form: request.Form, Value: request.Species}
	if request.Decision != nil {
		edit.Decision, edit.Entered, edit.Selected = request.Decision.Kind, &request.Decision.Entered, request.Decision.Selected
	}
	return edit
}

func validateSIVICreationRequest(request siviCreationRequest) error {
	for _, text := range []struct {
		name, value string
		maximum     int
	}{{"request ID", request.RequestID, 100}, {"context ID", request.ContextID, 100},
		{"project", request.Project, 255}, {"parent", request.Plot, 7}, {"Species", request.Species, 8}} {
		if text.value == "" {
			return fmt.Errorf("SIVI creation requires explicit nonempty %s", text.name)
		}
		if err := validateChildPhysicalText("SIVI creation "+text.name, text.value, text.maximum); err != nil {
			return err
		}
	}
	if !projectNamePattern.MatchString(request.Project) {
		return errors.New("SIVI creation requires an exact project name")
	}
	switch request.Form {
	case "SubVegA-SIVI", "SubVegA-SIVI_BC", "SubVegC-SIVI", "SubVegD-SIVI":
	default:
		return errors.New("unavailable SIVI creation form")
	}
	if request.Decision != nil && request.Decision.Kind == "" {
		return errors.New("SIVI creation decision must be explicit")
	}
	edit := request.speciesEdit()
	creation := VegetationCreationRequest{Form: edit.Form, Species: edit.Value, Decision: edit.Decision, Entered: edit.Entered, Selected: edit.Selected}
	if err := validateVegetationSpeciesDecision(creation.speciesUpdate()); err != nil {
		return err
	}
	if len(request.Covers) == 0 {
		return errors.New("SIVI creation needs a non-NULL source-view value; no cover or total is inferred")
	}
	seen := map[string]bool{}
	for _, cover := range request.Covers {
		if seen[cover.Column] {
			return errors.New("SIVI creation repeats a cover")
		}
		seen[cover.Column] = true
		if _, err := metadataCellValue(cover.Value); err != nil {
			return err
		}
		if cover.Value.Storage != "real" || cover.Value.Real == nil {
			return errors.New("SIVI creation omits NULL fields and requires explicit typed real covers")
		}
		if err := validateSIVICoverCell(cover.Column, cover.Value); err != nil {
			return err
		}
	}
	return nil
}

func planSIVICreation(ctx context.Context, request siviCreationRequest, veg ProjectMetadataTable, id int64) (ProjectMetadataRow, ProjectMetadataRow, error) {
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil || len(columns) != len(siviCreationColumns) {
		return ProjectMetadataRow{}, ProjectMetadataRow{}, errors.Join(err, errors.New("SIVI creation requires all44 canonical fields without extra defaults"))
	}
	original := ProjectMetadataRow{Cells: make([]ProjectMetadataCell, len(columns))}
	for i := range original.Cells {
		original.Cells[i] = ProjectMetadataCell{Storage: "null"}
	}
	original.Cells[columns["Flag"]] = siviCreationInteger("0")
	created := ProjectMetadataRow{RowID: "1", Cells: append([]ProjectMetadataCell{}, original.Cells...)}
	created.Cells[columns["PlotNumber"]] = siviCreationText(request.Plot)
	created.Cells[columns["Species"]] = siviCreationText(request.Species)
	created.Cells[columns["ID"]] = siviCreationInteger(strconv.FormatInt(id, 10))
	if err := validateSIVIIdentityCell("ID", created.Cells[columns["ID"]]); err != nil {
		return original, created, err
	}
	allowed := map[string]bool{}
	for _, column := range siviCoverWritePolicy().Columns {
		allowed[column] = true
	}
	for _, cover := range request.Covers {
		if !allowed[cover.Column] {
			return original, created, fmt.Errorf("SIVI creation field %s is not a source cover/total", cover.Column)
		}
		created.Cells[columns[cover.Column]] = cloneSiteUnitCell(cover.Value)
	}
	groups, err := projectSIVIVegetation(ctx, request.Plot, request.Form == "SubVegA-SIVI",
		ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{created}})
	if err != nil {
		return original, created, err
	}
	for _, group := range groups {
		if group.Form != request.Form || len(group.Rows) != 1 {
			continue
		}
		for _, cover := range request.Covers {
			visible := false
			for _, column := range group.Columns {
				visible = visible || column == cover.Column
			}
			if request.Form == "SubVegA-SIVI_BC" && (cover.Column == "Cover5a" || cover.Column == "Cover5b" || cover.Column == "Cover5c") {
				visible = false
			}
			if !visible {
				return original, created, fmt.Errorf("SIVI creation field %s is hidden in %s", cover.Column, request.Form)
			}
		}
		return original, created, nil
	}
	return original, created, errors.New("SIVI creation has no explicit source-view membership")
}

// Allocation is the existing positive signed32 desktop adaptation, not engine RNG equivalence.
func allocateSIVICreation(ctx context.Context, tx *sql.Tx, project string, before map[string]ProjectMetadataTable) (int64, map[string]ProjectMetadataTable, ProjectMetadataTable, error) {
	reserved, err := siviIdentityReservedIDs(project, before)
	if err != nil {
		return 0, nil, ProjectMetadataTable{}, err
	}
	assignments := []siviHeightAssignment{}
	for id := range reserved {
		assignments = append(assignments, siviHeightAssignment{Before: ProjectMetadataCell{Storage: "null"}, After: siviCreationInteger(id)})
	}
	baseline, ledger, err := reserveSIVIIdentityPlan(ctx, tx, project, before, assignments)
	if err != nil {
		return 0, nil, ProjectMetadataTable{}, err
	}
	id, err := allocateChildID(tx, quoteHeaderIdentifier(project+"_Veg"))
	if err != nil {
		return 0, nil, ProjectMetadataTable{}, err
	}
	columns, err := siteUnitTransferColumns(before[project+"_Veg"], "ID")
	if err != nil {
		return 0, nil, ProjectMetadataTable{}, err
	}
	for _, row := range before[project+"_Veg"].Rows {
		if siviIdentityCellMatches(row.Cells[columns["ID"]], strconv.FormatInt(id, 10)) {
			return 0, nil, ProjectMetadataTable{}, errors.New("SIVI creation allocator collided with physical storage")
		}
	}
	next := make(map[string]ProjectMetadataTable, len(baseline))
	for name, table := range baseline {
		next[name] = table
	}
	next[siviIdentityLedger] = ledger
	_, ledger, err = reserveSIVIIdentityPlan(ctx, tx, project, next,
		[]siviHeightAssignment{{Before: ProjectMetadataCell{Storage: "null"}, After: siviCreationInteger(strconv.FormatInt(id, 10))}})
	return id, baseline, ledger, err
}

func sameSIVICreationRequest(a, b siviCreationRequest) bool {
	// A reopened same-project owner may retry the durable request; it must still own the parent.
	a.ContextID, b.ContextID = "", ""
	return reflect.DeepEqual(a, b)
}

func verifySIVICreationHistory(ctx context.Context, request siviCreationRequest, history siviCreationHistory, tables map[string]ProjectMetadataTable) error {
	if !sameSIVICreationRequest(request, history.Request) || history.Result.RequestID != request.RequestID ||
		history.Result.HistoryID != request.RequestID || history.Result.Project != request.Project ||
		history.Result.Plot != request.Plot || history.Result.RowID != history.Committed.RowID ||
		history.Result.ID <= 0 || history.AuditStrength < 0 || history.AuditStrength > 3 || history.Actor == "" {
		return errors.New("SIVI creation history request/result authority differs")
	}
	if err := validateSIVICreationRequest(history.Request); err != nil {
		return err
	}
	if err := validateChildPhysicalText("SIVI creation actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI creation history has invalid time provenance")
	}
	veg := tables[request.Project+"_Veg"]
	original, planned, err := planSIVICreation(ctx, request, veg, history.Result.ID)
	if err != nil {
		return err
	}
	planned.RowID = history.Result.RowID
	if !reflect.DeepEqual(history.Columns, veg.Columns) || !reflect.DeepEqual(history.Original, original) ||
		!reflect.DeepEqual(history.Committed, planned) {
		return errors.New("SIVI creation history full typed buffer/row differs from its request")
	}
	found := false
	identityOccupants := 0
	vegColumns, err := siteUnitTransferColumns(veg, "ID")
	if err != nil {
		return err
	}
	for _, row := range veg.Rows {
		if row.RowID == planned.RowID {
			found = reflect.DeepEqual(row, planned)
		}
		if siviIdentityCellMatches(row.Cells[vegColumns["ID"]], strconv.FormatInt(history.Result.ID, 10)) {
			identityOccupants++
		}
	}
	if !found || identityOccupants != 1 {
		return errors.New("SIVI creation replay requires the exact committed physical row and unique application identity")
	}
	reserved, err := siviIdentityReservedIDs(request.Project, tables)
	if err != nil || !reserved[strconv.FormatInt(history.Result.ID, 10)] {
		return errors.Join(err, errors.New("SIVI creation replay lost its identity reservation"))
	}
	ledger := tables[siviIdentityLedger]
	ledgerColumns, err := siteUnitTransferColumns(ledger, "ChildTable", "ID")
	if err != nil {
		return err
	}
	ownedReservation := false
	for _, row := range ledger.Rows {
		table, id := row.Cells[ledgerColumns["ChildTable"]], row.Cells[ledgerColumns["ID"]]
		ownedReservation = ownedReservation || table.Text != nil && *table.Text == quoteHeaderIdentifier(request.Project+"_Veg") &&
			id.Integer != nil && *id.Integer == strconv.FormatInt(history.Result.ID, 10)
	}
	if !ownedReservation {
		return errors.New("SIVI creation replay lost its durable ledger entry")
	}
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for name, index := range columns {
		if name == "ID" || name == "PlotNumber" || name == "Flag" {
			continue
		}
		value, err := metadataCellValue(planned.Cells[index])
		if err != nil {
			return err
		}
		if auditHeaderChange(nil, value, history.AuditStrength) {
			expected[name] = true
		}
	}
	if len(expected) != len(history.Audits) {
		return errors.New("SIVI creation history audit completeness differs")
	}
	auditPlan, err := appendSIVIPlannedAudits(ProjectMetadataTable{Columns: tables[request.Project+"_Audit"].Columns}, history.Audits)
	if err != nil {
		return err
	}
	for i, record := range history.Audits {
		index, present := columns[record.EditField]
		if !present || !expected[record.EditField] || record.Project != request.Project || record.PlotNumber != request.Plot ||
			record.User != history.Actor || record.EditWhen != history.When || record.Table != "_Veg" ||
			record.ID == nil || *record.ID != history.Result.ID || record.BeforeEdit != nil || record.Restore || record.Flag {
			return errors.New("SIVI creation history audit authority differs")
		}
		delete(expected, record.EditField)
		value, err := metadataCellValue(planned.Cells[index])
		if err != nil || !metadataAuditTextEqual(record.AfterEdit, value) {
			return errors.Join(err, errors.New("SIVI creation history audit value differs"))
		}
		found := false
		for _, row := range tables[request.Project+"_Audit"].Rows {
			if reflect.DeepEqual(row, auditPlan.Rows[i]) {
				found = true
			}
		}
		if !found {
			return errors.New("SIVI creation replay lost its exact source audit")
		}
	}
	return nil
}

func readSIVICreationReplay(ctx context.Context, request siviCreationRequest, before map[string]ProjectMetadataTable) (*siviCreationHistory, error) {
	table, present := before[siviCreationHistoryTable]
	if !present {
		return nil, nil
	}
	columns, err := siteUnitTransferColumns(table, "RequestID", "Created", "Proposal")
	if err != nil || len(columns) != 3 {
		return nil, errors.Join(err, errors.New("SIVI creation history schema differs"))
	}
	var matched *siviCreationHistory
	for _, row := range table.Rows {
		id, created, proposal := row.Cells[columns["RequestID"]], row.Cells[columns["Created"]], row.Cells[columns["Proposal"]]
		if id.Storage != "text" || id.Text == nil || created.Storage != "text" || created.Text == nil || proposal.Storage != "text" || proposal.Text == nil {
			return nil, errors.New("SIVI creation history has invalid typed storage")
		}
		raw := []byte(*proposal.Text)
		if _, err := sourceChildJSONObject(raw, "SIVI creation history",
			[]string{"Request", "Result", "Actor", "When", "AuditStrength", "Columns", "Original", "Committed", "Audits"}, nil); err != nil {
			return nil, err
		}
		var history siviCreationHistory
		if err := json.Unmarshal(raw, &history); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != *proposal.Text || *created.Text != history.When {
			return nil, errors.Join(err, errors.New("SIVI creation history is not its complete canonical typed provenance"))
		}
		if history.Request.RequestID != *id.Text || history.Result.RequestID != *id.Text ||
			history.Result.HistoryID != *id.Text || history.Request.Project != history.Result.Project ||
			history.Request.Plot != history.Result.Plot {
			return nil, errors.New("SIVI creation history key/request/result identities disagree")
		}
		if *id.Text != request.RequestID {
			continue
		}
		if history.Request.Project != request.Project {
			return nil, errors.New("SIVI creation request ID already belongs to another project")
		}
		if matched != nil {
			return nil, errors.New("SIVI creation history repeats a request ID")
		}
		if err := verifySIVICreationHistory(ctx, request, history, before); err != nil {
			return nil, err
		}
		matched = &history
	}
	return matched, ctx.Err()
}

func (s *ContextService) lookupSIVICreationReceipt(ctx context.Context, contextID string, request siviCreationRequest) (*siviCreationResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI creation receipt requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID != request.ContextID {
		return nil, errors.New("SIVI creation receipt differs from the exact context owner")
	}
	if err := validateSIVICreationRequest(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviCreationResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviCreationResult, error) {
			if request.Project != owner.selection.Project {
				return nil, errors.New("SIVI creation receipt belongs to another project")
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, request.Project, request.Plot); err != nil {
				return nil, err
			}
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
			if !present[siviCreationHistoryTable] {
				return nil, nil
			}
			tables := map[string]ProjectMetadataTable{}
			for _, name := range []string{siviCreationHistoryTable, request.Project + "_Veg", request.Project + "_Audit", siviIdentityLedger} {
				if !present[name] {
					continue
				}
				table, err := readSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
				if err != nil {
					return nil, err
				}
				tables[name] = table
			}
			history, err := readSIVICreationReplay(ctx, request, tables)
			if err != nil || history == nil {
				return nil, err
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			return siviCreationReceipt(*history, contextID, false), nil
		})
	})
}

func (s *ContextService) createSIVIVegetation(ctx context.Context, contextID string, request siviCreationRequest) (*siviCreationResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI creation requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID != request.ContextID {
		return nil, errors.New("SIVI creation request differs from the exact context owner")
	}
	if err := validateSIVICreationRequest(request); err != nil {
		return nil, err
	}
	return withSIVIIdentityWriter(ctx, s, contextID, request.Plot, "creation",
		func(plots *PlotService, owner *sqliteContext, tx *sql.Tx) (*siviCreationResult, bool, error) {
			if request.Project != owner.selection.Project {
				return nil, false, errors.New("SIVI creation request belongs to another project")
			}
			if plots.currentUser == "" {
				return nil, false, errors.New("SIVI creation requires an explicit audit actor")
			}
			if err := validateChildPhysicalText("SIVI creation actor", plots.currentUser, 100); err != nil {
				return nil, false, err
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			replay, err := readSIVICreationReplay(ctx, request, before)
			if err != nil {
				return nil, false, err
			}
			if replay != nil {
				if err := owner.validateMetadataWriterFiles(); err != nil {
					return nil, false, err
				}
				if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
					return nil, false, err
				}
				return siviCreationReceipt(*replay, contextID, false), false, nil
			}
			if owner.attachmentInfo["VUser"] == nil || owner.attachments["VUser"] == "" {
				return nil, false, errors.New("SIVI creation requires the owned personal reference file")
			}
			if _, err := tx.ExecContext(ctx, `ATTACH DATABASE ? AS "sivi_creation_user"`, sqliteFileURI(owner.attachments["VUser"], "ro")); err != nil {
				return nil, false, err
			}
			references, err := readSIVISpeciesReferences(ctx, tx, contextID, request.Project, request.Plot, "reference", "sivi_creation_user")
			if err != nil {
				return nil, false, err
			}
			if err := validateSIVISpeciesMembership(ctx, request.speciesEdit(), references); err != nil {
				return nil, false, err
			}
			id, baseline, ledger, err := allocateSIVICreation(ctx, tx, request.Project, before)
			if err != nil {
				return nil, false, err
			}
			veg := before[request.Project+"_Veg"]
			original, created, err := planSIVICreation(ctx, request, veg, id)
			if err != nil {
				return nil, false, err
			}
			names, marks, values := []string{}, []string{}, []any{}
			for i, column := range veg.Columns {
				value, err := metadataCellValue(created.Cells[i])
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
				return nil, false, errors.Join(err, errors.New("SIVI creation did not insert exactly one row"))
			}
			rowID, err := inserted.LastInsertId()
			if err != nil {
				return nil, false, err
			}
			created.RowID = strconv.FormatInt(rowID, 10)
			result := siviCreationIdentity{
				RequestID: request.RequestID, Project: request.Project,
				Plot: request.Plot, RowID: created.RowID, HistoryID: request.RequestID, ID: id,
			}
			when := time.Now().Format("2006-01-02 15:04:05")
			fields, after := []childField{{column: "Species"}}, []any{request.Species}
			for _, cover := range request.Covers {
				fields, after = append(fields, childField{column: cover.Column}), append(after, *cover.Value.Real)
			}
			audits, err := auditChildFieldsTracked(tx, request.Project, "Veg", request.Plot, id, fields, make([]any, len(fields)), after,
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
					return nil, false, errors.Join(err, errors.New("SIVI creation audit identity differs"))
				}
				records = append(records, selected[0])
			}
			plannedVeg := ProjectMetadataTable{Columns: veg.Columns, Rows: append(append([]ProjectMetadataRow{}, veg.Rows...), created)}
			sort.Slice(plannedVeg.Rows, func(i, j int) bool {
				a, _ := strconv.ParseInt(plannedVeg.Rows[i].RowID, 10, 64)
				b, _ := strconv.ParseInt(plannedVeg.Rows[j].RowID, 10, 64)
				return a < b
			})
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
			history := siviCreationHistory{request, result, plots.currentUser, when, plots.auditStrength, veg.Columns, original, created, records}
			if err := verifySIVICreationHistory(ctx, request, history, observed); err != nil {
				return nil, false, err
			}
			proposal, err := json.Marshal(history)
			if err != nil {
				return nil, false, err
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_SIVICreationHistory" (
				RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
				return nil, false, err
			}
			entry, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_SIVICreationHistory"(RequestID,Created,Proposal) VALUES(?,?,?)`,
				request.RequestID, when, string(proposal))
			if err != nil {
				return nil, false, err
			}
			if count, err := entry.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI creation history did not insert exactly one request"))
			}
			historyRowID, err := entry.LastInsertId()
			if err != nil {
				return nil, false, err
			}
			historyPlan := before[siviCreationHistoryTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]ProjectMetadataRow{}, historyPlan.Rows...), ProjectMetadataRow{
				RowID: strconv.FormatInt(historyRowID, 10), Cells: []ProjectMetadataCell{siviCreationText(request.RequestID), siviCreationText(when), siviCreationText(string(proposal))}})
			observed, err = environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(baseline, observed, expected, siviCreationHistoryTable); err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(observed[siviCreationHistoryTable], historyPlan) {
				return nil, false, errors.New("SIVI creation durable history differs from its exact typed plan")
			}
			verified, err := readSIVICreationReplay(ctx, request, observed)
			if err != nil || verified == nil || !reflect.DeepEqual(verified.Result, result) {
				return nil, false, errors.Join(err, errors.New("SIVI creation history/result identities disagree"))
			}
			return siviCreationReceipt(*verified, contextID, true), true, nil
		})
}
