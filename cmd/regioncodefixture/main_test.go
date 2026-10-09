package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func fixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "region-codes-fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join("..", "..", "resources", "region-codes-fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data, manifest
}

func TestRegionFixtureChecksumManifestAndNonoverwriting(t *testing.T) {
	data, manifest := fixture(t)
	rows, p, err := fixtureRows(data, manifest)
	if err != nil || len(rows) != 164 {
		t.Fatalf("fixture %v", err)
	}
	if _, _, err := fixtureRows(append(append([]byte{}, data...), '\n'), manifest); err == nil {
		t.Fatal("corrupt snapshot accepted")
	}
	for _, replacement := range []struct{ before, after string }{
		{`"nativeRows": 164`, `"nativeRows": 163`},
		{`"sourceReadonly": true`, `"sourceReadonly": false`},
		{`"Region": 27`, `"Region": 26`},
		{`"actualListNameCasePreserved": true`, `"actualListNameCasePreserved": false`},
	} {
		modified := bytes.Replace(manifest, []byte(replacement.before), []byte(replacement.after), 1)
		if bytes.Equal(modified, manifest) {
			t.Fatal("manifest test replacement absent")
		}
		if _, _, err := fixtureRows(data, modified); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
	dir := t.TempDir()
	database, provenance := filepath.Join(dir, "catalog.db"), filepath.Join(dir, "provenance.json")
	if err := listcatalog.GenerateDatabase(database, provenance, rows, p, listcatalog.RegionProfile()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	if err := listcatalog.GenerateDatabase(database, filepath.Join(dir, "new.json"), rows, p, listcatalog.RegionProfile()); err == nil {
		t.Fatal("database overwritten")
	}
	after, err := os.ReadFile(database)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("owned database bytes changed")
	}
	if err := listcatalog.GenerateDatabase(filepath.Join(dir, "new.db"), provenance, rows, p, listcatalog.RegionProfile()); err == nil {
		t.Fatal("provenance overwritten")
	}
	var output bytes.Buffer
	if err := run(nil, &output); err == nil {
		t.Fatal("missing flags accepted")
	}
}

func TestRegionImportDuplicateMetadataSignedZeroFiniteAndRollback(t *testing.T) {
	data, manifest := fixture(t)
	rows, p, err := fixtureRows(data, manifest)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Synthetic metadata exercises storage, never substitutes for native evidence.
	code, note, description := "same", "", "different"
	rows[1].Code, rows[2].Code = &code, &code
	rows[1].Note, rows[2].Description = &note, &description
	negativeZero := math.Copysign(0, -1)
	rows[1].ItemOrder = &negativeZero
	p.TypedCellsSHA256, err = listcatalog.TypedHash(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := listcatalog.ImportRows(db, rows, p, listcatalog.RegionProfile()); err != nil {
		t.Fatal(err)
	}
	actual, err := listcatalog.ReadChoicesFor(db, "Ecosection", listcatalog.RegionProfile())
	if err != nil || len(actual) != 137 || actual[1].RowID == actual[2].RowID ||
		*actual[1].Code != *actual[2].Code || actual[1].Note == nil || *actual[1].Note != "" || actual[2].Note != nil ||
		actual[1].ItemOrder == nil || math.Float64bits(*actual[1].ItemOrder) != math.Float64bits(negativeZero) {
		t.Fatal("typed metadata/duplicates/IEEE64 changed")
	}
	if _, err := listcatalog.ReadChoicesFor(db, "ecosection", listcatalog.RegionProfile()); err == nil {
		t.Fatal("case alias invented")
	}
	if _, err := db.Exec(`UPDATE SiteCodeProvenance SET Exporter='wrong'`); err != nil {
		t.Fatal(err)
	}
	if err := listcatalog.ValidateDatabaseFor(db, p, listcatalog.RegionProfile()); err == nil {
		t.Fatal("provenance corruption accepted")
	}
	for _, corruption := range []string{"ordinal", "nonfinite", "wrong-list"} {
		bad := append([]listcatalog.Choice{}, rows...)
		switch corruption {
		case "ordinal":
			bad[2].RowID = bad[1].RowID
		case "nonfinite":
			value := math.Inf(1)
			bad[2].ItemOrder = &value
		default:
			bad[2].ListName = &code
		}
		hash, hashErr := listcatalog.TypedHash(bad)
		if hashErr != nil {
			if corruption != "nonfinite" {
				t.Fatal(hashErr)
			}
			continue
		}
		badP := p
		badP.TypedCellsSHA256 = hash
		other, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		other.SetMaxOpenConns(1)
		if err := listcatalog.ImportRows(other, bad, badP, listcatalog.RegionProfile()); err == nil {
			t.Fatal("invalid import committed")
		}
		var count int
		if err := other.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('SiteCodeChoices','SiteCodeProvenance')`).Scan(&count); err != nil || count != 0 {
			t.Fatal("failed import left tables")
		}
		if err := other.Close(); err != nil {
			t.Fatal(err)
		}
	}
	var generic map[string]any
	if err := json.Unmarshal(manifest, &generic); err != nil {
		t.Fatal(err)
	}
	generic["unexpected"] = strings.Repeat("x", 2)
	modified, err := json.Marshal(generic)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixtureRows(data, modified); err == nil {
		t.Fatal("unknown manifest member accepted")
	}
}
