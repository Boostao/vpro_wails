package twopage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type TwoPageVegetationAttemptOutcome struct {
	ContextID   string                        `json:"contextId"`
	RequestID   string                        `json:"requestId"`
	RequestJSON string                        `json:"requestJSON"`
	Status      string                        `json:"status"`
	Reason      string                        `json:"reason"`
	Receipt     *TwoPageVegetationWriteResult `json:"receipt"`
}

func BeginTwoPageVegetationAttempt(s *vprocore.ContextService, ctx context.Context, request twoPageVegetationRequest) (bool, error) {
	ledger := &s.TwoPageVegetationAttempts
	if err := sqlutil.AcquireMutexLease(ctx, &ledger.Mu); err != nil {
		return false, err
	}
	defer ledger.Mu.Unlock()
	entry, err := ledger.Entry(request.RequestID, request)
	if err != nil {
		return false, err
	}
	if entry.Sealed {
		return false, nil
	}
	entry.Active++
	return true, nil
}

func FinishTwoPageVegetationAttempt(s *vprocore.ContextService, requestID string) {
	ledger := &s.TwoPageVegetationAttempts
	ledger.Mu.Lock()
	defer ledger.Mu.Unlock()
	ledger.Requests[requestID].Active--
}

func (s *TwoPageVegetationService) RecoverOutcome(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationAttemptOutcome, error) {
	return s.recoverTwoPageVegetationOutcome(ctx, contextID, requestJSON, nil)
}

func (s *TwoPageVegetationService) recoverTwoPageVegetationOutcome(ctx context.Context, contextID, requestJSON string,
	afterSeal func()) (*TwoPageVegetationAttemptOutcome, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	ledger := &s.contexts.TwoPageVegetationAttempts
	if err := sqlutil.AcquireMutexLease(ctx, &ledger.Mu); err != nil {
		return nil, err
	}
	defer ledger.Mu.Unlock()
	entry, err := ledger.Entry(request.RequestID, request)
	if err != nil {
		return nil, err
	}
	outcome := &TwoPageVegetationAttemptOutcome{ContextID: contextID, RequestID: request.RequestID, RequestJSON: requestJSON}
	if entry.Active != 0 {
		outcome.Status, outcome.Reason = "running", "An owned Save attempt is still active; retain the original request."
		return outcome, nil
	}
	// Sealing excludes a matching RPC that has not yet reached the writer.
	entry.Sealed = true
	if afterSeal != nil {
		afterSeal()
	}
	receipt, err := LookupTwoPageVegetationReceipt(s.contexts, ctx, contextID, request)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if receipt != nil {
		outcome.Status, outcome.Reason, outcome.Receipt = "committed", "Exact durable history proves the committed source plan.", receipt
	} else if contextID != request.Original.ContextID {
		outcome.Status, outcome.Reason = "unknown", "Absent history in another context cannot certify the original attempt."
	} else {
		outcome.Status, outcome.Reason = "notCommitted", "No active or future matching Save can commit; the original owned history is absent."
	}
	return outcome, nil
}

type twoPageVegetationOriginal struct {
	ContextID    string                        `json:"contextId"`
	Project      string                        `json:"project"`
	Plot         string                        `json:"plot"`
	Form         string                        `json:"form"`
	RecordSource string                        `json:"recordSource"`
	Table        string                        `json:"table"`
	Records      vprocore.ProjectMetadataTable `json:"records"`
	SourceRowIDs []string                      `json:"sourceRowIds"`
}

func twoPageVegetationSource(form string) (string, []string, error) {
	switch form {
	case "SubVegAXL_BC", "SubVegAXL":
		return "USysVegA", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"}, nil
	case "SubVegCXL":
		return "USysVegC", []string{"Cover6"}, nil
	case "SubVegDXL":
		return "USysVegD", []string{"Cover7", "Cover8", "Cover9"}, nil
	default:
		return "", nil, fmt.Errorf("two-page vegetation original does not support source form %q", form)
	}
}

func ReadTwoPageVegetationOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string) (*twoPageVegetationOriginal, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page vegetation original requires an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page vegetation original requires an exact valid literal parent")
	}
	source, covers, err := twoPageVegetationSource(form)
	if err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*twoPageVegetationOriginal, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*twoPageVegetationOriginal, error) {
			project := owner.Selection.Project
			table := project + "_Veg"
			if err := sourceform.RequireTwoPageChildReadScope(ctx, tx, project, table, plot, "vegetation"); err != nil {
				return nil, err
			}
			if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page vegetation originals"); err != nil {
				return nil, fmt.Errorf("two-page vegetation original schema: %w", err)
			}
			records, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "project", table, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation original storage: %w", err)
			}
			sourceRows, err := twoPageVegetationSourceRows(records, source, covers)
			if err != nil {
				return nil, err
			}
			return &twoPageVegetationOriginal{ContextID: contextID, Project: project, Plot: plot, Form: form,
				RecordSource: source, Table: table, Records: records, SourceRowIDs: sourceRows}, nil
		})
	})
}

