package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
)

type SIVIDeletionOriginal = sourceform.SiviDeletionOriginal
type SIVIDeletionResult = sourceform.SiviDeletionResult
type SIVIDeletionRestorationReview = siviDeletionRestorationReview
type SIVIDeletionRestorationResult = siviDeletionRestorationResult
type SIVIDeletionHistoryEvent = siviDeletionHistoryEvent
type SIVIDeletionHistoryList = siviDeletionHistoryList
type SIVIDeletionTargets = siviDeletionTargets

type SIVIDeletionService struct {
	contexts *vprocore.ContextService
}

func NewSIVIDeletionService(contexts *vprocore.ContextService) (*SIVIDeletionService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI deletion requires explicit context ownership")
	}
	return &SIVIDeletionService{contexts: contexts}, nil
}

func (s *SIVIDeletionService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("SIVI deletion requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI deletion requires explicit context ownership")
	}
	return nil
}

func (s *SIVIDeletionService) GetOriginal(ctx context.Context, contextID, plot, form, rowID string) (*SIVIDeletionOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIDeletionOriginal(s.contexts, ctx, contextID, plot, form, rowID)
}

func (s *SIVIDeletionService) Delete(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request sourceform.SiviDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.DeleteSIVIVegetation(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request sourceform.SiviDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.LookupSIVIDeletionReceipt(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) ReviewRestoration(ctx context.Context, contextID, plot, historyID string) (*SIVIDeletionRestorationReview, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return ReviewSIVIDeletionRestoration(s.contexts, ctx, contextID, plot, historyID)
}

func (s *SIVIDeletionService) Restore(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request siviDeletionRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return RestoreSIVIDeletion(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) LookupRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request siviDeletionRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupSIVIDeletionRestorationReceipt(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) GetHistory(ctx context.Context, contextID, plot string) (*SIVIDeletionHistoryList, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return ReadSIVIDeletionHistory(s.contexts, ctx, contextID, plot)
}

func (s *SIVIDeletionService) GetTargets(ctx context.Context, contextID, plot string) (*SIVIDeletionTargets, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadSIVIDeletionTargets(s.contexts, ctx, contextID, plot)
}
