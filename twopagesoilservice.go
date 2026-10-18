package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const twoPageSoilReviewEnvironment = "VPRO_TWO_PAGE_SOIL_REVIEW"
const twoPageSoilWritingEnvironment = "VPRO_TWO_PAGE_SOIL_WRITING"
const twoPageSoilCreationEnvironment = "VPRO_TWO_PAGE_SOIL_CREATION"

type TwoPageSoilOriginal = twoPageSoilOriginal
type TwoPageSoilWriteResult = twoPageSoilWriteResult
type TwoPageSoilReference = twoPageSoilReference
type TwoPageSoilReferences = twoPageSoilReferences

type TwoPageSoilService struct {
	contexts        *ContextService
	review, writing bool
	creation        bool
}

func NewTwoPageSoilService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageSoilService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page soil service requires owned contexts and explicit feature lookup")
	}
	review, err := siviFeature(twoPageSoilReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	writing, err := siviFeature(twoPageSoilWritingEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	creation, err := siviFeature(twoPageSoilCreationEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageSoilService{contexts: contexts, review: review, writing: writing, creation: creation}, nil
}

func (s *TwoPageSoilService) require(ctx context.Context, writing bool) error {
	if ctx == nil {
		return errors.New("two-page soil service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.review {
		return errors.New("two-page soil reads require independently enabled soil review")
	}
	if writing && !s.writing {
		return errors.New("two-page soil writes require independently enabled soil writing")
	}
	return nil
}

func (s *TwoPageSoilService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageSoilOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageSoilOriginal(ctx, contextID, plot, form)
}

func (s *TwoPageSoilService) GetReferences(ctx context.Context, contextID, plot, form string) (*TwoPageSoilReferences, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readTwoPageSoilReferences(ctx, contextID, plot, form)
}

func (s *TwoPageSoilService) Save(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	return s.saveTwoPageSoilObserved(ctx, contextID, requestJSON, nil, nil)
}

func (s *TwoPageSoilService) saveTwoPageSoilObserved(ctx context.Context, contextID, requestJSON string,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageSoilWriteResult) error) (*TwoPageSoilWriteResult, error) {
	return s.dispatchTwoPageSoilObserved(ctx, contextID, requestJSON, false, beforeCommit, afterCommit)
}

func (s *TwoPageSoilService) CreateHorizon(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	return s.dispatchTwoPageSoilObserved(ctx, contextID, requestJSON, true, nil, nil)
}

func (s *TwoPageSoilService) dispatchTwoPageSoilObserved(ctx context.Context, contextID, requestJSON string, creation bool,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageSoilWriteResult) error) (*TwoPageSoilWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	if creation && !s.creation {
		return nil, errors.New("soil horizon creation requires independently enabled creation review")
	}
	var request twoPageSoilRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if creation != (request.Horizon != nil) {
		return nil, errors.New("soil horizon creation requires its explicit action; ordinary Save cannot create records")
	}
	started, err := s.contexts.beginTwoPageSoilAttempt(ctx, request)
	if err != nil {
		return nil, err
	}
	if !started {
		receipt, err := s.contexts.lookupTwoPageSoilReceipt(ctx, contextID, request)
		if err != nil {
			return nil, err
		}
		if receipt == nil {
			return nil, errors.New("two-page soil request was sealed without a commit; review a fresh original and use a new UUID")
		}
		return receipt, nil
	}
	defer s.contexts.finishTwoPageSoilAttempt(request.RequestID)
	return s.contexts.writeTwoPageSoilObserved(ctx, contextID, request, beforeCommit, afterCommit)
}

func (s *TwoPageSoilService) RecoverReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageSoilRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.lookupTwoPageSoilReceipt(ctx, contextID, request)
}
