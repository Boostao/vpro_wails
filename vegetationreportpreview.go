package main

import (
	"context"
	"database/sql"
	"errors"
)

func (s *ContextService) previewLongVegetationLayers(ctx context.Context, contextID string) (vegetationLayerReport, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (vegetationLayerReport, error) {
		options, err := loadLongVegetationOptions(plots)
		if err != nil {
			return vegetationLayerReport{}, err
		}
		if err := s.checkLongVegetationGrouping(options); err != nil {
			return vegetationLayerReport{}, err
		}
		return readLongVegetationLayers(ctx, plots, options)
	})
}

func readLongVegetationLayers(ctx context.Context, plots *PlotService, options longVegetationOptions) (result vegetationLayerReport, resultErr error) {
	owner := plots.projects.sqlite
	if owner.selection.SU == "None" {
		return result, errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback")
	}
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		return result, err
	}
	defer owner.mu.Unlock()
	if err := profileOwnedFiles(owner); err != nil {
		return result, err
	}
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = vegetationLayerReport{}
		}
	}()
	source, err := readLongVegetationReportSource(ctx, owner, tx, options)
	if err != nil {
		return result, err
	}
	result = source.Report
	if err := profileOwnedFiles(owner); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
