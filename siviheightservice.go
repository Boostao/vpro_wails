package main

import (
	"context"
	"errors"
	"fmt"
)

const siviHeightFeatureEnvironment = "VPRO_SIVI_HEIGHT_EDITING"

func siviHeightFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(siviHeightFeatureEnvironment, lookup)
}

func siviFeature(name string, lookup func(string) (string, bool)) (bool, error) {
	value, present := lookup(name)
	if !present {
		return false, nil
	}
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s requires literal true or false", name)
	}
}

func (s *ContextService) requireSIVIHeights(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.siviHeightEnabled {
		return errors.New("SIVI aggregate-height editing is disabled in this session")
	}
	return nil
}

func (s *ContextService) GetSIVIVegetation(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if err := s.requireSIVIHeights(ctx); err != nil {
		return nil, err
	}
	return s.readSIVIVegetation(ctx, contextID, plot, extended)
}

func (s *ContextService) SaveSIVIHeights(ctx context.Context, contextID, plot string, extended bool, original []SIVIVegetationProjection, edits []SIVIHeightEdit) (*SIVIHeightWriteResult, error) {
	if err := s.requireSIVIHeights(ctx); err != nil {
		return nil, err
	}
	return s.writeSIVIHeights(ctx, contextID, plot, extended, original, edits)
}

func (s *ContextService) RestoreSIVIHeights(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.requireSIVIHeights(ctx); err != nil {
		return nil, err
	}
	return s.restoreSIVIHeights(ctx, contextID, plot, historyID, action)
}
