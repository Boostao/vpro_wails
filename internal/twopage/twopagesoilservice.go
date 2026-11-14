package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type TwoPageSoilOriginal = twoPageSoilOriginal
type TwoPageSoilWriteResult = twoPageSoilWriteResult
type TwoPageSoilReference = twoPageSoilReference
type TwoPageSoilReferences = twoPageSoilReferences

type TwoPageSoilService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageSoilService(contexts *vprocore.ContextService) (*TwoPageSoilService, error) {
	if contexts == nil {
		return nil, errors.New("two-page soil service requires owned contexts")
	}
	return &TwoPageSoilService{contexts: contexts}, nil
}

func (s *TwoPageSoilService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page soil service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page soil service requires owned contexts")
	}
	return nil
}

func (s *TwoPageSoilService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageSoilOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageSoilOriginal(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageSoilService) GetReferences(ctx context.Context, contextID, plot, form string) (*TwoPageSoilReferences, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageSoilReferences(s.contexts, ctx, contextID, plot, form)
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
	var request twoPageSoilRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if creation != (request.Horizon != nil) {
		return nil, errors.New("soil horizon creation requires its explicit action; ordinary Save cannot create records")
	}
	started, err := BeginTwoPageSoilAttempt(s.contexts, ctx, request)
	if err != nil {
		return nil, err
	}
	if !started {
		receipt, err := LookupTwoPageSoilReceipt(s.contexts, ctx, contextID, request)
		if err != nil {
			return nil, err
		}
		if receipt == nil {
			return nil, errors.New("two-page soil request was sealed without a commit; review a fresh original and use a new UUID")
		}
		return receipt, nil
	}
	defer FinishTwoPageSoilAttempt(s.contexts, request.RequestID)
	return WriteTwoPageSoilObserved(s.contexts, ctx, contextID, request, beforeCommit, afterCommit)
}

func (s *TwoPageSoilService) RecoverReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageSoilRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupTwoPageSoilReceipt(s.contexts, ctx, contextID, request)
}
