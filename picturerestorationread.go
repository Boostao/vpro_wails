package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func withPictureHistoryReadSnapshot[T any](ctx context.Context, source *ownedPictureLibrary,
	read func(*sql.Tx) (T, error)) (result T, resultErr error) {
	var zero T
	if err := source.checkFile(); err != nil {
		return zero, err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro")+"&_query_only=on&_busy_timeout=5000")
	if err != nil {
		return zero, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, db.Close())
		if resultErr != nil {
			result = zero
		}
	}()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
	reader := &sqliteContext{conn: conn}
	tx, err := reader.beginReadSnapshot(ctx)
	if err != nil {
		return zero, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	result, err = read(tx)
	if err != nil {
		return zero, err
	}
	if err := errors.Join(source.checkFile(), ctx.Err()); err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, fmt.Errorf("picture history receipt snapshot: %w", err)
	}
	return result, nil
}

func (s *ContextService) lookupPictureRestorationReceipt(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureRestorationRequest) (*pictureRestorationResult, error) {
	if ctx == nil || s == nil || s.projects == nil || s.plots == nil {
		return nil, errors.New("picture restoration recovery requires an owned service and request context")
	}
	if err := validatePictureRestoration(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*pictureRestorationResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (*pictureRestorationResult, error) {
			if err := requirePictureParent(ctx, owner, parent, request.Deletion.PlotNumber); err != nil {
				return nil, err
			}
			authority, err := pictureRestorationAuthority(ctx, plots, owner, contextID, source, request.Deletion)
			if err != nil {
				return nil, err
			}
			return withPictureHistoryReadSnapshot(ctx, source, func(tx *sql.Tx) (*pictureRestorationResult, error) {
				_, receipt, err := findPictureRestoration(ctx, tx, source, request, authority)
				return receipt, err
			})
		})
	})
}
