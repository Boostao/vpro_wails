package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

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
