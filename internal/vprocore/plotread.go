package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"database/sql"
)

// A queued switch must not leave a cancelled transport waiting on its lease.

func (s *PlotService) OperationContext() context.Context {
	if s.requestContext != nil {
		return s.requestContext
	}
	return context.Background()
}

func (s *PlotService) ReadDB(db *sql.DB) sqlutil.HeaderDB {
	return sqlutil.ContextPlotDB{Db: db, Ctx: s.OperationContext()}
}
