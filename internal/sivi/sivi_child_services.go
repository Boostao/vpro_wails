package sivi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const siviCollectedHistoryTable = "__VPRO_SIVICollectedHistory"
const siviCollectedHistorySQL = `CREATE TABLE "__VPRO_SIVICollectedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviCollectedWritePolicy() sourceform.SiviVegetationWritePolicy {
	return sourceform.SiviVegetationWritePolicy{Kind: "collected", HistoryTable: siviCollectedHistoryTable, HistorySQL: siviCollectedHistorySQL,
		Columns: []string{"Collected"}, Plan: planSIVICollectedCells}
}

func collectedSIVIDrafts(ctx context.Context, edits []SIVICollectedEdit) ([]sourceform.SiviHeightEdit, error) {
	if ctx == nil {
		return nil, errors.New("SIVI Collected planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI Collected planning requires explicit clicks")
	}
	drafts := make([]sourceform.SiviHeightEdit, len(edits))
	for i, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := strconv.ParseInt(edit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != edit.RowID {
			return nil, errors.New("SIVI Collected requires the original signed64 physical row identity")
		}
		if edit.Clicks < 1 || edit.Clicks > 3 {
			return nil, errors.New("SIVI Collected requires exactly one to three clicks")
		}
		if _, err := vprocore.MetadataCellValue(edit.Expected); err != nil {
			return nil, fmt.Errorf("SIVI Collected expected value: %w", err)
		}
		if edit.Expected.Storage != "null" && edit.Expected.Storage != "text" {
			return nil, errors.New("historical non-text Collected storage is not cyclable")
		}
		next := vprocore.CollectedAfterClicks(edit.Expected.Text, edit.Clicks)
		value := vprocore.ProjectMetadataCell{Storage: "null"}
		if next != nil {
			value, err = vprocore.ProjectMetadataCellInternal("text", []byte(*next))
			if err != nil {
				return nil, fmt.Errorf("SIVI Collected derived value: %w", err)
			}
		}
		drafts[i] = sourceform.SiviHeightEdit{edit.RowID, edit.Form, "Collected", vprocore.CloneSiteUnitCell(edit.Expected), value}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return drafts, nil
}

func planSIVICollectedEdits(ctx context.Context, plot string, extended bool, veg vprocore.ProjectMetadataTable, edits []SIVICollectedEdit) ([]sourceform.SiviHeightAssignment, error) {
	drafts, err := collectedSIVIDrafts(ctx, edits)
	if err != nil {
		return nil, err
	}
	return planSIVICollectedCells(ctx, plot, extended, veg, drafts)
}

func planSIVICollectedCells(ctx context.Context, plot string, extended bool, veg vprocore.ProjectMetadataTable, edits []sourceform.SiviHeightEdit) ([]sourceform.SiviHeightAssignment, error) {
	if ctx == nil {
		return nil, errors.New("SIVI Collected planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sourceform.PlanSIVIChildEdits(ctx, plot, extended, veg, edits, "Collected",
		func(group int, column string) bool { return group >= 0 && group < 3 && column == "Collected" },
		func(_ string, cell vprocore.ProjectMetadataCell) error {
			if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil || (*cell.Text != "C" && *cell.Text != "V")) {
				return errors.New("new Collected values must be NULL, C or V")
			}
			value, err := vprocore.MetadataCellValue(cell)
			if err != nil {
				return err
			}
			return vprocore.ValidateChildPhysicalText("Veg.Collected", value, 1)
		})
}

type SIVICollectedEdit struct {
	RowID    string                       `json:"rowId"`
	Form     string                       `json:"form"`
	Expected vprocore.ProjectMetadataCell `json:"expected"`
	Clicks   int                          `json:"clicks"`
}

func (edit *SIVICollectedEdit) UnmarshalJSON(data []byte) error {
	type plain SIVICollectedEdit
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI Collected draft", "rowId", "form", "expected", "clicks"); err != nil {
		return fmt.Errorf("SIVI Collected draft transport: %w", err)
	}
	*edit = SIVICollectedEdit(decoded)
	return nil
}

type SIVICollectedWrite struct {
	Original []sourceform.SIVIVegetationProjection `json:"original"`
	Edits    []SIVICollectedEdit                   `json:"edits"`
}

func (request *SIVICollectedWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICollectedWrite
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI Collected Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI Collected transport: %w", err)
	}
	*request = SIVICollectedWrite(decoded)
	return nil
}

type SIVICollectedService struct {
	contexts *vprocore.ContextService
}

func NewSIVICollectedService(contexts *vprocore.ContextService) (*SIVICollectedService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI Collected service requires context ownership")
	}
	return &SIVICollectedService{contexts: contexts}, nil
}

func (s *SIVICollectedService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI Collected service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI Collected service requires context ownership")
	}
	return nil
}

func (s *SIVICollectedService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s.contexts, ctx, contextID, plot, extended)
}

func (s *SIVICollectedService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*sourceform.SIVIHeightWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request SIVICollectedWrite
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if len(request.Original) != 3 {
		return nil, errors.New("SIVI Collected Save requires three reviewed source groups")
	}
	edits, err := collectedSIVIDrafts(ctx, request.Edits)
	if err != nil {
		return nil, err
	}
	return sourceform.WriteSIVIVegetationCells(s.contexts, ctx, contextID, plot, extended, request.Original, edits, siviCollectedWritePolicy())
}

func (s *SIVICollectedService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVIVegetationCells(s.contexts, ctx, contextID, plot, historyID, action, siviCollectedWritePolicy())
}

const siviCombinedHistoryTable = "__VPRO_SIVICombinedHistory"
const siviCombinedHistorySQL = `CREATE TABLE "__VPRO_SIVICombinedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

