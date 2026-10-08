package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSQLitePinnedReadSnapshotCancellationPreservesContext(t *testing.T) {
	selection, support := sqliteContextFixture(t)
	owner, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	before := databaseBytes(t, owner.attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	err = tx.QueryRowContext(ctx, `WITH RECURSIVE numbers(x) AS (
		SELECT 1 UNION ALL SELECT x+1 FROM numbers WHERE x<100000000)
		SELECT SUM(x) FROM numbers`).Scan(&sum)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("in-flight snapshot query was not cancelled", err)
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Fatal(err)
	}
	var count int
	if err := owner.conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM USysEnv").Scan(&count); err != nil {
		t.Fatal("cancelled read discarded the pinned coordinator and its views", err)
	}
	if count == 0 {
		t.Fatal("context scope was lost")
	}
	after := databaseBytes(t, owner.attachments)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("cancelled snapshot changed attached files")
	}
}

func sqliteContextFixture(t *testing.T) (desktopSelection, map[string]string) {
	t.Helper()
	root := t.TempDir()
	paths, err := installDatabaseFamily(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := sampleFiles.ReadFile("resources/Sample.db")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Shared ' # \u00e9.db")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return desktopSelection{Project: "Sample", ProjectPath: path, SU: "None", Hierarchy: "None"}, paths
}

func mutateContextFixture(t *testing.T, path, query string) {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func databaseBytes(t *testing.T, paths map[string]string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for name, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result[name] = data
	}
	return result
}

func TestDatabaseFamilyInstallationPreservesOriginalsAndExistingFiles(t *testing.T) {
	root := t.TempDir()
	paths, err := installDatabaseFamily(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range databaseFamilySeeds {
		original, err := databaseFamilyFiles.ReadFile("resources/database-family/" + seed.name + ".db")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(paths[seed.name])
		if err != nil || !bytes.Equal(original, actual) || fmt.Sprintf("%x", sha256.Sum256(actual)) != seed.hash {
			t.Fatalf("%s lost byte identity with its seed", seed.name)
		}
	}
	mutateContextFixture(t, paths["VUser"], `CREATE TABLE UserExtra (value TEXT); INSERT INTO UserExtra VALUES ('retained')`)
	before := databaseBytes(t, paths)
	if _, err := installDatabaseFamily(root); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, paths)) {
		t.Fatal("existing database family was overwritten")
	}
	if files, err := filepath.Glob(filepath.Join(root, "database-family", ".family-*.db")); err != nil || len(files) != 0 {
		t.Fatal("installation leaked staging files")
	}
}

