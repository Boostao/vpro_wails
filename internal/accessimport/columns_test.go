package accessimport

import (
	"context"
	"database/sql"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlanColumnsActualSQLiteAffinitiesKeepNativeKinds(t *testing.T) {
	source := []SourceColumn{
		{"bool", "boolean"}, {"integer", "integer"}, {"real", "real"},
		{"money", "decimal"}, {"date", "datetime"}, {"literal", "text"},
		{"guid", "guid"}, {"binary", "binary"},
	}
	plan, err := PlanColumns(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	declarations, names, marks := make([]string, 0, len(plan)), make([]string, 0, len(plan)), make([]string, 0, len(plan))
	for i, column := range plan {
		if column.Name != source[i].Name || column.SourceType != source[i].Type {
			t.Fatal("source column order/name/type changed", plan)
		}
		name := `"` + strings.ReplaceAll(column.Name, `"`, `""`) + `"`
		declarations = append(declarations, name+" "+string(column.Affinity))
		names = append(names, name)
		marks = append(marks, "?")
	}
	if _, err := db.Exec("CREATE TABLE imported(" + strings.Join(declarations, ",") + ")"); err != nil {
		t.Fatal(err)
	}
	values := []any{
		true, int64(math.MinInt64), float64(1.25), "9007199254740993.0001",
		time.Date(2020, 2, 20, 0, 0, 0, 0, time.UTC), "00012", "001234", []byte{},
	}
	parameters := make([]any, len(values))
	for i, value := range values {
		parameters[i], err = SQLiteValue(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO imported VALUES("+strings.Join(marks, ",")+")", parameters...); err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		var storage string
		var actual any
		if err := db.QueryRow("SELECT typeof("+name+"),"+name+" FROM imported").Scan(&storage, &actual); err != nil {
			t.Fatal(err)
		}
		expectedStorage := []string{"integer", "integer", "real", "text", "text", "text", "text", "blob"}[i]
		if storage != expectedStorage || !reflect.DeepEqual(actual, parameters[i]) {
			t.Fatalf("%s affinity coerced native value: %s %#v, expected %s %#v", source[i].Name, storage, actual, expectedStorage, parameters[i])
		}
	}
}

func TestPlanColumnsKeepsLiteralNamesAndSQLiteAsciiCollisionRules(t *testing.T) {
	source := []SourceColumn{
		{"Ä", "text"}, {"ä", "text"}, {"  spaced  ", "text"},
		{`quote"; DROP TABLE imported; --`, "text"}, {"line\r\nname", "text"},
	}
	plan, err := PlanColumns(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	for i := range source {
		if plan[i].Name != source[i].Name || plan[i].Affinity != TextAffinity {
			t.Fatal("literal name changed", plan)
		}
	}
	source[0].Name = "changed source"
	plan[1].Name = "changed plan"
	if plan[0].Name != "Ä" || source[1].Name != "ä" {
		t.Fatal("column plan aliases mutable input")
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE imported("Ä" TEXT,"ä" TEXT)`); err != nil {
		t.Fatal("SQLite non-ASCII identifier behavior differs", err)
	}
	if _, err := db.Exec(`CREATE TABLE collision("Name" TEXT,"NAME" TEXT)`); err == nil {
		t.Fatal("SQLite ASCII collision evidence missing")
	}
}

func TestPlanColumnsRejectsMalformedUnknownAndCollidingSourceWithoutPartialPlan(t *testing.T) {
	for _, source := range [][]SourceColumn{
		nil, {}, {{"", "text"}}, {{"bad\x00name", "text"}}, {{"\xff", "text"}},
		{{"same", "text"}, {"SAME", "text"}}, {{"same", "text"}, {"same", "integer"}},
		{{"valid", "integer"}, {"unknown", "unsupported"}}, {{"raw", "TEXT"}},
		{{"raw", ""}}, {{"raw", "decimal "}},
	} {
		plan, err := PlanColumns(context.Background(), source)
		if err == nil || plan != nil {
			t.Fatal("invalid source returned successful/partial plan", source, plan, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if plan, err := PlanColumns(ctx, []SourceColumn{{"valid", "integer"}}); err != context.Canceled || plan != nil {
		t.Fatal("cancellation returned successful/partial plan", plan, err)
	}
}

type cancellingColumnContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (ctx *cancellingColumnContext) Err() error {
	ctx.remaining--
	if ctx.remaining == 0 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestPlanColumnsCancellationDuringAndAfterPlanningReturnsNoPartialPlan(t *testing.T) {
	source := []SourceColumn{{"first", "text"}, {"second", "decimal"}}
	for _, checks := range []int{2, 3, 4} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &cancellingColumnContext{Context: base, cancel: cancel, remaining: checks}
		plan, err := PlanColumns(ctx, source)
		cancel()
		if err != context.Canceled || plan != nil {
			t.Fatal("in-flight or final cancellation returned partial success", checks, plan, err)
		}
	}
}