func siviCombinedWritePolicy() sourceform.SiviVegetationWritePolicy {
	columns := append([]string{}, sourceform.SiviCoverWritePolicy().Columns...)
	columns = append(columns, sourceform.SiviHeightWritePolicy().Columns...)
	return sourceform.SiviVegetationWritePolicy{Kind: "combined", HistoryTable: siviCombinedHistoryTable, HistorySQL: siviCombinedHistorySQL, Columns: columns, Plan: planSIVICombinedEdits}
}

func planSIVICombinedEdits(ctx context.Context, plot string, extended bool, veg vprocore.ProjectMetadataTable, edits []sourceform.SiviHeightEdit) ([]sourceform.SiviHeightAssignment, error) {
	if ctx == nil {
		return nil, errors.New("SIVI combined planning requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		return nil, errors.New("SIVI combined planning requires explicit edits")
	}
	heightColumns := map[string]bool{}
	for _, column := range sourceform.SiviHeightWritePolicy().Columns {
		heightColumns[column] = true
	}
	covers, heights := []sourceform.SiviHeightEdit{}, []sourceform.SiviHeightEdit{}
	for _, edit := range edits {
		if heightColumns[edit.Column] {
			heights = append(heights, edit)
		} else {
			covers = append(covers, edit)
		}
	}
	planned := map[[2]string]sourceform.SiviHeightAssignment{}
	for _, domain := range []struct {
		edits []sourceform.SiviHeightEdit
		plan  func(context.Context, string, bool, vprocore.ProjectMetadataTable, []sourceform.SiviHeightEdit) ([]sourceform.SiviHeightAssignment, error)
	}{{covers, sourceform.PlanSIVICoverEdits}, {heights, sourceform.PlanSIVIHeightEdits}} {
		if len(domain.edits) == 0 {
			continue
		}
		assignments, err := domain.plan(ctx, plot, extended, veg, domain.edits)
		if err != nil {
			return nil, err
		}
		for _, assignment := range assignments {
			planned[[2]string{assignment.RowID, assignment.Column}] = assignment
		}
	}
	// Domain planners validate every draft (including noops); only changed cells
	// are merged back into the reviewed order for one transaction and history.
	assignments := []sourceform.SiviHeightAssignment{}
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if assignment, present := planned[[2]string{edit.RowID, edit.Column}]; present {
			assignments = append(assignments, assignment)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return assignments, nil
}

type SIVICombinedWrite struct {
	Original []sourceform.SIVIVegetationProjection `json:"original"`
	Edits    []SIVICoverEdit                       `json:"edits"`
}

func (request *SIVICombinedWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICombinedWrite
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI combined Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI combined transport: %w", err)
	}
	*request = SIVICombinedWrite(decoded)
	return nil
}

type SIVICombinedService struct {
	contexts *vprocore.ContextService
}

func NewSIVICombinedService(contexts *vprocore.ContextService) (*SIVICombinedService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI combined service requires context ownership")
	}
	return &SIVICombinedService{contexts: contexts}, nil
}

