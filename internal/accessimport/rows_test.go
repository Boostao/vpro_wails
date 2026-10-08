package accessimport

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSQLiteRowKeepsCompleteNativeTypesAndDetachedValues(t *testing.T) {
	columns := []SourceColumn{
		{"flag", "boolean"}, {"integer", "integer"}, {"real", "real"}, {"money", "decimal"},
		{"text", "text"}, {"guid", "guid"}, {"date", "datetime"}, {"bytes", "binary"},
	}
	blob := []byte{0, 1, 255}
	source := []any{
		true, int64(math.MaxInt64), float64(2.25), "9007199254740993.0001",
		"  \x00\r\n\U0001f332  ", "0001", time.Date(2020, 2, 20, 0, 0, 0, 0, time.UTC), blob,
	}
	result, err := SQLiteRow(context.Background(), columns, source)
	expected := []any{
		int64(-1), int64(math.MaxInt64), float64(2.25), "9007199254740993.0001",
		"  \x00\r\n\U0001f332  ", "0001", "2020-02-20 00:00:00", []byte{0, 1, 255},
	}
	if err != nil || !reflect.DeepEqual(result, expected) {
		t.Fatal("complete native row changed", result, err)
	}
	source[0] = false
	blob[0] = 9
	result[7].([]byte)[1] = 7
	if result[0] != int64(-1) || result[7].([]byte)[0] != 0 || blob[1] != 1 {
		t.Fatal("complete row aliases mutable source")
	}
	nulls := make([]any, len(columns))
	result, err = SQLiteRow(context.Background(), columns, nulls)
	if err != nil || !reflect.DeepEqual(result, nulls) {
		t.Fatal("unknown source nullability silently imposed", result, err)
	}
}

func TestSQLiteRowRejectsTypeMismatchCardinalityAndMalformedValuesWithoutPartialOutput(t *testing.T) {
	for _, test := range []struct {
		kind  string
		value any
	}{
		{"boolean", int64(-1)}, {"integer", true}, {"integer", int(1)},
		{"real", int64(1)}, {"decimal", float64(1.25)}, {"text", []byte("text")},
		{"guid", int64(1)}, {"datetime", "2020-02-20"}, {"binary", ""},
		{"real", math.NaN()}, {"text", "\xff"},
		{"datetime", time.Date(2020, 2, 20, 0, 0, 0, 0, time.FixedZone("not-native", 0))},
	} {
		columns := []SourceColumn{{"first", "integer"}, {"rejected", test.kind}}
		row, err := SQLiteRow(context.Background(), columns, []any{int64(1), test.value})
		if err == nil || row != nil || !strings.Contains(err.Error(), `"rejected"`) {
			t.Fatal("invalid cell returned partial success or missing column identity", test, row, err)
		}
	}
	columns := []SourceColumn{{"one", "integer"}, {"two", "text"}}
	for _, source := range [][]any{nil, {}, {int64(1)}, {int64(1), "text", nil}} {
		row, err := SQLiteRow(context.Background(), columns, source)
		if err == nil || row != nil {
			t.Fatal("wrong cardinality returned partial row", source, row, err)
		}
	}
	for _, source := range [][]SourceColumn{
		nil, {{"same", "text"}, {"SAME", "text"}}, {{"bad", "unsupported"}},
	} {
		row, err := SQLiteRow(context.Background(), source, make([]any, len(source)))
		if err == nil || row != nil {
			t.Fatal("NULL cells bypassed malformed source schema", source, row, err)
		}
	}
}

func TestSQLiteRowRetainsNullAndEmptyTextBlobDistinctions(t *testing.T) {
	row, err := SQLiteRow(context.Background(),
		[]SourceColumn{{"null", "binary"}, {"empty", "binary"}, {"text", "text"}},
		[]any{nil, []byte(nil), ""})
	if err != nil || !reflect.DeepEqual(row, []any{nil, []byte{}, ""}) {
		t.Fatal("complete row lost NULL/empty storage distinctions", row, err)
	}
}

func TestSQLiteRowCancellationBeforeDuringAndAfterAdaptation(t *testing.T) {
	columns := []SourceColumn{{"one", "integer"}, {"two", "binary"}}
	// PlanColumns makes four checks; row conversion then checks each cell and completion.
	for _, checks := range []int{1, 5, 6, 7} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &cancellingColumnContext{Context: base, cancel: cancel, remaining: checks}
		row, err := SQLiteRow(ctx, columns, []any{int64(1), []byte{1}})
		cancel()
		if err != context.Canceled || row != nil {
			t.Fatal("row cancellation returned partial success", checks, row, err)
		}
	}
}
