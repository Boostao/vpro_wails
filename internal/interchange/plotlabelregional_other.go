//go:build !windows

package interchange

import (
	"errors"
	"time"
)

func formatPlotLabelNativeDate(date time.Time) (plotLabelNativeDate, error) {
	if date.Year() < 1601 || date.Year() > 9999 {
		return plotLabelNativeDate{}, errors.New("native label date is outside the supported calendar range")
	}
	return plotLabelNativeDate{
		text:    date.Format("Monday, January 2, 2006"),
		locale:  "en-US",
		pattern: "dddd, MMMM d, yyyy",
	}, nil
}