func (s *SIVICombinedService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI combined service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI combined service requires context ownership")
	}
	return nil
}

func (s *SIVICombinedService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s.contexts, ctx, contextID, plot, extended)
}

func (s *SIVICombinedService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*sourceform.SIVIHeightWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request SIVICombinedWrite
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if len(request.Original) != 3 || len(request.Edits) == 0 {
		return nil, errors.New("SIVI combined Save requires three reviewed source groups and explicit edits")
	}
	edits := make([]sourceform.SiviHeightEdit, len(request.Edits))
	for i, edit := range request.Edits {
		edits[i] = sourceform.SiviHeightEdit{edit.RowID, edit.Form, edit.Column, edit.Expected, edit.Value}
	}
	return sourceform.WriteSIVIVegetationCells(s.contexts, ctx, contextID, plot, extended, request.Original, edits, siviCombinedWritePolicy())
}

func (s *SIVICombinedService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVIVegetationCells(s.contexts, ctx, contextID, plot, historyID, action, siviCombinedWritePolicy())
}

type SIVICoverEdit struct {
	RowID    string                       `json:"rowId"`
	Form     string                       `json:"form"`
	Column   string                       `json:"column"`
	Expected vprocore.ProjectMetadataCell `json:"expected"`
	Value    vprocore.ProjectMetadataCell `json:"value"`
}

func (edit *SIVICoverEdit) UnmarshalJSON(data []byte) error {
	type plain SIVICoverEdit
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI cover draft", "rowId", "form", "column", "expected", "value"); err != nil {
		return fmt.Errorf("SIVI cover draft transport: %w", err)
	}
	*edit = SIVICoverEdit(decoded)
	return nil
}

type SIVICoverWrite struct {
	Original []sourceform.SIVIVegetationProjection `json:"original"`
	Edits    []SIVICoverEdit                       `json:"edits"`
}

func (request *SIVICoverWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICoverWrite
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI cover Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI cover transport: %w", err)
	}
	*request = SIVICoverWrite(decoded)
	return nil
}

type SIVICoverWriteResult struct {
	ChangedCells int
	HistoryID    string
}

type SIVICoverService struct {
	contexts *vprocore.ContextService
}

func NewSIVICoverService(contexts *vprocore.ContextService) (*SIVICoverService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI cover service requires context ownership")
	}
	return &SIVICoverService{contexts: contexts}, nil
}

func (s *SIVICoverService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI cover service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI cover service requires context ownership")
	}
	return nil
}

func (s *SIVICoverService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s.contexts, ctx, contextID, plot, extended)
}

