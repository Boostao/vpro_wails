package main

import (
	"context"
	"testing"
)

func TestPrinterObservationRequiresLiteralNamesAndExplicitProvenance(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, test := range []struct {
		value PrinterObservation
		valid bool
	}{
		{PrinterObservation{Names: []string{}, Source: printerObservationSource}, true},
		{PrinterObservation{Names: []string{" leading literal ", "\u00e9", "same", "same"}, Default: text("other read"), Source: printerObservationSource}, true},
		{PrinterObservation{Names: nil, Source: printerObservationSource}, false},
		{PrinterObservation{Names: []string{""}, Source: printerObservationSource}, false},
		{PrinterObservation{Names: []string{"bad\x00name"}, Source: printerObservationSource}, false},
		{PrinterObservation{Names: []string{string([]byte{0xff})}, Source: printerObservationSource}, false},
		{PrinterObservation{Names: []string{}, Default: text(""), Source: printerObservationSource}, false},
		{PrinterObservation{Names: []string{}, Default: text("bad\x00name"), Source: printerObservationSource}, false},
		{PrinterObservation{Names: []string{}, Source: "inferred"}, false},
	} {
		if err := validatePrinterObservation(context.Background(), test.value); (err == nil) != test.valid {
			t.Fatal(test, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		if err := validatePrinterObservation(ctx, PrinterObservation{Names: []string{}, Source: printerObservationSource}); err == nil {
			t.Fatal("missing/cancelled context accepted")
		}
	}
}
