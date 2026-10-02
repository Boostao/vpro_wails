package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
)

type VegetationNumberUpdate struct {
	ID       int                 `json:"id"`
	Values   map[string]*float64 `json:"values"`
	Expected map[string]*float64 `json:"expected"`
	Forms    map[string]string   `json:"forms"`
}

func (update *VegetationNumberUpdate) UnmarshalJSON(data []byte) error {
	var numbers HeightRecordUpdate
	if err := numbers.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("vegetation numeric transport: %w", err)
	}
	var sources struct {
		Forms map[string]string `json:"forms"`
	}
	if err := json.Unmarshal(data, &sources); err != nil {
		return err
	}
	*update = VegetationNumberUpdate{numbers.ID, numbers.Values, numbers.Expected, sources.Forms}
	return nil
}

var vegetationNumberForms = map[string]map[string]bool{
	"SubVegAXL_BC": {"cover1": true, "cover2": true, "cover3": true, "totalA": true,
		"cover4": true, "cover5": true, "totalB": true},
	"SubVegCXL": {"cover6": true},
	"SubVegDXL": {"cover7": true, "cover8": true, "cover9": true},
	"SubVegAhtXL": {"cover1": true, "cover2": true, "cover3": true, "totalA": true,
		"cover4": true, "cover5": true, "totalB": true, "height1": true, "height2": true,
		"height3": true, "height4": true, "height5": true},
	"SubVegChtXL": {"cover6": true, "height6": true},
}

func (s *PlotService) UpdateVegetationNumbers(plot string, updates []VegetationNumberUpdate) error {
	patches := make([]childRecordPatch, 0, len(updates))
	for _, update := range updates {
		if len(update.Values) == 0 || len(update.Values) != len(update.Expected) || len(update.Values) != len(update.Forms) {
			return errors.New("vegetation numbers require nonempty matching values/expected/forms keys")
		}
		predicates := map[string]string{}
		patch := childRecordPatch{kind: "Veg", id: update.ID, cells: map[string]childExpectedValue{}}
		for property, value := range update.Values {
			expected, present := update.Expected[property]
			form := update.Forms[property]
			if !present || !vegetationNumberForms[form][property] {
				return fmt.Errorf("vegetation numeric property %q is unavailable in source form %q or has no expected value", property, form)
			}
			if expected != nil && (math.IsNaN(*expected) || math.IsInf(*expected, 0)) {
				return fmt.Errorf("vegetation numeric expected %q must be finite", property)
			}
			if err := validateHeightNumericChange(property, expected, value); err != nil {
				return err
			}
			predicate, err := vegetationSpeciesRowPredicate(form)
			if err != nil {
				return err
			}
			predicates[form] = predicate
			patch.cells[property] = childExpectedValue{heightNumber(expected), heightNumber(value), reflect.Float64}
		}
		patch.validate = func(tx *sql.Tx, table, plot string) error {
			for form, predicate := range predicates {
				var visible bool
				if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM `+table+
					` WHERE PlotNumber=? AND ID=? AND `+predicate+`)`, plot, update.ID).Scan(&visible); err != nil {
					return fmt.Errorf("vegetation numeric source row could not be checked: %w", err)
				}
				if !visible {
					return fmt.Errorf("vegetation row %d no longer belongs to %s; cancel and reload", update.ID, form)
				}
			}
			return nil
		}
		patches = append(patches, patch)
	}
	return s.updateChildPatches(plot, patches)
}

func (s *ContextService) UpdateVegetationNumbers(contextID, plot string, updates []VegetationNumberUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateVegetationNumbers(plot, updates) })
}
