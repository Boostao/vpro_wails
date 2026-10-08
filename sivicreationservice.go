package main

import (
	"context"
	"encoding/json"
	"errors"
)

const siviCreationFeatureEnvironment = "VPRO_SIVI_CREATION_WRITING"

type SIVICreationResult = siviCreationResult

type SIVICreationService struct {
	contexts *ContextService
	enabled  bool
}

func NewSIVICreationService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVICreationService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI creation requires explicit context ownership and feature lookup")
	}
	enabled, err := siviFeature(siviCreationFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVICreationService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVICreationService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI creation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return errors.New("SIVI creation is disabled in this session")
	}
	return nil
}

func (s *SIVICreationService) GetReferences(ctx context.Context, contextID, plot string) (SIVISpeciesReferences, error) {
	if err := s.require(ctx); err != nil {
		return SIVISpeciesReferences{}, err
	}
	return s.contexts.getSIVISpeciesReferences(ctx, contextID, plot)
}

func (s *SIVICreationService) Create(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.createSIVIVegetation(ctx, contextID, request)
}

func (s *SIVICreationService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.lookupSIVICreationReceipt(ctx, contextID, request)
}
