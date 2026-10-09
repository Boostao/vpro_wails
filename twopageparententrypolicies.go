package main

import (
	"errors"
	"fmt"
)

type TwoPageEntryFieldPolicy struct {
	Column  string `json:"column"`
	Owner   string `json:"owner"`
	Kind    string `json:"kind"`
	Maximum int    `json:"maximum"`
}

func twoPageEntryFieldPolicies(form string) ([]TwoPageEntryFieldPolicy, error) {
	bindings, err := twoPageParentBindings(form)
	if err != nil {
		return nil, err
	}
	xl, err := twoPageXLFields(form)
	if err != nil {
		return nil, err
	}
	commonTextBounds := map[string]int{"SV_PolygonNumber": 25, "SV_CanopyComposition": 50,
		"SV_RootZoneTexture": 100, "SV_AhorizonType": 5, "PlotType": 10}
	result := []TwoPageEntryFieldPolicy{}
	seen := map[string]bool{}
	for _, binding := range bindings {
		column := binding.Binding
		if seen[column] || column == "PlotNumber" {
			continue
		}
		seen[column] = true
		policy := TwoPageEntryFieldPolicy{Column: column}
		switch {
		case xl[column].kind != "":
			field := xl[column]
			policy.Owner, policy.Kind, policy.Maximum = field.owner, field.kind, field.maximum
		case twoPageParentCommonFields[column].domain != "":
			field := twoPageParentCommonFields[column]
			policy.Owner, policy.Kind = field.owner, field.domain
			switch field.domain {
			case "scalar":
				policy.Kind = "single"
				if column == "SV_FloodPlain" {
					policy.Kind = "boolean"
				}
			case "text", "categorical", "plot-type":
				policy.Kind, policy.Maximum = field.domain, commonTextBounds[column]
				if policy.Maximum == 0 {
					return nil, fmt.Errorf("complete-entry %s has no reviewed common text bound", column)
				}
			}
		case twoPageParentExtraFields[column].domain != "":
			field := twoPageParentExtraFields[column]
			policy.Owner, policy.Kind, policy.Maximum = field.owner, field.domain, field.maximum
		default:
			return nil, fmt.Errorf("complete-entry %s has no reviewed editable domain", column)
		}
		result = append(result, policy)
	}
	expected := 117
	if form == "FS882-8x6XL-CHARS" {
		expected = 119
	}
	if len(result) != expected {
		return nil, errors.New("complete-entry editable policy cohort changed")
	}
	return result, nil
}