func TestSQLiteContextOfflineReadonlyViewsAndOwnedClose(t *testing.T) {
	selection, support := sqliteContextFixture(t)
	selection.SU, selection.SUPath = "Sample", selection.ProjectPath
	selection.Hierarchy, selection.HierarchyPath = "Sample", selection.ProjectPath
	files := map[string]string{"project": selection.ProjectPath}
	for name, path := range support {
		files[name] = path
	}
	before := databaseBytes(t, files)
	candidate, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := candidate.Close(); err != nil {
			t.Error(err)
		}
	})
	canonical, err := existingDatabasePath(selection.ProjectPath)
	if err != nil || candidate.selection.ProjectPath != canonical || len(candidate.attachments) != 8 {
		t.Fatalf("attachment identity/role ownership was lost: %q vs %q, roles %d, error %v",
			candidate.selection.ProjectPath, canonical, len(candidate.attachments), err)
	}
	views := map[string]string{
		"USysEnv":            `SELECT DISTINCT env.*,admin.* FROM project.Sample_Env env INNER JOIN project.Sample_Admin admin ON env.PlotNumber=admin.Plot WHERE EXISTS(SELECT 1 FROM su.Sample_SU su WHERE su.PlotNumber=env.PlotNumber)`,
		"Filtered_Env":       `SELECT DISTINCT env.*,admin.* FROM project.Sample_Env env INNER JOIN project.Sample_Admin admin ON env.PlotNumber=admin.Plot`,
		"USysVegA":           `SELECT DISTINCT ID,PlotNumber,Species,Cover1,Cover2,Cover3,TotalA,HeightA,Cover4,Cover5,Cover5a,Cover5b,Cover5c,TotalB,HeightB,Collected FROM project.Sample_Veg WHERE Cover1 IS NOT NULL OR Cover2 IS NOT NULL OR Cover3 IS NOT NULL OR TotalA IS NOT NULL OR Cover4 IS NOT NULL OR Cover5 IS NOT NULL OR TotalB IS NOT NULL OR Cover5a IS NOT NULL OR Cover5b IS NOT NULL OR Cover5c IS NOT NULL`,
		"USysVegB":           `SELECT DISTINCT ID,PlotNumber,Species,Cover4,Cover5,Cover5a,Cover5b,Cover5c,TotalB,Collected FROM project.Sample_Veg WHERE Cover4 IS NOT NULL OR Cover5 IS NOT NULL OR TotalB IS NOT NULL OR Cover5a IS NOT NULL OR Cover5b IS NOT NULL OR Cover5c IS NOT NULL`,
		"USysVegC":           `SELECT DISTINCT ID,PlotNumber,Species,Cover6,Height6,Collected FROM project.Sample_Veg WHERE Cover6 IS NOT NULL`,
		"USysVegD":           `SELECT DISTINCT ID,PlotNumber,Species,Cover7,Cover8,Cover9,Collected FROM project.Sample_Veg WHERE Cover7 IS NOT NULL OR Cover8 IS NOT NULL OR Cover9 IS NOT NULL`,
		"MasterSiteUnitList": `SELECT * FROM VLists.MasterSiteUnitList UNION SELECT * FROM VUser.UserSiteUnitList`,
	}
	for name, query := range views {
		var differs int
		check := "SELECT COUNT(*) FROM (SELECT * FROM " + quoteHeaderIdentifier(name) + " EXCEPT SELECT * FROM (" + query + "))"
		if err := candidate.conn.QueryRowContext(context.Background(), check).Scan(&differs); err != nil || differs != 0 {
			t.Fatalf("view %s includes unexpected rows: %d %v", name, differs, err)
		}
		check = "SELECT COUNT(*) FROM (SELECT * FROM (" + query + ") EXCEPT SELECT * FROM " + quoteHeaderIdentifier(name) + ")"
		if err := candidate.conn.QueryRowContext(context.Background(), check).Scan(&differs); err != nil || differs != 0 {
			t.Fatalf("view %s omitted expected rows: %d %v", name, differs, err)
		}
	}
	for name, query := range map[string]string{
		"project":   `UPDATE project.Sample_Env SET PlotRepresenting='forbidden'`,
		"reference": `DELETE FROM VLists.USysTableOfLists`,
		"user":      `DELETE FROM VUser.UserSiteUnitList`,
		"metadata":  `DELETE FROM VMetaData.ProjectMetadata`,
		"messages":  `DELETE FROM VMessageBoard.tblMessageBoard`,
		"system":    `DELETE FROM VPro64.USysEnvTable`,
	} {
		if _, err := candidate.conn.ExecContext(context.Background(), query); err == nil || !strings.Contains(err.Error(), "readonly") {
			t.Fatalf("%s attachment was writable: %v", name, err)
		}
	}
	if candidate.descriptions["VPro64"] != nil || len(candidate.descriptions["VLists"]) != 14 {
		t.Fatal("missing metadata was synthesized or reference descriptions lost")
	}
	conn := candidate.conn
	if err := candidate.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "SELECT * FROM USysEnv"); !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("owned connection remained usable after close: %v", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, files)) {
		t.Fatal("readonly context changed a source file/schema/description")
	}
}

func TestSQLiteContextPreservesDuplicateNullEmptyDescriptionMetadata(t *testing.T) {
	selection, support := sqliteContextFixture(t)
	mutateContextFixture(t, selection.ProjectPath, `ALTER TABLE _table_metadata RENAME TO _metadata_original;
		CREATE TABLE _table_metadata AS SELECT * FROM _metadata_original; DROP TABLE _metadata_original;
		ALTER TABLE _table_metadata ADD COLUMN Extra TEXT;
		INSERT INTO _table_metadata(table_name,description,Extra) VALUES ('Custom',NULL,'first'),('Custom','','second');
		CREATE TABLE ExtraObject (ID INTEGER)`)
	before, err := os.ReadFile(selection.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		t.Fatal(err)
	}
	defer candidate.Close()
	var custom []map[string]any
	for _, row := range candidate.descriptions["project"] {
		if row["table_name"] == "Custom" {
			custom = append(custom, row)
		}
	}
	if len(custom) != 2 || custom[0]["description"] != nil || custom[1]["description"] != "" ||
		custom[0]["Extra"] != "first" || custom[1]["Extra"] != "second" {
		t.Fatalf("descriptions/duplicates/extra columns collapsed: %#v", custom)
	}
	after, err := os.ReadFile(selection.ProjectPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("metadata inspection modified storage")
	}
}

