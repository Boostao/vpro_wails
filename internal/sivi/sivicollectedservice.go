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
