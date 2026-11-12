package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

const SiviCreationHistoryTable = "__VPRO_SIVICreationHistory"

type SiviCreationCover struct {
	Column string                       `json:"column"`
	Value  vprocore.ProjectMetadataCell `json:"value"`
}

type SiviCreationDecision struct {
	Kind     string  `json:"kind"`
	Entered  string  `json:"entered"`
	Selected *string `json:"selected,omitempty"`
}

type SiviCreationRequest struct {
	RequestID string                `json:"requestId"`
	ContextID string                `json:"contextId"`
	Project   string                `json:"project"`
	Plot      string                `json:"plot"`
	Form      string                `json:"form"`
	Species   string                `json:"species"`
	Decision  *SiviCreationDecision `json:"decision,omitempty"`
	Covers    []SiviCreationCover   `json:"covers"`
}

func (request *SiviCreationRequest) UnmarshalJSON(data []byte) error {
	required := []string{"requestId", "contextId", "project", "plot", "form", "species", "covers"}
	properties, err := SourceChildJSONObject(data, "SIVI creation", required, []string{"decision"})
	if err != nil {
		return err
	}
	if err := SourceChildJSONNonNull(properties, "SIVI creation", append(required, "decision")...); err != nil {
		return err
	}
	if raw, present := properties["decision"]; present {
		decision, err := SourceChildJSONObject(raw, "SIVI creation decision", []string{"kind", "entered"}, []string{"selected"})
		if err != nil {
			return err
		}
		if err := SourceChildJSONNonNull(decision, "SIVI creation decision", "kind", "entered", "selected"); err != nil {
			return err
		}
	}
	if err := SourceChildArrayJSON(properties["covers"], func(raw json.RawMessage) error {
		cover, err := SourceChildJSONObject(raw, "SIVI creation cover", []string{"column", "value"}, nil)
		if err != nil {
			return err
		}
		if err := SourceChildJSONNonNull(cover, "SIVI creation cover", "column", "value"); err != nil {
			return err
		}
		return SourceChildCellJSON(cover["value"], "SIVI creation cover")
	}); err != nil {
		return err
	}
	type plain SiviCreationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = SiviCreationRequest(decoded)
	return nil
}

type SiviCreationResult struct {
	RequestID     string                           `json:"requestId"`
	ContextID     string                           `json:"contextId"`
	Project       string                           `json:"project"`
	Plot          string                           `json:"plot"`
	Form          string                           `json:"form"`
	RowID         string                           `json:"rowId"`
	HistoryID     string                           `json:"historyId"`
	ID            int64                            `json:"id"`
	Actor         string                           `json:"actor"`
	AuditStrength int                              `json:"auditStrength"`
	EditWhen      string                           `json:"editWhen"`
	Columns       []vprocore.ProjectMetadataColumn `json:"columns"`
	Original      vprocore.ProjectMetadataRow      `json:"original"`
	Committed     vprocore.ProjectMetadataRow      `json:"committed"`
	Covers        []SiviCreationCover              `json:"covers"`
	Request       SiviCreationRequest              `json:"request"`
	DidCommit     bool                             `json:"didCommit"`
	Replayed      bool                             `json:"replayed"`
}

type SiviCreationIdentity struct {
	RequestID, Project, Plot, RowID, HistoryID string
	ID                                         int64
}

type SiviCreationHistory struct {
	Request       SiviCreationRequest
	Result        SiviCreationIdentity
	Actor, When   string
	AuditStrength int
	Columns       []vprocore.ProjectMetadataColumn
	Original      vprocore.ProjectMetadataRow
	Committed     vprocore.ProjectMetadataRow
	Audits        []vprocore.AuditEntry
}

func SiviCreationReceipt(history SiviCreationHistory, contextID string, didCommit bool) *SiviCreationResult {
	// didCommit describes this invocation; a verified lookup/retry only acknowledges the durable commit.
	return &SiviCreationResult{
		RequestID: history.Result.RequestID, ContextID: contextID, Project: history.Result.Project,
		Plot: history.Result.Plot, Form: history.Request.Form, RowID: history.Result.RowID,
		HistoryID: history.Result.HistoryID, ID: history.Result.ID, Actor: history.Actor,
		AuditStrength: history.AuditStrength, EditWhen: history.When, Columns: history.Columns,
		Original: history.Original, Committed: history.Committed, Covers: history.Request.Covers,
		Request: history.Request, DidCommit: didCommit, Replayed: !didCommit,
	}
}

