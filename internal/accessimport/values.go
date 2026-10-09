package accessimport

import (
	"fmt"
	"math"
	"time"
	"unicode/utf8"
)

// SQLiteValue adapts go-mdbtools values without authorizing an import or inferring column affinity.
func SQLiteValue(value any) (any, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case bool:
		if value {
			return int64(-1), nil
		}
		return int64(0), nil
	case int64:
		return value, nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("Access import requires finite numeric source values")
		}
		return value, nil
	case string:
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("Access import text is not valid UTF-8; no decoder repair")
		}
		return value, nil
	case []byte:
		detached := make([]byte, len(value))
		copy(detached, value)
		return detached, nil
	case time.Time:
		if value.Location() != time.UTC || value.Year() < 1 || value.Year() > 9999 {
			return nil, fmt.Errorf("Access import dates require the reader's neutral UTC representation and supported calendar range")
		}
		return value.Format("2006-01-02 15:04:05.999999999"), nil
	default:
		return nil, fmt.Errorf("Access import does not support source value type %T", value)
	}
}
