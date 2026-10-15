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

func sourceAuditFixture(t *testing.T, external bool, affinity string) (*TwoPageSourceAuditService, ProjectState, *sql.DB) {
	t.Helper()
	contexts, state, db := twoPageSoilReadFixture(t, external, "TEXT")
	if _, err := db.Exec(`DROP TABLE IF EXISTS Sample_Audit;
		CREATE TABLE Sample_Audit(Project TEXT,User,PlotNumber ` + affinity + `,
		"Table",EditField,EditWhen,BeforeEdit,AfterEdit,Restore,Flag,ID)`); err != nil {
		t.Fatal(err)
	}
	service, err := NewTwoPageSourceAuditService(contexts, func(name string) (string, bool) {
		return "true", name == twoPageSourceAuditReviewEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

func TestTwoPageSourceAuditTypedRowsOrderAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, db := sourceAuditFixture(t, external, "TEXT")
			if _, err := db.Exec(`INSERT INTO Sample_Audit VALUES
				('Sample','historical','108050','_Env','Latitude','2020-02-01',NULL,'',-1,0,NULL),
				('Sample','','108050','_Veg','Species','2020-01-01','',NULL,0,-1,-2147483648),
				('Sample',NULL,'108050','_Veg','Species','2020-01-01',X'FF00',2.5,NULL,NULL,'01'),
				('Sample','outside','other','_Env','Latitude',NULL,NULL,NULL,0,0,NULL)`); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
				got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
				if err != nil {
					t.Fatal(err)
				}
				if got.ContextID != state.ContextID || got.Project != "Sample" || got.Plot != "108050" ||
					got.Form != form || got.RecordSource != "UsysAuditTrail" || got.Table != "Sample_Audit" {
					t.Fatalf("source ownership = %+v", got)
				}
				ids := []string{}
				for _, row := range got.Records.Rows {
					ids = append(ids, row.RowID)
				}
				if !reflect.DeepEqual(ids, []string{"2", "3", "1"}) {
					t.Fatalf("source ascending storage order/tie identity = %v", ids)
				}
				row := got.Records.Rows[2].Cells
				if row[6].Storage != "null" || !reflect.DeepEqual(row[7], metadataText("")) ||
					!reflect.DeepEqual(row[8], metadataInteger("-1")) || !reflect.DeepEqual(row[9], metadataInteger("0")) ||
					row[10].Storage != "null" {
					t.Fatalf("before/after/BOOLEAN/ID distinctions = %+v", row)
				}
				if !reflect.DeepEqual(got.Records.Rows[0].Cells[10], metadataInteger("-2147483648")) ||
					!reflect.DeepEqual(got.Records.Rows[1].Cells[10], metadataText("01")) ||
					got.Records.Rows[1].Cells[6].Storage != "blob" || *got.Records.Rows[1].Cells[6].BlobHex != "ff00" {
					t.Fatalf("historical tagged physical values = %+v", got.Records.Rows)
				}
			}
			if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
				t.Fatal("readonly audit changed physical database bytes")
			}
		})
	}
}

func TestTwoPageSourceAuditLiteralScopeAcrossAffinities(t *testing.T) {
	for _, affinity := range []string{"TEXT", "BLOB", "", "INTEGER", "NUMERIC", "REAL"} {
		t.Run(affinity, func(t *testing.T) {
			service, state, db := sourceAuditFixture(t, false, affinity)
			if _, err := db.Exec(`INSERT INTO Sample_Audit(Project,PlotNumber,EditWhen) VALUES
				('Sample','108050','a'),('Sample',108050,'b'),('Sample',CAST('108050' AS BLOB),'c')`); err != nil {
				t.Fatal(err)
			}
			got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]int{"TEXT": 2, "BLOB": 1, "": 1, "INTEGER": 0, "NUMERIC": 0, "REAL": 0}[affinity]
			if len(got.Records.Rows) != want {
				t.Fatalf("literal storage scope returned %d rows, want %d", len(got.Records.Rows), want)
			}
		})
	}
}

