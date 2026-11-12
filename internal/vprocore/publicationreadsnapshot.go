package vprocore

import (
	"context"
	"database/sql"
	"errors"
)

type PublicationReadSnapshotHooks struct {
	CommitRead   func(*sql.Tx) error `json:"-" yaml:"-"`
	RollbackRead func(*sql.Tx) error `json:"-" yaml:"-"`
}

// Only read observations belong here; cleanup errors must never zero a committed file.
func WithPublicationReadSnapshot[T any](ctx context.Context, owner *SqliteContext, hooks PublicationReadSnapshotHooks, read func(*sql.Tx) (T, error)) (result T, resultErr error) {
	var zero T
	if err := ProfileOwnedFiles(owner); err != nil {
		return zero, err
	}
	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return zero, err
	}
	defer func() {
		err := tx.Rollback()
		if errors.Is(err, sql.ErrTxDone) {
			err = nil
		}
		if hooks.RollbackRead != nil {
			err = errors.Join(err, hooks.RollbackRead(tx))
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
	if err := ProfileOwnedFiles(owner); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if hooks.CommitRead != nil {
		if err := hooks.CommitRead(tx); err != nil {
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
