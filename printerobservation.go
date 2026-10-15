package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type PrinterObservation struct {
	Names   []string `json:"names"`
	Default *string  `json:"default"`
	Source  string   `json:"source"`
}

const printerObservationSource = "installed printers and current default observed read-only; separate host reads, not an atomic snapshot or permission to change the default"

func validatePrinterObservation(ctx context.Context, value PrinterObservation) error {
	if ctx == nil {
		return errors.New("printer observation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if value.Names == nil || value.Source != printerObservationSource {
		return errors.New("printer observation requires explicit installed names and read-only provenance")
	}
	for _, name := range value.Names {
		if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
			return errors.New("installed printer observation contains an invalid literal name")
		}
	}
	if value.Default != nil && (*value.Default == "" || !utf8.ValidString(*value.Default) || strings.ContainsRune(*value.Default, 0)) {
		return fmt.Errorf("default printer observation contains an invalid literal name")
	}
	return nil
}
