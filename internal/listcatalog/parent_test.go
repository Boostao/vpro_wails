package listcatalog

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parentFixture(t *testing.T) ([][]byte, Provenance, []Choice) {
	t.Helper()
	var inputs [][]byte
	for _, name := range []string{"parent-codes-fixture.json", "parent-codes-source-provenance.json",
		"parent-codes-fixture-manifest.json", "geology-codes-fixture.json.gz"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "resources", name))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, data)
	}
	profile := ParentProfile()
	p := Provenance{
		Version:           1,
		Exporter:          profile.Exporter,
		SourceSHA256:      profile.SourceSHA256,
		SnapshotSHA256:    profile.SnapshotSHA256,
		DatabaseSHA256:    strings.Repeat("0", 64),
		TypedCellsSHA256:  strings.Repeat("0", 64),
		Rows:              347,
		Cells:             3470,
		SourceTable:       "USysTableOfLists",
		SQL:               profile.SQL,
		SourceSchema:      ExpectedSchema(),
		ItemOrderEncoding: ItemOrderEncoding,
	}
	rows, err := ParentSnapshotRows(inputs[0], inputs[1], inputs[2], inputs[3], p)
	if err != nil {
		t.Fatal(err)
	}
	p.TypedCellsSHA256, err = TypedHash(rows)
	if err != nil {
		t.Fatal(err)
	}
	return inputs, p, rows
}

func TestParentDAO3470Cells16ListsAndStorage(t *testing.T) {
	_, p, expected := parentFixture(t)
	db := memoryDB(t)
	if err := ImportRows(db, expected, p, ParentProfile()); err != nil {
		t.Fatal(err)
	}
	actual, err := ReadChoicesFor(db, "", ParentProfile())
	if err != nil || len(actual) != 347 || !reflect.DeepEqual(actual, expected) {
		t.Fatal("3470 cells/native order changed", err)
	}
	for _, list := range ParentProfile().Lists {
		rows, err := ReadChoicesFor(db, list.Name, ParentProfile())
		if err != nil || len(rows) != list.Rows {
			t.Fatal(list.Name, err)
		}
	}
	if _, err := ReadChoicesFor(db, "HydroGeoSystem", ParentProfile()); err == nil {
		t.Fatal("invented list casing alias accepted")
	}
}

func TestParentSnapshotIndependentSealsAndSourceGuards(t *testing.T) {
	inputs, p, _ := parentFixture(t)
	for i := range inputs {
		bad := append([][]byte{}, inputs...)
		bad[i] = append(append([]byte{}, bad[i]...), ' ')
		if _, err := ParentSnapshotRows(bad[0], bad[1], bad[2], bad[3], p); err == nil {
			t.Fatal("independent source checksum ignored", i)
		}
	}
	for _, change := range []struct {
		input         int
		before, after string
	}{
		{0, `"Readonly":true`, `"Readonly":false`},
		{0, `"NativeGUIStarted":false`, `"NativeGUIStarted":true`},
		{0, `"DAOHandlesClosed":true`, `"DAOHandlesClosed":false`},
		{0, `HumusForm`, `humusform`},
		{0, `"Size":8`, `"Size":4`},
		{1, `"ByteIdentityUnchanged":true`, `"ByteIdentityUnchanged":false`},
		{1, `"BeforeSHA256":"` + SourceSHA256, `"BeforeSHA256":"` + strings.Repeat("0", 64)},
		{2, `"capturedRows":260`, `"capturedRows":259`},
		{2, GeologySnapshotSHA256, strings.Repeat("0", 64)},
	} {
		t.Run(change.before, func(t *testing.T) {
			bad := append([][]byte{}, inputs...)
			bad[change.input] = bytes.Replace(bad[change.input], []byte(change.before), []byte(change.after), 1)
			if bytes.Equal(bad[change.input], inputs[change.input]) {
				t.Fatal("test mutation absent")
			}
			if _, err := decodeParentSnapshot(bad[0], bad[1], bad[2], bad[3], p); err == nil {
				t.Fatal("source query/schema/provenance mutation accepted")
			}
		})
	}
	// Re-seal a cell mutation locally to exercise conversion rather than the outer byte guard.
	var snapshot struct{ Catalogues []daoSnapshotCatalogue }
	if err := json.Unmarshal(inputs[0], &snapshot); err != nil {
		t.Fatal(err)
	}
	catalogue := snapshot.Catalogues[0]
	var cells []daoNativeCell
	if err := json.Unmarshal([]byte(catalogue.Rows[0]), &cells); err != nil {
		t.Fatal(err)
	}
	cells[3].DAOType = 7
	encoded, err := json.Marshal(cells)
	if err != nil {
		t.Fatal(err)
	}
	catalogue.Rows[0] = string(encoded)
	catalogue.SHA256 = checksum([]byte(strings.Join(catalogue.Rows, "\n")))
	if _, _, err := decodeDAORows(catalogue, ParentProfile().Lists[1]); err == nil {
		t.Fatal("malformed native cell accepted")
	}
}

func TestParentStorageNullableDuplicateFractionalSignedZeroAndRollback(t *testing.T) {
	_, p, rows := parentFixture(t)
	testDAOStorage(t, ParentProfile(), p, rows)
}

func TestBundledParentCatalogue2026(t *testing.T) {
	database := filepath.Join("..", "..", "resources", "parent-codes.db")
	provenance := filepath.Join("..", "..", "resources", "parent-codes-provenance.json")
	data, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(provenance)
	if err != nil {
		t.Fatal(err)
	}
	profile := ParentBundledProfile2026()
	p, err := DecodeProvenanceFor(encoded, profile)
	if err != nil {
		t.Fatal(err)
	}
	if checksum(data) != p.DatabaseSHA256 {
		t.Fatal("bundled parent database checksum mismatch")
	}
	db, err := sql.Open("sqlite3", "file:"+database+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ValidateDatabaseFor(db, p, profile); err != nil {
		t.Fatal(err)
	}
	durRows, err := ReadChoicesFor(db, "FloodingRegimeDur", profile)
	if err != nil || len(durRows) != 9 {
		t.Fatalf("FloodingRegimeDur: %d %v", len(durRows), err)
	}
	foundS := false
	for _, r := range durRows {
		if r.Code != nil && *r.Code == "S" && r.Description != nil && strings.Contains(*r.Description, "subirrigated") {
			foundS = true
		}
	}
	if !foundS {
		t.Fatal("bundled FloodingRegimeDur missing LMH25 2023 S description")
	}
}
