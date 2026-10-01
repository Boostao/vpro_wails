package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func TestSoilFixtureDeterministicNonoverwritingAndPinned(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "soil-codes-fixture.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	rows, p, err := fixtureRows(data)
	if err != nil || len(rows) != 101 {
		t.Fatal("actual current DAO fixture", err)
	}
	if _, _, err := fixtureRows(append(append([]byte{}, data...), 0)); err == nil {
		t.Fatal("corrupt compressed fixture accepted")
	}
	var outputs [][]byte
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		database, provenance := filepath.Join(dir, "soil.db"), filepath.Join(dir, "soil.json")
		if err := listcatalog.GenerateDatabase(database, provenance, rows, p, listcatalog.SoilProfile()); err != nil {
			t.Fatal(err)
		}
		encoded, err := os.ReadFile(database)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, encoded)
		if err := listcatalog.GenerateDatabase(database, filepath.Join(dir, "new.json"), rows, p, listcatalog.SoilProfile()); err == nil {
			t.Fatal("overwrote existing DB")
		}
		after, err := os.ReadFile(database)
		if err != nil || !bytes.Equal(encoded, after) {
			t.Fatal("existing DB changed")
		}
		if err := listcatalog.GenerateDatabase(filepath.Join(dir, "new.db"), provenance, rows, p, listcatalog.SoilProfile()); err == nil {
			t.Fatal("overwrote existing provenance")
		}
	}
	if !bytes.Equal(outputs[0], outputs[1]) {
		t.Fatal("nondeterministic database")
	}
	var out bytes.Buffer
	if err := run(nil, &out); err == nil {
		t.Fatal("required generator flags skipped")
	}
}
