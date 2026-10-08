package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type twoPageXLField struct {
	owner, kind string
	maximum     int
}

func twoPageXLFields(form string) (map[string]twoPageXLField, error) {
	bindings, err := twoPageParentBindings(form)
	if err != nil {
		return nil, err
	}
	texts := make(map[string]int, len(siviParentSharedTextBounds))
	for column, maximum := range siviParentSharedTextBounds {
		texts[column] = maximum
	}
	for column, maximum := range map[string]int{
		"ProjectID": 30, "TransDistrib": 3, "SurfaceTopographyType": 3,
		"SurfaceTopographySize": 2, "OfficeNotes": 0, "UserSiteUnit": 100, "BECSiteUnit": 100,
	} {
		texts[column] = maximum
	}
	for _, field := range siviParentSharedReferenceFields {
		texts[field.column] = field.maximum
	}
	for _, fields := range [][]becHeaderField{
		becHeaderFields(FS882Header{}), qualityHeaderFields(FS882Header{}),
		siteCodeHeaderFields(FS882Header{}), regionHeaderFields(FS882Header{}),
		soilHeaderFields(FS882Header{}), geologyHeaderFields(FS882Header{}),
		parentCodeHeaderFields(FS882Header{}), ordinaryTextFields(FS882Header{}),
	} {
		for _, field := range fields {
			if previous, exists := texts[field.name]; exists && previous != field.maximum {
				return nil, fmt.Errorf("two-page XL %s has conflicting accepted text bounds", field.name)
			}
			texts[field.name] = field.maximum
		}
	}
	source := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		source[binding.Binding] = true
	}
	result := map[string]twoPageXLField{}
	header := reflect.TypeOf(FS882Header{})
	for _, field := range headerFields {
		if field.column == "" || field.column == "PlotNumber" || !source[field.column] {
			continue
		}
		member, exists := header.FieldByName(field.member)
		if !exists || member.Type.Kind() != reflect.Pointer {
			return nil, fmt.Errorf("two-page XL %s has no accepted nullable header type", field.column)
		}
		policy := twoPageXLField{owner: field.table}
		switch member.Type.Elem().Kind() {
		case reflect.String:
			if field.column == "Date" {
				policy.kind = "date"
			} else {
				var found bool
				policy.maximum, found = texts[field.column]
				if !found {
					return nil, fmt.Errorf("two-page XL %s has no accepted text policy", field.column)
				}
				policy.kind = "text"
			}
		case reflect.Int:
			policy.kind = "integer"
		case reflect.Float64:
			policy.kind = "single"
			if field.column == "Latitude" || field.column == "Longitude" {
				policy.kind = strings.ToLower(field.column)
			}
		case reflect.Bool:
			policy.kind = "boolean"
		default:
			return nil, fmt.Errorf("two-page XL %s has no verified physical domain", field.column)
		}
		result[field.column] = policy
	}
	want := 95
	if form == "FS882-8x6XL-CHARS" {
		want = 97
	}
	if len(result) != want {
		return nil, errors.New("two-page XL field scope no longer matches the accepted source/header intersection")
	}
	return result, nil
}

func validateTwoPageXLField(column string, field twoPageXLField, value ProjectMetadataCell) error {
	if value.Storage == "null" {
		return nil
	}
	switch field.kind {
	case "text", "date":
		if value.Storage != "text" || value.Text == nil || strings.ContainsRune(*value.Text, '\x00') {
			return fmt.Errorf("two-page XL %s requires nullable literal TEXT without NUL", column)
		}
		if field.kind == "date" {
			return validateSIVIParentDateTimestamp(value.Text)
		}
		return validateOrdinaryText(becHeaderField{name: column, table: field.owner, maximum: field.maximum, value: value.Text})
	case "integer", "single", "latitude", "longitude":
		return validateSIVIParentSharedNumber(column, field.kind, value)
	case "boolean":
		if value.Storage != "integer" || value.Integer == nil || (*value.Integer != "0" && *value.Integer != "-1") {
			return fmt.Errorf("two-page XL %s requires Access BOOLEAN INTEGER0/-1 or NULL", column)
		}
		return nil
	default:
		return fmt.Errorf("two-page XL %s has an unavailable physical domain", column)
	}
}

// Physical planning is not reference-membership approval or write authorization.
func planTwoPageXLProjection(ctx context.Context, original *siviParentProjection, edits []siviParentScalarEdit, masterAllowed bool) ([]siviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("two-page XL planning requires one owned physical pair and explicit edits")
	}
	fields, err := twoPageXLFields(original.Form)
	if err != nil {
		return nil, err
	}
	parent, err := projectTwoPageParent(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}},
		ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(fields))
	for column, field := range fields {
		owners[column] = field.owner
	}
	assignments, err := planProjectedParentCells(ctx, original.ContextID, original.Project, parent, edits, owners, nil,
		func(column string, value ProjectMetadataCell) error {
			return validateTwoPageXLField(column, fields[column], value)
		})
	if err != nil {
		return nil, fmt.Errorf("two-page XL physical planning: %w", err)
	}
	for _, assignment := range assignments {
		if assignment.Column == "BECSiteUnit" && !masterAllowed {
			return nil, errors.New("two-page XL Master changes require independent source-user authorization")
		}
	}
	return assignments, nil
}
