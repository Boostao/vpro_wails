package accessimport

import (
	"database/sql"
	"math"
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteValueActualStorageKeepsAccessBooleanAndNullEmptyKinds(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE imported(value)"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input   any
		storage string
		value   any
	}{
		{nil, "null", nil}, {true, "integer", int64(-1)}, {false, "integer", int64(0)},
		{"", "text", ""}, {[]byte(nil), "blob", []byte{}}, {[]byte{0, 255}, "blob", []byte{0, 255}},
		{"9007199254740993.0001", "text", "9007199254740993.0001"},
		{"  \x00\r\n\U0001f332  ", "text", "  \x00\r\n\U0001f332  "},
		{int64(math.MaxInt64), "integer", int64(math.MaxInt64)},
		{time.Date(2020, 2, 20, 0, 0, 0, 0, time.UTC), "text", "2020-02-20 00:00:00"},
	} {
		value, err := SQLiteValue(test.input)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO imported(value) VALUES(?)", value); err != nil {
			t.Fatal(err)
		}
		var actual any
		var storage string
		if err := db.QueryRow("SELECT typeof(value),value FROM imported ORDER BY rowid DESC LIMIT 1").Scan(&storage, &actual); err != nil {
			t.Fatal(err)
		}
		if storage != test.storage || !reflect.DeepEqual(actual, test.value) {
			t.Fatalf("actual SQLite storage changed: %T -> %s %#v; expected %s %#v", test.input, storage, actual, test.storage, test.value)
		}
	}
}

func TestSQLiteValuePreservesReaderStorageAndHistoricalLiterals(t *testing.T) {
	for _, test := range []struct{ original, expected any }{
		{nil, nil}, {true, int64(-1)}, {false, int64(0)},
		{int64(math.MinInt64), int64(math.MinInt64)}, {int64(math.MaxInt64), int64(math.MaxInt64)},
		{float64(123.25), float64(123.25)}, {"", ""}, {"001", "001"},
		{"9007199254740993.0001", "9007199254740993.0001"}, {"{ABC-DEF}", "{ABC-DEF}"},
		{"  exact\r\nNUL\x00Unicode \U0001f332  ", "  exact\r\nNUL\x00Unicode \U0001f332  "},
		{[]byte(nil), []byte{}}, {[]byte{}, []byte{}}, {[]byte{0, 255}, []byte{0, 255}},
		{time.Date(2020, 2, 20, 0, 0, 0, 0, time.UTC), "2020-02-20 00:00:00"},
		{time.Date(1899, 12, 29, 23, 59, 58, 123456789, time.UTC), "1899-12-29 23:59:58.123456789"},
	} {
		got, err := SQLiteValue(test.original)
		if err != nil || !reflect.DeepEqual(got, test.expected) {
			t.Fatalf("%T source changed: got %#v, expected %#v: %v", test.original, got, test.expected, err)
		}
	}
}

func TestSQLiteValuePreservesRealSignedZeroAndDetachesBinary(t *testing.T) {
	for _, original := range []float64{0, math.Copysign(0, -1)} {
		got, err := SQLiteValue(original)
		if err != nil || math.Signbit(got.(float64)) != math.Signbit(original) {
			t.Fatal("real signed zero changed", err)
		}
	}
	original := []byte{0, 1, 255}
	got, err := SQLiteValue(original)
	if err != nil {
		t.Fatal(err)
	}
	original[0] = 4
	got.([]byte)[1] = 5
	if got.([]byte)[0] != 0 || original[1] != 1 {
		t.Fatal("binary import value aliases source storage")
	}
}

func TestSQLiteValueRejectsUnsupportedOrRepairedValuesWithoutSuccessFallback(t *testing.T) {
	for _, original := range []any{
		"\xff", "\xed\xa0\x80", int(1), uint64(1), float32(1),
		math.NaN(), math.Inf(1), math.Inf(-1),
		time.Date(2020, 2, 20, 0, 0, 0, 0, time.FixedZone("offset", 3600)),
		time.Date(2020, 2, 20, 0, 0, 0, 0, time.FixedZone("not-reader-UTC", 0)),
		time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		struct{}{}, []string{"value"},
	} {
		got, err := SQLiteValue(original)
		if err == nil || got != nil {
			t.Fatalf("invalid %T returned success/partial value: %#v, %v", original, got, err)
		}
	}
}
