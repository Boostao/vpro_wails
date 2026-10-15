package main

import (
	"context"
	"database/sql"
	"errors"
)

const twoPageVegetationSpeciesListEnvironment = "VPRO_TWO_PAGE_VEGETATION_SPECIES_LIST"

type TwoPageVegetationSpeciesListService struct {
	vegetation *TwoPageVegetationService
	enabled    bool
}

func NewTwoPageVegetationSpeciesListService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageVegetationSpeciesListService, error) {
	vegetation, err := NewTwoPageVegetationService(contexts, lookup)
	if err != nil {
		return nil, err
	}
	enabled, err := siviFeature(twoPageVegetationSpeciesListEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageVegetationSpeciesListService{vegetation, enabled}, nil
}

func (s *TwoPageVegetationSpeciesListService) Save(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	return s.saveObserved(ctx, contextID, requestJSON, nil, nil)
}

func (s *TwoPageVegetationSpeciesListService) saveObserved(ctx context.Context, contextID, requestJSON string,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*twoPageVegetationWriteResult, error) {
	if s == nil || s.vegetation == nil {
		return nil, errors.New("two-page Species list selection requires owned vegetation services")
	}
	if err := s.vegetation.require(ctx, true); err != nil {
		return nil, err
	}
	if !s.enabled {
		return nil, errors.New("two-page Species list selection requires its independently enabled feature")
	}
	return s.vegetation.saveTwoPageVegetationQualifiedObserved(ctx, contextID, requestJSON, true, beforeCommit, afterCommit)
}