func twoPageVegetationSourceRows(records vprocore.ProjectMetadataTable, source string, covers []string) ([]string, error) {
	indices := make([]int, 0, len(covers))
	for _, cover := range covers {
		index, matches := -1, 0
		for i, column := range records.Columns {
			if strings.EqualFold(column.Name, cover) {
				index, matches = i, matches+1
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("two-page vegetation source %s requires one physical cover column %q", source, cover)
		}
		indices = append(indices, index)
	}
	sourceRows := []string{}
	for _, row := range records.Rows {
		if len(row.Cells) != len(records.Columns) {
			return nil, errors.New("two-page vegetation source requires complete physical rows")
		}
		for _, index := range indices {
			if row.Cells[index].Storage != "null" {
				sourceRows = append(sourceRows, row.RowID)
				break
			}
		}
	}
	return sourceRows, nil
}

type twoPageVegetationRequest struct {
	RequestID   string                              `json:"requestId"`
	Original    *twoPageVegetationOriginal          `json:"original"`
	Edits       []twoPageVegetationEdit             `json:"edits"`
	SpeciesList *twoPageVegetationSpeciesReferences `json:"speciesList,omitempty"`
}

func (request *twoPageVegetationRequest) UnmarshalJSON(data []byte) error {
	properties, err := sourceform.SourceChildJSONObject(data, "two-page vegetation request", []string{"requestId", "original", "edits"}, []string{"speciesList"})
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, "two-page vegetation request", "requestId", "original", "edits"); err != nil {
		return err
	}
	if _, present := properties["speciesList"]; present {
		if err := sourceform.SourceChildJSONNonNull(properties, "two-page vegetation request", "speciesList"); err != nil {
			return err
		}
	}
	required := []string{"contextId", "project", "plot", "form", "recordSource", "table", "records", "sourceRowIds"}
	original, err := sourceform.SourceChildJSONObject(properties["original"], "two-page vegetation original", required, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(original, "two-page vegetation original", required...); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(original["sourceRowIds"], func(raw json.RawMessage) error {
		var rowID string
		if err := json.Unmarshal(raw, &rowID); err != nil {
			return err
		}
		id, err := strconv.ParseInt(rowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != rowID {
			return errors.New("two-page vegetation source row requires a canonical physical identity")
		}
		return nil
	}); err != nil {
		return err
	}
	records, err := sourceform.SourceChildJSONObject(original["records"], "two-page vegetation records", []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(records["columns"], func(raw json.RawMessage) error {
		column, err := sourceform.SourceChildJSONObject(raw, "two-page vegetation column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceform.SourceChildJSONNonNull(column, "two-page vegetation column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceform.SourceChildRowsJSON(records["rows"], "two-page vegetation original"); err != nil {
		return err
	}
	type plain twoPageVegetationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !vprocore.SiviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("two-page vegetation request requires a stable lowercase UUIDv4")
	}
	// The wire also carries durable precalibration requests; only a new write requires canonical SINGLE assignments.
	if _, err := planTwoPageVegetationRequest(context.Background(), twoPageVegetationRequest(decoded), false); err != nil {
		return err
	}
	*request = twoPageVegetationRequest(decoded)
	return nil
}

func detachTwoPageVegetationRequest(ctx context.Context, service *vprocore.ContextService, request twoPageVegetationRequest) (twoPageVegetationRequest, error) {
	if ctx == nil || service == nil {
		return twoPageVegetationRequest{}, errors.New("two-page vegetation request requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return twoPageVegetationRequest{}, err
	}
	if !vprocore.SiviDeletionUUID.MatchString(request.RequestID) {
		return twoPageVegetationRequest{}, errors.New("two-page vegetation request requires a stable lowercase UUIDv4")
	}
	if _, err := planTwoPageVegetationRequest(ctx, request, false); err != nil {
		return twoPageVegetationRequest{}, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return twoPageVegetationRequest{}, err
	}
	var detached twoPageVegetationRequest
	if err := json.Unmarshal(raw, &detached); err != nil {
		return twoPageVegetationRequest{}, err
	}
	return detached, nil
}

type TwoPageVegetationOriginal = twoPageVegetationOriginal
type TwoPageVegetationSpeciesReferences = twoPageVegetationSpeciesReferences
type TwoPageVegetationWriteResult = twoPageVegetationWriteResult

type TwoPageVegetationService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageVegetationService(contexts *vprocore.ContextService) (*TwoPageVegetationService, error) {
	if contexts == nil {
		return nil, errors.New("two-page vegetation service requires owned contexts")
	}
	return &TwoPageVegetationService{contexts: contexts}, nil
}

func (s *TwoPageVegetationService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page vegetation service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page vegetation service requires owned contexts")
	}
	return nil
}

func (s *TwoPageVegetationService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageVegetationOriginal(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageVegetationService) GetSpeciesReferences(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationSpeciesReferences, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageVegetationSpeciesReferences(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageVegetationService) Save(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	return s.saveTwoPageVegetationObserved(ctx, contextID, requestJSON, nil, nil)
}

func (s *TwoPageVegetationService) saveTwoPageVegetationObserved(ctx context.Context, contextID, requestJSON string,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*TwoPageVegetationWriteResult, error) {
	return s.saveTwoPageVegetationQualifiedObserved(ctx, contextID, requestJSON, false, beforeCommit, afterCommit)
}

func (s *TwoPageVegetationService) saveTwoPageVegetationQualifiedObserved(ctx context.Context, contextID, requestJSON string,
	speciesList bool, beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*twoPageVegetationWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if (request.SpeciesList != nil) != speciesList {
		return nil, errors.New("two-page Species list requests require their independently enabled selection service; legacy Save does not grant Species")
	}
	started, err := BeginTwoPageVegetationAttempt(s.contexts, ctx, request)
	if err != nil {
		return nil, err
	}
	if !started {
		receipt, err := LookupTwoPageVegetationReceipt(s.contexts, ctx, contextID, request)
		if err != nil {
			return nil, err
		}
		if receipt == nil {
			return nil, errors.New("two-page vegetation request was sealed without a commit; review a fresh original and use a new UUID")
		}
		return receipt, nil
	}
	defer FinishTwoPageVegetationAttempt(s.contexts, request.RequestID)
	return WriteTwoPageVegetationObserved(s.contexts, ctx, contextID, request, beforeCommit, afterCommit)
}

func (s *TwoPageVegetationService) RecoverReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupTwoPageVegetationReceipt(s.contexts, ctx, contextID, request)
}
