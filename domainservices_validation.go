package main

import (
	"context"
	"errors"
)

type ValidationService struct {
	contexts *ContextService
}

func NewValidationService(contexts *ContextService) *ValidationService {
	return &ValidationService{contexts: contexts}
}

func (s *ValidationService) CheckSpeciesCodes(ctx context.Context, contextID string) (SpeciesCodeCheckReview, error) {
	if s == nil || s.contexts == nil {
		return SpeciesCodeCheckReview{}, errors.New("validation context is unavailable")
	}
	return s.contexts.ReviewSpeciesCodes(ctx, contextID)
}

func (s *ValidationService) LookupSpeciesCodeCheckTarget(ctx context.Context, contextID string, lookup VegetationSpeciesLookup) ([]SpeciesCodeCheckOption, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("validation context is unavailable")
	}
	return s.contexts.LookupSpeciesCodeCheckTarget(ctx, contextID, lookup)
}

func (s *ValidationService) ApplySpeciesCodeCheck(contextID string, updates []SpeciesCodeCheckUpdate) error {
	if s == nil || s.contexts == nil {
		return errors.New("validation context is unavailable")
	}
	return s.contexts.SaveSpeciesCodeCheck(contextID, updates)
}
