package main

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

func workingUnitProvenanceFixture(t *testing.T) workingUnitProvenance {
	t.Helper()
	var p workingUnitProvenance
	if err := decodeWorkingUnitJSON(workingUnitProvenanceJSON, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWorkingUnitFreezeNativeSnapshot(t *testing.T) {
	if os.Getenv("VPRO_FREEZE_WORKING_UNIT") != "1" {
		t.Skip("one-time explicit typed native snapshot resource generation")
	}
	if bytes.HasPrefix(workingUnitDatabase, []byte("SQLite format 3")) {
		t.Fatal("Working Unit resource is already frozen")
	}
	data, err := os.ReadFile(filepath.Join("evidence", "private", "working-unit-native", "working-unit-reference-snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := workingUnitProvenanceFixture(t)
	snapshot, err := validateWorkingUnitSnapshot(data, p)
	if err != nil {
		t.Fatal(err)
	}
	p.SourceCopies, p.SourceSchema = snapshot.SourceCopies, snapshot.Schema
	p.IdentityEncoding, p.SourceContext = snapshot.IdentityEncoding, snapshot.SourceContext
	cells, err := json.Marshal(snapshot.Tables)
	if err != nil {
		t.Fatal(err)
	}
	p.TypedCellsSHA256 = becHash(cells)
	file, err := os.CreateTemp("resources", ".working-unit-import-*.db")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Error(err)
		}
	})
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	err = importWorkingUnitSnapshot(db, data, p)
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("freeze/import every typed source cell: %v / %v", err, closeErr)
	}
	database, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p.DatabaseSHA256 = becHash(database)
	if err := os.Rename(path, filepath.Join("resources", "working-unit.db")); err != nil {
		t.Fatal(err)
	}
	provenance, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("resources", "working-unit-provenance.json"), append(provenance, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("3508 rows / 17540 typed cells frozen; database SHA256=%s cells SHA256=%s", p.DatabaseSHA256, p.TypedCellsSHA256)
}

func workingUnitSnapshotFixture(t *testing.T) ([]byte, workingUnitProvenance, workingUnitSnapshot) {
	t.Helper()
	p := workingUnitProvenanceFixture(t)
	root := becTestDir(t)
	path := filepath.Join(root, "catalogue.db")
	if err := os.WriteFile(path, workingUnitDatabase, 0644); err != nil {
		t.Fatal(err)
	}
	db, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := validateWorkingUnitDatabase(db, p); err != nil {
		t.Fatal(err)
	}
	tables, err := workingUnitSourceTables(db)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := workingUnitSnapshot{Version: p.Version, Exporter: p.Exporter, ReadOnlyNativeReferenceAccess: true,
		SourceCopies: p.SourceCopies, Schema: p.SourceSchema, Tables: tables,
		IdentityEncoding: p.IdentityEncoding, SourceContext: p.SourceContext}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	p.SnapshotSHA256 = becHash(data)
	return data, p, snapshot
}

func TestWorkingUnitImportAllTypedCellsAndSourceGuards(t *testing.T) {
	data, p, snapshot := workingUnitSnapshotFixture(t)
	if len(snapshot.Tables["MasterSiteUnitList"]) != 3504 || len(snapshot.Tables["UserSiteUnitList"]) != 4 ||
		workingUnitProvenanceFixture(t).SnapshotSHA256 != "f5f7ad951e46e0c4c031067199781a72037917715a8bbd9bd0f199c6c3eba0f3" {
		t.Fatal("frozen native row boundary changed")
	}
	db, err := sql.Open("sqlite3", filepath.Join(becTestDir(t), "import.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := importWorkingUnitSnapshot(db, data, p); err != nil {
		t.Fatal(err)
	}
	actual, err := workingUnitSourceTables(db)
	if err != nil || !reflect.DeepEqual(actual, snapshot.Tables) {
		t.Fatalf("17540 source cells changed: %v", err)
	}
	for _, mutation := range []string{"hash", "schema", "default", "identity", "ordinal", "source", "level", "count", "null-id"} {
		t.Run(mutation, func(t *testing.T) {
			var changed workingUnitSnapshot
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "schema":
				changed.Schema["MasterSiteUnitList"][2].Size = 100
			case "default":
				changed.Schema["UserSiteUnitList"][5].Default = "99"
			case "identity":
				changed.Tables["MasterSiteUnitList"][0].ID = becString("2147483648")
			case "ordinal":
				changed.Tables["MasterSiteUnitList"][0].SourceOrdinal = "01"
			case "source":
				changed.SourceCopies[0].Hash = strings.Repeat("0", 64)
			case "level":
				value := 32768
				changed.Tables["MasterSiteUnitList"][0].Level = &value
			case "count":
				changed.Tables["MasterSiteUnitList"] = changed.Tables["MasterSiteUnitList"][1:]
			case "null-id":
				changed.Tables["MasterSiteUnitList"][0].ID = nil
			}
			invalid, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			provenance := p
			if mutation == "hash" {
				provenance.SnapshotSHA256 = strings.Repeat("0", 64)
			} else {
				provenance.SnapshotSHA256 = becHash(invalid)
			}
			if _, err := validateWorkingUnitSnapshot(invalid, provenance); err == nil {
				t.Fatal("invalid native snapshot accepted")
			}
		})
	}
	if _, err := db.Exec(`UPDATE WorkingUnits SET Description='corrupt cell' WHERE Origin='master' AND RowID='1'`); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkingUnitDatabase(db, p); err == nil {
		t.Fatal("typed cell corruption passed integrity validation")
	}
	failed, err := sql.Open("sqlite3", filepath.Join(becTestDir(t), "rejected-import.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer failed.Close()
	wrongCells := p
	wrongCells.TypedCellsSHA256 = strings.Repeat("0", 64)
	if err := importWorkingUnitSnapshot(failed, data, wrongCells); err == nil {
		t.Fatal("import cell integrity failure accepted")
	}
	var created int
	if err := failed.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('WorkingUnits','WorkingUnitProvenance')`).Scan(&created); err != nil || created != 0 {
		t.Fatalf("failed typed import left partially committed data/schema: %d %v", created, err)
	}
}
