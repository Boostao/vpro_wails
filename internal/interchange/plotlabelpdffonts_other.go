//go:build !windows

package interchange

import (
	"context"
	"errors"
	"os"
)

func loadPlotLabelPDFFonts(ctx context.Context) (plotLabelPDFFonts, error) {
	if ctx == nil {
		return plotLabelPDFFonts{}, errors.New("runtime label font requires a context")
	}
	if err := ctx.Err(); err != nil {
		return plotLabelPDFFonts{}, err
	}
	candidates := []struct {
		regularPath string
		boldPath    string
		regularName string
		boldName    string
	}{
		// macOS & Linux Microsoft CoreFonts Arial
		{"/Library/Fonts/Arial.ttf", "/Library/Fonts/Arial Bold.ttf", "ArialMT", "Arial-BoldMT"},
		{"/System/Library/Fonts/Supplemental/Arial.ttf", "/System/Library/Fonts/Supplemental/Arial Bold.ttf", "ArialMT", "Arial-BoldMT"},
		{"/usr/share/fonts/msttcore/arial.ttf", "/usr/share/fonts/msttcore/arialbd.ttf", "ArialMT", "Arial-BoldMT"},
		{"/usr/share/fonts/truetype/msttcorefonts/Arial.ttf", "/usr/share/fonts/truetype/msttcorefonts/Arial_Bold.ttf", "ArialMT", "Arial-BoldMT"},
		// Metric-identical Liberation Sans (Ascender Arial metrics on Fedora/RHEL, Debian/Ubuntu, Arch)
		{"/usr/share/fonts/liberation-sans-fonts/LiberationSans-Regular.ttf", "/usr/share/fonts/liberation-sans-fonts/LiberationSans-Bold.ttf", "LiberationSans", "LiberationSans-Bold"},
		{"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf", "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf", "LiberationSans", "LiberationSans-Bold"},
		{"/usr/share/fonts/truetype/liberation2/LiberationSans-Regular.ttf", "/usr/share/fonts/truetype/liberation2/LiberationSans-Bold.ttf", "LiberationSans", "LiberationSans-Bold"},
		{"/usr/share/fonts/liberation/LiberationSans-Regular.ttf", "/usr/share/fonts/liberation/LiberationSans-Bold.ttf", "LiberationSans", "LiberationSans-Bold"},
		{"/usr/share/fonts/TTF/LiberationSans-Regular.ttf", "/usr/share/fonts/TTF/LiberationSans-Bold.ttf", "LiberationSans", "LiberationSans-Bold"},
	}
	for _, cand := range candidates {
		if _, err := os.Stat(cand.regularPath); err != nil {
			continue
		}
		if _, err := os.Stat(cand.boldPath); err != nil {
			continue
		}
		regular, rErr := readPlotLabelPDFFont(ctx, cand.regularPath, cand.regularName, nil)
		if rErr != nil {
			if errors.Is(rErr, context.Canceled) || errors.Is(rErr, context.DeadlineExceeded) {
				return plotLabelPDFFonts{}, rErr
			}
			continue
		}
		bold, bErr := readPlotLabelPDFFont(ctx, cand.boldPath, cand.boldName, nil)
		if bErr != nil {
			if errors.Is(bErr, context.Canceled) || errors.Is(bErr, context.DeadlineExceeded) {
				return plotLabelPDFFonts{}, bErr
			}
			continue
		}
		return plotLabelPDFFonts{
			regular:     regular,
			bold:        bold,
			regularName: cand.regularName,
			boldName:    cand.boldName,
		}, nil
	}
	return loadBundledPlotLabelPDFFonts(ctx)
}
