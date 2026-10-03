package main

import (
	"context"
	"encoding/json"
	"fmt"
)

type VegetationSpeciesOption struct {
	Code           *string `json:"code"`
	ScientificName *string `json:"scientificName"`
	Lifeform       *int    `json:"lifeform"`
	EnglishName    *string `json:"englishName"`
	CodeType       *string `json:"codeType"`
}

type VegetationSpeciesAlias struct {
	VegetationSpeciesOption
	OldCode *string `json:"oldCode"`
}

type VegetationSpeciesLookup struct {
	Code string `json:"code"`
}

func (lookup *VegetationSpeciesLookup) UnmarshalJSON(data []byte) error {
	type plain VegetationSpeciesLookup
	if err := validateJSONTextProperties(data, map[string]string{"code": "Species lookup code"}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	raw, present := properties["code"]
	if !present || string(raw) == "null" {
		return fmt.Errorf("species lookup requires an explicit non-NULL code")
	}
	return json.Unmarshal(data, (*plain)(lookup))
}

const vegetationSpeciesUnion = `SELECT Code,ScientificName,Lifeform,EnglishName,Codetype
	FROM USysAllSpecs WHERE Codetype COLLATE NOCASE <> 's'
	UNION SELECT Code,ScientificName,LifeForm,EnglishName,Codetype
	FROM USysUserSpp WHERE Codetype COLLATE NOCASE <> 's'`

func vegetationSpeciesLifeforms(form string) (string, error) {
	switch form {
	case "SubVegAXL_BC", "SubVegAXL", "SubVegAhtXL":
		return "1,2,3,4", nil
	case "SubVegCXL", "SubVegChtXL":
		return "5,6,7,8,12", nil
	case "SubVegDXL":
		return "1,2,9,10,11", nil
	default:
		return "", fmt.Errorf("vegetation species source form %q is unavailable", form)
	}
}

func vegetationSpeciesPredicate(form string) (string, error) {
	lifeforms, err := vegetationSpeciesLifeforms(form)
	if err != nil {
		return "", err
	}
	return "Codetype COLLATE NOCASE IN ('u','x') AND Lifeform IN (" + lifeforms + ")", nil
}

func validateVegetationSpeciesLookup(code string) error {
	if code == "" {
		return fmt.Errorf("species lookup requires a nonempty literal code")
	}
	return validateChildPhysicalText("Veg.Species", code, 8)
}

func (s *ContextService) ListVegetationSpecies(ctx context.Context, contextID, form string) ([]VegetationSpeciesOption, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]VegetationSpeciesOption, error) {
		predicate, err := vegetationSpeciesPredicate(form)
		if err != nil {
			return nil, err
		}
		rows, err := plots.projects.sqlite.conn.QueryContext(ctx, `SELECT Code,ScientificName,Lifeform,EnglishName,Codetype
			FROM (`+vegetationSpeciesUnion+`) WHERE `+predicate+`
			ORDER BY Code COLLATE NOCASE,Code,ScientificName,EnglishName,Codetype`)
		if err != nil {
			return nil, fmt.Errorf("vegetation species references unavailable: %w", err)
		}
		defer rows.Close()
		result := []VegetationSpeciesOption{}
		for rows.Next() {
			var option VegetationSpeciesOption
			if err := rows.Scan(&option.Code, &option.ScientificName, &option.Lifeform, &option.EnglishName, &option.CodeType); err != nil {
				return nil, fmt.Errorf("vegetation species metadata could not be read: %w", err)
			}
			result = append(result, option)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("vegetation species reference read failed: %w", err)
		}
		return result, nil
	})
}

func (s *ContextService) ListVegetationSpeciesAliases(ctx context.Context, contextID string, lookup VegetationSpeciesLookup) ([]VegetationSpeciesAlias, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]VegetationSpeciesAlias, error) {
		code := lookup.Code
		if err := validateVegetationSpeciesLookup(code); err != nil {
			return nil, err
		}
		rows, err := plots.projects.sqlite.conn.QueryContext(ctx, `SELECT Code,ScientificName,Lifeform,EnglishName,Codetype,OldCode
			FROM USysAllSpecs WHERE OldCode COLLATE NOCASE=?
			ORDER BY Code COLLATE NOCASE,Code,ScientificName,EnglishName,Codetype`, code)
		if err != nil {
			return nil, fmt.Errorf("vegetation species aliases unavailable: %w", err)
		}
		defer rows.Close()
		result := []VegetationSpeciesAlias{}
		for rows.Next() {
			var alias VegetationSpeciesAlias
			if err := rows.Scan(&alias.Code, &alias.ScientificName, &alias.Lifeform, &alias.EnglishName, &alias.CodeType, &alias.OldCode); err != nil {
				return nil, fmt.Errorf("vegetation species alias metadata could not be read: %w", err)
			}
			result = append(result, alias)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("vegetation species alias read failed: %w", err)
		}
		return result, nil
	})
}

func (s *ContextService) ListVegetationSpeciesUsers(ctx context.Context, contextID string, lookup VegetationSpeciesLookup) ([]VegetationSpeciesOption, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]VegetationSpeciesOption, error) {
		if err := validateVegetationSpeciesLookup(lookup.Code); err != nil {
			return nil, err
		}
		rows, err := plots.projects.sqlite.conn.QueryContext(ctx, `SELECT Code,ScientificName,Lifeform,EnglishName,Codetype
			FROM USysUserSpp WHERE Code COLLATE NOCASE=?
			ORDER BY Code,ScientificName,EnglishName,Codetype,Lifeform`, lookup.Code)
		if err != nil {
			return nil, fmt.Errorf("vegetation personal species references unavailable: %w", err)
		}
		defer rows.Close()
		result := []VegetationSpeciesOption{}
		for rows.Next() {
			var option VegetationSpeciesOption
			if err := rows.Scan(&option.Code, &option.ScientificName, &option.Lifeform, &option.EnglishName, &option.CodeType); err != nil {
				return nil, fmt.Errorf("vegetation personal species metadata could not be read: %w", err)
			}
			result = append(result, option)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("vegetation personal species read failed: %w", err)
		}
		return result, nil
	})
}
