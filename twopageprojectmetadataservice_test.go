package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func twoPageProjectMetadataFixture(t *testing.T, external bool) (*TwoPageProjectMetadataService, ProjectState, *sql.DB) {
	t.Helper()
	environment, state, db := twoPageEnvironmentTableFixture(t, external)
	contexts := environment.contexts
	columns, err := readSQLiteStorageColumns(context.Background(), db, "main", "Sample_Metadata")
	if err != nil {
		t.Fatal(err)
	}
	var definitions []string
	for _, column := range columns {
		definitions = append(definitions, quoteHeaderIdentifier(column.Name)+" "+column.DeclaredType)
	}
	// Exercise Access-style independent rowid/nullable application ID, retaining every physical column/type.
	if _, err := db.Exec(`DROP TABLE Sample_Metadata;CREATE TABLE Sample_Metadata(` +
		strings.Join(definitions, ",") + `);CREATE UNIQUE INDEX MetadataApplicationID ON Sample_Metadata(ID)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID='Alpha' WHERE PlotNumber='108050';
		DELETE FROM Sample_Metadata;
		ALTER TABLE Sample_Metadata ADD COLUMN Unmapped;
		INSERT INTO Sample_Metadata(rowid,ProjectID,ID,ProjectTitle,Notes,CollectedSite,Unmapped) VALUES
		(-7,'Alpha',NULL,NULL,'',-1,X'FF00'),
		(2,'Alpha',-2147483648,'duplicate',NULL,3,NULL),
		(3,'alpha','historical invalid ID','lowercase',NULL,NULL,NULL),
		(4,'007',7,'leading zero',NULL,NULL,NULL),
		(5,'7',8,'numeric looking',NULL,NULL,NULL),
		(6,'',9,'empty',NULL,NULL,NULL),
		(7,NULL,10,'null',NULL,NULL,NULL),
		(8,'spaced',11,'no terminal space',NULL,NULL,NULL),
		(9,'spaced ',12,'terminal space',NULL,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	service, err := NewTwoPageProjectMetadataService(contexts, func(name string) (string, bool) {
		return "true", name == twoPageProjectMetadataEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

func TestTwoPageProjectMetadataSourceMatchingOriginalsAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, db := twoPageProjectMetadataFixture(t, external)
			for _, test := range []struct {
				id   string
				rows []string
			}{{"Alpha", []string{"-7", "2", "3"}}, {"alpha", []string{"-7", "2", "3"}},
				{"007", []string{"4"}}, {"7", []string{"5"}}, {"", []string{"6"}},
				{"spaced", []string{"8", "9"}}, {"spaced ", []string{"8", "9"}}, {"Absent", []string{}}} {
				if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID=? WHERE PlotNumber='108050'`, test.id); err != nil {
					t.Fatal(err)
				}
				before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
				configBefore, err := os.ReadFile(service.contexts.projects.preferences.path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 200)
				if err != nil {
					t.Fatal(err)
				}
				ids := []string{}
				for _, row := range got.MetadataOriginal.Rows {
					ids = append(ids, row.RowID)
				}
				if !reflect.DeepEqual(ids, test.rows) || got.TotalRows != len(test.rows) ||
					got.ProjectID != test.id || got.ContextID != state.ContextID || got.Project != state.ActiveProject ||
					got.ProjectPath != state.ProjectPath || got.SU != state.ActiveSU || got.SUPath != state.SUPath ||
					got.TargetForm != "frmProjectMetaData" || got.MetadataTable != "Sample_Metadata" ||
					got.Plot != "108050" || got.MatchingPolicy != twoPageProjectMetadataMatching || got.GeneralLocaleVerified ||
					len(got.EnvOriginal.Rows) != 1 || len(got.AdminOriginal.Rows) != 1 ||
					len(got.MetadataOriginal.Columns) < 75 {
					t.Fatal("source matching/complete physical ownership changed", test.id, ids, got)
				}
				if test.id == "Alpha" {
					index, err := siteUnitTransferColumns(got.MetadataOriginal, "ID", "Notes", "CollectedSite", "Unmapped", "ProjectID")
					if err != nil {
						t.Fatal(err)
					}
					rows := got.MetadataOriginal.Rows
					if rows[0].Cells[index["ID"]].Storage != "null" ||
						rows[0].Cells[index["Notes"]].Text == nil || *rows[0].Cells[index["Notes"]].Text != "" ||
						rows[0].Cells[index["CollectedSite"]].Integer == nil || *rows[0].Cells[index["CollectedSite"]].Integer != "-1" ||
						rows[0].Cells[index["Unmapped"]].BlobHex == nil || *rows[0].Cells[index["Unmapped"]].BlobHex != "ff00" ||
						rows[2].Cells[index["ID"]].Text == nil || *rows[2].Cells[index["ID"]].Text != "historical invalid ID" ||
						*rows[2].Cells[index["ProjectID"]].Text != "alpha" {
						t.Fatal("historical IDs/NULL/empty/source text/storage repaired", rows)
					}
					page, err := service.GetReview(context.Background(), state.ContextID, "108050", 1, 1)
					if err != nil || page.TotalRows != 3 || len(page.MetadataOriginal.Rows) != 1 ||
						!reflect.DeepEqual(page.MetadataOriginal.Rows[0], rows[1]) {
						t.Fatal("physical-page boundaries changed", page, err)
					}
				}
				if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
					t.Fatal("readonly source review changed family/audits")
				}
				configAfter, err := os.ReadFile(service.contexts.projects.preferences.path)
				if err != nil || !reflect.DeepEqual(configBefore, configAfter) {
					t.Fatal("readonly source review changed effective/persisted config", err)
				}
			}
		})
	}
}

