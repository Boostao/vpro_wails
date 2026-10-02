package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

type VegetationSpeciesUpdate struct {
	ID       int    `json:"id"`
	Form     string `json:"form"`
	Expected string `json:"expected"`
	Value    string `json:"value"`
}

func (update *VegetationSpeciesUpdate) UnmarshalJSON(data []byte) error {
	type plain VegetationSpeciesUpdate
	if err := validateJSONTextProperties(data, map[string]string{
		"form": "Species source form", "expected": "Species expected", "value": "Species value",
	}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, property := range []string{"id", "form", "expected", "value"} {
		raw, present := properties[property]
		if !present || strings.TrimSpace(string(raw)) == "null" {
			return fmt.Errorf("species draft requires explicit non-NULL %s", property)
		}
	}
	return json.Unmarshal(data, (*plain)(update))
}

func vegetationSpeciesRowPredicate(form string) (string, error) {
	var columns []string
	switch form {
	case "SubVegAXL_BC":
		columns = []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"}
	case "SubVegAhtXL":
		columns = []string{"Cover1", "Height1", "Cover2", "Height2", "Cover3", "Height3", "TotalA", "Cover4", "Height4", "Cover5", "Height5", "TotalB"}
	case "SubVegCXL", "SubVegChtXL":
		columns = []string{"Cover6"}
	case "SubVegDXL":
		columns = []string{"Cover7", "Cover8", "Cover9"}
	default:
		return "", fmt.Errorf("vegetation species source form %q is unavailable", form)
	}
	for i, column := range columns {
		columns[i] = quoteHeaderIdentifier(column) + " IS NOT NULL"
	}
	return "(" + strings.Join(columns, " OR ") + ")", nil
}

func (s *PlotService) UpdateVegetationSpecies(plot string, updates []VegetationSpeciesUpdate) error {
	patches := make([]childRecordPatch, 0, len(updates))
	for _, update := range updates {
		predicate, err := vegetationSpeciesPredicate(update.Form)
		if err != nil {
			return err
		}
		rowPredicate, err := vegetationSpeciesRowPredicate(update.Form)
		if err != nil {
			return err
		}
		if !utf8.ValidString(update.Expected) {
			return fmt.Errorf("species expected contains malformed UTF-8")
		}
		patches = append(patches, childRecordPatch{
			kind: "Veg", id: update.ID,
			cells: map[string]childExpectedValue{"species": {update.Expected, update.Value, reflect.String}},
			validate: func(tx *sql.Tx, table, plot string) error {
				var visible, member bool
				if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM `+table+
					` WHERE PlotNumber=? AND ID=? AND `+rowPredicate+`)`, plot, update.ID).Scan(&visible); err != nil {
					return fmt.Errorf("species source row could not be checked: %w", err)
				}
				if !visible {
					return fmt.Errorf("vegetation row %d no longer belongs to %s; cancel and reload", update.ID, update.Form)
				}
				referenceQuery := tx.QueryRowContext
				s.projects.mu.RLock()
				coordinator := s.projects.sqlite
				s.projects.mu.RUnlock()
				if coordinator != nil {
					// The context lease owns readonly family aliases, not the project writer pool.
					referenceQuery = coordinator.conn.QueryRowContext
				}
				if err := referenceQuery(s.operationContext(), `SELECT EXISTS(SELECT 1 FROM (`+vegetationSpeciesUnion+
					`) WHERE `+predicate+` AND Code COLLATE BINARY=?)`, update.Value).Scan(&member); err != nil {
					return fmt.Errorf("species references unavailable: %w", err)
				}
				if !member {
					return fmt.Errorf("species %q is not an exact code in %s; old-code decisions and personal-list creation are not yet available", update.Value, update.Form)
				}
				return nil
			},
		})
	}
	return s.updateChildPatches(plot, patches)
}

func (s *ContextService) UpdateVegetationSpecies(contextID, plot string, updates []VegetationSpeciesUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateVegetationSpecies(plot, updates) })
}