func TestSQLiteContextRejectsInvalidCandidatesWithoutMutation(t *testing.T) {
	for _, tc := range []struct{ name, query, diagnostic string }{
		{"unknown version", `UPDATE _table_metadata SET description=NULL WHERE table_name='Sample_Env'`, "unambiguous VP08"},
		{"old version", `UPDATE _table_metadata SET description='VP07' WHERE table_name='Sample_Env'`, "conversion is unavailable"},
		{"ambiguous version", `ALTER TABLE _table_metadata RENAME TO _metadata_original;
			CREATE TABLE _table_metadata AS SELECT * FROM _metadata_original; DROP TABLE _metadata_original;
			INSERT INTO _table_metadata(table_name,description) VALUES ('Sample_Env','VP08')`, "unambiguous VP08"},
		{"missing metadata", `DROP TABLE _table_metadata`, "unambiguous VP08"},
		{"missing family", `DROP TABLE Sample_Humus`, "physical table"},
		{"view is not storage", `DROP TABLE Sample_Other; CREATE VIEW Sample_Other AS SELECT * FROM Sample_Humus`, "physical table"},
		{"missing column", `ALTER TABLE Sample_Veg RENAME COLUMN Cover9 TO Unavailable`, "column Cover9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, support := sqliteContextFixture(t)
			mutateContextFixture(t, selection.ProjectPath, tc.query)
			before, err := os.ReadFile(selection.ProjectPath)
			if err != nil {
				t.Fatal(err)
			}
			candidate, err := newSQLiteContext(context.Background(), selection, support)
			if err == nil || candidate != nil || !strings.Contains(err.Error(), tc.diagnostic) {
				if candidate != nil {
					candidate.Close()
				}
				t.Fatalf("candidate unexpectedly accepted: %v", err)
			}
			after, err := os.ReadFile(selection.ProjectPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("rejected candidate was mutated/repaired")
			}
		})
	}
}

func TestSQLiteContextPathSupportSchemaCancellationAndCollision(t *testing.T) {
	selection, support := sqliteContextFixture(t)
	missing := filepath.Join(t.TempDir(), "absent.db")
	bad := selection
	bad.ProjectPath = missing
	if _, err := newSQLiteContext(context.Background(), bad, support); err == nil {
		t.Fatal("missing database was accepted")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("readonly attachment created the missing file")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newSQLiteContext(ctx, selection, support); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context was accepted: %v", err)
	}
	candidate, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := candidate.attach(context.Background(), "project", selection.ProjectPath); err == nil {
		t.Fatal("owned alias collision was accepted")
	}
	if err := candidate.Close(); err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, support["VLists"], `ALTER TABLE USysAllSpecs RENAME COLUMN CombinedEnglishName TO LegacyName`)
	before := databaseBytes(t, support)
	if _, err := newSQLiteContext(context.Background(), selection, support); err == nil || !strings.Contains(err.Error(), "CombinedEnglishName") {
		t.Fatalf("incompatible support schema was accepted: %v", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, support)) {
		t.Fatal("support failure repaired or replaced existing files")
	}
}

func TestSQLiteContextFamiliesShareOnePhysicalFileWithoutImplicitIdentity(t *testing.T) {
	selection, support := sqliteContextFixture(t)
	for _, suffix := range coreTables {
		mutateContextFixture(t, selection.ProjectPath,
			`CREATE TABLE "Second_`+suffix+`" AS SELECT * FROM "Sample_`+suffix+`"`)
	}
	mutateContextFixture(t, selection.ProjectPath,
		`INSERT INTO _table_metadata(table_name,description) VALUES ('Second_Env','VP08');
		 UPDATE Second_Env SET PlotRepresenting='second family'`)
	before, err := os.ReadFile(selection.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	first, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	selection.Project = "Second"
	second, err := newSQLiteContext(context.Background(), selection, support)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	for candidate, expected := range map[*sqliteContext]bool{first: false, second: true} {
		var observed bool
		if err := candidate.conn.QueryRowContext(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotRepresenting='second family')`).Scan(&observed); err != nil || observed != expected {
			t.Fatalf("context implicitly changed family identity: %t %v", observed, err)
		}
	}
	after, err := os.ReadFile(selection.ProjectPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("two owned contexts altered their shared source")
	}
}

func TestSQLiteContextExternalSUAuthorizationAndHierarchySchema(t *testing.T) {
	for _, tc := range []struct{ name, query, diagnostic string }{
		{"unauthorized SU", `CREATE TABLE _vpro_su_policy(table_name TEXT,kind TEXT);
			INSERT INTO _vpro_su_policy VALUES ('Sample_SU','master')`, "not authorized"},
		{"invalid SU types", `DROP TABLE Sample_SU; CREATE TABLE Sample_SU(PlotNumber INTEGER, SiteUnit TEXT)`, "text column"},
		{"invalid hierarchy types", `DROP TABLE Sample_Hierarchy;
			CREATE TABLE Sample_Hierarchy(ID TEXT,Name TEXT,Parent INTEGER,Level INTEGER)`, "integer ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, support := sqliteContextFixture(t)
			selection.SU, selection.SUPath = "Sample", selection.ProjectPath
			selection.Hierarchy, selection.HierarchyPath = "Sample", selection.ProjectPath
			mutateContextFixture(t, selection.ProjectPath, tc.query)
			if candidate, err := newSQLiteContext(context.Background(), selection, support); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				if candidate != nil {
					candidate.Close()
				}
				t.Fatalf("invalid external child context accepted: %v", err)
			}
		})
	}
}
