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

type TwoPageParentExtraWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Edits    []sourceform.SIVIParentCellEdit  `json:"edits"`
}

func (request *TwoPageParentExtraWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentExtraWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("two-page additional-field transport: %w", err)
	}
	*request = TwoPageParentExtraWrite(decoded)
	return nil
}

type TwoPageParentExtraService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageParentExtraService(contexts *vprocore.ContextService) (*TwoPageParentExtraService, error) {
	if contexts == nil {
		return nil, errors.New("two-page parent service requires owned contexts")
	}
	return &TwoPageParentExtraService{contexts: contexts}, nil
}

func (s *TwoPageParentExtraService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page parent service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page parent service requires owned contexts")
	}
	return nil
}

func (s *TwoPageParentExtraService) GetOriginal(ctx context.Context, contextID, plot, form string) (*sourceform.SIVIParentProjection, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageParent(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageParentExtraService) Save(ctx context.Context, contextID, plot, form string, request TwoPageParentExtraWrite) (*sourceform.SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return WriteTwoPageParentExtra(s.contexts, ctx, contextID, plot, form, request.Original, request.Edits)
}

func (s *TwoPageParentExtraService) Restore(ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return RestoreTwoPageParentExtra(s.contexts, ctx, contextID, plot, form, historyID, action)
}