func TestTwoPageProjectMetadataUnsupportedSourceAndCorrection(t *testing.T) {
	service, state, db := twoPageProjectMetadataFixture(t, false)
	for _, expression := range []string{"NULL", "7", "'O''Neil'", "'Unicodeé'", "X'414c504841'", "CAST(X'FF' AS TEXT)"} {
		if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID=` + expression + ` WHERE PlotNumber='108050'`); err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
		// TEXT affinity preserves a numeric assignment as literal "7".
		if expression == "7" {
			if err != nil || got == nil || got.ProjectID != "7" {
				t.Fatal("original TEXT numeric-looking identity refused", got, err)
			}
		} else if err == nil || got != nil {
			t.Fatal("unsupported parent converted into identity", expression, got, err)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID='Alpha' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"ProjectID", "Notes"} {
		if _, err := db.Exec(`UPDATE Sample_Metadata SET ` + column + `=CAST(X'FF' AS TEXT) WHERE rowid=9`); err != nil {
			t.Fatal(err)
		}
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
			t.Fatal("malformed physical text repaired in JSON", column, got, err)
		}
		if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID='spaced ',Notes=NULL WHERE rowid=9`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID='Unicodeé' WHERE rowid=9`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("unknown Unicode membership silently omitted", got, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID='spaced ',ProjectTitle=? WHERE rowid=9`, strings.Repeat("x", 256)); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got == nil || got.TotalRows != 3 {
		t.Fatal("source correction/retry or historical overlength retention failed", got, err)
	}
}

