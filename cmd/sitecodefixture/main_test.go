package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func fixturePaths() (string, string) {
	base := filepath.Join("..", "..", "resources")
	return filepath.Join(base, "site-codes-fixture.json"), filepath.Join(base, "site-codes-fixture-manifest.json")
}

func workspace(t *testing.T) string {
	t.Helper()
	dir := ".sitecodefixture-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return dir
}

func TestGenerateAndNonOverwriting(t *testing.T) {
	dir := workspace(t)
	snapshot, manifest := fixturePaths()
	database, provenance := filepath.Join(dir, "site-codes.db"), filepath.Join(dir, "provenance.json")
	args := []string{"-snapshot", snapshot, "-manifest", manifest, "-database", database, "-provenance", provenance}
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	rawDB, _ := os.ReadFile(database)
	rawProvenance, _ := os.ReadFile(provenance)
	p, err := listcatalog.DecodeProvenance(rawProvenance)
	if err != nil || p.DatabaseSHA256 != hash(rawDB) {
		t.Fatalf("generated checksum/provenance: %v", err)
	}
	db, err := sql.Open("sqlite3", database)
	if err != nil {
		t.Fatal(err)
	}
	if err := listcatalog.ValidateDatabase(db, p); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := run(args, io.Discard); err == nil {
		t.Fatal("overwrote existing outputs")
	}
	got, _ := os.ReadFile(database)
	if hash(got) != hash(rawDB) {
		t.Fatal("existing database changed")
	}
	got, _ = os.ReadFile(provenance)
	if string(got) != string(rawProvenance) {
		t.Fatal("existing provenance changed")
	}
	otherProvenance := filepath.Join(dir, "other.json")
	args[len(args)-1] = otherProvenance
	if err := run(args, io.Discard); err == nil {
		t.Fatal("existing database collision accepted")
	}
	if _, err := os.Stat(otherProvenance); !os.IsNotExist(err) {
		t.Fatal("collision leaked reserved output")
	}
}

func TestManifestAndFlagFailures(t *testing.T) {
	snapshot, manifestPath := fixturePaths()
	data, _ := os.ReadFile(snapshot)
	rawManifest, _ := os.ReadFile(manifestPath)
	if _, err := fixtureProvenance(data, rawManifest); err != nil {
		t.Fatal(err)
	}
	var m manifest
	json.Unmarshal(rawManifest, &m)
	for _, change := range []func(*manifest){
		func(m *manifest) { m.SHA256 = strings.Repeat("0", 64) },
		func(m *manifest) { m.CanonicalSourceSHA256 = strings.Repeat("0", 64) },
		func(m *manifest) { m.NativeRows-- },
		func(m *manifest) { m.NativeCells-- },
		func(m *manifest) { m.TypedRowsSHA256 = strings.Repeat("0", 64) },
		func(m *manifest) { m.Path = "" },
	} {
		bad := m
		change(&bad)
		encoded, _ := json.Marshal(bad)
		if _, err := fixtureProvenance(data, encoded); err == nil {
			t.Fatal("bad manifest accepted")
		}
	}
	for _, malformed := range [][]byte{[]byte(`{`), []byte(`{"unknown":1}`), append(rawManifest, []byte(` {}`)...)} {
		if _, err := fixtureProvenance(data, malformed); err == nil {
			t.Fatal("malformed manifest accepted")
		}
	}
	if _, err := fixtureProvenance(append(append([]byte{}, data...), ' '), rawManifest); err == nil {
		t.Fatal("changed snapshot accepted")
	}
	if err := run(nil, io.Discard); err == nil {
		t.Fatal("missing flags accepted")
	}
}

func TestGenerationFailureRemovesOnlyOwnedOutputs(t *testing.T) {
	dir := workspace(t)
	snapshot, manifest := fixturePaths()
	data, _ := os.ReadFile(snapshot)
	rawManifest, _ := os.ReadFile(manifest)
	p, err := fixtureProvenance(data, rawManifest)
	if err != nil {
		t.Fatal(err)
	}
	p.TypedCellsSHA256 = strings.Repeat("0", 64)
	database, provenance := filepath.Join(dir, "bad.db"), filepath.Join(dir, "bad.json")
	if err := generate(database, provenance, data, p); err == nil {
		t.Fatal("bad typed hash accepted")
	}
	for _, path := range []string{database, provenance} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("failed generation left output %s", path)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("failed generation left SQLite sidecars")
	}
	if err := generate(provenance, provenance, data, p); err == nil {
		t.Fatal("identical output paths accepted")
	}
}
