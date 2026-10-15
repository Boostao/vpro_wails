//go:build !windows

package main

import (
	"errors"
	"time"
)

func formatPlotLabelNativeDate(time.Time) (plotLabelNativeDate, error) {
	return plotLabelNativeDate{}, errors.New("native Windows regional label dates are unavailable on this platform; no locale fallback was inferred")
}
