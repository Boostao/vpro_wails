package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

type TwoPageVegetationSpeciesListService struct {
	vegetation *TwoPageVegetationService
}

func NewTwoPageVegetationSpeciesListService(contexts *vprocore.ContextService) (*TwoPageVegetationSpeciesListService, error) {
	vegetation, err := NewTwoPageVegetationService(contexts)
	if err != nil {
		return nil, err
	}
	return &TwoPageVegetationSpeciesListService{vegetation: vegetation}, nil
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
	return s.vegetation.saveTwoPageVegetationQualifiedObserved(ctx, contextID, requestJSON, true, beforeCommit, afterCommit)
}
