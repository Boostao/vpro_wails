package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func twoPageSoilReadFixture(t *testing.T, external bool, affinity string) (*ContextService, ProjectState, *sql.DB) {
	t.Helper()
	service, state := reportServiceFixture(t, external)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["project"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, kind := range []string{"Humus", "Mineral"} {
		if _, err := db.Exec(`DROP TABLE IF EXISTS "Sample_` + kind + `"`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE "Sample_` + kind + `" (
			ID, PlotNumber ` + affinity + `, UpperDepth, Horizon, Comment, Unmapped)`); err != nil {
			t.Fatal(err)
		}
	}
	return service, state, db
}

func TestTwoPageSoilOriginalPhysicalRowsAndNoMutations(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
			for _, kind := range []string{"Humus", "Mineral"} {
				if _, err := db.Exec(`INSERT INTO "Sample_` + kind + `"(rowid,ID,PlotNumber,UpperDepth,Horizon,Comment,Unmapped)
					VALUES(-7,NULL,'108050',-10.25,'',NULL,X'FF00'),(3,0,'108050',NULL,'overlength historical', '',7),
					(4,'01','108050',0,'NULL is not text', 'unchanged',NULL),
					(5,-2147483648,'108050',2.5,NULL,'negative',NULL),
					(6,2147483647,'other',8,NULL,'outside scope',NULL)`); err != nil {
					t.Fatal(err)
				}
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			for _, test := range []struct{ form, kind, source string }{
				{"SoilHumusXL", "Humus", "UsysHumus"}, {"SoilMineralXL", "Mineral", "UsysMineral"},
			} {
				got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", test.form)
				if err != nil {
					t.Fatal(err)
				}
				if got.ContextID != state.ContextID || got.Project != "Sample" || got.Plot != "108050" ||
					got.Form != test.form || got.RecordSource != test.source || got.Table != "Sample_"+test.kind {
					t.Fatalf("physical source ownership = %+v", got)
				}
				wantColumns := []ProjectMetadataColumn{
					{Name: "ID"}, {Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "UpperDepth"},
					{Name: "Horizon"}, {Name: "Comment"}, {Name: "Unmapped"},
				}
				if !reflect.DeepEqual(got.Records.Columns, wantColumns) {
					t.Fatalf("complete physical schema = %#v", got.Records.Columns)
				}
				var rowIDs []string
				for _, row := range got.Records.Rows {
					rowIDs = append(rowIDs, row.RowID)
				}
				if !reflect.DeepEqual(rowIDs, []string{"-7", "3", "4", "5"}) {
					t.Fatalf("physical rowid scope/order = %v", rowIDs)
				}
				blobHex := "ff00"
				want := []ProjectMetadataCell{
					{Storage: "null"}, metadataText("108050"), siviReal(-10.25), metadataText(""),
					{Storage: "null"}, {Storage: "blob", BlobHex: &blobHex},
				}
				if !reflect.DeepEqual(got.Records.Rows[0].Cells, want) {
					t.Fatalf("NULL/empty/real/BLOB original cells = %#v", got.Records.Rows[0].Cells)
				}
				if got.Records.Rows[1].Cells[0].Storage != "integer" || *got.Records.Rows[1].Cells[0].Integer != "0" ||
					got.Records.Rows[2].Cells[0].Storage != "text" || *got.Records.Rows[2].Cells[0].Text != "01" ||
					*got.Records.Rows[3].Cells[0].Integer != "-2147483648" {
					t.Fatal("historical nullable/noninteger/signed identities were repaired")
				}
			}
			if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
				t.Fatal("physical original read changed an owned database")
			}
		})
	}
}

func TestTwoPageSoilOriginalLiteralStorageAcrossAffinities(t *testing.T) {
	for _, affinity := range []string{"TEXT", "INTEGER", "NUMERIC", "REAL", "BLOB", ""} {
		t.Run("affinity-"+affinity, func(t *testing.T) {
			service, state, db := twoPageSoilReadFixture(t, false, affinity)
			if _, err := db.Exec(`INSERT INTO Sample_Humus(rowid,ID,PlotNumber) VALUES
				(1,1,'108050'),(2,2,108050),(3,3,CAST('108050' AS BLOB)),
				(4,4,'0108050'),(5,5,'108050 '),(6,6,NULL)`); err != nil {
				t.Fatal(err)
			}
			got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{}
			switch affinity {
			case "TEXT":
				want = []string{"1", "2"}
			case "BLOB", "":
				want = []string{"1"}
			}
			rowIDs := []string{}
			for _, row := range got.Records.Rows {
				rowIDs = append(rowIDs, row.RowID)
			}
			if !reflect.DeepEqual(rowIDs, want) {
				t.Fatalf("literal TEXT ownership = %v, want %v", rowIDs, want)
			}
		})
	}
}

func TestTwoPageSoilOriginalScopeAndPhysicalTableRefusals(t *testing.T) {
	service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
	ctx := context.Background()
	for _, input := range []struct{ contextID, plot, form string }{
		{state.ContextID, "108050", "SoilHumusXL "},
		{state.ContextID, "", "SoilHumusXL"},
		{state.ContextID, string([]byte{0xff}), "SoilHumusXL"},
		{state.ContextID, "108050 ", "SoilHumusXL"},
		{state.ContextID, "absent", "SoilHumusXL"},
		{state.ContextID + "-stale", "108050", "SoilHumusXL"},
	} {
		if got, err := service.readTwoPageSoilOriginal(ctx, input.contextID, input.plot, input.form); err == nil || got != nil {
			t.Fatalf("invalid literal scope returned %v, %v", got, err)
		}
	}
	if got, err := service.readTwoPageSoilOriginal(nil, state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil {
		t.Fatal("nil context accepted")
	}
	var absent *ContextService
	if got, err := absent.readTwoPageSoilOriginal(ctx, state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil {
		t.Fatal("nil owner accepted")
	}
	if _, err := service.projects.sqlite.conn.ExecContext(ctx, `CREATE TEMP TABLE Sample_Humus(ID,PlotNumber,FalseShadow)`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readTwoPageSoilOriginal(ctx, state.ContextID, "108050", "SoilHumusXL"); err != nil ||
		len(got.Records.Columns) != 6 || got.Records.Columns[5].Name != "Unmapped" {
		t.Fatalf("TEMP shadow replaced physical project family: %v, %v", got, err)
	}
	if _, err := db.Exec(`DROP TABLE Sample_Humus; CREATE VIEW Sample_Humus AS SELECT 1 ID,'108050' PlotNumber`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readTwoPageSoilOriginal(ctx, state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil ||
		!strings.Contains(err.Error(), "original physical project table") {
		t.Fatalf("project view substituted for canonical physical storage: %v, %v", got, err)
	}
}

func TestTwoPageSoilOriginalRejectsMissingParentColumns(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
			for _, plot := range []string{"108050", "PlotNumber"} {
				t.Run(form+"/"+plot+map[bool]string{false: "/embedded", true: "/external"}[external], func(t *testing.T) {
					service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
					if _, err := db.Exec(`UPDATE Sample_Env SET PlotNumber=? WHERE PlotNumber='108050'`, plot); err != nil {
						t.Fatal(err)
					}
					if _, err := db.Exec(`UPDATE Sample_Admin SET Plot=? WHERE Plot='108050'`, plot); err != nil {
						t.Fatal(err)
					}
					if plot == "PlotNumber" {
						if _, err := service.projects.sqlite.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
							CREATE TEMP VIEW USysEnv AS SELECT * FROM project.Sample_Env`); err != nil {
							t.Fatal(err)
						}
					}
					original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, plot, form)
					if err != nil {
						t.Fatal(err)
					}
					table := quoteHeaderIdentifier(original.Table)
					if _, err := db.Exec(`INSERT INTO ` + table + `(ID,PlotNumber,Horizon) VALUES (999,'other','unrelated');
						ALTER TABLE ` + table + ` RENAME COLUMN PlotNumber TO WrongParent`); err != nil {
						t.Fatal(err)
					}
					before := databaseBytes(t, service.projects.sqlite.attachments)
					if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, plot, form); err == nil || got != nil ||
						!strings.Contains(err.Error(), "missing required physical parent column") {
						t.Fatalf("missing soil parent produced empty success or unrelated rows: %v, %v", got, err)
					}
					if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
						t.Fatal("missing-parent refusal changed an owned database")
					}
					if _, err := db.Exec(`ALTER TABLE ` + table + ` RENAME COLUMN WrongParent TO PlotNumber`); err != nil {
						t.Fatal(err)
					}
					if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, plot, form); err != nil || got == nil ||
						len(got.Records.Rows) != 0 {
						t.Fatalf("corrected parent schema did not recover an exact empty scope: %v, %v", got, err)
					}
				})
			}
		}
	}
	for _, parent := range []struct{ table, column string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		t.Run("physical-"+parent.table, func(t *testing.T) {
			service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
			table := quoteHeaderIdentifier("Sample_" + parent.table)
			if _, err := db.Exec(`DROP TABLE ` + table + `; CREATE TABLE ` + table + `(WrongParent TEXT)`); err != nil {
				t.Fatal(err)
			}
			for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
				if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", form); err == nil || got != nil ||
					!strings.Contains(err.Error(), "missing required physical parent column") {
					t.Fatalf("missing physical %s parent column was accepted: %v, %v", parent.table, got, err)
				}
			}
		})
	}
}

