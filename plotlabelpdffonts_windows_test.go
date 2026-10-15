package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type plotLabelPDFFaultFont struct {
	*os.File
	phase  string
	cancel context.CancelFunc
	closed *bool
}

func (f *plotLabelPDFFaultFont) Stat() (os.FileInfo, error) {
	if f.phase == "stat" {
		return nil, errors.New("planned font stat fault")
	}
	return f.File.Stat()
}

func (f *plotLabelPDFFaultFont) Read(data []byte) (int, error) {
	if f.phase == "read" {
		return 0, errors.New("planned font read fault")
	}
	n, err := f.File.Read(data)
	if f.phase == "cancel" {
		f.cancel()
	}
	return n, err
}

func (f *plotLabelPDFFaultFont) Close() error {
	*f.closed = true
	err := f.File.Close()
	if f.phase == "close" {
		err = errors.Join(err, errors.New("planned font close fault"))
	}
	if f.phase == "close-cancel" {
		f.cancel()
	}
	return err
}

func TestPlotLabelPDFFontsRuntimeAndFaults(t *testing.T) {
	fonts, err := loadPlotLabelPDFFonts(context.Background())
	if err != nil || len(fonts.regular) == 0 || len(fonts.bold) == 0 {
		t.Fatalf("runtime fonts: %v", err)
	}
	directory := t.TempDir()
	for _, font := range []struct {
		name string
		data []byte
	}{{"arial.ttf", fonts.regular}, {"arialbd.ttf", fonts.bold}} {
		if err := os.WriteFile(filepath.Join(directory, font.name), font.data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, phase := range []string{"stat", "read", "close", "cancel", "close-cancel"} {
		ctx, cancel := context.WithCancel(context.Background())
		closed := false
		open := func(path string) (plotLabelPDFFontHandle, error) {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			return &plotLabelPDFFaultFont{file, phase, cancel, &closed}, nil
		}
		actual, err := readPlotLabelPDFFonts(ctx, directory, open)
		cancel()
		if err == nil || !closed || actual.regular != nil || actual.bold != nil {
			t.Fatalf("%s: fonts=%v, closed=%v, err=%v", phase, actual, closed, err)
		}
		if strings.Contains(phase, "cancel") && !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight cancellation: %v", err)
		}
		if !strings.Contains(phase, "cancel") && !strings.Contains(err.Error(), phase+" fault") {
			t.Fatalf("%s fault hidden: %v", phase, err)
		}
	}
	if err := os.Remove(filepath.Join(directory, "arialbd.ttf")); err != nil {
		t.Fatal(err)
	}
	if actual, err := readPlotLabelPDFFonts(context.Background(), directory, nil); !errors.Is(err, os.ErrNotExist) ||
		actual.regular != nil || actual.bold != nil {
		t.Fatalf("partial font pair on missing bold: %v, %v", actual, err)
	}
}

func TestPlotLabelPDFFontReadBounds(t *testing.T) {
	ctx := context.Background()
	for _, size := range []int64{0, plotLabelPDFFontMaxBytes + 1} {
		path := filepath.Join(t.TempDir(), "font.ttf")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if data, err := readPlotLabelPDFFont(ctx, path, "ArialMT", nil); err == nil || data != nil {
			t.Fatalf("size %d: %d bytes, %v", size, len(data), err)
		}
	}
	if data, err := readPlotLabelPDFFont(ctx, t.TempDir(), "ArialMT", nil); err == nil || data != nil {
		t.Fatalf("directory font accepted: %v", err)
	}
	if data, err := readPlotLabelPDFFont(nil, "unused", "ArialMT", nil); err == nil || data != nil {
		t.Fatalf("nil context accepted: %v", err)
	}
}
