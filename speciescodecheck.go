package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

type SpeciesCodeCheckRow struct {
	ID         int     `json:"id"`
	PlotNumber string  `json:"plotNumber"`
	Code       *string `json:"code"`
	Status     string  `json:"status"`
}

type SpeciesCodeCheckReview struct {
	Project string                `json:"project"`
	SU      string                `json:"su"`
	Rows    []SpeciesCodeCheckRow `json:"rows"`
}

type SpeciesCodeCheckOption struct {
	VegetationSpeciesOption
	Source string `json:"source"`
}

type SpeciesCodeCheckUpdate struct {
	ID         int     `json:"id"`
	PlotNumber string  `json:"plotNumber"`
	Expected   *string `json:"expected"`
	Value      string  `json:"value"`
}

func (update *SpeciesCodeCheckUpdate) UnmarshalJSON(data []byte) error {
	if err := validateJSONTextProperties(data, map[string]string{
		"plotNumber": "Species check plot", "expected": "Species check original code", "value": "Species check replacement",
	}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, name := range []string{"id", "plotNumber", "expected", "value"} {
		raw, present := properties[name]
		if !present || name != "expected" && strings.TrimSpace(string(raw)) == "null" {
			return fmt.Errorf("species check requires explicit %s, preserving NULL originals", name)
		}
	}
	for name := range properties {
		if name != "id" && name != "plotNumber" && name != "expected" && name != "value" {
			return fmt.Errorf("species check property %q is unavailable; scope and database ownership are context-selected", name)
		}
	}
	type plain SpeciesCodeCheckUpdate
	return json.Unmarshal(data, (*plain)(update))
}

func (s *ContextService) ReviewSpeciesCodes(ctx context.Context, contextID string) (SpeciesCodeCheckReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (SpeciesCodeCheckReview, error) {
		c := plots.projects.sqlite
		table := `"project".` + quoteHeaderIdentifier(c.selection.Project+"_Veg")
		rows, err := c.conn.QueryContext(ctx, `SELECT v.ID,v.PlotNumber,v.Species,typeof(v.Species),
			EXISTS(SELECT 1 FROM USysAllSpecs WHERE Code COLLATE BINARY=v.Species),
			EXISTS(SELECT 1 FROM USysUserSpp WHERE Code COLLATE BINARY=v.Species)
			FROM `+table+` AS v WHERE EXISTS(SELECT 1 FROM USysEnv e WHERE e.PlotNumber=v.PlotNumber)
			ORDER BY v.PlotNumber COLLATE BINARY,v.ID`)
		if err != nil {
			return SpeciesCodeCheckReview{}, fmt.Errorf("species check scope/references unavailable: %w", err)
		}
		defer rows.Close()
		result := SpeciesCodeCheckReview{Project: c.selection.Project, SU: c.selection.SU, Rows: []SpeciesCodeCheckRow{}}
		seen := map[int]bool{}
		for rows.Next() {
			var row SpeciesCodeCheckRow
			var storage string
			var master, user bool
			if err := rows.Scan(&row.ID, &row.PlotNumber, &row.Code, &storage, &master, &user); err != nil {
				return SpeciesCodeCheckReview{}, fmt.Errorf("species check row could not be read: %w", err)
			}
			if row.ID < -2147483648 || row.ID > 2147483647 || seen[row.ID] {
				return SpeciesCodeCheckReview{}, errors.New("species check scope has invalid or ambiguous physical identities")
			}
			seen[row.ID] = true
			if storage != "text" && storage != "null" || row.Code != nil && !utf8.ValidString(*row.Code) {
				return SpeciesCodeCheckReview{}, fmt.Errorf("species check row %d contains unsupported historical storage; no code was repaired", row.ID)
			}
			switch {
			case row.Code == nil:
				row.Status = "missing"
			case master:
				row.Status = "master"
			case user:
				row.Status = "user"
			default:
				row.Status = "unlisted"
			}
			result.Rows = append(result.Rows, row)
		}
		if err := rows.Err(); err != nil {
			return SpeciesCodeCheckReview{}, fmt.Errorf("species check scope read failed: %w", err)
		}
		return result, nil
	})
}

