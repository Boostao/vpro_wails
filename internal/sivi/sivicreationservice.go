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

type SIVICreationResult = sourceform.SiviCreationResult

type SIVICreationService struct {
	contexts *vprocore.ContextService
}

func NewSIVICreationService(contexts *vprocore.ContextService) (*SIVICreationService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI creation requires explicit context ownership")
	}
	return &SIVICreationService{contexts: contexts}, nil
}

func (s *SIVICreationService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI creation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI creation requires explicit context ownership")
	}
	return nil
}

func (s *SIVICreationService) GetReferences(ctx context.Context, contextID, plot string) (sourceform.SIVISpeciesReferences, error) {
	if err := s.require(ctx); err != nil {
		return sourceform.SIVISpeciesReferences{}, err
	}
	return sourceform.GetSIVISpeciesReferences(s.contexts, ctx, contextID, plot)
}

func (s *SIVICreationService) Create(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request sourceform.SiviCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.CreateSIVIVegetation(s.contexts, ctx, contextID, request)
}

func (s *SIVICreationService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request sourceform.SiviCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.LookupSIVICreationReceipt(s.contexts, ctx, contextID, request)
}
