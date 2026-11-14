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

type TwoPageVegetationOriginal = twoPageVegetationOriginal
type TwoPageVegetationSpeciesReferences = twoPageVegetationSpeciesReferences
type TwoPageVegetationWriteResult = twoPageVegetationWriteResult

type TwoPageVegetationService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageVegetationService(contexts *vprocore.ContextService) (*TwoPageVegetationService, error) {
	if contexts == nil {
		return nil, errors.New("two-page vegetation service requires owned contexts")
	}
	return &TwoPageVegetationService{contexts: contexts}, nil
}

func (s *TwoPageVegetationService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page vegetation service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page vegetation service requires owned contexts")
	}
	return nil
}

func (s *TwoPageVegetationService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageVegetationOriginal(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageVegetationService) GetSpeciesReferences(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationSpeciesReferences, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageVegetationSpeciesReferences(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageVegetationService) Save(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	return s.saveTwoPageVegetationObserved(ctx, contextID, requestJSON, nil, nil)
}

func (s *TwoPageVegetationService) saveTwoPageVegetationObserved(ctx context.Context, contextID, requestJSON string,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*TwoPageVegetationWriteResult, error) {
	return s.saveTwoPageVegetationQualifiedObserved(ctx, contextID, requestJSON, false, beforeCommit, afterCommit)
}

func (s *TwoPageVegetationService) saveTwoPageVegetationQualifiedObserved(ctx context.Context, contextID, requestJSON string,
	speciesList bool, beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*twoPageVegetationWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if (request.SpeciesList != nil) != speciesList {
		return nil, errors.New("two-page Species list requests require their independently enabled selection service; legacy Save does not grant Species")
	}
	started, err := BeginTwoPageVegetationAttempt(s.contexts, ctx, request)
	if err != nil {
		return nil, err
	}
	if !started {
		receipt, err := LookupTwoPageVegetationReceipt(s.contexts, ctx, contextID, request)
		if err != nil {
			return nil, err
		}
		if receipt == nil {
			return nil, errors.New("two-page vegetation request was sealed without a commit; review a fresh original and use a new UUID")
		}
		return receipt, nil
	}
	defer FinishTwoPageVegetationAttempt(s.contexts, request.RequestID)
	return WriteTwoPageVegetationObserved(s.contexts, ctx, contextID, request, beforeCommit, afterCommit)
}

func (s *TwoPageVegetationService) RecoverReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupTwoPageVegetationReceipt(s.contexts, ctx, contextID, request)
}
