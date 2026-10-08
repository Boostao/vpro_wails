package accessimport

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

type stagingRows struct {
	values    [][]any
	index     int
	readErr   error
	closeErr  error
	closes    int
	nextCalls int
	onNext    func()
	onClose   func()
}

func (r *stagingRows) Next() bool {
	r.nextCalls++
	if r.onNext != nil {
		r.onNext()
	}
	if r.index == len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *stagingRows) Row() []any { return r.values[r.index-1] }
func (r *stagingRows) Err() error { return r.readErr }
func (r *stagingRows) Close() error {
	r.closes++
	if r.onClose != nil {
		r.onClose()
	}
	return r.closeErr
}

func stagingTestLimits() StagingLimits {
	return StagingLimits{MaxRows: 100, MaxBytes: 1024 * 1024}
}

func TestPrepareTableActualImagePreservesCompleteValuesMappingsAndPhysicalRows(t *testing.T) {
	columns := []SourceColumn{
		{"rowid", "boolean"}, {"_rowid_", "integer"}, {"oid", "real"}, {"money", "decimal"},
		{`quote"; DROP TABLE stage_rows; --`, "text"}, {"  guid  ", "guid"},
		{"date", "datetime"}, {"blob", "binary"}, {"Ä", "text"}, {"ä", "text"},
	}
	blob := []byte{0, 255}
	native := []any{true, int64(math.MaxInt64), 1.25, "9007199254740993.0001", "\x00 \U0001f332\r\n",
		"0001", time.Date(2023, 3, 15, 0, 0, 0, 123456633, time.UTC), blob, "", "  "}
	reader := &stagingRows{values: [][]any{native, native, make([]any, len(columns))}}
	staged, err := PrepareTable(context.Background(), `  source"; --  `, columns, reader, stagingTestLimits())
	if err != nil {
		t.Fatal(err)
	}
	if staged.RowCount != 3 || len(staged.Columns) != len(columns) || reader.closes != 1 {
		t.Fatal("incomplete result/closure", staged.RowCount, staged.Columns, reader.closes)
	}
	if !strings.HasPrefix(string(staged.SQLite), "SQLite format 3\x00") {
		t.Fatal("not an actual detached SQLite image")
	}
	digest := sha256.Sum256(staged.SQLite)
	if staged.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("image checksum differs")
	}
	err = withStagingConnection(context.Background(), func(conn *sql.Conn) (resultErr error) {
		if err := conn.Raw(func(driverConn any) error {
			return driverConn.(interface{ Deserialize([]byte, string) error }).Deserialize(staged.SQLite, "main")
		}); err != nil {
			return err
		}
		rows, err := conn.QueryContext(context.Background(), `SELECT ordinal,typeof(c0),c0,typeof(c1),c1,typeof(c2),c2,typeof(c3),c3,c4,c5,c6,typeof(c7),c7,c8,c9 FROM stage_rows ORDER BY ordinal`)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
		count := 0
		for rows.Next() {
			values := make([]any, 16)
			targets := make([]any, len(values))
			for i := range values {
				targets[i] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				return err
			}
			count++
			want := []any{int64(count), "integer", int64(-1), "integer", int64(math.MaxInt64),
				"real", 1.25, "text", "9007199254740993.0001", "\x00 \U0001f332\r\n",
				"0001", "2023-03-15 00:00:00.123456633", "blob", []byte{0, 255}, "", "  "}
			if count == 3 {
				want = []any{int64(3), "null", nil, "null", nil, "null", nil, "null", nil, nil, nil, nil, "null", nil, nil, nil}
			}
			if !reflect.DeepEqual(values, want) {
				t.Fatalf("independent actual image storage differs: %#v != %#v", values, want)
			}
		}
		if count != 3 {
			t.Fatal("duplicate physical rows collapsed", count)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	columns[0].Name, blob[0] = "caller mutation", 99
	if staged.Columns[0].Name != "rowid" {
		t.Fatal("staged metadata aliases caller")
	}
}

func TestPrepareTableEmptyTableAndNullEmptyKinds(t *testing.T) {
	for _, values := range [][][]any{nil, {{nil, []byte(nil), ""}}} {
		reader := &stagingRows{values: values}
		result, err := PrepareTable(context.Background(), "empty", []SourceColumn{
			{"null", "binary"}, {"blob", "binary"}, {"text", "text"},
		}, reader, stagingTestLimits())
		if err != nil || result.RowCount != int64(len(values)) || reader.closes != 1 {
			t.Fatal("empty table/NULL/empty changed", result, err, reader.closes)
		}
	}
}

func TestPrepareTableRejectsArgumentsRowsAndBudgetsWithoutPartialImage(t *testing.T) {
	for _, test := range []struct {
		name    string
		columns []SourceColumn
		values  [][]any
		limits  StagingLimits
	}{
		{"", []SourceColumn{{"one", "text"}}, nil, stagingTestLimits()},
		{"bad\x00name", []SourceColumn{{"one", "text"}}, nil, stagingTestLimits()},
		{"\xff", []SourceColumn{{"one", "text"}}, nil, stagingTestLimits()},
		{"ok", nil, nil, stagingTestLimits()},
		{"ok", []SourceColumn{{"same", "text"}, {"SAME", "text"}}, nil, stagingTestLimits()},
		{"ok", []SourceColumn{{"one", "unknown"}}, nil, stagingTestLimits()},
		{"ok", []SourceColumn{{"one", "boolean"}}, [][]any{{int64(-1)}}, stagingTestLimits()},
		{"ok", []SourceColumn{{"one", "text"}}, [][]any{{}}, stagingTestLimits()},
		{"ok", []SourceColumn{{"one", "text"}}, [][]any{{"ok"}, {"\xff"}}, stagingTestLimits()},
		{"ok", []SourceColumn{{"one", "real"}}, [][]any{{math.NaN()}}, stagingTestLimits()},
		{"ok", []SourceColumn{{"one", "text"}}, [][]any{{"one"}, {"two"}}, StagingLimits{MaxRows: 1, MaxBytes: 1 << 20}},
		{"ok", []SourceColumn{{"one", "text"}}, [][]any{{strings.Repeat("x", 50000)}}, StagingLimits{MaxRows: 1, MaxBytes: 16384}},
		{"ok", []SourceColumn{{"one", "text"}}, nil, StagingLimits{}},
		{"ok", []SourceColumn{{"one", "text"}}, nil, StagingLimits{MaxRows: 1, MaxBytes: math.MaxInt32 + 1}},
	} {
		reader := &stagingRows{values: test.values}
		result, err := PrepareTable(context.Background(), test.name, test.columns, reader, test.limits)
		if err == nil || !reflect.DeepEqual(result, StagedTable{}) || reader.closes != 1 {
			t.Fatal("rejected staging returned partial result/leaked reader", test.name, result, err, reader.closes)
		}
		retry := &stagingRows{values: [][]any{{"retry"}}}
		if result, err := PrepareTable(context.Background(), "retry", []SourceColumn{{"one", "text"}}, retry, stagingTestLimits()); err != nil || result.RowCount != 1 {
			t.Fatal("fresh owned retry failed", result, err)
		}
	}
}

func TestPrepareTableCancellationReadAndCloseErrorsBlockImage(t *testing.T) {
	readFailure, closeFailure := errors.New("source read failure"), errors.New("source close failure")
	for _, phase := range []string{"nil-context", "before", "during", "close", "read-error", "close-error", "both-errors"} {
		ctx, cancel := context.WithCancel(context.Background())
		reader := &stagingRows{values: [][]any{{"one"}, {"two"}}}
		var supplied context.Context = ctx
		switch phase {
		case "nil-context":
			supplied = nil
		case "before":
			cancel()
		case "during":
			reader.onNext = cancel
		case "close":
			reader.onClose = cancel
		case "read-error":
			reader.readErr = readFailure
		case "close-error":
			reader.closeErr = closeFailure
		case "both-errors":
			reader.readErr, reader.closeErr = readFailure, closeFailure
		}
		result, err := PrepareTable(supplied, "source", []SourceColumn{{"one", "text"}}, reader, stagingTestLimits())
		cancel()
		if err == nil || !reflect.DeepEqual(result, StagedTable{}) || reader.closes != 1 {
			t.Fatal(phase, result, err, reader.closes)
		}
		if (phase == "before" || phase == "during" || phase == "close") && !errors.Is(err, context.Canceled) {
			t.Fatal("lost cancellation identity", phase, err)
		}
		if (phase == "read-error" || phase == "both-errors") && !errors.Is(err, readFailure) {
			t.Fatal("lost read failure", err)
		}
		if (phase == "close-error" || phase == "both-errors") && !errors.Is(err, closeFailure) {
			t.Fatal("lost close failure", err)
		}
		if phase == "before" && reader.nextCalls != 0 {
			t.Fatal("cancelled call still read native source")
		}
	}
	if result, err := PrepareTable(context.Background(), "source", []SourceColumn{{"one", "text"}}, nil, stagingTestLimits()); err == nil || !reflect.DeepEqual(result, StagedTable{}) {
		t.Fatal("nil reader accepted", result, err)
	}
}

func TestPrepareTableRefusesSQLiteNegativeZeroRepair(t *testing.T) {
	result, err := PrepareTable(context.Background(), "source", []SourceColumn{{"one", "real"}},
		&stagingRows{values: [][]any{{math.Copysign(0, -1)}}}, stagingTestLimits())
	if err == nil || !strings.Contains(err.Error(), "stored types/values/count differ") || !reflect.DeepEqual(result, StagedTable{}) {
		t.Fatal("SQLite negative-zero normalization silently accepted", result, err)
	}
}

func TestPrepareTableCancellationDuringFinalImageHashClearsCompleteResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &stagingRows{values: [][]any{{"complete"}}}
	hashCalled := false
	result, err := prepareTable(ctx, "source", []SourceColumn{{"one", "text"}}, reader, stagingTestLimits(),
		func(image []byte) [sha256.Size]byte {
			hashCalled = true
			if reader.closes != 1 || len(image) == 0 {
				t.Fatal("hash preceded source closure/complete image")
			}
			cancel()
			return sha256.Sum256(image)
		})
	if !hashCalled || !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, StagedTable{}) || reader.closes != 1 {
		t.Fatal("final hashing cancellation returned success/partial image", hashCalled, result, err, reader.closes)
	}
}

