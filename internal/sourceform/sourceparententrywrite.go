package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "database/sql"

type SourceParentEntryLifecycle struct {
	Prepare  func(*vprocore.PlotService, *sql.Conn) (func(), error) `json:"-" yaml:"-"`
	Verify   func(*vprocore.PlotService, *sql.Tx) error             `json:"-" yaml:"-"`
	Decorate func(*SiviParentHistory)                               `json:"-" yaml:"-"`
}
