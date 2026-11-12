package vprocore

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type ChildExpectedValue struct {
	Expected, Value any          `json:"-" yaml:"-"`
	Scalar          reflect.Kind `json:"-" yaml:"-"`
}

type ChildRecordPatch struct {
	Kind     string                              `json:"-" yaml:"-"`
	Id       int                                 `json:"-" yaml:"-"`
	Cells    map[string]ChildExpectedValue       `json:"-" yaml:"-"`
	Validate func(*sql.Tx, string, string) error `json:"-" yaml:"-"`
	Observe  func(*sql.Tx, string, string) error `json:"-" yaml:"-"`
}

type ChildPlotPatch struct {
	Plot  string           `json:"-" yaml:"-"`
	Patch ChildRecordPatch `json:"-" yaml:"-"`
}

func ChildPatchShape(kind string) reflect.Value {
	switch kind {
	case "Veg":
		return reflect.ValueOf(VegRecord{})
	case "Humus":
		return reflect.ValueOf(HumusRecord{})
	case "Mineral":
		return reflect.ValueOf(MineralRecord{})
	default:
		return reflect.ValueOf(OtherRecord{})
	}
}

func (s *PlotService) UpdateChildPatches(plot string, updates []ChildRecordPatch) error {
	patches := make([]ChildPlotPatch, len(updates))
	for i, update := range updates {
		patches[i] = ChildPlotPatch{plot, update}
	}
	return s.UpdateChildPlotPatches(patches)
}

func (s *PlotService) UpdateChildPlotPatches(updates []ChildPlotPatch) error {
	if err := s.RequireContextEdit(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return errors.New("child drafts require a plot and at least one row")
	}
	s.Mu.RLock()
	user, strength := s.CurrentUser, s.AuditStrength
	s.Mu.RUnlock()
	db, project, release, err := s.GetActiveDB()
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	type identity struct {
		kind string
		id   int
	}
	seen := map[identity]bool{}
	capabilities := map[string]map[string]bool{}
	parents := map[string]bool{}
	var observations []ChildPlotPatch
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, scoped := range updates {
		plot, update := scoped.Plot, scoped.Patch
		if strings.TrimSpace(plot) == "" {
			return errors.New("child drafts require a plot and at least one row")
		}
		if !parents[plot] {
			if err := ChildParent(tx, project, plot); err != nil {
				return err
			}
			parents[plot] = true
		}
		if update.Kind != "Veg" && update.Kind != "Other" && update.Kind != "Humus" && update.Kind != "Mineral" {
			return fmt.Errorf("unsupported child draft kind %q", update.Kind)
		}
		key := identity{update.Kind, update.Id}
		if update.Id < math.MinInt32 || update.Id > math.MaxInt32 || seen[key] {
			return errors.New("child drafts require distinct exact signed32 identities per table")
		}
		seen[key] = true
		caps, available := capabilities[update.Kind]
		if !available {
			caps, err = ChildCapabilities(tx, project, update.Kind)
			if err != nil {
				return err
			}
			capabilities[update.Kind] = caps
		}
		table := QuoteHeaderIdentifier(project + "_" + update.Kind)
		if err := RequireChildIdentity(tx, table, update.Kind, plot, int64(update.Id)); err != nil {
			return err
		}
		shape := ChildPatchShape(update.Kind)
		var fields []ChildField
		var before, after, args []any
		var assignments []string
		supplied := 0
		for _, field := range ChildFields[update.Kind] {
			property := ChildJSONKey(shape, field)
			change, present := update.Cells[property]
			if !present {
				continue
			}
			supplied++
			member := shape.FieldByName(field.Member)
			scalar := member.Kind()
			if scalar == reflect.Pointer {
				scalar = member.Type().Elem().Kind()
			}
			if !caps[strings.ToLower(field.Column)] || scalar != change.Scalar {
				return fmt.Errorf("%s property %q is unavailable or has conflicting types", update.Kind, property)
			}
			column := QuoteHeaderIdentifier(field.Column)
			var current any
			if err := tx.QueryRow(`SELECT `+headerReadColumn(column, member)+` FROM `+table+
				` WHERE "PlotNumber" = ? AND "ID" = ?`, plot, update.Id).Scan(&current); err != nil {
				return err
			}
			if current != nil {
				switch scalar {
				case reflect.Bool, reflect.Int:
					number, ok := current.(int64)
					if !ok {
						return fmt.Errorf("%s.%s did not return a normalized integer", update.Kind, field.Column)
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
			if !reflect.DeepEqual(current, change.Expected) {
				return fmt.Errorf("%s row %d.%s changed; cancel drafts and reload before editing", update.Kind, update.Id, field.Column)
			}
			if reflect.DeepEqual(current, change.Value) {
				continue
			}
			if err := validateChildField(update.Kind, field.Column, change.Value); err != nil {
				return err
			}
			fields = append(fields, field)
			before, after = append(before, current), append(after, change.Value)
			assignments = append(assignments, column+" = ?")
			args = append(args, HeaderStorageValue(change.Value))
		}
		if supplied == 0 || supplied != len(update.Cells) {
			return errors.New("child drafts require nonempty allowlisted property sets")
		}
		if len(fields) == 0 {
			continue
		}
		if update.Validate != nil {
			if err := update.Validate(tx, table, plot); err != nil {
				return err
			}
		}
		result, err := tx.Exec(`UPDATE `+table+` SET `+strings.Join(assignments, ",")+
			` WHERE "PlotNumber" = ? AND "ID" = ?`, append(args, plot, update.Id)...)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return fmt.Errorf("%s mutation expected one row: affected=%d error=%v", update.Kind, affected, err)
		}
		if update.Observe != nil {
			observations = append(observations, scoped)
		}
		if err := auditChildFields(tx, project, update.Kind, plot, int64(update.Id), fields, before, after, user, strength, when); err != nil {
			return err
		}
	}
	for _, scoped := range observations {
		table := QuoteHeaderIdentifier(project + "_" + scoped.Patch.Kind)
		if err := RequireChildIdentity(tx, table, scoped.Patch.Kind, scoped.Plot, int64(scoped.Patch.Id)); err != nil {
			return err
		}
		if err := scoped.Patch.Observe(tx, table, scoped.Plot); err != nil {
			return err
		}
	}
	return tx.Commit()
}
