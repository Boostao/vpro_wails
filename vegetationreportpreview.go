package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Internal acceptance boundary: no Wails binding or report UI is enabled yet.
func (s *ContextService) previewLongVegetationLayers(ctx context.Context, contextID string) (vegetationLayerReport, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result vegetationLayerReport, resultErr error) {
		values, err := plots.projects.preferences.snapshot()
		if err != nil {
			return result, err
		}
		options, err := decodeLongVegetationOptions(values)
		if err != nil {
			return result, err
		}
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
		tx, err := owner.conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
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
		tables := make([]ProjectMetadataTable, 4)
		for i, source := range []struct{ role, table string }{
			{"project", owner.selection.Project + "_Veg"},
			{"su", owner.selection.SU + "_SU"},
			{"VLists", "USysAllSpecs"},
			{"VPro64", "LayerCode"},
		} {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
				`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
				return result, err
			}
			if count != 1 {
				return result, fmt.Errorf("Long Vegetation requires original physical table %s.%s", source.role, source.table)
			}
			tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
			if err != nil {
				return result, err
			}
		}
		prepared, err := prepareLongVegetation(ctx, owner.selection.Project, owner.selection.SU, tables[0], tables[1], tables[3])
		if err != nil {
			return result, err
		}
		result, err = planLongVegetationLayers(ctx, prepared, tables[2], tables[3], options)
		if err != nil {
			return result, err
		}
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return result, nil
	})
}
