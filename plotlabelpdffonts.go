package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const plotLabelPDFFontMaxBytes = 8 * 1024 * 1024

type plotLabelPDFFontHandle interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

func readPlotLabelPDFFont(ctx context.Context, path, name string, open func(string) (plotLabelPDFFontHandle, error)) (data []byte, resultErr error) {
	if ctx == nil {
		return nil, errors.New("runtime label font requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if open == nil {
		open = func(path string) (plotLabelPDFFontHandle, error) { return os.Open(path) }
	}
	file, err := open(path)
	if err != nil {
		return nil, fmt.Errorf("runtime label font %s open: %w", name, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("runtime label font %s close: %w", name, err))
		}
		resultErr = errors.Join(resultErr, ctx.Err())
		if resultErr != nil {
			data = nil
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("runtime label font %s stat: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > plotLabelPDFFontMaxBytes {
		return nil, fmt.Errorf("runtime label font %s requires a nonempty bounded regular file", name)
	}
	data, err = io.ReadAll(io.LimitReader(artifactPublicationReader{ctx, file}, plotLabelPDFFontMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("runtime label font %s read: %w", name, err)
	}
	if len(data) != int(info.Size()) || len(data) > plotLabelPDFFontMaxBytes {
		return nil, fmt.Errorf("runtime label font %s size changed or exceeded its bound", name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validatePlotLabelPDFFont(data, name); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func readPlotLabelPDFFonts(ctx context.Context, directory string, open func(string) (plotLabelPDFFontHandle, error)) (plotLabelPDFFonts, error) {
	if directory == "" || !filepath.IsAbs(directory) {
		return plotLabelPDFFonts{}, errors.New("runtime label fonts require an absolute system font directory")
	}
	regular, err := readPlotLabelPDFFont(ctx, filepath.Join(directory, "arial.ttf"), "ArialMT", open)
	if err != nil {
		return plotLabelPDFFonts{}, err
	}
	bold, err := readPlotLabelPDFFont(ctx, filepath.Join(directory, "arialbd.ttf"), "Arial-BoldMT", open)
	if err != nil {
		return plotLabelPDFFonts{}, err
	}
	return plotLabelPDFFonts{regular, bold}, nil
}
