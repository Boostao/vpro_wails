package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func pictureLibraryFixture(t *testing.T, schema string) (*ownedPictureLibrary, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pictures #%.sqlite")
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(schema)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	source, err := newOwnedPictureLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	return source, path
}

const pictureFixtureSchema = `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
	INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment) VALUES
	(9223372036854775806,42,'Default','Pic01.jpg','108050',NULL),
	(-9,NULL,'',NULL,'108050',''),
	(3,42,' external literal ','  spaced.jpg  ','108050',x'0061'),
	(4,-2147483648,'Default','Pic02.jpg','108050','original'),
	(5,9,'Default','other.jpg','108050x',NULL),
	(6,10,'Default','numeric.jpg',108050,NULL);`

func TestPictureMetadataOwnedPhysicalRowsAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			source, path := pictureLibraryFixture(t, pictureFixtureSchema)
			files := databaseBytes(t, service.projects.sqlite.attachments)
			pictureBytes := databaseBytes(t, map[string]string{"pictures": path})["pictures"]
			review, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
			if err != nil {
				t.Fatal(err)
			}
			if review.ContextID != state.ContextID || review.Project != "Sample" || review.PlotNumber != "108050" ||
				len(review.Records.Columns) != 5 || len(review.Records.Rows) != 4 {
				t.Fatal("picture review changed scope or included nonliteral numeric/foreign plots", review)
			}
			rows := review.Records.Rows
			if rows[0].RowID != "-9" || rows[0].Cells[0].Storage != "null" ||
				rows[0].Cells[1].Text == nil || *rows[0].Cells[1].Text != "" ||
				rows[0].Cells[2].Storage != "null" || *rows[0].Cells[4].Text != "" ||
				rows[1].RowID != "3" || *rows[1].Cells[0].Integer != "42" ||
				*rows[1].Cells[1].Text != " external literal " || *rows[1].Cells[2].Text != "  spaced.jpg  " ||
				*rows[1].Cells[4].BlobHex != "0061" || *rows[2].Cells[0].Integer != "-2147483648" ||
				rows[3].RowID != "9223372036854775806" || *rows[3].Cells[0].Integer != "42" ||
				rows[3].Cells[4].Storage != "null" {
				t.Fatal("NULL/empty/duplicate ID/physical identity/literal/storage evidence was repaired", rows)
			}
			*rows[1].Cells[0].Integer = "changed returned copy"
			again, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
			if err != nil || *again.Records.Rows[1].Cells[0].Integer != "42" {
				t.Fatal("detached metadata aliased its source", again, err)
			}
			assertProfileSUFiles(t, service, files)
			if got := databaseBytes(t, map[string]string{"pictures": path})["pictures"]; !reflect.DeepEqual(got, pictureBytes) {
				t.Fatal("read-only picture metadata changed its source bytes")
			}
		})
	}
}

func TestPictureMetadataRejectsUnavailableOrRepairedBindings(t *testing.T) {
	for name, schema := range map[string]string{
		"staging":        `CREATE TABLE stage_rows(value);`,
		"view":           `CREATE TABLE physical(ID,PicDir,PicName,PlotNumber,PicComment); CREATE VIEW tblVPics AS SELECT * FROM physical;`,
		"recased-table":  `CREATE TABLE tblvpics(ID,PicDir,PicName,PlotNumber,PicComment);`,
		"recased-column": `CREATE TABLE tblVPics(ID,picdir,PicName,PlotNumber,PicComment);`,
		"extra-column":   `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment,Extra);`,
		"missing":        `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber);`,
		"reordered":      `CREATE TABLE tblVPics(PicDir,ID,PicName,PlotNumber,PicComment);`,
		"generated":      `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment AS (PicName));`,
		"without-rowid":  `CREATE TABLE tblVPics(ID PRIMARY KEY,PicDir,PicName,PlotNumber,PicComment) WITHOUT ROWID;`,
		"malformed":      `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment); INSERT INTO tblVPics VALUES(1,'Default',CAST(x'ff' AS TEXT),'108050',NULL);`,
	} {
		t.Run(name, func(t *testing.T) {
			source, _ := pictureLibraryFixture(t, schema)
			got, err := readPictureLibrary(context.Background(), source, "108050")
			if err == nil || !reflect.DeepEqual(got, ProjectMetadataTable{}) {
				t.Fatal("unavailable binding returned a success/partial projection", got, err)
			}
		})
	}
}

