package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPrinterObservationWindowsNativeReadOnly(t *testing.T) {
	if os.Getenv("VPRO_TEST_READONLY_PRINTER_OBSERVATION") != "true" {
		t.Skip("read-only installed/default printer observation requires explicit host-test enablement")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	observation, err := observePrinters(ctx)
	if err != nil {
		t.Fatal("actual read-only Windows printer observation unavailable", err)
	}
	if err := validatePrinterObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}
	t.Logf("read-only Windows observation validated: installed=%d default-present=%t; no setter or spool output invoked",
		len(observation.Names), observation.Default != nil)
}
