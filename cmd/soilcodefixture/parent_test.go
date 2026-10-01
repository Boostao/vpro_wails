package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParentProfileGeneratorDeterministicPinnedNonoverwriting(t *testing.T) {
	resource := filepath.Join("..", "..", "resources")
	var outputs [][]byte
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		database, provenance := filepath.Join(dir, "parent.db"), filepath.Join(dir, "parent.json")
		args := []string{"-profile", "parent", "-snapshot", filepath.Join(resource, "parent-codes-fixture.json"),
			"-source-provenance", filepath.Join(resource, "parent-codes-source-provenance.json"),
			"-manifest", filepath.Join(resource, "parent-codes-fixture-manifest.json"),
			"-bedrock", filepath.Join(resource, "geology-codes-fixture.json.gz"),
			"-database", database, "-provenance", provenance}
		var out bytes.Buffer
		if err := run(args, &out); err != nil || !bytes.Contains(out.Bytes(), []byte(`"cells":3470`)) {
			t.Fatal("parent profile generation failed", err)
		}
		data, err := os.ReadFile(database)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, data)
		if err := run(args, &out); err == nil {
			t.Fatal("existing outputs overwritten")
		}
		after, err := os.ReadFile(database)
		if err != nil || !bytes.Equal(after, data) {
			t.Fatal("existing database changed")
		}
		for _, index := range []int{5, 7, 9} {
			bad := append([]string{}, args...)
			corrupt := filepath.Join(dir, "bad"+string(rune('a'+index)))
			if err := os.WriteFile(corrupt, []byte("not sealed"), 0600); err != nil {
				t.Fatal(err)
			}
			bad[index] = corrupt
			bad[11], bad[13] = filepath.Join(dir, "bad.db"), filepath.Join(dir, "bad.json")
			if err := run(bad, &out); err == nil {
				t.Fatal("boundary checksum ignored", index)
			}
			if _, err := os.Stat(bad[11]); !os.IsNotExist(err) {
				t.Fatal("failed import left database")
			}
		}
	}
	if !bytes.Equal(outputs[0], outputs[1]) {
		t.Fatal("parent database nondeterministic")
	}
	var out bytes.Buffer
	if err := run([]string{"-profile", "parent", "-snapshot", "missing", "-database", "not-created.db", "-provenance", "not-created.json"}, &out); err == nil {
		t.Fatal("parent independent inputs not required")
	}
}