func (s *ContextService) LookupSpeciesCodeCheckTarget(ctx context.Context, contextID string, lookup VegetationSpeciesLookup) ([]SpeciesCodeCheckOption, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]SpeciesCodeCheckOption, error) {
		if err := validateVegetationSpeciesLookup(lookup.Code); err != nil {
			return nil, err
		}
		rows, err := plots.projects.sqlite.conn.QueryContext(ctx, `SELECT Code,ScientificName,Lifeform,EnglishName,Codetype,'master'
			FROM USysAllSpecs WHERE Code COLLATE BINARY=?
			UNION ALL SELECT Code,ScientificName,LifeForm,EnglishName,Codetype,'user'
			FROM USysUserSpp WHERE Code COLLATE BINARY=?
			ORDER BY 6,1,2,4,5,3`, lookup.Code, lookup.Code)
		if err != nil {
			return nil, fmt.Errorf("species check target metadata unavailable: %w", err)
		}
		defer rows.Close()
		result := []SpeciesCodeCheckOption{}
		for rows.Next() {
			var option SpeciesCodeCheckOption
			if err := rows.Scan(&option.Code, &option.ScientificName, &option.Lifeform, &option.EnglishName, &option.CodeType, &option.Source); err != nil {
				return nil, fmt.Errorf("species check target metadata could not be read: %w", err)
			}
			result = append(result, option)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("species check target read failed: %w", err)
		}
		return result, nil
	})
}

func (s *ContextService) SaveSpeciesCodeCheck(contextID string, updates []SpeciesCodeCheckUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error {
		patches := make([]childPlotPatch, 0, len(updates))
		for _, update := range updates {
			if update.Expected != nil && !utf8.ValidString(*update.Expected) {
				return errors.New("species check original contains malformed UTF-8; it was not repaired")
			}
			if err := validateVegetationSpeciesLookup(update.Value); err != nil {
				return err
			}
			if update.Expected != nil && *update.Expected == update.Value {
				return errors.New("species check replacement must change the original literal code; Ignore requires no write")
			}
			var expected any
			if update.Expected != nil {
				expected = *update.Expected
			}
			validate := func(tx *sql.Tx, table, plot string) error {
				c := plots.projects.sqlite
				var allowed, listed bool
				if err := c.conn.QueryRowContext(plots.operationContext(), `SELECT
					EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?),
					EXISTS(SELECT 1 FROM USysAllSpecs WHERE Code COLLATE BINARY=?) OR
					EXISTS(SELECT 1 FROM USysUserSpp WHERE Code COLLATE BINARY=?)`,
					plot, update.Value, update.Value).Scan(&allowed, &listed); err != nil {
					return fmt.Errorf("species check scope/target could not be revalidated: %w", err)
				}
				if !allowed || !listed {
					return errors.New("species check row or literal replacement is no longer in the reviewed context scope/references")
				}
				return nil
			}
			patches = append(patches, childPlotPatch{update.PlotNumber, childRecordPatch{
				kind: "Veg", id: update.ID, cells: map[string]childExpectedValue{
					"species": {expected, update.Value, reflect.String},
				},
				validate: validate,
				observe: func(tx *sql.Tx, table, plot string) error {
					var actual, storage string
					if err := tx.QueryRow(`SELECT Species,typeof(Species) FROM `+table+` WHERE PlotNumber=? AND ID=?`,
						plot, update.ID).Scan(&actual, &storage); err != nil {
						return fmt.Errorf("species check stored replacement could not be observed: %w", err)
					}
					if storage != "text" || actual != update.Value {
						return errors.New("species check stored replacement differs from the explicit reviewed value")
					}
					return validate(tx, table, plot)
				},
			}})
		}
		return plots.updateChildPlotPatches(patches)
	})
}
