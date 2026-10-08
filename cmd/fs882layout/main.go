package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fs882layout:", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("fs882layout", flag.ContinueOnError)
	flags.SetOutput(output)
	source := flags.String("source", "", "Read-only Access SaveAsText export root")
	out := flags.String("out", "", "JSON output outside the source subtree (existing directory)")
	form := flags.String("form", "", "Root form definition, e.g. FS882-6x4XL")
	report := flags.String("report", "", "Root report definition; mutually exclusive with -form")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *source == "" || *out == "" || (*form == "") == (*report == "") || flags.NArg() != 0 {
		return fmt.Errorf("usage: fs882layout -source EXPORT_ROOT -out LAYOUT.json (-form FORM | -report REPORT)")
	}
	sourcePath, err := filepath.Abs(*source)
	if err != nil {
		return err
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	outPath, err := safeOutput(sourcePath, *out)
	if err != nil {
		return err
	}
	var layout *fs882layout.Layout
	if *report != "" {
		layout, err = fs882layout.BuildReport(sourcePath, *report)
	} else {
		layout, err = fs882layout.Build(sourcePath, *form)
	}
	if err != nil {
		return err
	}
	data, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	if err := publish(outPath, append(data, '\n')); err != nil {
		return err
	}
	nodes, bindings, pages, embedded := 0, 0, 0, 0
	for _, definition := range layout.Forms {
		bound := 0
		for _, field := range definition.Fields {
			if field.Binding != "" {
				bound++
			}
		}
		nodes += len(definition.Fields)
		bindings += bound
		pages += len(definition.Pages)
		embedded += len(definition.Embedded)
		if _, err := fmt.Fprintf(output, "%s: %d nodes, %d binding instances, %d columns, %d pages\n",
			definition.Name, len(definition.Fields), bound, len(definition.BindingColumns()), len(definition.Pages)); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(output, "%s: %d definitions, %d nodes, %d bindings, %d pages, %d embedded instances\nLayout: %s (%d bytes)\nRead-only metadata; implementation UNMAPPED, not Access parity.\n",
		layout.Root, len(layout.Forms), nodes, bindings, pages, embedded, outPath, len(data)+1)
	return err
}

// Publish by replacement, so an existing hard link cannot truncate source data.
// The exclusive sibling staging file also leaves the old resource intact if a
// write fails. It is removed on all error paths; no system temp directory is used.
func publish(path string, data []byte) error {
	staging := path + ".pending"
	file, err := os.OpenFile(staging, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer os.Remove(staging)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(staging, path)
}

func safeOutput(source, destination string) (string, error) {
	path, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	path = filepath.Join(parent, filepath.Base(path))
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve output: %w", err)
	}
	relative, err := filepath.Rel(source, path)
	if err != nil {
		if strings.EqualFold(filepath.VolumeName(source), filepath.VolumeName(path)) {
			return "", err
		}
	} else if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to write inside canonical Access exports")
	}
	return path, nil
}
