package main

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func loadPlotLabelPDFFonts(ctx context.Context) (plotLabelPDFFonts, error) {
	directory, err := windows.GetWindowsDirectory()
	if err != nil {
		return plotLabelPDFFonts{}, fmt.Errorf("runtime label font Windows directory: %w", err)
	}
	return readPlotLabelPDFFonts(ctx, filepath.Join(directory, "Fonts"), nil)
}
