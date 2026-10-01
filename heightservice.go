package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

// HeightRecordUpdate patches existing vegetation values; nil explicitly means
// SQL NULL. Expected contains the same keys, using values returned by SQLite.
type HeightRecordUpdate struct {
	ID       int                 `json:"id"`
	Values   map[string]*float64 `json:"values"`
	Expected map[string]*float64 `json:"expected"`
}

func (update *HeightRecordUpdate) UnmarshalJSON(data []byte) error {
	type plain HeightRecordUpdate
	var decoded plain
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	id, exists := properties["id"]
	if !exists || strings.TrimSpace(string(id)) == "null" {
		return errors.New("height update requires an explicit non-NULL integer id (zero is valid)")
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := json.Unmarshal(id, &decoded.ID); err != nil {
		return err
	}
	*update = HeightRecordUpdate(decoded)
	return nil
}

var heightProperties = map[string]bool{
	"cover1": true, "cover2": true, "cover3": true, "cover4": true,
	"cover5": true, "cover6": true, "totalA": true, "totalB": true,
	"height1": true, "height2": true, "height3": true,
	"height4": true, "height5": true, "height6": true,
}

type heightPatch struct {
	id       int64
	fields   []childField
	values   []*float64
	expected []*float64
}

func sameHeightValue(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Float64bits(*a) == math.Float64bits(*b)
}

func heightNumber(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func copyHeightNumber(value *float64) *float64 {
	if value == nil {
		return nil
	}
	number := *value
	return &number
}

// Finite historical values outside the domain may be retained or cleared.
// Every new value keeps float64 precision within the native storage range.
func validateHeightNumericChange(property string, before, after *float64) error {
	if err := validateSingleRangeChange(property, before, after); err != nil {
		return err
	}
	if after == nil || sameHeightValue(before, after) {
		return nil
	}
	if !strings.HasPrefix(property, "height") && *after >= 100 {
		return fmt.Errorf("property %q must be less than 100 or NULL", property)
	}
	return nil
}

func prepareHeightUpdates(updates []HeightRecordUpdate) ([]heightPatch, error) {
	if len(updates) == 0 {
		return nil, errors.New("height updates must not be empty")
	}
	seen := make(map[int]bool, len(updates))
	patches := make([]heightPatch, 0, len(updates))
	for _, update := range updates {
		if seen[update.ID] {
			return nil, fmt.Errorf("duplicate height update ID %d in one plot batch", update.ID)
		}
		seen[update.ID] = true
		if len(update.Values) == 0 || len(update.Values) != len(update.Expected) {
			return nil, fmt.Errorf("height update ID %d requires nonempty matching values/expected keys", update.ID)
		}
		for property, value := range update.Values {
			expected, exists := update.Expected[property]
			if !heightProperties[property] || !exists {
				return nil, fmt.Errorf("height update ID %d has unsupported or unmatched property %q", update.ID, property)
			}
			if expected != nil && (math.IsNaN(*expected) || math.IsInf(*expected, 0)) {
				return nil, fmt.Errorf("height update ID %d property %q expected must be finite", update.ID, property)
			}
			if err := validateHeightNumericChange(property, expected, value); err != nil {
				return nil, fmt.Errorf("height update ID %d: %w", update.ID, err)
			}
		}
		patch := heightPatch{id: int64(update.ID)}
		for _, field := range childFields["Veg"] {
			property := childJSONKey(reflect.ValueOf(VegRecord{}), field)
			if value, exists := update.Values[property]; exists {
				patch.fields = append(patch.fields, field)
				patch.values = append(patch.values, copyHeightNumber(value))
				patch.expected = append(patch.expected, copyHeightNumber(update.Expected[property]))
			}
		}
		patches = append(patches, patch)
	}
	return patches, nil
}

// UpdateHeightRecords atomically patches one plot's existing rows and audits.
// It neither allocates/reserves IDs nor inserts/deletes vegetation records.
// Float64 precision is intentionally retained within the physical Single range.
func (s *PlotService) UpdateHeightRecords(plotNumber string, updates []HeightRecordUpdate) error {
	if err := s.requireContextEdit(); err != nil {
		return err
	}
	if strings.TrimSpace(plotNumber) == "" {
		return errors.New("PlotNumber is required")
	}
	patches, err := prepareHeightUpdates(updates)
	if err != nil {
		return err
	}
	db, project, err := s.getActiveDB()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := childParent(tx, project, plotNumber); err != nil {
		return err
	}
	caps, err := childCapabilities(tx, project, "Veg")
	if err != nil {
		return err
	}
	s.mu.RLock()
	user, strength := s.currentUser, s.auditStrength
	s.mu.RUnlock()
	when := time.Now().Format("2006-01-02 15:04:05")
	table := quoteHeaderIdentifier(project + "_Veg")
	for _, patch := range patches {
		if err := requireChildIdentity(tx, table, "Veg", plotNumber, patch.id); err != nil {
			return err
		}
		columns := make([]string, len(patch.fields))
		destinations := make([]any, len(patch.fields))
		for i, field := range patch.fields {
			if !caps[strings.ToLower(field.column)] {
				return fmt.Errorf("unsupported height property %q: active project is missing verified column %q",
					childJSONKey(reflect.ValueOf(VegRecord{}), field), field.column)
			}
			columns[i] = quoteHeaderIdentifier(field.column)
			destinations[i] = new(sql.NullFloat64)
		}
		if err := tx.QueryRow(`SELECT `+strings.Join(columns, ",")+` FROM `+table+
			` WHERE "PlotNumber" = ? AND "ID" = ?`, plotNumber, patch.id).Scan(destinations...); err != nil {
			return err
		}
		before, after := make([]any, len(patch.fields)), make([]any, len(patch.fields))
		var assignments []string
		var args []any
		for i, destination := range destinations {
			number := destination.(*sql.NullFloat64)
			var actual *float64
			if number.Valid {
				actual = &number.Float64
			}
			if !sameHeightValue(actual, patch.expected[i]) {
				return fmt.Errorf("height conflict for plot %q ID %d field %q: stored value changed; reload before saving",
					plotNumber, patch.id, patch.fields[i].column)
			}
			before[i], after[i] = heightNumber(actual), heightNumber(patch.values[i])
			if !sameHeightValue(actual, patch.values[i]) {
				assignments = append(assignments, columns[i]+` = ?`)
				args = append(args, after[i])
			}
		}
		if len(assignments) == 0 {
			continue
		}
		args = append(args, plotNumber, patch.id)
		result, err := tx.Exec(`UPDATE `+table+` SET `+strings.Join(assignments, ",")+` WHERE "PlotNumber" = ? AND "ID" = ?`, args...)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("height mutation for plot %q ID %d expected one row, found %d", plotNumber, patch.id, count)
		}
		if err := auditChildFields(tx, project, "Veg", plotNumber, patch.id, patch.fields, before, after, user, strength, when); err != nil {
			return err
		}
	}
	return tx.Commit()
}
