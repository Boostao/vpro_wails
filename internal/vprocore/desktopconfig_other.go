//go:build !windows

package vprocore

import "os"

func InstallDesktopConfig(from, to string) error {
	return os.Link(from, to)
}
