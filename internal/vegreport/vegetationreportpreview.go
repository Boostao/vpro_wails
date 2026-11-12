package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
)

func PreviewLongVegetationLayers(s *vprocore.ContextService, ctx context.Context, contextID string) (vegetationLayerReport, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (vegetationLayerReport, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (vegetationLayerReport, error) {
			values, err := plots.Projects.Preferences.Snapshot()
			if err != nil {
				return vegetationLayerReport{}, err
			}
			options, err := DecodeOwnedLongVegetationOptions(s, ctx, plots, tx, values)
			if err != nil {
				return vegetationLayerReport{}, err
			}
			if err := CheckLongVegetationGrouping(s, options); err != nil {
				return vegetationLayerReport{}, err
			}
			source, err := readLongVegetationReportSource(ctx, owner, tx, options)
			return source.Report, err
		})
	})
}

func readLongVegetationLayers(ctx context.Context, plots *vprocore.PlotService, options longVegetationOptions) (result vegetationLayerReport, resultErr error) {
	owner := plots.Projects.Sqlite
	if owner.Selection.SU == "None" {
		return result, errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return result, err
	}
	defer owner.Mu.Unlock()
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return result, err
	}
	tx, err := owner.BeginReadSnapshot(ctx)
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
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
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
