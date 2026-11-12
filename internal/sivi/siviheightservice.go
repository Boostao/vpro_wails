package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

const siviHeightFeatureEnvironment = "VPRO_SIVI_HEIGHT_EDITING"

func SiviHeightFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(siviHeightFeatureEnvironment, lookup)
}

func RequireSIVIHeights(s *vprocore.ContextService, ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.SiviHeightEnabled {
		return errors.New("SIVI aggregate-height editing is disabled in this session")
	}
	return nil
}

func GetSIVIVegetation(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := RequireSIVIHeights(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s, ctx, contextID, plot, extended)
}

func SaveSIVIHeights(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool, original []sourceform.SIVIVegetationProjection, edits []sourceform.SIVIHeightEdit) (*sourceform.SIVIHeightWriteResult, error) {
	if err := RequireSIVIHeights(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.WriteSIVIHeights(s, ctx, contextID, plot, extended, original, edits)
}

func RestoreSIVIHeights(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := RequireSIVIHeights(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVIHeights(s, ctx, contextID, plot, historyID, action)
}
