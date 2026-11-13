package interchange

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func loadPlotLabelPDFFonts(ctx context.Context) (plotLabelPDFFonts, error) {
	directory, err := windows.GetWindowsDirectory()
	if err != nil {
		return plotLabelPDFFonts{}, fmt.Errorf("runtime label font Windows directory: %w", err)
	}
	fonts, err := readPlotLabelPDFFonts(ctx, filepath.Join(directory, "Fonts"), nil)
	if err == nil {
		return fonts, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return plotLabelPDFFonts{}, ctx.Err()
	}
	if errors.Is(err, os.ErrNotExist) {
		return loadBundledPlotLabelPDFFonts(ctx)
	}
	return plotLabelPDFFonts{}, err
}
