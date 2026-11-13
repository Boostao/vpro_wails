//go:build !windows

package vprocore

import "github.com/boostao/vpro-wails/internal/sqlutil"

func InstallDesktopConfig(from, to string) error {
	return sqlutil.LinkOrCreateExclusive(from, to)
}
