package main

import (
	"context"
	"errors"

	projectopspkg "github.com/boostao/vpro-wails/internal/projectops"
	validationpkg "github.com/boostao/vpro-wails/internal/validation"
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

func (s *ValidationService) RunComplianceValidation(ctx context.Context, contextID string, opts validationpkg.ComplianceOptions) (validationpkg.ComplianceValidationReport, error) {
	if s == nil || s.contexts == nil {
		return validationpkg.ComplianceValidationReport{}, errors.New("validation context is unavailable")
	}
	return s.contexts.RunComplianceValidation(ctx, contextID, opts)
}

func (s *ValidationService) QuarantineOrphanRecords(ctx context.Context, contextID string, req validationpkg.QuarantineOrphanRequest) (validationpkg.QuarantineOrphanResult, error) {
	if s == nil || s.contexts == nil {
		return validationpkg.QuarantineOrphanResult{}, errors.New("validation context is unavailable")
	}
	return s.contexts.QuarantineOrphanRecords(ctx, contextID, req)
}

func (s *ValidationService) PreviewOptimizeVeg(ctx context.Context, contextID string, opts projectopspkg.OptimizeVegOptions) (projectopspkg.OptimizeVegPreview, error) {
	if s == nil || s.contexts == nil {
		return projectopspkg.OptimizeVegPreview{}, errors.New("validation context is unavailable")
	}
	return s.contexts.PreviewOptimizeVeg(ctx, contextID, opts)
}

func (s *ValidationService) ApplyOptimizeVeg(ctx context.Context, contextID string, req projectopspkg.ApplyOptimizeVegRequest) (projectopspkg.OptimizeVegResult, error) {
	if s == nil || s.contexts == nil {
		return projectopspkg.OptimizeVegResult{}, errors.New("validation context is unavailable")
	}
	return s.contexts.ApplyOptimizeVeg(ctx, contextID, req)
}

func (s *ValidationService) PreviewThreeWayMerge(ctx context.Context, contextID string, req projectopspkg.ThreeWayMergePreviewRequest) (projectopspkg.ThreeWayMergePreview, error) {
	if s == nil || s.contexts == nil {
		return projectopspkg.ThreeWayMergePreview{}, errors.New("validation context is unavailable")
	}
	return s.contexts.PreviewThreeWayMerge(ctx, contextID, req)
}

func (s *ValidationService) ExecuteThreeWayMerge(ctx context.Context, contextID string, req projectopspkg.ThreeWayMergeExecuteRequest) (projectopspkg.ThreeWayMergeResult, error) {
	if s == nil || s.contexts == nil {
		return projectopspkg.ThreeWayMergeResult{}, errors.New("validation context is unavailable")
	}
	return s.contexts.ExecuteThreeWayMerge(ctx, contextID, req)
}
