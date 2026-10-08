package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type siviParentActionEdit struct {
	ContextID, ControlID, Table, RowID string
	Expected                           ProjectMetadataCell
	Option                             *int
}

type siviParentActionSource struct {
	ControlName, Binding, Owner string
	Implicit                    bool
}

var siviParentActionSources = sync.OnceValues(func() (map[string]siviParentActionSource, error) {
	layout, err := siviParentSourceLayout()
	if err != nil {
		return nil, err
	}
	names := map[string]siviParentActionSource{
		"optPlotType":            {ControlName: "optPlotType", Binding: "PlotType", Owner: "Admin"},
		"optSpeciesListComplete": {ControlName: "optSpeciesListComplete", Binding: "SpeciesListComplete", Owner: "Env", Implicit: true},
	}
	result := map[string]siviParentActionSource{}
	seen := map[string]bool{}
	for _, field := range layout.Forms[0].Fields {
		source, allowed := names[field.ControlName]
		if !allowed {
			continue
		}
		if seen[field.ControlName] || field.ControlID == "" || field.Type != "OptionGroup" || field.Binding != "" ||
			!field.ReadOnly || field.Implementation != "unmapped" {
			return nil, errors.New("SIVI actions require original unbound option groups")
		}
		resolved := 0
		for _, event := range field.Events {
			if event.Property == "AfterUpdate" && event.Binding == "[Event Procedure]" &&
				event.Procedure == field.ControlName+"_AfterUpdate" && event.Resolved {
				resolved++
			}
		}
		if resolved != 1 {
			return nil, errors.New("SIVI action requires its resolved original AfterUpdate")
		}
		if _, exists := result[field.ControlID]; exists {
			return nil, errors.New("SIVI action source identity is repeated")
		}
		result[field.ControlID] = source
		seen[field.ControlName] = true
	}
	if len(result) != 2 {
		return nil, errors.New("SIVI requires exactly two original parent action groups")
	}
	return result, nil
})

func planSIVIParentActions(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentActionEdit) ([]siviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sources, err := siviParentActionSources()
	if err != nil {
		return nil, err
	}
	targets := make([]siviParentScalarEdit, 0, len(edits))
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source, allowed := sources[edit.ControlID]
		if !allowed {
			return nil, fmt.Errorf("SIVI action source %q is outside the normal parent scope", edit.ControlID)
		}
		value := ProjectMetadataCell{Storage: "null"}
		switch source.Binding {
		case "PlotType":
			if edit.Option == nil || *edit.Option < 1 || *edit.Option > 5 {
				return nil, errors.New("SIVI PlotType action requires a selected source option1..5; NULL/other paths are unverified")
			}
			text := []string{"Ground", "Visual", "Note", "FS882", "Other"}[*edit.Option-1]
			value = ProjectMetadataCell{Storage: "text", Text: &text}
		case "SpeciesListComplete":
			if edit.Option != nil {
				if *edit.Option != 1 && *edit.Option != 2 {
					return nil, errors.New("SIVI species-list action requires option1/2 or explicit NULL")
				}
				number := "0"
				if *edit.Option == 1 {
					number = "-1"
				}
				value = ProjectMetadataCell{Storage: "integer", Integer: &number}
			}
		default:
			return nil, errors.New("SIVI action source target is unsupported")
		}
		targets = append(targets, siviParentScalarEdit{
			ContextID: edit.ContextID, Table: edit.Table, RowID: edit.RowID,
			Column: source.Binding, Expected: edit.Expected, Value: value,
		})
	}
	owners := map[string]string{}
	implicit := map[string]bool{}
	for _, source := range sources {
		owners[source.Binding] = source.Owner
		implicit[source.Binding] = source.Implicit
	}
	return planSIVIParentSourceCells(ctx, contextID, project, plot, env, admin, targets, owners, implicit, func(column string, value ProjectMetadataCell) error {
		if column == "PlotType" {
			return validateSiteCodeText(column, value.Text, 10)
		}
		return validateSIVIParentScalar("SV_FloodPlain", value)
	})
}
