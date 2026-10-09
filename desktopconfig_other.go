//go:build !windows

package main

import "os"

func installDesktopConfig(from, to string) error {
	return os.Link(from, to)
}
