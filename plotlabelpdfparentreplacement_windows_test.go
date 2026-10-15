package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnedPlotLabelPDFParentReplacementCannotPublish(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	parent, moved := filepath.Join(root, "destination"), filepath.Join(root, "owned-original")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "label.pdf")
	hooks := plotLabelPDFPublicationHooks{runtime: runtime}
	hooks.publication.observe = func(phase, stage string) error {
		if phase != "staged" {
			return nil
		}
		if err := os.Rename(parent, moved); err != nil {
			return err
		}
		return os.Mkdir(parent, 0700)
	}
	result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, hooks)
	if err == nil || result.Published || !strings.Contains(err.Error(), "parent") {
		t.Fatalf("replacement destination acquired authority: %+v, %v", result, err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement parent contains publication: %v", err)
	}
	stages, err := os.ReadDir(moved)
	if err != nil || len(stages) != 1 || !strings.HasPrefix(stages[0].Name(), ".vpro-label-pdf-") {
		t.Fatalf("owned stage was not retained under changed parent identity: %v, %v", stages, err)
	}
}