func TestTwoPageProjectMetadataPlannerHistoricalStorageAndSchemas(t *testing.T) {
	textID, blobID, flag, blob := "01", "ff00", "-1", "ff00"
	source := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "ProjectID"}, {Name: "ID"}, {Name: "Notes"}, {Name: "Flag"}, {Name: "Extra"}},
		Rows: []ProjectMetadataRow{
			{RowID: "-1", Cells: []ProjectMetadataCell{metadataText("Alpha"), {Storage: "null"},
				metadataText(strings.Repeat("x", 256)), {Storage: "integer", Integer: &flag}, {Storage: "blob", BlobHex: &blob}}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("alpha "), {Storage: "text", Text: &textID},
				metadataText(""), {Storage: "null"}, {Storage: "null"}}},
			{RowID: "3", Cells: []ProjectMetadataCell{metadataText("ALPHA"), {Storage: "blob", BlobHex: &blobID},
				{Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
		},
	}
	page, total, err := planTwoPageProjectMetadata(context.Background(), source, "Alpha", 0, 200)
	if err != nil || total != 3 || !reflect.DeepEqual(page, source) {
		t.Fatal("readonly planner repaired ID/NULL/empty/overlength/blob/original key", page, total, err)
	}
	for _, columns := range [][]ProjectMetadataColumn{
		{{Name: "Other"}}, {{Name: "ProjectID"}, {Name: "ProjectID"}}, {{Name: ""}},
	} {
		if _, _, err := planTwoPageProjectMetadata(context.Background(), ProjectMetadataTable{Columns: columns}, "Alpha", 0, 1); err == nil {
			t.Fatal("missing/ambiguous physical schema accepted", columns)
		}
	}
	for _, key := range []ProjectMetadataCell{{Storage: "integer", Integer: &flag}, {Storage: "blob", BlobHex: &blobID}} {
		physical := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ProjectID"}},
			Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{key}}}}
		if _, _, err := planTwoPageProjectMetadata(context.Background(), physical, "Alpha", 0, 1); err == nil {
			t.Fatal("non-TEXT source identity coerced", key)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := planTwoPageProjectMetadata(ctx, source, "Alpha", 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatal("planner cancellation lost", err)
	}
}

func TestTwoPageProjectMetadataPhysicalSourcesAndOwnership(t *testing.T) {
	for _, test := range []struct{ name, sql string }{
		{"missingMetadata", `DROP TABLE Sample_Metadata`},
		{"metadataView", `ALTER TABLE Sample_Metadata RENAME TO Backup_Metadata;CREATE VIEW Sample_Metadata AS SELECT * FROM Backup_Metadata`},
		{"missingProjectID", `ALTER TABLE Sample_Metadata RENAME COLUMN ProjectID TO OtherProjectID`},
		{"missingParentProjectID", `ALTER TABLE Sample_Env RENAME COLUMN ProjectID TO OtherProjectID`},
		{"missingAdmin", `DELETE FROM Sample_Admin WHERE Plot='108050'`},
		{"duplicateAdmin", `INSERT INTO Sample_Admin(Plot) VALUES('108050')`},
		{"duplicateEnv", `INSERT INTO Sample_Env(PlotNumber,ProjectID) VALUES('108050','Alpha')`},
		{"nonTextMetadataKey", `UPDATE Sample_Metadata SET ProjectID=X'414c504841' WHERE rowid=9`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, state, db := twoPageProjectMetadataFixture(t, false)
			if _, err := db.Exec(test.sql); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
				t.Fatal("unavailable/ambiguous physical scope returned result", got, err)
			}
		})
	}
	service, state, _ := twoPageProjectMetadataFixture(t, true)
	for _, plot := range []string{"", "108050 ", "unselected", "\xff"} {
		if got, err := service.GetReview(context.Background(), state.ContextID, plot, 0, 1); err == nil || got != nil {
			t.Fatal("literal selected parent ownership bypassed", plot, got, err)
		}
	}
	if got, err := service.GetReview(context.Background(), "foreign-context", "108050", 0, 1); err == nil || got != nil {
		t.Fatal("foreign context returned review", got, err)
	}
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("stale context returned review", got, err)
	}
	got, err := service.GetReview(context.Background(), next.ContextID, "108050", 0, 1)
	if err != nil || got == nil || got.SU != "None" || got.TotalRows != 3 {
		t.Fatal("None context owned read unavailable", got, err)
	}
}