func TestTwoPageSoilOriginalRejectsMalformedTextAndAllowsCorrection(t *testing.T) {
	service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Humus(ID,PlotNumber,Comment) VALUES (NULL,'108050',CAST(X'FF' AS TEXT))`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil {
		t.Fatalf("malformed source TEXT crossed tagged/JSON boundary: %v, %v", got, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Humus SET Comment=''`); err != nil {
		t.Fatal(err)
	}
	got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL")
	if err != nil || len(got.Records.Rows) != 1 || got.Records.Rows[0].Cells[4].Text == nil ||
		*got.Records.Rows[0].Cells[4].Text != "" || got.Records.Rows[0].Cells[0].Storage != "null" {
		t.Fatalf("corrected empty text/nullable ID did not remain distinct: %v, %v", got, err)
	}
}

func TestTwoPageSoilOriginalRejectsHiddenOrShadowedPhysicalColumns(t *testing.T) {
	for _, column := range []string{
		`Generated TEXT GENERATED ALWAYS AS ('source drift') VIRTUAL`,
		`rowid TEXT`, `_rowid_ INTEGER`, `oid TEXT`,
	} {
		t.Run(column, func(t *testing.T) {
			service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
			if _, err := db.Exec(`ALTER TABLE Sample_Humus ADD COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
			if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil ||
				!strings.Contains(err.Error(), "ordinary columns") {
				t.Fatalf("omitted generated field or shadowed physical identity accepted: %v, %v", got, err)
			}
		})
	}
}

func TestTwoPageSoilOriginalRequiresPhysicalParents(t *testing.T) {
	service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
	if _, err := db.Exec(`ALTER TABLE Sample_Admin RENAME TO Archived_Admin;
		CREATE VIEW Sample_Admin AS SELECT * FROM Archived_Admin`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil ||
		!strings.Contains(err.Error(), "original physical project table") {
		t.Fatalf("view impersonated physical source parent: %v, %v", got, err)
	}
}

func TestTwoPageSoilOriginalAfterActualContextSwitch(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
			if _, err := db.Exec(`INSERT INTO Sample_Humus(ID,PlotNumber,Horizon) VALUES (NULL,'108050','')`); err != nil {
				t.Fatal(err)
			}
			original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL")
			if err != nil {
				t.Fatal(err)
			}
			selection := service.projects.sqlite.selection
			request := ContextSelection{Project: selection.Project, ProjectPath: selection.ProjectPath,
				SU: selection.SU, SUPath: selection.SUPath, Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath}
			next, err := service.SwitchContext(state.ContextID, request)
			if err != nil || next.ContextID == state.ContextID {
				t.Fatalf("actual context did not rotate: %v, %v", next, err)
			}
			if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil {
				t.Fatalf("stale context inherited new read authority: %v, %v", got, err)
			}
			current, err := service.readTwoPageSoilOriginal(context.Background(), next.ContextID, "108050", "SoilHumusXL")
			if err != nil || current.ContextID != next.ContextID || !reflect.DeepEqual(current.Records, original.Records) {
				t.Fatalf("newly owned context changed physical source values: %v, %v", current, err)
			}
		})
	}
}

func TestTwoPageSoilOriginalRejectsDuplicatePhysicalParents(t *testing.T) {
	service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
	if _, err := db.Exec(`CREATE TABLE Duplicate_Env AS SELECT * FROM Sample_Env;
		INSERT INTO Duplicate_Env SELECT * FROM Sample_Env WHERE PlotNumber='108050';
		ALTER TABLE Sample_Env RENAME TO Archived_Env;
		ALTER TABLE Duplicate_Env RENAME TO Sample_Env`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || got != nil ||
		!strings.Contains(err.Error(), "one literal physical Env parent") {
		t.Fatalf("ambiguous physical parents yielded an owned source original: %v, %v", got, err)
	}
}

func TestTwoPageSoilOriginalInFlightCancellationRetainsCoordinator(t *testing.T) {
	service, state, _ := twoPageSoilReadFixture(t, false, "TEXT")
	owner := service.projects.sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("soil cancellation test requires the actual SQLite driver")
		}
		return conn.RegisterFunc("twoPageSoilReadBarrier", func(value any) any {
			once.Do(func() { close(entered) })
			<-release
			return value
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	var originalView string
	if err := owner.conn.QueryRowContext(context.Background(), `SELECT sql FROM sqlite_temp_master WHERE name='USysEnv'`).Scan(&originalView); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT twoPageSoilReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.readTwoPageSoilOriginal(ctx, state.ContextID, "108050", "SoilHumusXL")
		if got != nil {
			err = errors.Join(err, errors.New("cancelled physical read returned a source original"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("actual in-flight SQLite read did not reach the barrier")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight cancellation = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("actual in-flight SQLite read did not cancel")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv; `+
		strings.Replace(originalView, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatalf("caller-owned snapshot lost its pinned coordinator: %v", err)
	}
	if got, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err != nil || got == nil {
		t.Fatalf("read after cancellation lost context attachments/TEMP views: %v, %v", got, err)
	}
}
