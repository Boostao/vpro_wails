package accessimport

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SourceColumn retains the ordered name and native type exposed by go-mdbtools.
type SourceColumn struct {
	Name string
	Type string
}

type SQLiteAffinity string

const (
	IntegerAffinity SQLiteAffinity = "INTEGER"
	RealAffinity    SQLiteAffinity = "REAL"
	TextAffinity    SQLiteAffinity = "TEXT"
	BlobAffinity    SQLiteAffinity = "BLOB"
)

type SQLiteColumn struct {
	Name       string
	SourceType string
	Affinity   SQLiteAffinity
}

// PlanColumns plans value-storage affinities, not original constraints or executable DDL.
func PlanColumns(ctx context.Context, source []SourceColumn) ([]SQLiteColumn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(source) == 0 {
		return nil, fmt.Errorf("Access import requires observed source columns")
	}
	result := make([]SQLiteColumn, 0, len(source))
	seen := make(map[string]bool, len(source))
	for _, column := range source {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if column.Name == "" || !utf8.ValidString(column.Name) || strings.ContainsRune(column.Name, 0) {
			return nil, fmt.Errorf("Access import column name is empty or malformed; no identifier repair")
		}
		// SQLite identifier comparison folds ASCII only, not Unicode case pairs.
		key := strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + ('a' - 'A')
			}
			return r
		}, column.Name)
		if seen[key] {
			return nil, fmt.Errorf("Access import columns collide in SQLite: %q", column.Name)
		}
		seen[key] = true
		var affinity SQLiteAffinity
		switch column.Type {
		case "boolean", "integer":
			affinity = IntegerAffinity
		case "real":
			affinity = RealAffinity
		case "decimal", "datetime", "text", "guid":
			affinity = TextAffinity
		case "binary":
			affinity = BlobAffinity
		default:
			return nil, fmt.Errorf("Access import column %q has unsupported native type %q", column.Name, column.Type)
		}
		result = append(result, SQLiteColumn{Name: column.Name, SourceType: column.Type, Affinity: affinity})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
