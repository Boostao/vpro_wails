package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *ContextService) readSIVIVegetation(ctx context.Context, contextID, plot string, extended bool) ([]siviVegetationProjection, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]siviVegetationProjection, error) {
		return readOwnedSIVIVegetation(ctx, plots, plot, extended)
	})
}

func readOwnedSIVIVegetation(ctx context.Context, plots *PlotService, plot string, extended bool) (result []siviVegetationProjection, resultErr error) {
	owner := plots.projects.sqlite
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		return nil, err
	}
	defer owner.mu.Unlock()
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = nil
		}
	}()
	table := owner.selection.Project + "_Veg"
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("SIVI requires original physical project table %q", table)
	}
	veg, err := readSQLiteStorageRows(ctx, tx, "project", table, "PlotNumber", &plot, "")
	if err != nil {
		return nil, err
	}
	result, err = projectSIVIVegetation(ctx, plot, extended, veg)
	if err != nil {
		return nil, err
	}
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
