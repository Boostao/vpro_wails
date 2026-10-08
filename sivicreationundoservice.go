package main

import (
	"context"
	"encoding/json"
	"errors"
)

const siviCreationUndoFeatureEnvironment = "VPRO_SIVI_CREATION_UNDO"

type SIVICreationUndoReview = siviCreationUndoReview
type SIVICreationUndoResult = siviCreationUndoResult
type SIVICreationUndoHistoryEvent = siviCreationUndoHistoryEvent
type SIVICreationUndoHistoryList = siviCreationUndoHistoryList

type SIVICreationUndoService struct {
	contexts *ContextService
	enabled  bool
}

func NewSIVICreationUndoService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVICreationUndoService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI creation Undo requires explicit context ownership and feature lookup")
	}
	enabled, err := siviFeature(siviCreationUndoFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVICreationUndoService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVICreationUndoService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI creation Undo requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI creation Undo requires explicit context ownership")
	}
	if !s.enabled {
		return errors.New("SIVI creation Undo is disabled in this session")
	}
	return nil
}

func (s *SIVICreationUndoService) GetHistory(ctx context.Context, contextID, plot string) (*SIVICreationUndoHistoryList, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVICreationUndoHistory(ctx, contextID, plot)
}

func (s *SIVICreationUndoService) Review(ctx context.Context, contextID, plot, historyID string) (*SIVICreationUndoReview, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.reviewSIVICreationUndo(ctx, contextID, plot, historyID)
}

func (s *SIVICreationUndoService) Undo(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationUndoRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.undoSIVICreation(ctx, contextID, request)
}

func (s *SIVICreationUndoService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationUndoRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.lookupSIVICreationUndoReceipt(ctx, contextID, request)
}
