package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func TestStandards2026ProvenanceAndBundledArtifacts(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	provBytes, err := os.ReadFile(filepath.Join(repoRoot, "resources", "standards-2026-provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc Standards2026Provenance
	dec := json.NewDecoder(bytes.NewReader(provBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 1 || doc.Generator != "cmd/standards2026" || len(doc.Sources) != 2 || len(doc.Artifacts) != 7 {
		t.Fatalf("unexpected standards-2026-provenance shape: %+v", doc)
	}
	if doc.Sources[0].ExtractedSHA != listcatalog.BECdbV132026AccdbSHA256 ||
		doc.Sources[1].ExtractedSHA != listcatalog.BCPlantTaxa2025AccdbSHA256 ||
		doc.LMH25PDFSHA != listcatalog.LMH252023PDFSHA256 {
		t.Fatalf("unexpected source SHA256s: %+v", doc.Sources)
	}
	for _, art := range doc.Artifacts {
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(art.Path))
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read artifact %s: %v", art.Path, err)
		}
		if got := sha256Hex(raw); got != art.SHA256 {
			t.Fatalf("artifact %s SHA256 mismatch: got %s, want %s", art.Path, got, art.SHA256)
		}
	}
}

func TestStandards2026CLIArgGuards(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"unexpected-positional"}, &out); err == nil {
		t.Fatal("accepted unexpected positional argument")
	}
	if err := run(nil, &out); err == nil {
		t.Fatal("accepted missing required -bec-zip and -taxa-zip flags")
	}
	if err := run([]string{"-bec-zip", "nonexistent.zip"}, &out); err == nil {
		t.Fatal("accepted missing -taxa-zip flag")
	}
	if err := run([]string{"-bec-zip", "nonexistent.zip", "-taxa-zip", "nonexistent-taxa.zip"}, &out); err == nil {
		t.Fatal("accepted missing BEC zip")
	}
}
