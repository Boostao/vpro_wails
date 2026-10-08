package accessimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
)

type NativeRows interface {
	Next() bool
	Row() []any
	Err() error
	Close() error
}

type StagingLimits struct {
	MaxRows  int64
	MaxBytes int64
}

// StagedTable is a detached staging image, not a canonical project or publication.
type StagedTable struct {
	SourceName string
	Columns    []SQLiteColumn
	RowCount   int64
	SQLite     []byte
	SHA256     string
}

// PrepareTable takes ownership of rows, including on validation or cancellation failure.
func PrepareTable(ctx context.Context, name string, columns []SourceColumn, rows NativeRows, limits StagingLimits) (result StagedTable, resultErr error) {
	return prepareTable(ctx, name, columns, rows, limits, sha256.Sum256)
}

func prepareTable(ctx context.Context, name string, columns []SourceColumn, rows NativeRows, limits StagingLimits, imageDigest func([]byte) [sha256.Size]byte) (result StagedTable, resultErr error) {
	if rows == nil {
		return result, errors.New("Access staging requires an owned native row reader")
	}
	closed := false
	closeSource := func() error {
		if closed {
			return nil
		}
		closed = true
		if err := rows.Close(); err != nil {
			return fmt.Errorf("Access staging source close: %w", err)
		}
		return nil
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeSource())
		if resultErr != nil {
			result = StagedTable{}
		}
	}()
	if ctx == nil {
		return result, errors.New("Access staging requires a context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return result, errors.New("Access staging requires a literal valid source table name")
	}
	if limits.MaxRows <= 0 || limits.MaxBytes < 4*4096 || limits.MaxBytes > math.MaxInt32 {
		return result, errors.New("Access staging requires positive row and 16384..2147483647 byte limits")
	}
	if len(columns) > 256 {
		return result, errors.New("Access staging exceeds the supported 256 native columns")
	}
	columns = append([]SourceColumn(nil), columns...)
	plan, err := PlanColumns(ctx, columns)
	if err != nil {
		return result, err
	}
	err = withStagingConnection(ctx, func(conn *sql.Conn) (operationErr error) {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA max_page_count=%d", limits.MaxBytes/4096)); err != nil {
			return fmt.Errorf("Access staging page budget: %w", err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				operationErr = errors.Join(operationErr, fmt.Errorf("Access staging rollback: %w", err))
			}
		}()
		declarations, marks := make([]string, len(plan)), make([]string, len(plan)+1)
		for i, column := range plan {
			declarations[i] = fmt.Sprintf(`"c%d" %s`, i, column.Affinity)
		}
		for i := range marks {
			marks[i] = "?"
		}
		for _, statement := range []string{
			`CREATE TABLE stage_source(name TEXT)`,
			`CREATE TABLE stage_columns(ordinal INTEGER PRIMARY KEY,name TEXT,source_type TEXT,affinity TEXT)`,
			`CREATE TABLE stage_rows(ordinal INTEGER PRIMARY KEY,` + strings.Join(declarations, ",") + `)`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("Access staging schema: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stage_source VALUES(?)`, name); err != nil {
			return err
		}
		for i, column := range plan {
			if _, err := tx.ExecContext(ctx, `INSERT INTO stage_columns VALUES(?,?,?,?)`, i, column.Name, column.SourceType, column.Affinity); err != nil {
				return err
			}
		}
		insert, err := tx.PrepareContext(ctx, `INSERT INTO stage_rows VALUES(`+strings.Join(marks, ",")+`)`)
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, insert.Close()) }()
		expected := sha256.New()
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !rows.Next() {
				break
			}
			if result.RowCount == limits.MaxRows {
				return errors.New("Access staging exceeds the explicit row budget")
			}
			values, err := SQLiteRow(ctx, columns, rows.Row())
			if err != nil {
				return fmt.Errorf("Access staging row %d: %w", result.RowCount+1, err)
			}
			if err := stagingRowDigest(expected, values); err != nil {
				return err
			}
			parameters := append([]any{result.RowCount + 1}, values...)
			if _, err := insert.ExecContext(ctx, parameters...); err != nil {
				return fmt.Errorf("Access staging row %d storage: %w", result.RowCount+1, err)
			}
			result.RowCount++
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("Access staging source read: %w", err)
		}
		if err := closeSource(); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := insert.Close(); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if err := conn.Raw(func(driverConn any) error {
			serializer, ok := driverConn.(interface{ Serialize(string) ([]byte, error) })
			if !ok {
				return errors.New("Access staging SQLite driver does not support serialization")
			}
			result.SQLite, err = serializer.Serialize("main")
			return err
		}); err != nil {
			return fmt.Errorf("Access staging serialization: %w", err)
		}
		if int64(len(result.SQLite)) > limits.MaxBytes {
			return errors.New("Access staging image exceeds the explicit byte budget")
		}
		result.SourceName, result.Columns = name, plan
		return verifyStagedTable(ctx, result, expected.Sum(nil))
	})
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	digest := imageDigest(result.SQLite)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.SHA256 = hex.EncodeToString(digest[:])
	return result, nil
}

