package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

var otherTextMaximum = map[string]int{
	"DataName": 50, "DataItem": 255, "UserItem1": 255, "UserItem2": 255, "UserItem3": 255,
}

var otherTextJSONNames = map[string]string{
	"dataname": "Other.DataName", "dataitem": "Other.DataItem",
	"useritem1": "Other.UserItem1", "useritem2": "Other.UserItem2", "useritem3": "Other.UserItem3",
}

func validateOtherField(column string, value any) error {
	if value == nil {
		return nil
	}
	if maximum, text := otherTextMaximum[column]; text {
		entry, ok := value.(string)
		if !ok || !utf8.ValidString(entry) {
			return fmt.Errorf("Other.%s requires nullable valid Unicode text", column)
		}
		if len(utf16.Encode([]rune(entry))) > maximum {
			return fmt.Errorf("Other.%s exceeds %d UTF-16 units", column, maximum)
		}
		return nil
	}
	switch column {
	case "UserFlag1", "UserFlag2", "UserFlag3":
		if _, ok := value.(bool); ok {
			return nil
		}
	}
	return fmt.Errorf("unsupported Other value for %q", column)
}

type OtherTextUpdate struct {
	Value    *string `json:"value"`
	Expected *string `json:"expected"`
}

func (change *OtherTextUpdate) UnmarshalJSON(data []byte) error {
	type plain OtherTextUpdate
	if err := validateJSONTextProperties(data, map[string]string{"value": "Other value", "expected": "Other expected"}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if _, ok := properties["value"]; !ok {
		return errors.New("Other text change requires explicit value and expected")
	}
	if _, ok := properties["expected"]; !ok {
		return errors.New("Other text change requires explicit value and expected")
	}
	return json.Unmarshal(data, (*plain)(change))
}

type OtherFlagUpdate struct {
	Value    *bool `json:"value"`
	Expected *bool `json:"expected"`
}

func (change *OtherFlagUpdate) UnmarshalJSON(data []byte) error {
	type plain OtherFlagUpdate
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if _, ok := properties["value"]; !ok {
		return errors.New("Other flag change requires explicit value and expected")
	}
	if _, ok := properties["expected"]; !ok {
		return errors.New("Other flag change requires explicit value and expected")
	}
	return json.Unmarshal(data, (*plain)(change))
}

type OtherRecordUpdate struct {
	ID    int                        `json:"id"`
	Text  map[string]OtherTextUpdate `json:"text"`
	Flags map[string]OtherFlagUpdate `json:"flags"`
}

func (update *OtherRecordUpdate) UnmarshalJSON(data []byte) error {
	type plain OtherRecordUpdate
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	raw, present := properties["id"]
	if !present || string(raw) == "null" {
		return errors.New("Other draft requires an explicit signed32 identity")
	}
	return json.Unmarshal(data, (*plain)(update))
}

func otherNullable[T string | bool](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}

// Patch only explicit cells; expected values prevent stale full-row replacement.
func (s *PlotService) UpdateOtherRecords(plot string, updates []OtherRecordUpdate) error {
	if err := s.requireContextEdit(); err != nil {
		return err
	}
	if strings.TrimSpace(plot) == "" || len(updates) == 0 {
		return errors.New("Other drafts require a plot and at least one row")
	}
	s.mu.RLock()
	user, strength := s.currentUser, s.auditStrength
	s.mu.RUnlock()
	db, project, release, err := s.getActiveDB()
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := childParent(tx, project, plot); err != nil {
		return err
	}
	caps, err := childCapabilities(tx, project, "Other")
	if err != nil {
		return err
	}
	table := quoteHeaderIdentifier(project + "_Other")
	seen := map[int]bool{}
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, update := range updates {
		if update.ID < math.MinInt32 || update.ID > math.MaxInt32 || seen[update.ID] {
			return errors.New("Other drafts require distinct exact signed32 identities")
		}
		seen[update.ID] = true
		if err := requireChildIdentity(tx, table, "Other", plot, int64(update.ID)); err != nil {
			return err
		}
		var fields []childField
		var before, after []any
		var assignments []string
		var args []any
		supplied := 0
		for _, field := range childFields["Other"] {
			property := childJSONKey(reflect.ValueOf(OtherRecord{}), field)
			text, hasText := update.Text[property]
			flag, hasFlag := update.Flags[property]
			if !hasText && !hasFlag {
				continue
			}
			supplied++
			if !caps[strings.ToLower(field.column)] || hasText == hasFlag {
				return fmt.Errorf("Other property %q is unavailable or has conflicting types", property)
			}
			var expected, value any
			if _, isText := otherTextMaximum[field.column]; isText {
				if !hasText {
					return fmt.Errorf("Other property %q requires a text change", property)
				}
				expected, value = otherNullable(text.Expected), otherNullable(text.Value)
			} else {
				if !hasFlag {
					return fmt.Errorf("Other property %q requires a flag change", property)
				}
				expected, value = otherNullable(flag.Expected), otherNullable(flag.Value)
			}
			column := quoteHeaderIdentifier(field.column)
			member := reflect.ValueOf(OtherRecord{}).FieldByName(field.member)
			var current any
			if err := tx.QueryRow(`SELECT `+headerReadColumn(column, member)+` FROM `+table+
				` WHERE "PlotNumber" = ? AND "ID" = ?`, plot, update.ID).Scan(&current); err != nil {
				return err
			}
			if current != nil && !hasText {
				number, ok := current.(int64)
				if !ok {
					return fmt.Errorf("Other.%s did not return normalized Access BOOLEAN", field.column)
				}
				current = number != 0
			}
			if !reflect.DeepEqual(current, expected) {
				return fmt.Errorf("Other row %d.%s changed; cancel drafts and reload before editing", update.ID, field.column)
			}
			if reflect.DeepEqual(current, value) {
				continue
			}
			if err := validateOtherField(field.column, value); err != nil {
				return err
			}
			fields = append(fields, field)
			before, after = append(before, current), append(after, value)
			assignments = append(assignments, column+" = ?")
			args = append(args, headerStorageValue(value))
		}
		if supplied == 0 || supplied != len(update.Text)+len(update.Flags) {
			return errors.New("Other drafts require nonempty allowlisted property sets")
		}
		if len(fields) == 0 {
			continue
		}
		result, err := tx.Exec(`UPDATE `+table+` SET `+strings.Join(assignments, ",")+
			` WHERE "PlotNumber" = ? AND "ID" = ?`, append(args, plot, update.ID)...)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return fmt.Errorf("Other mutation expected one row: affected=%d error=%v", affected, err)
		}
		if err := auditChildFields(tx, project, "Other", plot, int64(update.ID), fields, before, after, user, strength, when); err != nil {
			return err
		}
	}
	return tx.Commit()
}
