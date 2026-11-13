//go:build !windows

package vprocore

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

func ObservePrinters(ctx context.Context) (PrinterObservation, error) {
	if ctx == nil {
		return PrinterObservation{}, errors.New("printer observation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return PrinterObservation{}, err
	}
	names := []string{}
	var defaultName *string
	if lpstatPath, err := exec.LookPath("lpstat"); err == nil && lpstatPath != "" {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var outE bytes.Buffer
		cmdE := exec.CommandContext(probeCtx, lpstatPath, "-e")
		cmdE.Stdout = &outE
		if cmdE.Run() == nil {
			for _, line := range strings.Split(outE.String(), "\n") {
				candidate := strings.TrimRight(line, "\r\n")
				if candidate != "" && utf8.ValidString(candidate) && !strings.ContainsRune(candidate, 0) {
					names = append(names, candidate)
				}
			}
		}
		if ctx.Err() != nil {
			return PrinterObservation{}, ctx.Err()
		}
		var outD bytes.Buffer
		cmdD := exec.CommandContext(probeCtx, lpstatPath, "-d")
		cmdD.Stdout = &outD
		if cmdD.Run() == nil {
			firstLine := strings.TrimRight(strings.Split(outD.String(), "\n")[0], "\r\n")
			if _, after, found := strings.Cut(firstLine, ": "); found {
				if after != "" && utf8.ValidString(after) && !strings.ContainsRune(after, 0) {
					d := after
					defaultName = &d
				}
			}
		}
		if ctx.Err() != nil {
			return PrinterObservation{}, ctx.Err()
		}
	}
	result := PrinterObservation{Names: names, Default: defaultName, Source: PrinterObservationSource}
	if err := ValidatePrinterObservation(ctx, result); err != nil {
		return PrinterObservation{}, err
	}
	return result, nil
}
