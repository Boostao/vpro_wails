package sivi

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
)

type SIVICreationUndoReview = siviCreationUndoReview
type SIVICreationUndoResult = siviCreationUndoResult
type SIVICreationUndoHistoryEvent = siviCreationUndoHistoryEvent
type SIVICreationUndoHistoryList = siviCreationUndoHistoryList

type SIVICreationUndoService struct {
	contexts *vprocore.ContextService
}

func NewSIVICreationUndoService(contexts *vprocore.ContextService) (*SIVICreationUndoService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI creation Undo requires explicit context ownership")
	}
	return &SIVICreationUndoService{contexts: contexts}, nil
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
	return nil
}

func (s *SIVICreationUndoService) GetHistory(ctx context.Context, contextID, plot string) (*SIVICreationUndoHistoryList, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return ReadSIVICreationUndoHistory(s.contexts, ctx, contextID, plot)
}

func (s *SIVICreationUndoService) Review(ctx context.Context, contextID, plot, historyID string) (*SIVICreationUndoReview, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return ReviewSIVICreationUndo(s.contexts, ctx, contextID, plot, historyID)
}

func (s *SIVICreationUndoService) Undo(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationUndoRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return UndoSIVICreation(s.contexts, ctx, contextID, request)
}

func (s *SIVICreationUndoService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationUndoRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupSIVICreationUndoReceipt(s.contexts, ctx, contextID, request)
}
