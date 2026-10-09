package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const siviCoverFeatureEnvironment = "VPRO_SIVI_COVER_EDITING"

type SIVICoverEdit struct {
	RowID    string              `json:"rowId"`
	Form     string              `json:"form"`
	Column   string              `json:"column"`
	Expected ProjectMetadataCell `json:"expected"`
	Value    ProjectMetadataCell `json:"value"`
}

func (edit *SIVICoverEdit) UnmarshalJSON(data []byte) error {
	type plain SIVICoverEdit
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "SIVI cover draft", "rowId", "form", "column", "expected", "value"); err != nil {
		return fmt.Errorf("SIVI cover draft transport: %w", err)
	}
	*edit = SIVICoverEdit(decoded)
	return nil
}

type SIVICoverWrite struct {
	Original []SIVIVegetationProjection `json:"original"`
	Edits    []SIVICoverEdit            `json:"edits"`
}

func (request *SIVICoverWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICoverWrite
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "SIVI cover Save", "original", "edits"); err != nil {
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
	contexts *ContextService
	enabled  bool
}

func NewSIVICoverService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVICoverService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI cover service requires context ownership and an explicit feature lookup")
	}
	enabled, err := siviFeature(siviCoverFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVICoverService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVICoverService) require(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return errors.New("SIVI source-cover editing is disabled in this session")
	}
	return nil
}

func (s *SIVICoverService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIVegetation(ctx, contextID, plot, extended)
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
	edits := make([]siviHeightEdit, len(request.Edits))
	for i, edit := range request.Edits {
		edits[i] = siviHeightEdit{edit.RowID, edit.Form, edit.Column, edit.Expected, edit.Value}
	}
	result, err := s.contexts.writeSIVICovers(ctx, contextID, plot, extended, request.Original, edits)
	if err != nil {
		return nil, err
	}
	return &SIVICoverWriteResult{result.ChangedCells, result.HistoryID}, nil
}

func (s *SIVICoverService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVICovers(ctx, contextID, plot, historyID, action)
}
