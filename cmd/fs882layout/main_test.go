package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func TestExplicitArgumentsAndSourceGuard(t *testing.T) {
	for _, args := range [][]string{nil, {"-source", ".", "-out", "x"}, {"-source", ".", "-form", "Parent"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted incomplete args: %v", args)
		}
	}
	root := filepath.Join(".", "test-work-"+t.Name())
	if err := os.MkdirAll(filepath.Join(root, "Forms"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	path := filepath.Join(root, "Forms", "Parent.txt")
	original := []byte("Begin Form\nEnd\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{path, filepath.Join(root, "new.json")} {
		if err := run([]string{"-source", root, "-out", out, "-form", "Parent"}, &bytes.Buffer{}); err == nil {
			t.Fatal("accepted canonical-source output")
		}
	}
	current, _ := os.ReadFile(path)
	if !bytes.Equal(current, original) {
		t.Fatal("canonical source changed")
	}
	out := filepath.Join(".", "test-layout-"+t.Name()+".json")
	t.Cleanup(func() { os.Remove(out) })
	if err := run([]string{"-source", root, "-out", out, "-form", "Parent"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var layout fs882layout.Layout
	if err := json.Unmarshal(data, &layout); err != nil || layout.Root != "Parent" || len(layout.Forms) != 1 {
		t.Fatalf("output: %+v, %v", layout, err)
	}
	if err := run([]string{"-source", root, "-out", out, "-form", "Missing"}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted missing definition")
	}
	current, _ = os.ReadFile(out)
	if !bytes.Equal(current, data) {
		t.Fatal("failed extraction damaged old output")
	}
	linked := filepath.Join(".", "test-link-"+t.Name()+".json")
	t.Cleanup(func() { os.Remove(linked) })
	if err := os.Link(path, linked); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-source", root, "-out", linked, "-form", "Parent"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	current, _ = os.ReadFile(path)
	if !bytes.Equal(current, original) {
		t.Fatal("hard-linked output modified canonical source")
	}
}

func TestPublishCollision(t *testing.T) {
	path := filepath.Join(".", "test-layout-"+t.Name()+".json")
	for _, file := range []string{path, path + ".pending"} {
		if err := os.WriteFile(file, []byte("existing"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".pending") })
	if err := publish(path, []byte("replacement")); err == nil {
		t.Fatal("overwrote colliding staging file")
	}
	for _, file := range []string{path, path + ".pending"} {
		data, _ := os.ReadFile(file)
		if string(data) != "existing" {
			t.Fatal("collision modified existing data")
		}
	}
}

func TestOutputSymlinkGuard(t *testing.T) {
	root := filepath.Join(".", "test-work-"+t.Name())
	if err := os.MkdirAll(filepath.Join(root, "source"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	source, _ := filepath.Abs(filepath.Join(root, "source"))
	source, _ = filepath.EvalSymlinks(source)
	link := filepath.Join(root, "alias")
	if err := os.Symlink(source, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := safeOutput(source, filepath.Join(link, "layout.json")); err == nil {
		t.Fatal("accepted source via directory symlink")
	}
}
