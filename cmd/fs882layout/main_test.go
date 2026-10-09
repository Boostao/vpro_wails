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
	for _, args := range [][]string{nil, {"-source", ".", "-out", "x"}, {"-source", ".", "-form", "Parent"},
		{"-source", ".", "-out", "x", "-form", "Parent", "-report", "Parent"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted incomplete args: %v", args)
		}
	}
}

func TestReportRootUsesExistingSourceAndOutputGuards(t *testing.T) {
	root := filepath.Join(".", "test-work-"+t.Name())
	if err := os.MkdirAll(filepath.Join(root, "Reports"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	source := filepath.Join(root, "Reports", "Labels.txt")
	original := []byte("Begin Report\nRecordSource =\"BecLabels\"\nBegin\nBegin Section\nName =\"Detail\"\nEnd\nEnd\nEnd\n")
	if err := os.WriteFile(source, original, 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(".", "test-layout-"+t.Name()+".json")
	t.Cleanup(func() { os.Remove(out) })
	if err := run([]string{"-source", root, "-out", out, "-report", "Labels"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var layout fs882layout.Layout
	if err := json.Unmarshal(data, &layout); err != nil || layout.Root != "Labels" || len(layout.Forms) != 1 ||
		layout.Forms[0].Controls[0].Type != "Report" {
		t.Fatalf("report output: %+v, %v", layout, err)
	}
	for _, args := range [][]string{
		{"-source", root, "-out", source, "-report", "Labels"},
		{"-source", root, "-out", out, "-form", "Labels"},
		{"-source", root, "-out", out, "-report", "Missing"},
	} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted report misuse: %v", args)
		}
	}
	current, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("report source changed: %v", err)
	}
	current, err = os.ReadFile(out)
	if err != nil || !bytes.Equal(current, data) {
		t.Fatalf("failed report extraction replaced accepted output: %v", err)
	}
}

func TestFormSourceGuard(t *testing.T) {
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
