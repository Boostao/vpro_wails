package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const summaryFieldListTestSchema = `CREATE TABLE VPro64.USysShortVegSummaryFieldList (
	"Order" INTEGER, Source TEXT(255), Label TEXT(255), Item TEXT(255),
	Report BIT DEFAULT 0, FunctionName TEXT(255), FieldNumber TEXT(255))`

func summaryFieldListFixture(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "field list # ' Unicode.sqlite")
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(`ATTACH DATABASE ? AS VPro64`, sqliteFileURI(path, "rwc")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(summaryFieldListTestSchema); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func summaryFieldListRead(t *testing.T, db *sql.DB, ctx context.Context) (ProjectMetadataTable, error) {
	t.Helper()
	tx, err := db.BeginTx(context.WithoutCancel(ctx), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	table, err := readSummaryVegetationFieldList(ctx, tx)
	err = errors.Join(err, tx.Rollback())
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	return table, nil
}

func summaryFieldListExpected(t *testing.T, values []any) []ProjectMetadataCell {
	t.Helper()
	cells := make([]ProjectMetadataCell, 0, len(values))
	for _, raw := range values {
		switch value := raw.(type) {
		case nil:
			cells = append(cells, ProjectMetadataCell{Storage: "null"})
		case string:
			cells = append(cells, ProjectMetadataCell{Storage: "text", Text: &value})
		case int64:
			literal := strconv.FormatInt(value, 10)
			cells = append(cells, ProjectMetadataCell{Storage: "integer", Integer: &literal})
		default:
			t.Fatalf("unsupported independent expected fixture type %T", raw)
		}
	}
	return cells
}

func TestSummaryVegetationFieldListCompleteSourceRowsAndSeparateFilters(t *testing.T) {
	db, path := summaryFieldListFixture(t)
	seed := [][]any{
		{int64(1), "Env", "visible A", "X", int64(0), "sum", "F1"},
		{int64(1), "Env", "visible A", "X", int64(0), "sum", "F1"},
		{int64(2), "Admin", "visible B", nil, int64(-1), nil, "F2"},
		{int64(0), "Env", "hidden zero", "zero", int64(0), "sum", "H0"},
		{int64(-1), "Env", "hidden negative", "negative", int64(-1), "sum", "HN"},
		{nil, nil, "hidden NULL order", nil, int64(0), nil, "HNULL"},
		{int64(3), nil, nil, nil, int64(0), nil, nil},
		{int64(32767), "Env", "visible high", "high", int64(-1), "sum", "MAX"},
	}
	for _, row := range seed {
		if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList VALUES(?,?,?,?,?,?,?)`, row...); err != nil {
			t.Fatal(err)
		}
	}
	for _, selection := range []string{"original", "-1", "0", "-1"} {
		if selection != "original" {
			if _, err := db.Exec(`UPDATE VPro64.USysShortVegSummaryFieldList SET Report=` + selection); err != nil {
				t.Fatal(err)
			}
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		table, err := summaryFieldListRead(t, db, t.Context())
		if err != nil {
			t.Fatal(err)
		}
		wantColumns := []ProjectMetadataColumn{{"Order", "INTEGER"}, {"Source", "TEXT(255)"}, {"Label", "TEXT(255)"},
			{"Item", "TEXT(255)"}, {"Report", "BIT"}, {"FunctionName", "TEXT(255)"}, {"FieldNumber", "TEXT(255)"}}
		if !reflect.DeepEqual(table.Columns, wantColumns) || len(table.Rows) != 8 {
			t.Fatal("complete source shape changed", table)
		}
		for i, row := range seed {
			expected := append([]any(nil), row...)
			if selection == "-1" {
				expected[4] = int64(-1)
			} else if selection == "0" {
				expected[4] = int64(0)
			}
			if table.Rows[i].RowID != strconv.Itoa(i+1) || !reflect.DeepEqual(table.Rows[i].Cells, summaryFieldListExpected(t, expected)) {
				t.Fatal("full independent physical row changed", i, table.Rows[i], expected)
			}
		}
		index, err := indexSummaryVegetationFieldList(t.Context(), table)
		if err != nil {
			t.Fatal(err)
		}
		selected := []string{"5", "3", "8"}
		if selection == "-1" {
			selected = []string{"6", "5", "4", "1", "2", "3", "7", "8"}
		} else if selection == "0" {
			selected = []string{}
		}
		want := summaryVegetationFieldListIndex{Visible: []string{"1", "2", "3", "7", "8"}, Selected: selected}
		if !reflect.DeepEqual(index, want) {
			t.Fatal("source positive-order view was conflated with global report selection", selection, index, want)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("read/index mutated fixture bytes", err)
		}
	}
}

func TestSummaryVegetationFieldListPresentEmptyNullAndHistoricalLiteralCells(t *testing.T) {
	db, _ := summaryFieldListFixture(t)
	empty, err := summaryFieldListRead(t, db, t.Context())
	if err != nil || len(empty.Columns) != 7 || empty.Rows == nil || len(empty.Rows) != 0 {
		t.Fatal("present empty source unavailable or null-shaped", empty, err)
	}
	index, err := indexSummaryVegetationFieldList(t.Context(), empty)
	if err != nil || index.Visible == nil || index.Selected == nil {
		t.Fatal("present empty indexes null-shaped", index, err)
	}
	overlength := strings.Repeat("x", 256)
	seed := []any{nil, "", " literal\x00label ", overlength, nil, nil, ""}
	if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList VALUES(?,?,?,?,?,?,?)`, seed...); err != nil {
		t.Fatal(err)
	}
	table, err := summaryFieldListRead(t, db, t.Context())
	if err != nil || len(table.Rows) != 1 || !reflect.DeepEqual(table.Rows[0].Cells, summaryFieldListExpected(t, seed)) {
		t.Fatal("historical NULL/empty/NUL/overlength values were normalized", table, err)
	}
	index, err = indexSummaryVegetationFieldList(t.Context(), table)
	if err != nil || len(index.Visible) != 0 || len(index.Selected) != 0 {
		t.Fatal("NULL predicate invented membership", index, err)
	}
	*table.Rows[0].Cells[2].Text = "caller"
	again, err := summaryFieldListRead(t, db, t.Context())
	if err != nil || *again.Rows[0].Cells[2].Text != " literal\x00label " {
		t.Fatal("physical review shared caller-owned values", again, err)
	}
}

func TestSummaryVegetationFieldListMalformedTextCorrectionRetry(t *testing.T) {
	for _, column := range []string{"Source", "Label", "Item", "FunctionName", "FieldNumber"} {
		t.Run(column, func(t *testing.T) {
			db, _ := summaryFieldListFixture(t)
			if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList("Order",Report) VALUES(1,-1);
				UPDATE VPro64.USysShortVegSummaryFieldList SET ` + quoteHeaderIdentifier(column) + `=CAST(X'FF' AS TEXT)`); err != nil {
				t.Fatal(err)
			}
			table, err := summaryFieldListRead(t, db, t.Context())
			if err == nil || !strings.Contains(err.Error(), "malformed Unicode") || table.Rows != nil {
				t.Fatal("malformed TEXT repaired/skipped or partial review returned", table, err)
			}
			if _, err := db.Exec(`UPDATE VPro64.USysShortVegSummaryFieldList SET ` + quoteHeaderIdentifier(column) + `=''`); err != nil {
				t.Fatal(err)
			}
			table, err = summaryFieldListRead(t, db, t.Context())
			if err != nil || len(table.Rows) != 1 {
				t.Fatal("corrected empty TEXT could not retry physical read", table, err)
			}
		})
	}
}

func TestSummaryVegetationFieldListHistoricalStorageRemainsPhysicalAndIndexRefuses(t *testing.T) {
	for _, column := range []string{"Order", "Report"} {
		values := []string{"'not numeric'", "1.5", "X'FF'", "32768", "-32769"}
		if column == "Report" {
			values = append(values, "1", "2")
		}
		for _, value := range values {
			t.Run(column+"/"+value, func(t *testing.T) {
				db, _ := summaryFieldListFixture(t)
				if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList("Order",Report) VALUES(1,-1);
					UPDATE VPro64.USysShortVegSummaryFieldList SET ` + quoteHeaderIdentifier(column) + `=` + value); err != nil {
					t.Fatal(err)
				}
				table, err := summaryFieldListRead(t, db, t.Context())
				if err != nil || len(table.Rows) != 1 {
					t.Fatal("incompatible historical filter storage discarded physical rows", table, err)
				}
				original := table
				index, err := indexSummaryVegetationFieldList(t.Context(), table)
				if err == nil || !strings.Contains(err.Error(), column) || index.Visible != nil || index.Selected != nil {
					t.Fatal("source index coerced incompatible storage or returned partial success", index, err)
				}
				again, err := summaryFieldListRead(t, db, t.Context())
				if err != nil || !reflect.DeepEqual(again, original) {
					t.Fatal("failed index changed physical source", again, original, err)
				}
			})
		}
	}
}

