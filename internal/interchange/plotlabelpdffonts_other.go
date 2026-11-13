//go:build !windows

package interchange

import (
	"context"
	"path/filepath"
)

func loadPlotLabelPDFFonts(ctx context.Context) (plotLabelPDFFonts, error) {
	for _, candidateDir := range []string{
		"/Library/Fonts",
		"/System/Library/Fonts/Supplemental",
		"/usr/share/fonts/truetype/msttcorefonts",
	} {
		if fonts, err := readPlotLabelPDFFonts(ctx, filepath.Clean(candidateDir), nil); err == nil {
			return fonts, nil
		}
		if ctx != nil && ctx.Err() != nil {
			return plotLabelPDFFonts{}, ctx.Err()
		}
	}
	return loadBundledPlotLabelPDFFonts(ctx)
}
