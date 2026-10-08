package main

import (
	"context"
	"errors"
	"fmt"
)

const twoPageParentReviewEnvironment = "VPRO_TWO_PAGE_PARENT_REVIEW"
const twoPageParentExtraEditingEnvironment = "VPRO_TWO_PAGE_PARENT_EXTRA_EDITING"

type TwoPageParentExtraWrite struct {
	Original *SIVIParentProjection `json:"original"`
	Edits    []SIVIParentCellEdit  `json:"edits"`
}

func (request *TwoPageParentExtraWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentExtraWrite
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("two-page additional-field transport: %w", err)
	}
	*request = TwoPageParentExtraWrite(decoded)
	return nil
}

type TwoPageParentExtraService struct {
	contexts                    *ContextService
	reviewEnabled, editsEnabled bool
}

func NewTwoPageParentExtraService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageParentExtraService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page parent service requires owned contexts and an explicit feature lookup")
	}
	review, err := siviFeature(twoPageParentReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	editing, err := siviFeature(twoPageParentExtraEditingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageParentExtraService{contexts: contexts, reviewEnabled: review, editsEnabled: editing}, nil
}

func (s *TwoPageParentExtraService) require(ctx context.Context, editing bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.reviewEnabled {
		return errors.New("two-page parent reads require independently enabled source review")
	}
	if editing && !s.editsEnabled {
		return errors.New("two-page additional-field writes require independently enabled editing")
	}
	return nil
}

func (s *TwoPageParentExtraService) GetOriginal(ctx context.Context, contextID, plot, form string) (*SIVIParentProjection, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageParent(ctx, contextID, plot, form)
}

func (s *TwoPageParentExtraService) Save(ctx context.Context, contextID, plot, form string, request TwoPageParentExtraWrite) (*SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.writeTwoPageParentExtra(ctx, contextID, plot, form, request.Original, request.Edits)
}

func (s *TwoPageParentExtraService) Restore(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.restoreTwoPageParentExtra(ctx, contextID, plot, form, historyID, action)
}
