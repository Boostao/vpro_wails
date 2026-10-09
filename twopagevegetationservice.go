package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const twoPageVegetationReviewEnvironment = "VPRO_TWO_PAGE_VEGETATION_REVIEW"
const twoPageVegetationWritingEnvironment = "VPRO_TWO_PAGE_VEGETATION_WRITING"

type TwoPageVegetationOriginal = twoPageVegetationOriginal
type TwoPageVegetationSpeciesReferences = twoPageVegetationSpeciesReferences
type TwoPageVegetationWriteResult = twoPageVegetationWriteResult

type TwoPageVegetationService struct {
	contexts        *ContextService
	review, writing bool
}

func NewTwoPageVegetationService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageVegetationService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page vegetation service requires owned contexts and explicit feature lookup")
	}
	review, err := siviFeature(twoPageVegetationReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	writing, err := siviFeature(twoPageVegetationWritingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageVegetationService{contexts: contexts, review: review, writing: writing}, nil
}

func (s *TwoPageVegetationService) require(ctx context.Context, writing bool) error {
	if ctx == nil {
		return errors.New("two-page vegetation service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.review {
		return errors.New("two-page vegetation reads require independently enabled vegetation review")
	}
	if writing && !s.writing {
		return errors.New("two-page vegetation writes require independently enabled vegetation writing")
	}
	return nil
}

func (s *TwoPageVegetationService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageVegetationOriginal(ctx, contextID, plot, form)
}

func (s *TwoPageVegetationService) GetSpeciesReferences(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationSpeciesReferences, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageVegetationSpeciesReferences(ctx, contextID, plot, form)
}

func (s *TwoPageVegetationService) Save(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	return s.saveTwoPageVegetationObserved(ctx, contextID, requestJSON, nil, nil)
}

func (s *TwoPageVegetationService) saveTwoPageVegetationObserved(ctx context.Context, contextID, requestJSON string,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*TwoPageVegetationWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	started, err := s.contexts.beginTwoPageVegetationAttempt(ctx, request)
	if err != nil {
		return nil, err
	}
	if !started {
		receipt, err := s.contexts.lookupTwoPageVegetationReceipt(ctx, contextID, request)
		if err != nil {
			return nil, err
		}
		if receipt == nil {
			return nil, errors.New("two-page vegetation request was sealed without a commit; review a fresh original and use a new UUID")
		}
		return receipt, nil
	}
	defer s.contexts.finishTwoPageVegetationAttempt(request.RequestID)
	return s.contexts.writeTwoPageVegetationObserved(ctx, contextID, request, beforeCommit, afterCommit)
}

func (s *TwoPageVegetationService) RecoverReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.lookupTwoPageVegetationReceipt(ctx, contextID, request)
}
