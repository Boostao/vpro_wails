package main

import (
	"context"
	"database/sql"
	"errors"
)

func withOwnedSIVISnapshot[T any](ctx context.Context, plots *PlotService, read func(*sqliteContext, *sql.Tx) (T, error)) (result T, resultErr error) {
	var zero T
	owner := plots.projects.sqlite
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		return zero, err
	}
	defer owner.mu.Unlock()
	if err := profileOwnedFiles(owner); err != nil {
		return zero, err
	}
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		return zero, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = zero
		}
	}()
	result, err = read(owner, tx)
	if err != nil {
		return zero, err
	}
	if err := profileOwnedFiles(owner); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}
