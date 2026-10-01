package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeologyProfileGeneratorPinnedDeterministicAndNonoverwriting(t *testing.T) {
	snapshot := filepath.Join("..", "..", "resources", "geology-codes-fixture.json.gz")
	var databases, provenances [][]byte
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		database, provenance := filepath.Join(dir, "geology.db"), filepath.Join(dir, "geology.json")
		args := []string{"-profile", "geology", "-snapshot", snapshot, "-database", database, "-provenance", provenance}
		var out bytes.Buffer
		if err := run(args, &out); err != nil {
			t.Fatal(err)
		}
		var result struct{ Rows, Cells int }
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Rows != 87 || result.Cells != 870 {
			t.Fatal("generator output", err)
		}
		for path, target := range map[string]*[][]byte{database: &databases, provenance: &provenances} {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			*target = append(*target, data)
		}
		if err := run(args, &out); err == nil {
			t.Fatal("existing outputs overwritten")
		}
		data, err := os.ReadFile(database)
		if err != nil || !bytes.Equal(data, databases[i]) {
			t.Fatal("existing database changed")
		}
	}
	if !bytes.Equal(databases[0], databases[1]) || !bytes.Equal(provenances[0], provenances[1]) {
		t.Fatal("generation nondeterministic")
	}
	dir := t.TempDir()
	var out bytes.Buffer
	args := []string{"-profile", "unknown", "-snapshot", snapshot, "-database", filepath.Join(dir, "bad.db"), "-provenance", filepath.Join(dir, "bad.json")}
	if err := run(args, &out); err == nil {
		t.Fatal("unknown profile accepted")
	}
	data, err := os.ReadFile(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(dir, "corrupt.gz")
	if err := os.WriteFile(corrupt, append(data, 0), 0600); err != nil {
		t.Fatal(err)
	}
	args[1], args[3] = "geology", corrupt
	if err := run(args, &out); err == nil {
		t.Fatal("corrupt sealed fixture accepted")
	}
	if _, err := os.Stat(args[5]); !os.IsNotExist(err) {
		t.Fatal("failed capture left database output")
	}
}
