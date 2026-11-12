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

const SiviCombinedFeatureEnvironment = "VPRO_SIVI_COMBINED_EDITING"

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
	enabled  bool
}

func NewSIVICombinedService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*SIVICombinedService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI combined service requires context ownership and an explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(SiviCombinedFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVICombinedService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVICombinedService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI combined service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return errors.New("SIVI combined cover/height editing is disabled in this session")
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