func (s *SIVICoverService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVICoverWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request SIVICoverWrite
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if len(request.Original) != 3 || len(request.Edits) == 0 {
		return nil, errors.New("SIVI cover Save requires three reviewed source groups and explicit edits")
	}
	edits := make([]sourceform.SiviHeightEdit, len(request.Edits))
	for i, edit := range request.Edits {
		edits[i] = sourceform.SiviHeightEdit{edit.RowID, edit.Form, edit.Column, edit.Expected, edit.Value}
	}
	result, err := sourceform.WriteSIVICovers(s.contexts, ctx, contextID, plot, extended, request.Original, edits)
	if err != nil {
		return nil, err
	}
	return &SIVICoverWriteResult{result.ChangedCells, result.HistoryID}, nil
}

func (s *SIVICoverService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVICovers(s.contexts, ctx, contextID, plot, historyID, action)
}

func RequireSIVIHeights(s *vprocore.ContextService, ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI aggregate-height editing requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errors.New("SIVI aggregate-height editing requires owned contexts")
	}
	return nil
}

func GetSIVIVegetation(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := RequireSIVIHeights(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s, ctx, contextID, plot, extended)
}

func SaveSIVIHeights(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool, original []sourceform.SIVIVegetationProjection, edits []sourceform.SIVIHeightEdit) (*sourceform.SIVIHeightWriteResult, error) {
	if err := RequireSIVIHeights(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.WriteSIVIHeights(s, ctx, contextID, plot, extended, original, edits)
}

func RestoreSIVIHeights(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := RequireSIVIHeights(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVIHeights(s, ctx, contextID, plot, historyID, action)
}

type SIVIIdentityWrite struct {
	Original []sourceform.SIVIVegetationProjection `json:"original"`
	Edits    []sourceform.SIVIHeightEdit           `json:"edits"`
}

func (request *SIVIIdentityWrite) UnmarshalJSON(data []byte) error {
	const operation = "SIVI identity"
	properties, err := sourceform.SourceChildJSONObject(data, operation, []string{"original", "edits"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, operation, "original", "edits"); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(properties["original"], func(raw json.RawMessage) error {
		group, err := sourceform.SourceChildJSONObject(raw, operation, []string{"Form", "Query", "Columns", "Rows"}, nil)
		if err != nil {
			return err
		}
		if err := sourceform.SourceChildJSONNonNull(group, operation, "Form", "Query", "Columns", "Rows"); err != nil {
			return err
		}
		return sourceform.SourceChildRowsJSON(group["Rows"], operation)
	}); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(properties["edits"], func(raw json.RawMessage) error {
		edit, err := sourceform.SourceChildJSONObject(raw, operation, []string{"rowId", "form", "column", "expected", "value"}, nil)
		if err != nil {
			return err
		}
		if err := sourceform.SourceChildJSONNonNull(edit, operation, "rowId", "form", "column", "expected", "value"); err != nil {
			return err
		}
		for _, name := range []string{"expected", "value"} {
			if err := sourceform.SourceChildCellJSON(edit[name], operation); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	type plain SIVIIdentityWrite
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI identity Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI identity transport: %w", err)
	}
	*request = SIVIIdentityWrite(decoded)
	return nil
}

type SIVIIdentityService struct {
	contexts *vprocore.ContextService
}

func NewSIVIIdentityService(contexts *vprocore.ContextService) (*SIVIIdentityService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI identity service requires context ownership")
	}
	return &SIVIIdentityService{contexts: contexts}, nil
}

func (s *SIVIIdentityService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI identity service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI identity service requires context ownership")
	}
	return nil
}

func (s *SIVIIdentityService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s.contexts, ctx, contextID, plot, extended)
}

func (s *SIVIIdentityService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*sourceform.SIVIHeightWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request SIVIIdentityWrite
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if len(request.Original) != 3 {
		return nil, errors.New("SIVI identity Save requires three reviewed source groups")
	}
	return sourceform.WriteSIVIIdentities(s.contexts, ctx, contextID, plot, extended, request.Original, request.Edits)
}

func (s *SIVIIdentityService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVIIdentities(s.contexts, ctx, contextID, plot, historyID, action)
}
