package main

import (
	"context"
	"database/sql"
	"errors"
)

type publicationReadSnapshotHooks struct {
	commitRead   func(*sql.Tx) error
	rollbackRead func(*sql.Tx) error
}

// Only read observations belong here; cleanup errors must never zero a committed file.
func withPublicationReadSnapshot[T any](ctx context.Context, owner *sqliteContext, hooks publicationReadSnapshotHooks, read func(*sql.Tx) (T, error)) (result T, resultErr error) {
	var zero T
	if err := profileOwnedFiles(owner); err != nil {
		return zero, err
	}
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		return zero, err
	}
	defer func() {
		err := tx.Rollback()
		if errors.Is(err, sql.ErrTxDone) {
			err = nil
		}
		if hooks.rollbackRead != nil {
			err = errors.Join(err, hooks.rollbackRead(tx))
		}
		resultErr = errors.Join(resultErr, err)
		if resultErr != nil {
			result = zero
		}
	}()
	result, err = read(tx)
	if err != nil {
		return zero, err
	}
	if err := profileOwnedFiles(owner); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if hooks.commitRead != nil {
		if err := hooks.commitRead(tx); err != nil {
			return zero, err
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return result, nil
}