func TestPrepareTableMeasuresExactImageAndNativeColumnBudgets(t *testing.T) {
	reader := &stagingRows{values: [][]any{{"one"}}}
	result, err := PrepareTable(context.Background(), "source", []SourceColumn{{"one", "text"}}, reader,
		StagingLimits{MaxRows: 1, MaxBytes: 16384})
	if err != nil || len(result.SQLite) != 16384 || result.RowCount != 1 {
		t.Fatal("exact image/row budget did not succeed", len(result.SQLite), result.RowCount, err)
	}
	for _, limit := range []int64{16383, 0, -1} {
		reader := &stagingRows{values: [][]any{{"one"}}}
		result, err := PrepareTable(context.Background(), "source", []SourceColumn{{"one", "text"}}, reader,
			StagingLimits{MaxRows: 1, MaxBytes: limit})
		if err == nil || !reflect.DeepEqual(result, StagedTable{}) || reader.closes != 1 || reader.nextCalls != 0 {
			t.Fatal("undersized byte budget accepted/read source", limit, result, err, reader.nextCalls)
		}
	}
	columns := make([]SourceColumn, 257)
	for i := range columns {
		columns[i] = SourceColumn{Name: string(rune(0x1000 + i)), Type: "text"}
	}
	for _, count := range []int{256, 257} {
		reader := &stagingRows{}
		result, err := PrepareTable(context.Background(), "source", columns[:count], reader, stagingTestLimits())
		if count == 256 {
			if err != nil || len(result.Columns) != 256 {
				t.Fatal("supported native column boundary rejected", len(result.Columns), err)
			}
		} else if err == nil || !reflect.DeepEqual(result, StagedTable{}) || reader.nextCalls != 0 {
			t.Fatal("unsupported native column count accepted/read source", result, err)
		}
		if reader.closes != 1 {
			t.Fatal("boundary reader not closed exactly once", reader.closes)
		}
	}
}

