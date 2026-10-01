package main

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

func acquireReadLease(ctx context.Context, mutex *sync.RWMutex) error {
	return acquireContextLock(ctx, mutex.TryRLock, mutex.RLock, mutex.RUnlock)
}

func acquireMutexLease(ctx context.Context, mutex *sync.Mutex) error {
	return acquireContextLock(ctx, mutex.TryLock, mutex.Lock, mutex.Unlock)
}

func acquireContextLock(ctx context.Context, tryLock func() bool, lock, unlock func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Done() == nil {
		lock()
		return nil
	}
	if tryLock() {
		if err := ctx.Err(); err != nil {
			unlock()
			return err
		}
		return nil
	}
	// A queued switch must not leave a cancelled transport waiting on its lease.
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if tryLock() {
				if err := ctx.Err(); err != nil {
					unlock()
					return err
				}
				return nil
			}
		}
	}
}

func (s *PlotService) operationContext() context.Context {
	if s.requestContext != nil {
		return s.requestContext
	}
	return context.Background()
}

type contextPlotDB struct {
	db  *sql.DB
	ctx context.Context
}

func (s *PlotService) readDB(db *sql.DB) headerDB {
	return contextPlotDB{db: db, ctx: s.operationContext()}
}

func (db contextPlotDB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.db.QueryContext(db.ctx, query, args...)
}

func (db contextPlotDB) QueryRow(query string, args ...any) *sql.Row {
	return db.db.QueryRowContext(db.ctx, query, args...)
}
