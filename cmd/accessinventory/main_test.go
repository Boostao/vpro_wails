package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "Forms"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "Forms", "Test.txt"), []byte("Begin Form\nBegin\nBegin Section\nName =\"Detail\"\nEnd\nEnd\nEnd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return source
}

func TestCommandWritesInventory(t *testing.T) {
	source := fixture(t)
	target := filepath.Join(t.TempDir(), "inventory.json")
	var output bytes.Buffer
	args := []string{"-source", source, "-out", target, "-form", "Test"}
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), `"implementation": "unmapped"`) || !strings.Contains(output.String(), "extraction is not desktop parity") {
		t.Fatalf("coverage incorrectly implied: %s", output.String())
	}
	output.Reset()
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("inventory is not reproducible")
	}
}

func TestCommandRefusesCanonicalOutput(t *testing.T) {
	source := fixture(t)
	err := run([]string{"-source", source, "-out", filepath.Join(source, "inventory.json"), "-form", "Test"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "canonical Access exports") {
		t.Fatalf("unsafe output accepted: %v", err)
	}
}

func TestCommandFailureDoesNotPublishOutput(t *testing.T) {
	source := fixture(t)
	target := filepath.Join(t.TempDir(), "inventory.json")
	err := run([]string{"-source", source, "-out", target, "-form", "Missing"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("missing root form accepted")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("failure published output: %v", err)
	}
}
