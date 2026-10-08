package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const siviCollectedFeatureEnvironment = "VPRO_SIVI_COLLECTED_EDITING"

type SIVICollectedEdit struct {
	RowID    string              `json:"rowId"`
	Form     string              `json:"form"`
	Expected ProjectMetadataCell `json:"expected"`
	Clicks   int                 `json:"clicks"`
}

func (edit *SIVICollectedEdit) UnmarshalJSON(data []byte) error {
	type plain SIVICollectedEdit
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "SIVI Collected draft", "rowId", "form", "expected", "clicks"); err != nil {
		return fmt.Errorf("SIVI Collected draft transport: %w", err)
	}
	*edit = SIVICollectedEdit(decoded)
	return nil
}

type SIVICollectedWrite struct {
	Original []SIVIVegetationProjection `json:"original"`
	Edits    []SIVICollectedEdit        `json:"edits"`
}

func (request *SIVICollectedWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICollectedWrite
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "SIVI Collected Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI Collected transport: %w", err)
	}
	*request = SIVICollectedWrite(decoded)
	return nil
}

type SIVICollectedService struct {
	contexts *ContextService
	enabled  bool
}

func NewSIVICollectedService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVICollectedService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI Collected service requires context ownership and an explicit feature lookup")
	}
	enabled, err := siviFeature(siviCollectedFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVICollectedService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVICollectedService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI Collected service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return errors.New("SIVI Collected editing is disabled in this session")
	}
	return nil
}

func (s *SIVICollectedService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIVegetation(ctx, contextID, plot, extended)
}

func (s *SIVICollectedService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
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
	return s.contexts.writeSIVIVegetationCells(ctx, contextID, plot, extended, request.Original, edits, siviCollectedWritePolicy())
}

func (s *SIVICollectedService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVIVegetationCells(ctx, contextID, plot, historyID, action, siviCollectedWritePolicy())
}
