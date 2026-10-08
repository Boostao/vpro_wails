package main

import (
	"context"
	"encoding/json"
	"errors"
)

const siviDeletionWritingFeatureEnvironment = "VPRO_SIVI_DELETION_WRITING"
const siviDeletionRestoringFeatureEnvironment = "VPRO_SIVI_DELETION_RESTORING"

type SIVIDeletionOriginal = siviDeletionOriginal
type SIVIDeletionResult = siviDeletionResult
type SIVIDeletionRestorationReview = siviDeletionRestorationReview
type SIVIDeletionRestorationResult = siviDeletionRestorationResult
type SIVIDeletionHistoryEvent = siviDeletionHistoryEvent
type SIVIDeletionHistoryList = siviDeletionHistoryList
type SIVIDeletionTargets = siviDeletionTargets

type SIVIDeletionService struct {
	contexts  *ContextService
	writing   bool
	restoring bool
}

func NewSIVIDeletionService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVIDeletionService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI deletion requires explicit context ownership and feature lookup")
	}
	writing, err := siviFeature(siviDeletionWritingFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	restoring, err := siviFeature(siviDeletionRestoringFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVIDeletionService{contexts: contexts, writing: writing, restoring: restoring}, nil
}

func (s *SIVIDeletionService) require(ctx context.Context, restoration bool) error {
	if ctx == nil {
		return errors.New("SIVI deletion requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI deletion requires explicit context ownership")
	}
	if restoration {
		if !s.restoring {
			return errors.New("SIVI deletion restoration is disabled in this session")
		}
	} else if !s.writing {
		return errors.New("SIVI deletion writing is disabled in this session")
	}
	return nil
}

func (s *SIVIDeletionService) GetOriginal(ctx context.Context, contextID, plot, form, rowID string) (*SIVIDeletionOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIDeletionOriginal(ctx, contextID, plot, form, rowID)
}

func (s *SIVIDeletionService) Delete(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request siviDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.deleteSIVIVegetation(ctx, contextID, request)
}

func (s *SIVIDeletionService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request siviDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.lookupSIVIDeletionReceipt(ctx, contextID, request)
}

func (s *SIVIDeletionService) ReviewRestoration(ctx context.Context, contextID, plot, historyID string) (*SIVIDeletionRestorationReview, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.reviewSIVIDeletionRestoration(ctx, contextID, plot, historyID)
}

func (s *SIVIDeletionService) Restore(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request siviDeletionRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVIDeletion(ctx, contextID, request)
}

func (s *SIVIDeletionService) LookupRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request siviDeletionRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return s.contexts.lookupSIVIDeletionRestorationReceipt(ctx, contextID, request)
}

func (s *SIVIDeletionService) GetHistory(ctx context.Context, contextID, plot string) (*SIVIDeletionHistoryList, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIDeletionHistory(ctx, contextID, plot)
}

func (s *SIVIDeletionService) GetTargets(ctx context.Context, contextID, plot string) (*SIVIDeletionTargets, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIDeletionTargets(ctx, contextID, plot)
}
