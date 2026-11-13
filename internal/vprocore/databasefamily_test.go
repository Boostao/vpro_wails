package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseFamily2026VListsStandardsAndMetadata(t *testing.T) {
	root := t.TempDir()
	paths, err := InstallDatabaseFamily(root)
	if err != nil {
		t.Fatalf("installDatabaseFamily: %v", err)
	}
	if len(paths) != len(DatabaseFamilySeeds) {
		t.Fatalf("expected %d family databases, got %d", len(DatabaseFamilySeeds), len(paths))
	}

	vlistsPath := paths["VLists"]
	raw, err := os.ReadFile(vlistsPath)
	if err != nil {
		t.Fatal(err)
	}
	if sqlutil.BecHash(raw) != "81e259244106b14d6dd3c65f0a7e7e87bdb06f60e6c54947c0506e2039379441" {
		t.Fatalf("unexpected VLists.db SHA256: %s", sqlutil.BecHash(raw))
	}

	db, err := sqlutil.OpenReadOnly(vlistsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Verify 2026 row counts
	expectedCounts := map[string]int{
		"USysAllSpecs":        16728,
		"USysZoneList":        331,
		"USysSiteSeriesNames": 8900,
		"MasterSiteUnitList":  6067,
		"USysTableOfLists":    1566,
	}
	for tbl, want := range expectedCounts {
		var got int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "` + tbl + `"`).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d", tbl, got, want)
		}
	}

	// Verify USysAllSpecs.Lifeform remains INTEGER (INT32) and never TEXT/REAL
	var badLifeforms int
	if err := db.QueryRow(`SELECT COUNT(*) FROM USysAllSpecs WHERE Lifeform IS NOT NULL AND (typeof(Lifeform) <> 'integer' OR Lifeform < -2147483648 OR Lifeform > 2147483647)`).Scan(&badLifeforms); err != nil || badLifeforms != 0 {
		t.Fatalf("invalid Lifeform storage/width: %d %v", badLifeforms, err)
	}

	// Verify 2022/2025 tree codes & synonym OldCode -> Code mappings
	for _, tc := range []struct{ tree, code string }{
		{"Yc", "CALLNOO"},
		{"Pa", "PINUALB"},
		{"Plc", "PINUCON1"},
		{"Acx", "POPUHAS"},
		{"Gp", "CORNNUT"},
	} {
		var found int
		if err := db.QueryRow(`SELECT COUNT(*) FROM USysAllSpecs WHERE TreeCode=? AND Code=?`, tc.tree, tc.code).Scan(&found); err != nil || found == 0 {
			t.Fatalf("missing tree code %s -> %s: %v", tc.tree, tc.code, err)
		}
	}
	for _, syn := range []struct{ oldCode, code string }{
		{"CHAMNOO", "CALLNOO"},
		{"XANTNOO", "CALLNOO"},
		{"CUPRNOO", "CALLNOO"},
		{"POPUBAL2", "POPUTRI"},
		{"MITEBRE", "BREWBRE"},
	} {
		var found int
		if err := db.QueryRow(`SELECT COUNT(*) FROM USysAllSpecs WHERE OldCode=? AND Code=?`, syn.oldCode, syn.code).Scan(&found); err != nil || found == 0 {
			t.Fatalf("missing synonym OldCode %s -> %s: %v", syn.oldCode, syn.code, err)
		}
	}

	// Verify LMH25 (2023) vocabulary extensions in USysTableOfLists
	for _, item := range []struct{ list, code string }{
		{"StructuralStage", "3c"},
		{"StructuralStage", "3d"},
		{"SiviStructuralStage", "3c"},
		{"SiviStructuralStage", "3d"},
		{"StructuralStageMod", "v"},
		{"StructuralStageMod", "k"},
		{"SiteDisturbance", "A.w.th"},
		{"SiteDisturbance", "A.w.br"},
		{"Exposure", "CO"},
		{"FloodingRegimeDur", "S"},
		{"FloodingRegimeFreq", "S"},
	} {
		var found int
		if err := db.QueryRow(`SELECT COUNT(*) FROM USysTableOfLists WHERE ListName=? AND Item=?`, item.list, item.code).Scan(&found); err != nil || found == 0 {
			t.Fatalf("USysTableOfLists missing %s :: %s: %v", item.list, item.code, err)
		}
	}

	// Verify _table_metadata version stamps
	for _, stamp := range []struct{ tbl, substr string }{
		{"USysAllSpecs", "BCPlantTaxa_v19_2025"},
		{"USysZoneList", "BECdb_v13_2026"},
		{"USysSiteSeriesNames", "BECdb_v13_2026"},
		{"MasterSiteUnitList", "BECdb_v13_2026"},
		{"USysTableOfLists", "LMH25_2023"},
	} {
		var desc sql.NullString
		if err := db.QueryRow(`SELECT description FROM _table_metadata WHERE table_name=?`, stamp.tbl).Scan(&desc); err != nil || !desc.Valid || !strings.Contains(desc.String, stamp.substr) {
			t.Fatalf("_table_metadata[%s] = %#v (%v), want substring %q", stamp.tbl, desc, err, stamp.substr)
		}
	}
}

func TestDatabaseFamilyPreservesCustomExistingFiles(t *testing.T) {
	root := t.TempDir()
	familyDir := filepath.Join(root, "database-family")
	if err := os.MkdirAll(familyDir, 0700); err != nil {
		t.Fatal(err)
	}
	custom := []byte("custom user-modified VLists.db")
	vlistsDest := filepath.Join(familyDir, "VLists.db")
	if err := os.WriteFile(vlistsDest, custom, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallDatabaseFamily(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(vlistsDest)
	if err != nil || string(got) != string(custom) {
		t.Fatal("custom VLists.db was unexpectedly overwritten")
	}
}

func TestEnsureDefaultPictureLibraryCanonical5ColumnSchema(t *testing.T) {
	dataDir := t.TempDir()
	dbPath, imagesDir, err := EnsureDefaultPictureLibrary(dataDir)
	if err != nil {
		t.Fatalf("EnsureDefaultPictureLibrary failed: %v", err)
	}
	if dbPath != filepath.Join(dataDir, "pics", "VPics.db") || imagesDir != filepath.Join(dataDir, "pics", "images") {
		t.Fatalf("unexpected paths dbPath=%q imagesDir=%q", dbPath, imagesDir)
	}
	if info, err := os.Stat(imagesDir); err != nil || !info.IsDir() {
		t.Fatalf("expected images directory %s to exist: %v", imagesDir, err)
	}

	db, err := sqlutil.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`PRAGMA table_info("tblVPics")`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var colNames []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if dflt.Valid {
			t.Fatalf("tblVPics column %s must not declare default value (%q)", name, dflt.String)
		}
		colNames = append(colNames, name)
	}
	wantCols := []string{"ID", "PicDir", "PicName", "PlotNumber", "PicComment"}
	if strings.Join(colNames, ",") != strings.Join(wantCols, ",") {
		t.Fatalf("tblVPics columns = %v, want exact %v", colNames, wantCols)
	}

	var vp08 string
	if err := db.QueryRow(`SELECT description FROM "_table_metadata" WHERE table_name = 'tblVPics'`).Scan(&vp08); err != nil || vp08 != "VP08" {
		t.Fatalf("expected _table_metadata VP08 tag on tblVPics, got %q (%v)", vp08, err)
	}

	// Second call must preserve existing VPics.db without overwriting.
	db.Close()
	beforeBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureDefaultPictureLibrary(dataDir); err != nil {
		t.Fatal(err)
	}
	afterBytes, err := os.ReadFile(dbPath)
	if err != nil || string(beforeBytes) != string(afterBytes) {
		t.Fatal("EnsureDefaultPictureLibrary mutated existing VPics.db on subsequent call")
	}
}

