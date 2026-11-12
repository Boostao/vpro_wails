package main

import (
	"context"
	"database/sql"
)

// A queued switch must not leave a cancelled transport waiting on its lease.

func (s *PlotService) operationContext() context.Context {
	if s.requestContext != nil {
		return s.requestContext
	}
	return context.Background()
}

func (s *PlotService) readDB(db *sql.DB) headerDB {
	return contextPlotDB{db: db, ctx: s.operationContext()}
}