func TestVerifyStagedTableRejectsChangedMappingCountsSchemaAndImage(t *testing.T) {
	columns := []SourceColumn{{"one", "text"}}
	result, err := PrepareTable(context.Background(), "source", columns,
		&stagingRows{values: [][]any{{"exact"}}}, stagingTestLimits())
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.New()
	if err := stagingRowDigest(expected, []any{"exact"}); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*StagedTable){
		func(s *StagedTable) { s.SourceName = "different" },
		func(s *StagedTable) { s.RowCount++ },
		func(s *StagedTable) { s.Columns[0].Affinity = IntegerAffinity },
		func(s *StagedTable) { s.Columns[0].Name = "different" },
		func(s *StagedTable) { s.SQLite = []byte("not SQLite") },
	} {
		copy := result
		copy.Columns = append([]SQLiteColumn(nil), result.Columns...)
		mutate(&copy)
		if err := verifyStagedTable(context.Background(), copy, expected.Sum(nil)); err == nil {
			t.Fatal("changed image contract accepted", copy)
		}
	}
}

func TestPrepareTableConcurrentOwnersRemainIsolated(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			reader := &stagingRows{values: [][]any{{int64(i)}}}
			result, err := PrepareTable(context.Background(), t.Name(), []SourceColumn{{"one", "integer"}}, reader, stagingTestLimits())
			if err != nil || result.RowCount != 1 || result.SourceName != t.Name() || reader.closes != 1 {
				t.Fatal("owned staging crossed scopes", result, err, reader.closes)
			}
		})
	}
}