func TestTwoPageProjectMetadataGateBoundsAndMasterIndependence(t *testing.T) {
	service, state, db := twoPageProjectMetadataFixture(t, false)
	if _, err := db.Exec(`DELETE FROM Sample_Metadata;
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<201)
		INSERT INTO Sample_Metadata(ProjectID,ID) SELECT 'Alpha',i FROM n`); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ offset, limit, count int }{{0, 200, 200}, {200, 1, 1}, {201, 50, 0}, {100000, 200, 0}} {
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", test.offset, test.limit)
		if err != nil || got == nil || got.TotalRows != 201 || len(got.MetadataOriginal.Rows) != test.count ||
			got.Offset != test.offset || got.Limit != test.limit {
			t.Fatal("bounded/end paging failed", test, got, err)
		}
	}
	for _, page := range []struct{ offset, limit int }{{-1, 1}, {100001, 1}, {0, 0}, {0, 201}, {int(^uint(0) >> 1), 1}} {
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", page.offset, page.limit); err == nil || got != nil {
			t.Fatal("invalid page returned result", page, got, err)
		}
	}
	for _, value := range []string{"", "false", "TRUE", "1", " true "} {
		gated, err := NewTwoPageProjectMetadataService(service.contexts, func(string) (string, bool) { return value, value != "" })
		if value == "" || value == "false" {
			if err != nil {
				t.Fatal(err)
			}
			if got, err := gated.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
				t.Fatal("default/off gate returned review", got, err)
			}
		} else if err == nil {
			t.Fatal("nonliteral feature value accepted", value)
		}
	}
	if _, err := NewTwoPageProjectMetadataService(nil, os.LookupEnv); err == nil {
		t.Fatal("missing owned contexts accepted")
	}
	if _, err := NewTwoPageProjectMetadataService(service.contexts, nil); err == nil {
		t.Fatal("missing explicit feature lookup accepted")
	}
	owner := service.contexts.projects.sqlite
	master, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, dropErr := master.Exec(`DROP TABLE ProjectMetaData`)
	if err := errors.Join(dropErr, master.Close()); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got == nil || got.TotalRows != 201 {
		t.Fatal("existing-record review depends on Add/global templates", got, err)
	}
}

func TestTwoPageProjectMetadataMaximumPhysicalBound(t *testing.T) {
	service, state, db := twoPageProjectMetadataFixture(t, false)
	if _, err := db.Exec(`DELETE FROM Sample_Metadata;
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<100000)
		INSERT INTO Sample_Metadata(ProjectID,ID) SELECT 'Other',i FROM n`); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 100000, 1)
	if err != nil || got == nil || got.TotalRows != 0 || len(got.MetadataOriginal.Rows) != 0 {
		t.Fatal("exact100000 physical source refused", got, err)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Metadata(ProjectID,ID) VALUES('Other',100001)`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("over100000 returned partial review", got, err)
	}
}

func TestTwoPageProjectMetadataInFlightCancellationAndRetry(t *testing.T) {
	service, state, _ := twoPageProjectMetadataFixture(t, false)
	owner := service.contexts.projects.sqlite
	if got, err := service.GetReview(nil, state.ContextID, "108050", 0, 1); err == nil || got != nil {
		t.Fatal("nil request context returned data", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("metadata cancellation requires actual SQLite")
		}
		return conn.RegisterFunc("metadataReadBarrier", func(value any) any {
			once.Do(func() { close(entered) })
			<-release
			return value
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := owner.conn.QueryRowContext(context.Background(), `SELECT sql FROM sqlite_temp_master WHERE name='USysEnv'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT metadataReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.GetReview(ctx, state.ContextID, "108050", 0, 1)
		if got != nil {
			err = errors.Join(err, errors.New("cancelled metadata read returned data"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("owned metadata read did not reach SQLite")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled metadata read did not settle")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;`+
		strings.Replace(original, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetReview(context.Background(), state.ContextID, "108050", 0, 1)
	if err != nil || got == nil || got.TotalRows != 3 {
		t.Fatal("cancelled pinned coordinator lost attachments/TEMP views", got, err)
	}
}
