package main

import (
	"context"
	"errors"
	"testing"
)

type plotLabelPDFCancelContext struct {
	context.Context
	cancel     context.CancelFunc
	checks, at int
}

func (c *plotLabelPDFCancelContext) Err() error {
	c.checks++
	if c.checks == c.at {
		c.cancel()
	}
	return c.Context.Err()
}

func TestPlotLabelPDFCancellationAfterWork(t *testing.T) {
	fonts := plotLabelPDFWindowsTestFonts(t)
	for _, at := range []int{4, 10} {
		ctx, cancel := context.WithCancel(context.Background())
		owned := &plotLabelPDFCancelContext{Context: ctx, cancel: cancel, at: at}
		output, err := renderPlotLabelPDF(owned, plotLabelPDFTestPreview(), fonts)
		cancel()
		if !errors.Is(err, context.Canceled) || output != nil || owned.checks != at {
			t.Fatalf("cancel at %d after preparation work: checks=%d bytes=%d err=%v", at, owned.checks, len(output), err)
		}
	}
}
