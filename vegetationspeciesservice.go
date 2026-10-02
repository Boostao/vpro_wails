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
	ID       int     `json:"id"`
	Form     string  `json:"form"`
	Expected string  `json:"expected"`
	Value    string  `json:"value"`
	Decision string  `json:"decision,omitempty"`
	Entered  *string `json:"entered,omitempty"`
	Selected *string `json:"selected,omitempty"`
}

func (update *VegetationSpeciesUpdate) UnmarshalJSON(data []byte) error {
	type plain VegetationSpeciesUpdate
	if err := validateJSONTextProperties(data, map[string]string{
		"form": "Species source form", "expected": "Species expected", "value": "Species value",
		"entered": "Species entered code", "selected": "Species selected code",
		"decision": "Species decision",
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
		if raw, present := properties["decision"]; present && strings.TrimSpace(string(raw)) == "null" {
			return fmt.Errorf("species decision must not be NULL")
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

func vegetationSpeciesEventUpper(code string) (string, error) {
	if err := validateVegetationSpeciesLookup(code); err != nil {
		return "", err
	}
	for _, char := range code {
		if char > 127 {
			return "", fmt.Errorf("species event case conversion for non-ASCII codes is not yet verified; select an exact source-list code instead")
		}
	}
	return strings.ToUpper(code), nil
}

func validateVegetationSpeciesDecision(update VegetationSpeciesUpdate) error {
	if update.Decision == "" {
		if update.Entered != nil || update.Selected != nil {
			return fmt.Errorf("listed species selection must not carry an unrelated decision")
		}
		return nil
	}
	if update.Entered == nil {
		return fmt.Errorf("species decision requires the original entered code")
	}
	var target string
	switch update.Decision {
	case "keep":
		if update.Selected != nil {
			return fmt.Errorf("keep species decision must not select a replacement")
		}
		target = *update.Entered
	case "replace", "user":
		if update.Selected == nil {
			return fmt.Errorf("species decision requires an explicit selected code")
		}
		if _, err := vegetationSpeciesEventUpper(*update.Entered); err != nil {
			return err
		}
		target = *update.Selected
	default:
		return fmt.Errorf("species decision %q is unavailable", update.Decision)
	}
	value, err := vegetationSpeciesEventUpper(target)
	if err != nil {
		return err
	}
	if update.Value != value {
		return fmt.Errorf("species decision value does not match the explicit source UCase event")
	}
	return nil
}

func (s *PlotService) validateVegetationSpeciesReference(tx *sql.Tx, update VegetationSpeciesUpdate, predicate string) error {
	query := tx.QueryRowContext
	s.projects.mu.RLock()
	coordinator := s.projects.sqlite
	s.projects.mu.RUnlock()
	if coordinator != nil {
		// The context lease owns readonly family aliases, not the project writer pool.
		query = coordinator.conn.QueryRowContext
	}
	var member bool
	statement := `SELECT EXISTS(SELECT 1 FROM (` + vegetationSpeciesUnion + `) WHERE ` + predicate + ` AND Code COLLATE BINARY=?)`
	args := []any{update.Value}
	if update.Decision != "" {
		var hasAlias bool
		if err := query(s.operationContext(), `SELECT EXISTS(SELECT 1 FROM USysAllSpecs WHERE OldCode COLLATE NOCASE=? AND Code IS NOT NULL)`, *update.Entered).Scan(&hasAlias); err != nil {
			return fmt.Errorf("species aliases unavailable: %w", err)
		}
		switch update.Decision {
		case "keep":
			if !hasAlias {
				return fmt.Errorf("species old-code definition changed; review the entered code again")
			}
			return nil
		case "replace":
			statement = `SELECT EXISTS(SELECT 1 FROM USysAllSpecs WHERE OldCode COLLATE NOCASE=? AND Code COLLATE BINARY=?)`
			args = []any{*update.Entered, *update.Selected}
		case "user":
			if hasAlias {
				return fmt.Errorf("an old-code definition takes precedence; review replacement or keep explicitly")
			}
			statement = `SELECT EXISTS(SELECT 1 FROM USysUserSpp WHERE Code COLLATE NOCASE=? AND Code COLLATE BINARY=?)`
			args = []any{*update.Entered, *update.Selected}
		}
	}
	if err := query(s.operationContext(), statement, args...).Scan(&member); err != nil {
		return fmt.Errorf("species references unavailable: %w", err)
	}
	if !member {
		return fmt.Errorf("species %q is not available for this selection or decision; review the source references again", update.Value)
	}
	return nil
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
		if err := validateVegetationSpeciesDecision(update); err != nil {
			return err
		}
		patches = append(patches, childRecordPatch{
			kind: "Veg", id: update.ID,
			cells: map[string]childExpectedValue{"species": {update.Expected, update.Value, reflect.String}},
			validate: func(tx *sql.Tx, table, plot string) error {
				var visible bool
				if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM `+table+
					` WHERE PlotNumber=? AND ID=? AND `+rowPredicate+`)`, plot, update.ID).Scan(&visible); err != nil {
					return fmt.Errorf("species source row could not be checked: %w", err)
				}
				if !visible {
					return fmt.Errorf("vegetation row %d no longer belongs to %s; cancel and reload", update.ID, update.Form)
				}
				return s.validateVegetationSpeciesReference(tx, update, predicate)
			},
		})
	}
	return s.updateChildPatches(plot, patches)
}

func (s *ContextService) UpdateVegetationSpecies(contextID, plot string, updates []VegetationSpeciesUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateVegetationSpecies(plot, updates) })
}