var SiviCreationColumns = []string{
	"PlotNumber", "Species", "Layer", "Cover1", "Height1", "Cover2", "Height2", "Cover3", "Height3",
	"TotalA", "HeightA", "Cover4", "Height4", "Cover5", "Height5", "Cover5a", "Height5a",
	"Cover5b", "Height5b", "Cover5c", "Height5c", "TotalB", "HeightB", "Cover6", "Height6",
	"Cover7", "Cover8", "Cover9", "Cover10", "Collected", "Flag", "ID", "LL", "AF", "DC",
	"UT", "VI", "PV", "PG", "FFA", "Cultural1", "Cultural2", "Other1", "Other2",
}

func SiviCreationInteger(value string) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
}

func SiviCreationText(value string) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &value}
}

func (request SiviCreationRequest) SpeciesEdit() SIVISpeciesEdit {
	edit := SIVISpeciesEdit{Form: request.Form, Value: request.Species}
	if request.Decision != nil {
		edit.Decision, edit.Entered, edit.Selected = request.Decision.Kind, &request.Decision.Entered, request.Decision.Selected
	}
	return edit
}

func ValidateSIVICreationRequest(request SiviCreationRequest) error {
	for _, text := range []struct {
		name, value string
		maximum     int
	}{{"request ID", request.RequestID, 100}, {"context ID", request.ContextID, 100},
		{"project", request.Project, 255}, {"parent", request.Plot, 7}, {"Species", request.Species, 8}} {
		if text.value == "" {
			return fmt.Errorf("SIVI creation requires explicit nonempty %s", text.name)
		}
		if err := vprocore.ValidateChildPhysicalText("SIVI creation "+text.name, text.value, text.maximum); err != nil {
			return err
		}
	}
	if !vprocore.ProjectNamePattern.MatchString(request.Project) {
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
	edit := request.SpeciesEdit()
	creation := vprocore.VegetationCreationRequest{Form: edit.Form, Species: edit.Value, Decision: edit.Decision, Entered: edit.Entered, Selected: edit.Selected}
	if err := vprocore.ValidateVegetationSpeciesDecision(creation.SpeciesUpdate()); err != nil {
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
		if _, err := vprocore.MetadataCellValue(cover.Value); err != nil {
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

func PlanSIVICreation(ctx context.Context, request SiviCreationRequest, veg vprocore.ProjectMetadataTable, id int64) (vprocore.ProjectMetadataRow, vprocore.ProjectMetadataRow, error) {
	columns, err := vprocore.SiteUnitTransferColumns(veg, SiviCreationColumns...)
	if err != nil || len(columns) != len(SiviCreationColumns) {
		return vprocore.ProjectMetadataRow{}, vprocore.ProjectMetadataRow{}, errors.Join(err, errors.New("SIVI creation requires all44 canonical fields without extra defaults"))
	}
	original := vprocore.ProjectMetadataRow{Cells: make([]vprocore.ProjectMetadataCell, len(columns))}
	for i := range original.Cells {
		original.Cells[i] = vprocore.ProjectMetadataCell{Storage: "null"}
	}
	original.Cells[columns["Flag"]] = SiviCreationInteger("0")
	created := vprocore.ProjectMetadataRow{RowID: "1", Cells: append([]vprocore.ProjectMetadataCell{}, original.Cells...)}
	created.Cells[columns["PlotNumber"]] = SiviCreationText(request.Plot)
	created.Cells[columns["Species"]] = SiviCreationText(request.Species)
	created.Cells[columns["ID"]] = SiviCreationInteger(strconv.FormatInt(id, 10))
	if err := ValidateSIVIIdentityCell("ID", created.Cells[columns["ID"]]); err != nil {
		return original, created, err
	}
	allowed := map[string]bool{}
	for _, column := range SiviCoverWritePolicy().Columns {
		allowed[column] = true
	}
	for _, cover := range request.Covers {
		if !allowed[cover.Column] {
			return original, created, fmt.Errorf("SIVI creation field %s is not a source cover/total", cover.Column)
		}
		created.Cells[columns[cover.Column]] = vprocore.CloneSiteUnitCell(cover.Value)
	}
	groups, err := ProjectSIVIVegetation(ctx, request.Plot, request.Form == "SubVegA-SIVI",
		vprocore.ProjectMetadataTable{Columns: veg.Columns, Rows: []vprocore.ProjectMetadataRow{created}})
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
func allocateSIVICreation(ctx context.Context, tx *sql.Tx, project string, before map[string]vprocore.ProjectMetadataTable) (int64, map[string]vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, error) {
	reserved, err := siviIdentityReservedIDs(project, before)
	if err != nil {
		return 0, nil, vprocore.ProjectMetadataTable{}, err
	}
	assignments := []SiviHeightAssignment{}
	for id := range reserved {
		assignments = append(assignments, SiviHeightAssignment{Before: vprocore.ProjectMetadataCell{Storage: "null"}, After: SiviCreationInteger(id)})
	}
	baseline, ledger, err := reserveSIVIIdentityPlan(ctx, tx, project, before, assignments)
	if err != nil {
		return 0, nil, vprocore.ProjectMetadataTable{}, err
	}
	id, err := vprocore.AllocateChildID(tx, vprocore.QuoteHeaderIdentifier(project+"_Veg"))
	if err != nil {
		return 0, nil, vprocore.ProjectMetadataTable{}, err
	}
	columns, err := vprocore.SiteUnitTransferColumns(before[project+"_Veg"], "ID")
	if err != nil {
		return 0, nil, vprocore.ProjectMetadataTable{}, err
	}
	for _, row := range before[project+"_Veg"].Rows {
		if SiviIdentityCellMatches(row.Cells[columns["ID"]], strconv.FormatInt(id, 10)) {
			return 0, nil, vprocore.ProjectMetadataTable{}, errors.New("SIVI creation allocator collided with physical storage")
		}
	}
	next := make(map[string]vprocore.ProjectMetadataTable, len(baseline))
	for name, table := range baseline {
		next[name] = table
	}
	next[SiviIdentityLedger] = ledger
	_, ledger, err = reserveSIVIIdentityPlan(ctx, tx, project, next,
		[]SiviHeightAssignment{{Before: vprocore.ProjectMetadataCell{Storage: "null"}, After: SiviCreationInteger(strconv.FormatInt(id, 10))}})
	return id, baseline, ledger, err
}

func sameSIVICreationRequest(a, b SiviCreationRequest) bool {
	// A reopened same-project owner may retry the durable request; it must still own the parent.
	a.ContextID, b.ContextID = "", ""
	return reflect.DeepEqual(a, b)
}

func VerifySIVICreationHistory(ctx context.Context, request SiviCreationRequest, history SiviCreationHistory, tables map[string]vprocore.ProjectMetadataTable) error {
	if !sameSIVICreationRequest(request, history.Request) || history.Result.RequestID != request.RequestID ||
		history.Result.HistoryID != request.RequestID || history.Result.Project != request.Project ||
		history.Result.Plot != request.Plot || history.Result.RowID != history.Committed.RowID ||
		history.Result.ID <= 0 || history.AuditStrength < 0 || history.AuditStrength > 3 || history.Actor == "" {
		return errors.New("SIVI creation history request/result authority differs")
	}
	if err := ValidateSIVICreationRequest(history.Request); err != nil {
		return err
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI creation actor", history.Actor, 100); err != nil {
		return err
	}
	if when, err := time.Parse("2006-01-02 15:04:05", history.When); err != nil || when.Format("2006-01-02 15:04:05") != history.When {
		return errors.New("SIVI creation history has invalid time provenance")
	}
	veg := tables[request.Project+"_Veg"]
	original, planned, err := PlanSIVICreation(ctx, request, veg, history.Result.ID)
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
	vegColumns, err := vprocore.SiteUnitTransferColumns(veg, "ID")
	if err != nil {
		return err
	}
	for _, row := range veg.Rows {
		if row.RowID == planned.RowID {
			found = reflect.DeepEqual(row, planned)
		}
		if SiviIdentityCellMatches(row.Cells[vegColumns["ID"]], strconv.FormatInt(history.Result.ID, 10)) {
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
	ledger := tables[SiviIdentityLedger]
	ledgerColumns, err := vprocore.SiteUnitTransferColumns(ledger, "ChildTable", "ID")
	if err != nil {
		return err
	}
	ownedReservation := false
	for _, row := range ledger.Rows {
		table, id := row.Cells[ledgerColumns["ChildTable"]], row.Cells[ledgerColumns["ID"]]
		ownedReservation = ownedReservation || table.Text != nil && *table.Text == vprocore.QuoteHeaderIdentifier(request.Project+"_Veg") &&
			id.Integer != nil && *id.Integer == strconv.FormatInt(history.Result.ID, 10)
	}
	if !ownedReservation {
		return errors.New("SIVI creation replay lost its durable ledger entry")
	}
	columns, err := vprocore.SiteUnitTransferColumns(veg, SiviCreationColumns...)
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for name, index := range columns {
		if name == "ID" || name == "PlotNumber" || name == "Flag" {
			continue
		}
		value, err := vprocore.MetadataCellValue(planned.Cells[index])
		if err != nil {
			return err
		}
		if vprocore.AuditHeaderChange(nil, value, history.AuditStrength) {
			expected[name] = true
		}
	}
	if len(expected) != len(history.Audits) {
		return errors.New("SIVI creation history audit completeness differs")
	}
	auditPlan, err := AppendSIVIPlannedAudits(vprocore.ProjectMetadataTable{Columns: tables[request.Project+"_Audit"].Columns}, history.Audits)
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
		value, err := vprocore.MetadataCellValue(planned.Cells[index])
		if err != nil || !vprocore.MetadataAuditTextEqual(record.AfterEdit, value) {
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

func readSIVICreationReplay(ctx context.Context, request SiviCreationRequest, before map[string]vprocore.ProjectMetadataTable) (*SiviCreationHistory, error) {
	table, present := before[SiviCreationHistoryTable]
	if !present {
		return nil, nil
	}
	columns, err := vprocore.SiteUnitTransferColumns(table, "RequestID", "Created", "Proposal")
	if err != nil || len(columns) != 3 {
		return nil, errors.Join(err, errors.New("SIVI creation history schema differs"))
	}
	var matched *SiviCreationHistory
	for _, row := range table.Rows {
		id, created, proposal := row.Cells[columns["RequestID"]], row.Cells[columns["Created"]], row.Cells[columns["Proposal"]]
		if id.Storage != "text" || id.Text == nil || created.Storage != "text" || created.Text == nil || proposal.Storage != "text" || proposal.Text == nil {
			return nil, errors.New("SIVI creation history has invalid typed storage")
		}
		raw := []byte(*proposal.Text)
		if _, err := SourceChildJSONObject(raw, "SIVI creation history",
			[]string{"Request", "Result", "Actor", "When", "AuditStrength", "Columns", "Original", "Committed", "Audits"}, nil); err != nil {
			return nil, err
		}
		var history SiviCreationHistory
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
		if err := VerifySIVICreationHistory(ctx, request, history, before); err != nil {
			return nil, err
		}
		matched = &history
	}
	return matched, ctx.Err()
}

func LookupSIVICreationReceipt(s *vprocore.ContextService, ctx context.Context, contextID string, request SiviCreationRequest) (*SiviCreationResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI creation receipt requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID != request.ContextID {
		return nil, errors.New("SIVI creation receipt differs from the exact context owner")
	}
	if err := ValidateSIVICreationRequest(request); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiviCreationResult, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SiviCreationResult, error) {
			if request.Project != owner.Selection.Project {
				return nil, errors.New("SIVI creation receipt belongs to another project")
			}
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			if err := SiviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			if err := SiviSpeciesReadParents(ctx, tx, request.Project, request.Plot); err != nil {
				return nil, err
			}
			schema, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", "sqlite_master", "", nil, "")
			if err != nil {
				return nil, err
			}
			present := map[string]bool{}
			for _, row := range schema.Rows {
				if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" && row.Cells[1].Text != nil {
					present[*row.Cells[1].Text] = true
				}
			}
			if !present[SiviCreationHistoryTable] {
				return nil, nil
			}
			tables := map[string]vprocore.ProjectMetadataTable{}
			for _, name := range []string{SiviCreationHistoryTable, request.Project + "_Veg", request.Project + "_Audit", SiviIdentityLedger} {
				if !present[name] {
					continue
				}
				table, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
				if err != nil {
					return nil, err
				}
				tables[name] = table
			}
			history, err := readSIVICreationReplay(ctx, request, tables)
			if err != nil || history == nil {
				return nil, err
			}
			if err := SiviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			return SiviCreationReceipt(*history, contextID, false), nil
		})
	})
}

func CreateSIVIVegetation(s *vprocore.ContextService, ctx context.Context, contextID string, request SiviCreationRequest) (*SiviCreationResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI creation requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID != request.ContextID {
		return nil, errors.New("SIVI creation request differs from the exact context owner")
	}
	if err := ValidateSIVICreationRequest(request); err != nil {
		return nil, err
	}
	return WithSIVIIdentityWriter(ctx, s, contextID, request.Plot, "creation",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx) (*SiviCreationResult, bool, error) {
			if request.Project != owner.Selection.Project {
				return nil, false, errors.New("SIVI creation request belongs to another project")
			}
			if plots.CurrentUser == "" {
				return nil, false, errors.New("SIVI creation requires an explicit audit actor")
			}
			if err := vprocore.ValidateChildPhysicalText("SIVI creation actor", plots.CurrentUser, 100); err != nil {
				return nil, false, err
			}
			before, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			replay, err := readSIVICreationReplay(ctx, request, before)
			if err != nil {
				return nil, false, err
			}
			if replay != nil {
				if err := owner.ValidateMetadataWriterFiles(); err != nil {
					return nil, false, err
				}
				if err := SiviHeightContextParent(ctx, owner, request.Plot); err != nil {
					return nil, false, err
				}
				return SiviCreationReceipt(*replay, contextID, false), false, nil
			}
			if owner.AttachmentInfo["VUser"] == nil || owner.Attachments["VUser"] == "" {
				return nil, false, errors.New("SIVI creation requires the owned personal reference file")
			}
			if _, err := tx.ExecContext(ctx, `ATTACH DATABASE ? AS "sivi_creation_user"`, sqlutil.SqliteFileURI(owner.Attachments["VUser"], "ro")); err != nil {
				return nil, false, err
			}
			references, err := readSIVISpeciesReferences(ctx, tx, contextID, request.Project, request.Plot, "reference", "sivi_creation_user")
			if err != nil {
				return nil, false, err
			}
			if err := ValidateSIVISpeciesMembership(ctx, request.SpeciesEdit(), references); err != nil {
				return nil, false, err
			}
			id, baseline, ledger, err := allocateSIVICreation(ctx, tx, request.Project, before)
			if err != nil {
				return nil, false, err
			}
			veg := before[request.Project+"_Veg"]
			original, created, err := PlanSIVICreation(ctx, request, veg, id)
			if err != nil {
				return nil, false, err
			}
			names, marks, values := []string{}, []string{}, []any{}
			for i, column := range veg.Columns {
				value, err := vprocore.MetadataCellValue(created.Cells[i])
				if err != nil {
					return nil, false, err
				}
				names, marks, values = append(names, vprocore.QuoteHeaderIdentifier(column.Name)), append(marks, "?"), append(values, value)
			}
			inserted, err := tx.ExecContext(ctx, `INSERT INTO `+vprocore.QuoteHeaderIdentifier(request.Project+"_Veg")+
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
			result := SiviCreationIdentity{
				RequestID: request.RequestID, Project: request.Project,
				Plot: request.Plot, RowID: created.RowID, HistoryID: request.RequestID, ID: id,
			}
			when := time.Now().Format("2006-01-02 15:04:05")
			fields, after := []vprocore.ChildField{{Column: "Species"}}, []any{request.Species}
			for _, cover := range request.Covers {
				fields, after = append(fields, vprocore.ChildField{Column: cover.Column}), append(after, *cover.Value.Real)
			}
			audits, err := vprocore.AuditChildFieldsTracked(tx, request.Project, "Veg", request.Plot, id, fields, make([]any, len(fields)), after,
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
					return nil, false, errors.Join(err, errors.New("SIVI creation audit identity differs"))
				}
				records = append(records, selected[0])
			}
			plannedVeg := vprocore.ProjectMetadataTable{Columns: veg.Columns, Rows: append(append([]vprocore.ProjectMetadataRow{}, veg.Rows...), created)}
			sort.Slice(plannedVeg.Rows, func(i, j int) bool {
				a, _ := strconv.ParseInt(plannedVeg.Rows[i].RowID, 10, 64)
				b, _ := strconv.ParseInt(plannedVeg.Rows[j].RowID, 10, 64)
				return a < b
			})
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
			history := SiviCreationHistory{request, result, plots.CurrentUser, when, plots.AuditStrength, veg.Columns, original, created, records}
			if err := VerifySIVICreationHistory(ctx, request, history, observed); err != nil {
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
			historyPlan := before[SiviCreationHistoryTable]
			if historyPlan.Columns == nil {
				historyPlan.Columns = []vprocore.ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
			}
			historyPlan.Rows = append(append([]vprocore.ProjectMetadataRow{}, historyPlan.Rows...), vprocore.ProjectMetadataRow{
				RowID: strconv.FormatInt(historyRowID, 10), Cells: []vprocore.ProjectMetadataCell{SiviCreationText(request.RequestID), SiviCreationText(when), SiviCreationText(string(proposal))}})
			observed, err = vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := vprocore.VerifySiteUnitTransferTables(baseline, observed, expected, SiviCreationHistoryTable); err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(observed[SiviCreationHistoryTable], historyPlan) {
				return nil, false, errors.New("SIVI creation durable history differs from its exact typed plan")
			}
			verified, err := readSIVICreationReplay(ctx, request, observed)
			if err != nil || verified == nil || !reflect.DeepEqual(verified.Result, result) {
				return nil, false, errors.Join(err, errors.New("SIVI creation history/result identities disagree"))
			}
			return SiviCreationReceipt(*verified, contextID, true), true, nil
		})
}