func withStagingConnection(ctx context.Context, run func(*sql.Conn) error) (resultErr error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
	if _, err := conn.ExecContext(ctx, `PRAGMA page_size=4096`); err != nil {
		return err
	}
	return run(conn)
}

func verifyStagedTable(ctx context.Context, staged StagedTable, expected []byte) error {
	return withStagingConnection(ctx, func(conn *sql.Conn) (resultErr error) {
		if err := conn.Raw(func(driverConn any) error {
			deserializer, ok := driverConn.(interface{ Deserialize([]byte, string) error })
			if !ok {
				return errors.New("Access staging SQLite driver does not support deserialization")
			}
			return deserializer.Deserialize(staged.SQLite, "main")
		}); err != nil {
			return fmt.Errorf("Access staging independent image open: %w", err)
		}
		var name string
		var sources, objects int
		if err := conn.QueryRowContext(ctx, `SELECT count(*),min(name) FROM stage_source`).Scan(&sources, &name); err != nil {
			return err
		}
		if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema`).Scan(&objects); err != nil {
			return err
		}
		if sources != 1 || name != staged.SourceName || objects != 3 {
			return errors.New("Access staging image source/schema identity differs")
		}
		mapping, err := conn.QueryContext(ctx, `SELECT ordinal,name,source_type,affinity FROM stage_columns ORDER BY ordinal`)
		if err != nil {
			return err
		}
		count := 0
		for mapping.Next() {
			var ordinal int
			var column SQLiteColumn
			if err := mapping.Scan(&ordinal, &column.Name, &column.SourceType, &column.Affinity); err != nil {
				return errors.Join(err, mapping.Close())
			}
			if count >= len(staged.Columns) || ordinal != count || column != staged.Columns[count] {
				return errors.Join(errors.New("Access staging image column mapping differs"), mapping.Close())
			}
			count++
		}
		if err := errors.Join(mapping.Err(), mapping.Close()); err != nil {
			return err
		}
		if count != len(staged.Columns) {
			return errors.New("Access staging image column count differs")
		}
		schema, err := conn.QueryContext(ctx, `PRAGMA table_info(stage_rows)`)
		if err != nil {
			return err
		}
		count = 0
		for schema.Next() {
			var ordinal, required, primary int
			var name, affinity string
			var defaultValue any
			if err := schema.Scan(&ordinal, &name, &affinity, &required, &defaultValue, &primary); err != nil {
				return errors.Join(err, schema.Close())
			}
			expectedName, expectedAffinity, expectedPrimary := "ordinal", "INTEGER", 1
			if count > 0 && count <= len(staged.Columns) {
				expectedName = fmt.Sprintf("c%d", count-1)
				expectedAffinity, expectedPrimary = string(staged.Columns[count-1].Affinity), 0
			}
			if count > len(staged.Columns) || ordinal != count || name != expectedName ||
				affinity != expectedAffinity || primary != expectedPrimary || required != 0 || defaultValue != nil {
				return errors.Join(errors.New("Access staging image storage schema differs"), schema.Close())
			}
			count++
		}
		if err := errors.Join(schema.Err(), schema.Close()); err != nil {
			return err
		}
		if count != len(staged.Columns)+1 {
			return errors.New("Access staging image storage column count differs")
		}
		rows, err := conn.QueryContext(ctx, `SELECT * FROM stage_rows ORDER BY ordinal`)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
		actual := sha256.New()
		var rowCount int64
		for rows.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}
			values := make([]any, len(staged.Columns))
			targets := make([]any, len(values)+1)
			var ordinal int64
			targets[0] = &ordinal
			for i := range values {
				targets[i+1] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				return err
			}
			rowCount++
			if ordinal != rowCount {
				return errors.New("Access staging image physical row order differs")
			}
			if err := stagingRowDigest(actual, values); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if rowCount != staged.RowCount || !bytes.Equal(actual.Sum(nil), expected) {
			return errors.New("Access staging image stored types/values/count differ from complete source rows")
		}
		return nil
	})
}

func stagingRowDigest(digest hash.Hash, values []any) error {
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], uint64(len(values)))
	digest.Write(number[:])
	for _, value := range values {
		var tag byte
		var payload []byte
		switch value := value.(type) {
		case nil:
			tag = 'n'
		case int64:
			tag = 'i'
			binary.BigEndian.PutUint64(number[:], uint64(value))
			payload = number[:]
		case float64:
			tag = 'r'
			binary.BigEndian.PutUint64(number[:], math.Float64bits(value))
			payload = number[:]
		case string:
			tag, payload = 't', []byte(value)
		case []byte:
			tag, payload = 'b', value
		default:
			return fmt.Errorf("Access staging has unsupported stored type %T", value)
		}
		digest.Write([]byte{tag})
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(payload)))
		digest.Write(length[:])
		digest.Write(payload)
	}
	return nil
}
