package main

import "fmt"

type SIVICHARSParentEntryPolicy struct {
	Column     string `json:"column"`
	Binding    string `json:"binding"`
	ControlID  string `json:"controlId"`
	Owner      string `json:"owner"`
	Kind       string `json:"kind"`
	Maximum    int    `json:"maximum"`
	AllowEmpty bool   `json:"allowEmpty"`
	Lower      *int   `json:"lower"`
	Upper      *int   `json:"upper"`
	Writable   bool   `json:"writable"`
	Diagnostic string `json:"diagnostic"`
}

func siviCHARSParentEntryPolicies() ([]SIVICHARSParentEntryPolicy, error) {
	fields, err := siviCHARSParentFields()
	if err != nil {
		return nil, err
	}
	bindings, err := siviCHARSParentSourceBindings()
	if err != nil {
		return nil, err
	}
	policies := make([]SIVICHARSParentEntryPolicy, 0, len(fields))
	for _, binding := range bindings {
		column := binding.Binding
		if column == "siteNotes" {
			column = "SiteNotes"
		}
		field, exists := fields[column]
		if !exists {
			continue
		}
		if field.binding != binding.Binding {
			return nil, fmt.Errorf("CHARS entry policy %s changed its exact source binding", column)
		}
		policy := SIVICHARSParentEntryPolicy{Column: column, Binding: field.binding, ControlID: binding.ControlID,
			Owner: field.owner, Kind: field.kind, Maximum: field.maximum, AllowEmpty: field.allowEmpty, Writable: true}
		if field.kind == "integer" {
			lower, upper := -32768, 32767
			if column == "StartDate" {
				lower, upper = 1900, 2500
			}
			policy.Lower, policy.Upper = &lower, &upper
		}
		if column == "SubZone" {
			policy.Writable = false
			policy.Diagnostic = "CHARS source targets frmSIVIsite.cmbZone; changed SubZone assignments remain unavailable"
		}
		if column == "SV_AhorizonType" {
			policy.AllowEmpty = false
			policy.Diagnostic = "The source blank choice does not qualify bound NULL versus empty TEXT assignment"
		}
		if field.kind == "boolean" {
			policy.Diagnostic = "Source Yes/No converts NULL to No; choose No explicitly. Unchanged historical SQLite NULL is retained."
		}
		policies = append(policies, policy)
	}
	if len(policies) != len(fields) {
		return nil, fmt.Errorf("CHARS entry policy source cohort changed: got%d, expected%d", len(policies), len(fields))
	}
	return policies, nil
}