func TestTwoPageSourceAuditGuardsAndCorrection(t *testing.T) {
	for _, failure := range []string{"foreign", "malformed", "schema", "view", "hidden", "shadow"} {
		t.Run(failure, func(t *testing.T) {
			service, state, db := sourceAuditFixture(t, false, "TEXT")
			if _, err := db.Exec(`INSERT INTO Sample_Audit(Project,PlotNumber) VALUES('Sample','108050')`); err != nil {
				t.Fatal(err)
			}
			sqlText := map[string]string{
				"foreign":   `UPDATE Sample_Audit SET Project='Other'`,
				"malformed": `UPDATE Sample_Audit SET BeforeEdit=CAST(X'FF' AS TEXT)`,
				"schema":    `ALTER TABLE Sample_Audit RENAME COLUMN AfterEdit TO Unavailable`,
				"view":      `ALTER TABLE Sample_Audit RENAME TO Archived_Audit; CREATE VIEW Sample_Audit AS SELECT * FROM Archived_Audit`,
				"hidden":    `ALTER TABLE Sample_Audit ADD COLUMN Hidden TEXT GENERATED ALWAYS AS ('hidden') VIRTUAL`,
				"shadow":    `ALTER TABLE Sample_Audit ADD COLUMN rowid TEXT`,
			}[failure]
			if _, err := db.Exec(sqlText); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); err == nil || got != nil {
				t.Fatalf("invalid source crossed audit boundary: %+v, %v", got, err)
			}
			if failure == "malformed" {
				if _, err := db.Exec(`UPDATE Sample_Audit SET BeforeEdit=''`); err != nil {
					t.Fatal(err)
				}
				got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
				if err != nil || !reflect.DeepEqual(got.Records.Rows[0].Cells[6], metadataText("")) {
					t.Fatalf("corrected source cannot retry: %+v, %v", got, err)
				}
			}
		})
	}
	service, state, _ := sourceAuditFixture(t, false, "TEXT")
	for _, request := range []struct{ context, plot, form string }{
		{"stale", "108050", "FS882-8x6XL"}, {state.ContextID, "outside", "FS882-8x6XL"},
		{state.ContextID, "", "FS882-8x6XL"}, {state.ContextID, "\xff", "FS882-8x6XL"},
		{state.ContextID, "108050", "FS882-6x4XL"},
	} {
		if got, err := service.GetOriginal(context.Background(), request.context, request.plot, request.form); err == nil || got != nil {
			t.Fatalf("invalid request inherited read ownership: %+v, %v", got, err)
		}
	}
	service.review = false
	if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); err == nil || got != nil {
		t.Fatalf("disabled audit returned success: %+v, %v", got, err)
	}
	for _, value := range []string{"invalid", "false"} {
		got, err := NewTwoPageSourceAuditService(service.contexts, func(string) (string, bool) { return value, true })
		if value == "invalid" && err == nil || value == "false" && (err != nil || got.review) {
			t.Fatalf("feature parsing %q = %+v, %v", value, got, err)
		}
	}
}

func TestTwoPageSourceAuditInFlightCancellationRetainsOwnedCoordinator(t *testing.T) {
	service, state, _ := sourceAuditFixture(t, false, "TEXT")
	owner := service.contexts.projects.sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("audit cancellation fixture requires actual SQLite")
		}

		return conn.RegisterFunc("sourceAuditReadBarrier", func(value any) any {
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
		CREATE TEMP VIEW USysEnv AS SELECT sourceAuditReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.GetOriginal(ctx, state.ContextID, "108050", "FS882-8x6XL")
		if got != nil {
			err = errors.Join(err, errors.New("cancelled audit returned data"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("actual audit read did not enter SQLite barrier")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight cancellation = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("audit cancellation did not settle")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv; `+
		strings.Replace(originalView, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatalf("cancelled audit lost pinned coordinator: %v", err)
	}
	if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); err != nil || got == nil || len(got.Records.Rows) != 0 {
		t.Fatalf("same-context retry after actual cancellation = %+v, %v", got, err)
	}
}
