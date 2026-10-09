package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boostao/vpro-wails/internal/accessinventory"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "accessinventory:", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("accessinventory", flag.ContinueOnError)
	flags.SetOutput(output)
	source := flags.String("source", "", "Access SaveAsText root containing Forms, Queries, Modules and Tables_Def")
	destination := flags.String("out", "", "Output JSON path outside the canonical export directory")
	rootForm := flags.String("form", "FS882-6x4XL", "Root form for the reachable-subform summary")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *source == "" || *destination == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: accessinventory -source EXPORT_ROOT -out INVENTORY.json [-form FS882-6x4XL]")
	}
	sourcePath, err := filepath.Abs(*source)
	if err != nil {
		return err
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	outPath, err := filepath.Abs(*destination)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(outPath))
	if err != nil {
		return fmt.Errorf("resolve output directory (must already exist): %w", err)
	}
	outPath = filepath.Join(parent, filepath.Base(outPath))
	if target, err := filepath.EvalSymlinks(outPath); err == nil {
		outPath = target
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("resolve output file: %w", err)
	}
	relative, err := filepath.Rel(sourcePath, outPath)
	if err != nil {
		// Different Windows volumes cannot be descendants.
		if strings.EqualFold(filepath.VolumeName(sourcePath), filepath.VolumeName(outPath)) {
			return err
		}
	} else if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to write inside canonical Access exports")
	}
	inventory, err := accessinventory.Build(sourcePath)
	if err != nil {
		return err
	}
	forms, err := inventory.ReachableForms(*rootForm)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(parent, ".accessinventory-*.json")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, outPath); err != nil {
		return fmt.Errorf("publish inventory: %w", err)
	}
	controlCount, boundCount, eventCount, embeddedCount := 0, 0, 0, 0
	for _, form := range forms {
		bound := 0
		for _, control := range form.Controls {
			if control.Properties["ControlSource"].Value != "" {
				bound++
			}
			eventCount += len(control.Events)
			if control.Properties["SourceObject"].Value != "" {
				embeddedCount++
			}
		}
		controlCount += len(form.Controls)
		boundCount += bound
		if _, err := fmt.Fprintf(output, "%s: %d structural/control nodes, %d bindings\n", form.Name, len(form.Controls), bound); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(output, "\nApplication: %d objects, %d form/report definitions, %d dependencies, %d diagnostics\n%s closure: %d definitions, %d nodes, %d bindings, %d events, %d embedded instances\nInventory: %s\nImplementation coverage: UNMAPPED (extraction is not desktop parity)\n",
		len(inventory.Objects), len(inventory.Forms), len(inventory.Edges), len(inventory.Diagnostics), *rootForm, len(forms), controlCount, boundCount, eventCount, embeddedCount, outPath)
	return err
}
