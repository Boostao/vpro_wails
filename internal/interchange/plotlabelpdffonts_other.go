//go:build !windows

package interchange

import (
	"context"
	"errors"
)

func loadPlotLabelPDFFonts(ctx context.Context) (plotLabelPDFFonts, error) {
	return plotLabelPDFFonts{}, errors.New("runtime Arial label PDF fonts are unavailable on this platform")
}
