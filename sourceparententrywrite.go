package main

import "database/sql"

type sourceParentEntryLifecycle struct {
	prepare  func(*PlotService, *sql.Conn) (func(), error)
	verify   func(*PlotService, *sql.Tx) error
	decorate func(*siviParentHistory)
}
