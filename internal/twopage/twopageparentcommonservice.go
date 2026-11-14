package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
)

type TwoPageParentCommonWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Edits    []sourceform.SIVIParentCellEdit  `json:"edits"`
}

func (request *TwoPageParentCommonWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentCommonWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("two-page common-field transport: %w", err)
	}
	*request = TwoPageParentCommonWrite(decoded)
	return nil
}

type TwoPageParentCommonService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageParentCommonService(contexts *vprocore.ContextService) (*TwoPageParentCommonService, error) {
	if contexts == nil {
		return nil, errors.New("two-page common-field service requires owned contexts")
	}
	return &TwoPageParentCommonService{contexts: contexts}, nil
}

func (s *TwoPageParentCommonService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page common-field service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page common-field service requires owned contexts")
	}
	return nil
}

func (s *TwoPageParentCommonService) GetOriginal(ctx context.Context, contextID, plot, form string) (*sourceform.SIVIParentProjection, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageParent(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageParentCommonService) Save(ctx context.Context, contextID, plot, form string, request TwoPageParentCommonWrite) (*sourceform.SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return WriteTwoPageParentCommon(s.contexts, ctx, contextID, plot, form, request.Original, request.Edits)
}

func (s *TwoPageParentCommonService) Restore(ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return RestoreTwoPageParentCommon(s.contexts, ctx, contextID, plot, form, historyID, action)
}
