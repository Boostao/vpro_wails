package main

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type childExpectedValue struct {
	expected, value any
	scalar          reflect.Kind
}

type childRecordPatch struct {
	kind  string
	id    int
	cells map[string]childExpectedValue
}

func childPatchShape(kind string) reflect.Value {
	switch kind {
	case "Humus":
		return reflect.ValueOf(HumusRecord{})
	case "Mineral":
		return reflect.ValueOf(MineralRecord{})
	default:
		return reflect.ValueOf(OtherRecord{})
	}
}

func (s *PlotService) updateChildPatches(plot string, updates []childRecordPatch) error {
	if err := s.requireContextEdit(); err != nil {
		return err
	}
	if strings.TrimSpace(plot) == "" || len(updates) == 0 {
		return errors.New("child drafts require a plot and at least one row")
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
	type identity struct {
		kind string
		id   int
	}
	seen := map[identity]bool{}
	capabilities := map[string]map[string]bool{}
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, update := range updates {
		if update.kind != "Other" && update.kind != "Humus" && update.kind != "Mineral" {
			return fmt.Errorf("unsupported child draft kind %q", update.kind)
		}
		key := identity{update.kind, update.id}
		if update.id < math.MinInt32 || update.id > math.MaxInt32 || seen[key] {
			return errors.New("child drafts require distinct exact signed32 identities per table")
		}
		seen[key] = true
		caps, available := capabilities[update.kind]
		if !available {
			caps, err = childCapabilities(tx, project, update.kind)
			if err != nil {
				return err
			}
			capabilities[update.kind] = caps
		}
		table := quoteHeaderIdentifier(project + "_" + update.kind)
		if err := requireChildIdentity(tx, table, update.kind, plot, int64(update.id)); err != nil {
			return err
		}
		shape := childPatchShape(update.kind)
		var fields []childField
		var before, after, args []any
		var assignments []string
		supplied := 0
		for _, field := range childFields[update.kind] {
			property := childJSONKey(shape, field)
			change, present := update.cells[property]
			if !present {
				continue
			}
			supplied++
			member := shape.FieldByName(field.member)
			scalar := member.Type().Elem().Kind()
			if !caps[strings.ToLower(field.column)] || scalar != change.scalar {
				return fmt.Errorf("%s property %q is unavailable or has conflicting types", update.kind, property)
			}
			column := quoteHeaderIdentifier(field.column)
			var current any
			if err := tx.QueryRow(`SELECT `+headerReadColumn(column, member)+` FROM `+table+
				` WHERE "PlotNumber" = ? AND "ID" = ?`, plot, update.id).Scan(&current); err != nil {
				return err
			}
			if current != nil {
				switch scalar {
				case reflect.Bool, reflect.Int:
					number, ok := current.(int64)
					if !ok {
						return fmt.Errorf("%s.%s did not return a normalized integer", update.kind, field.column)
					}
					if scalar == reflect.Bool {
						current = number != 0
					} else {
						current = int(number)
					}
				case reflect.Float64:
					if number, integer := current.(int64); integer {
						current = float64(number)
					}
				}
			}
			if !reflect.DeepEqual(current, change.expected) {
				return fmt.Errorf("%s row %d.%s changed; cancel drafts and reload before editing", update.kind, update.id, field.column)
			}
			if reflect.DeepEqual(current, change.value) {
				continue
			}
			if err := validateChildField(update.kind, field.column, change.value); err != nil {
				return err
			}
			fields = append(fields, field)
			before, after = append(before, current), append(after, change.value)
			assignments = append(assignments, column+" = ?")
			args = append(args, headerStorageValue(change.value))
		}
		if supplied == 0 || supplied != len(update.cells) {
			return errors.New("child drafts require nonempty allowlisted property sets")
		}
		if len(fields) == 0 {
			continue
		}
		result, err := tx.Exec(`UPDATE `+table+` SET `+strings.Join(assignments, ",")+
			` WHERE "PlotNumber" = ? AND "ID" = ?`, append(args, plot, update.id)...)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return fmt.Errorf("%s mutation expected one row: affected=%d error=%v", update.kind, affected, err)
		}
		if err := auditChildFields(tx, project, update.kind, plot, int64(update.id), fields, before, after, user, strength, when); err != nil {
			return err
		}
	}
	return tx.Commit()
}