func TestSummaryVegetationFieldListSchemaAndCancellationRefusals(t *testing.T) {
	for name, ddl := range map[string]string{
		"absent":    "",
		"view":      `CREATE VIEW VPro64.USysShortVegSummaryFieldList AS SELECT 1 AS "Order"`,
		"missing":   `CREATE TABLE VPro64.USysShortVegSummaryFieldList("Order" INTEGER)`,
		"extra":     summaryFieldListTestSchema + `; ALTER TABLE VPro64.USysShortVegSummaryFieldList ADD COLUMN Extra TEXT`,
		"generated": summaryFieldListTestSchema + `; ALTER TABLE VPro64.USysShortVegSummaryFieldList ADD COLUMN Extra AS (1)`,
		"rowid":     summaryFieldListTestSchema + `; ALTER TABLE VPro64.USysShortVegSummaryFieldList ADD COLUMN RoWiD INTEGER`,
		"without-rowid": `CREATE TABLE VPro64.USysShortVegSummaryFieldList("Order" INTEGER PRIMARY KEY,
			Source TEXT, Label TEXT, Item TEXT, Report INTEGER, FunctionName TEXT, FieldNumber TEXT) WITHOUT ROWID`,
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := summaryFieldListFixture(t)
			if _, err := db.Exec(`DROP TABLE VPro64.USysShortVegSummaryFieldList`); err != nil {
				t.Fatal(err)
			}
			if ddl != "" {
				if _, err := db.Exec(ddl); err != nil {
					t.Fatal(err)
				}
			}
			if table, err := summaryFieldListRead(t, db, t.Context()); err == nil || table.Rows != nil {
				t.Fatal("unsupported physical schema accepted", table, err)
			}
		})
	}
	db, _ := summaryFieldListFixture(t)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := summaryFieldListRead(t, db, cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read accepted", err)
	}
	table, err := summaryFieldListRead(t, db, t.Context())
	if err != nil {
		t.Fatal("read cancellation lost attachment", err)
	}
	if _, err := indexSummaryVegetationFieldList(cancelled, table); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled index accepted", err)
	}
}
