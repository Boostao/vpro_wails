package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mattn/go-sqlite3"
)

func summaryFieldListTransactionFixture(t *testing.T) (*sql.DB, string, ProjectMetadataTable) {
	t.Helper()
	db, path := summaryFieldListFixture(t)
	if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList VALUES
		(1,'Env','duplicate','A',0,'sum','F1'),
		(1,'Env','duplicate','A',0,'sum','F1'),
		(-1,'Admin','hidden','',-1,NULL,'H'),
		(NULL,NULL,'NULL order',NULL,-1,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	table, err := summaryFieldListRead(t, db, t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return db, path, table
}

func TestSummaryVegetationFieldListTransactionCallerCommitAndIndependentReopen(t *testing.T) {
	db, path, original := summaryFieldListTransactionFixture(t)
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, applyErr := applySummaryVegetationFieldList(t.Context(), tx, original,
		[]summaryVegetationFieldListAction{{Kind: "all", Selected: true}, {Kind: "row", RowID: "2"}})
	if applyErr != nil {
		t.Fatal(errors.Join(applyErr, tx.Rollback()))
	}
	if len(plan.Assignments) != 1 || plan.Assignments[0].Before.RowID != "1" {
		t.Fatal("Report-only physical assignment plan differs", plan)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	independent, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := independent.Query(`SELECT rowid,"Order",Source,Label,Item,Report,FunctionName,FieldNumber
		FROM USysShortVegSummaryFieldList ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]any{
		{int64(1), int64(1), "Env", "duplicate", "A", int64(-1), "sum", "F1"},
		{int64(2), int64(1), "Env", "duplicate", "A", int64(0), "sum", "F1"},
		{int64(3), int64(-1), "Admin", "hidden", "", int64(-1), nil, "H"},
		{int64(4), nil, nil, "NULL order", nil, int64(-1), nil, nil},
	}
	count := 0
	for rows.Next() {
		values := make([]any, 8)
		dest := make([]any, 8)
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		if count >= len(want) || !reflect.DeepEqual(values, want[count]) {
			t.Fatal("independently reopened complete physical rows differ", count, values)
		}
		count++
	}
	if err := errors.Join(rows.Err(), rows.Close(), independent.Close()); err != nil || count != 4 {
		t.Fatal("independent reopen incomplete", count, err)
	}
}

func TestSummaryVegetationFieldListTransactionCallerRollbackAndNoop(t *testing.T) {
	db, _, original := summaryFieldListTransactionFixture(t)
	for _, action := range []summaryVegetationFieldListAction{{Kind: "all", Selected: true}, {Kind: "none"}} {
		tx, err := db.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan, err := applySummaryVegetationFieldList(t.Context(), tx, original,
			[]summaryVegetationFieldListAction{action}); err != nil || plan == nil {
			t.Fatal("caller-owned apply failed", plan, err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal("primitive committed before caller authorization", err)
		}
		observed, err := summaryFieldListRead(t, db, t.Context())
		if err != nil || !reflect.DeepEqual(observed, original) {
			t.Fatal("caller rollback did not restore all seven fields", observed, original, err)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER VPro64.block_any_assignment BEFORE UPDATE ON USysShortVegSummaryFieldList
		BEGIN SELECT RAISE(ABORT,'no-op must not write'); END`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, applyErr := applySummaryVegetationFieldList(t.Context(), tx, original,
		[]summaryVegetationFieldListAction{{Kind: "row", RowID: "1", Selected: true}, {Kind: "row", RowID: "1"}})
	if err := errors.Join(applyErr, tx.Rollback()); err != nil || len(plan.Assignments) != 0 {
		t.Fatal("net unchanged draft wrote a transient mutation", plan, err)
	}
}

func TestSummaryVegetationFieldListTransactionFailuresRollbackAndRetry(t *testing.T) {
	for _, ddl := range []string{
		`CREATE TRIGGER VPro64.reject_second BEFORE UPDATE ON USysShortVegSummaryFieldList WHEN OLD.rowid=2
			BEGIN SELECT RAISE(ABORT,'second assignment denied'); END`,
		`CREATE TRIGGER VPro64.ignore_second BEFORE UPDATE ON USysShortVegSummaryFieldList WHEN OLD.rowid=2
			BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER VPro64.corrupt_peer AFTER UPDATE ON USysShortVegSummaryFieldList WHEN OLD.rowid=1
			BEGIN UPDATE USysShortVegSummaryFieldList SET Source='trigger peer drift' WHERE rowid=3; END`,
	} {
		t.Run(strings.Fields(ddl)[2], func(t *testing.T) {
			db, _, original := summaryFieldListTransactionFixture(t)
			if _, err := db.Exec(ddl); err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			got, applyErr := applySummaryVegetationFieldList(t.Context(), tx, original,
				[]summaryVegetationFieldListAction{{Kind: "all", Selected: true}})
			if got != nil || applyErr == nil {
				t.Fatal("SQL suppression/rejection/peer drift returned success-shaped plan", got, applyErr)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			after, err := summaryFieldListRead(t, db, t.Context())
			if err != nil || !reflect.DeepEqual(after, original) {
				t.Fatal("failed partial assignments survived caller rollback", after, original, err)
			}
			if _, err := db.Exec("DROP TRIGGER " + strings.Fields(ddl)[2]); err != nil {
				t.Fatal(err)
			}
			tx, err = db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			plan, applyErr := applySummaryVegetationFieldList(t.Context(), tx, original,
				[]summaryVegetationFieldListAction{{Kind: "all", Selected: true}})
			if applyErr != nil {
				t.Fatal(errors.Join(applyErr, tx.Rollback()))
			}
			if err := tx.Commit(); err != nil || len(plan.Assignments) != 2 {
				t.Fatal("corrected source could not retry exact plan", plan, err)
			}
		})
	}
}

func TestSummaryVegetationFieldListTransactionStaleAndInflightCancellation(t *testing.T) {
	db, _, original := summaryFieldListTransactionFixture(t)
	if _, err := db.Exec(`UPDATE VPro64.USysShortVegSummaryFieldList SET Label='changed hidden row' WHERE rowid=4`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, applyErr := applySummaryVegetationFieldList(t.Context(), tx, original,
		[]summaryVegetationFieldListAction{{Kind: "all", Selected: true}})
	if got != nil || applyErr == nil || !strings.Contains(applyErr.Error(), "changed since review") {
		t.Fatal("stale full physical peer witness mutated rows", got, applyErr)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE VPro64.USysShortVegSummaryFieldList SET Label='NULL order' WHERE rowid=4`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Raw(func(driver any) error {
		sqlite, valid := driver.(*sqlite3.SQLiteConn)
		if !valid {
			return errors.New("fixture requires actual SQLite connection")
		}
		return sqlite.RegisterFunc("cancel_field_list", func() int { cancel(); return 0 }, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(t.Context(), `CREATE TRIGGER VPro64.cancel_first AFTER UPDATE ON USysShortVegSummaryFieldList
		WHEN OLD.rowid=1 BEGIN SELECT cancel_field_list(); END`); err != nil {
		t.Fatal(err)
	}
	tx, err = conn.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, applyErr = applySummaryVegetationFieldList(ctx, tx, original,
		[]summaryVegetationFieldListAction{{Kind: "all", Selected: true}})
	if got != nil || !errors.Is(applyErr, context.Canceled) {
		t.Fatal("in-flight cancellation after actual update accepted partial plan", got, applyErr)
	}
	if err := errors.Join(tx.Rollback(), conn.Close()); err != nil {
		t.Fatal(err)
	}
	after, err := summaryFieldListRead(t, db, t.Context())
	if err != nil || !reflect.DeepEqual(after, original) {
		t.Fatal("in-flight cancelled transaction lost attachments or retained writes", after, err)
	}
	if got, err := applySummaryVegetationFieldList(t.Context(), nil, original,
		[]summaryVegetationFieldListAction{}); got != nil || err == nil {
		t.Fatal("nil caller transaction accepted", got, err)
	}
}
