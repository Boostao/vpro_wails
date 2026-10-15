//go:build !windows

package main

import (
	"context"
	"errors"
)

func observePrinters(ctx context.Context) (PrinterObservation, error) {
	if ctx == nil {
		return PrinterObservation{}, errors.New("printer observation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return PrinterObservation{}, err
	}
	return PrinterObservation{}, errors.New("read-only installed/default printer observation is unavailable on this platform; no host setting was changed")
}