func TestVerifyStagedTableRefusesActualImageDrift(t *testing.T) {
	result, err := PrepareTable(context.Background(), "source", []SourceColumn{{"one", "text"}},
		&stagingRows{values: [][]any{{"exact"}}}, stagingTestLimits())
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.New()
	if err := stagingRowDigest(expected, []any{"exact"}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE stage_rows SET c0='changed'`,
		`DELETE FROM stage_rows`,
		`UPDATE stage_rows SET ordinal=2`,
		`INSERT INTO stage_rows VALUES(2,'exact')`,
		`UPDATE stage_columns SET ordinal=2`,
		`DELETE FROM stage_columns`,
		`INSERT INTO stage_source VALUES('source')`,
		`CREATE VIEW unexpected AS SELECT ordinal FROM stage_rows`,
		`ALTER TABLE stage_rows ADD COLUMN unexpected TEXT`,
		`DROP TABLE stage_rows; CREATE TABLE stage_rows(ordinal INTEGER PRIMARY KEY,c0 INTEGER); INSERT INTO stage_rows VALUES(1,'exact')`,
	} {
		copy := result
		err := withStagingConnection(context.Background(), func(conn *sql.Conn) error {
			if err := conn.Raw(func(raw any) error {
				return raw.(interface{ Deserialize([]byte, string) error }).Deserialize(result.SQLite, "main")
			}); err != nil {
				return err
			}
			if _, err := conn.ExecContext(context.Background(), statement); err != nil {
				return err
			}
			return conn.Raw(func(raw any) error {
				var err error
				copy.SQLite, err = raw.(interface{ Serialize(string) ([]byte, error) }).Serialize("main")
				return err
			})
		})
		if err != nil {
			t.Fatal("tamper fixture failed before verification", statement, err)
		}
		if err := verifyStagedTable(context.Background(), copy, expected.Sum(nil)); err == nil {
			t.Fatal("actual staging-image drift accepted", statement)
		}
	}
}
