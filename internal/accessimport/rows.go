package accessimport

import (
	"context"
	"fmt"
	"time"
)

// SQLiteRow validates and adapts a complete native row before any storage operation.
func SQLiteRow(ctx context.Context, columns []SourceColumn, row []any) ([]any, error) {
	plan, err := PlanColumns(ctx, columns)
	if err != nil {
		return nil, err
	}
	if len(row) != len(plan) {
		return nil, fmt.Errorf("Access import row has %d cells; observed schema requires %d", len(row), len(plan))
	}
	result := make([]any, len(row))
	for i, value := range row {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if value != nil && !matchesNativeType(plan[i].SourceType, value) {
			return nil, fmt.Errorf("Access import column %q requires native %s, not %T", plan[i].Name, plan[i].SourceType, value)
		}
		adapted, err := SQLiteValue(value)
		if err != nil {
			return nil, fmt.Errorf("Access import column %q: %w", plan[i].Name, err)
		}
		result[i] = adapted
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func matchesNativeType(sourceType string, value any) bool {
	switch sourceType {
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		_, ok := value.(int64)
		return ok
	case "real":
		_, ok := value.(float64)
		return ok
	case "decimal", "text", "guid":
		_, ok := value.(string)
		return ok
	case "datetime":
		_, ok := value.(time.Time)
		return ok
	case "binary":
		_, ok := value.([]byte)
		return ok
	default:
		return false
	}
}