func TestPictureMetadataLiteralScopeIgnoresSQLiteNumericAffinity(t *testing.T) {
	for _, affinity := range []string{"INTEGER", "NUMERIC", "REAL", "TEXT", "BLOB", ""} {
		t.Run(affinity, func(t *testing.T) {
			source, _ := pictureLibraryFixture(t, `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber `+affinity+`,PicComment);
				INSERT INTO tblVPics VALUES(1,'Default','numeric.jpg',108050,NULL),
					(2,'Default','literal.jpg','literal '' plot',NULL),
					(3,'Default','other.jpg','Literal '' plot',NULL);`)
			for _, plot := range []string{"108050", "0108050"} {
				got, err := readPictureLibrary(context.Background(), source, plot)
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if affinity == "TEXT" && plot == "108050" {
					want = 1
				}
				if len(got.Rows) != want {
					t.Fatal("SQLite affinity widened literal text picture membership", affinity, plot, got.Rows)
				}
			}
			got, err := readPictureLibrary(context.Background(), source, "literal ' plot")
			if err != nil || len(got.Rows) != 1 || *got.Rows[0].Cells[0].Integer != "2" {
				t.Fatal("literal UTF8/quote/case-sensitive text scope was repaired", affinity, got, err)
			}
			if affinity == "INTEGER" || affinity == "NUMERIC" || affinity == "REAL" {
				db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro"))
				if err != nil {
					t.Fatal(err)
				}
				leadingZero := "0108050"
				legacy, readErr := readSQLiteStorageRows(context.Background(), db, "main", "tblVPics", "PlotNumber", &leadingZero, "")
				if err := errors.Join(readErr, db.Close()); err != nil || len(legacy.Rows) != 1 {
					t.Fatal("fixture did not independently reproduce the existing affinity-coercion boundary", legacy, err)
				}
			}
		})
	}
}

func TestPictureMetadataContextParentAndFileOwnership(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	source, path := pictureLibraryFixture(t, pictureFixtureSchema)
	for _, test := range []struct{ contextID, plot string }{
		{"", "108050"}, {"stale", "108050"}, {state.ContextID, ""}, {state.ContextID, "108050 "},
		{state.ContextID, "missing"}, {state.ContextID, "108050\x00"}, {state.ContextID, string([]byte{0xff})},
	} {
		got, err := service.readPictureMetadata(context.Background(), test.contextID, test.plot, source)
		if err == nil || !reflect.DeepEqual(got, pictureMetadataReview{}) {
			t.Fatal("stale/outside/literal-invalid parent returned metadata", test, got, err)
		}
	}
	if _, err := newOwnedPictureLibrary("relative.sqlite"); err == nil {
		t.Fatal("relative source path was authorized")
	}
	if _, err := newOwnedPictureLibrary(filepath.Dir(path)); err == nil {
		t.Fatal("directory was authorized as a database")
	}
	if _, err := newOwnedPictureLibrary(filepath.Join(t.TempDir(), "missing.sqlite")); err == nil {
		t.Fatal("missing source was implicitly created")
	}
	if got, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", nil); err == nil || !reflect.DeepEqual(got, pictureMetadataReview{}) {
		t.Fatal("missing optional source returned a success-shaped review", got, err)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(pictureFixtureSchema)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", source); err == nil || !strings.Contains(err.Error(), "identity changed") || !reflect.DeepEqual(got, pictureMetadataReview{}) {
		t.Fatal("replacement source retained old authorization", got, err)
	}
	reloaded, err := newOwnedPictureLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", reloaded); err != nil {
		t.Fatal("explicitly reloaded source could not retry", err)
	}
	if err := service.projects.sqlite.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", reloaded); err == nil || !reflect.DeepEqual(got, pictureMetadataReview{}) {
		t.Fatal("closed context returned picture metadata", got, err)
	}
}

func TestPictureMetadataCancellationAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	source, _ := pictureLibraryFixture(t, pictureFixtureSchema)
	original, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.readPictureMetadata(cancelled, state.ContextID, "108050", source); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, pictureMetadataReview{}) {
		t.Fatal("pre-cancelled read returned metadata", got, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, finish := context.WithTimeout(context.Background(), 50*time.Millisecond)
	got, err := service.readPictureMetadata(ctx, state.ContextID, "108050", source)
	finish()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(got, pictureMetadataReview{}) {
		t.Fatal("cancelled owner lease returned metadata", got, err)
	}
	again, err := service.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
	if err != nil || !reflect.DeepEqual(again, original) {
		t.Fatal("cancelled read discarded source/context ownership or prevented retry", again, err)
	}
	ctx, finish = context.WithCancel(context.Background())
	snapshot, err := withPictureLibrarySnapshot(ctx, source, "108050", func(records ProjectMetadataTable) (ProjectMetadataTable, error) {
		finish()
		return records, nil
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(snapshot, ProjectMetadataTable{}) {
		t.Fatal("cancelled completed snapshot published partial/late metadata", snapshot, err)
	}
	again, err = service.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
	if err != nil || !reflect.DeepEqual(again, original) {
		t.Fatal("cancelled snapshot broke the next owned read", again, err)
	}
}
