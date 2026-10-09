package main

import (
	"context"
	"errors"
	"fmt"
)

const twoPageParentCommonEditingEnvironment = "VPRO_TWO_PAGE_PARENT_COMMON_EDITING"

type TwoPageParentCommonWrite struct {
	Original *SIVIParentProjection `json:"original"`
	Edits    []SIVIParentCellEdit  `json:"edits"`
}

func (request *TwoPageParentCommonWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentCommonWrite
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("two-page common-field transport: %w", err)
	}
	*request = TwoPageParentCommonWrite(decoded)
	return nil
}

type TwoPageParentCommonService struct {
	contexts                    *ContextService
	reviewEnabled, editsEnabled bool
}

func NewTwoPageParentCommonService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageParentCommonService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page common-field service requires owned contexts and an explicit feature lookup")
	}
	review, err := siviFeature(twoPageParentReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	editing, err := siviFeature(twoPageParentCommonEditingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageParentCommonService{contexts: contexts, reviewEnabled: review, editsEnabled: editing}, nil
}

func (s *TwoPageParentCommonService) require(ctx context.Context, editing bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.reviewEnabled {
		return errors.New("two-page common-field reads require independently enabled source review")
	}
	if editing && !s.editsEnabled {
		return errors.New("two-page common-field writes require independently enabled editing")
	}
	return nil
}

func (s *TwoPageParentCommonService) GetOriginal(ctx context.Context, contextID, plot, form string) (*SIVIParentProjection, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageParent(ctx, contextID, plot, form)
}

func (s *TwoPageParentCommonService) Save(ctx context.Context, contextID, plot, form string, request TwoPageParentCommonWrite) (*SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.writeTwoPageParentCommon(ctx, contextID, plot, form, request.Original, request.Edits)
}

func (s *TwoPageParentCommonService) Restore(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.restoreTwoPageParentCommon(ctx, contextID, plot, form, historyID, action)
}
