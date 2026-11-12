package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	texts := make(map[string]int, len(sourceform.SiviParentSharedTextBounds))
	for column, maximum := range sourceform.SiviParentSharedTextBounds {
		texts[column] = maximum
	}
	for column, maximum := range map[string]int{
		"ProjectID": 30, "TransDistrib": 3, "SurfaceTopographyType": 3,
		"SurfaceTopographySize": 2, "OfficeNotes": 0, "UserSiteUnit": 100, "BECSiteUnit": 100,
	} {
		texts[column] = maximum
	}
	for _, field := range sourceform.SiviParentSharedReferenceFields {
		texts[field.Column] = field.Maximum
	}
	for _, fields := range [][]vprocore.BecHeaderField{
		vprocore.BecHeaderFields(vprocore.FS882Header{}), vprocore.QualityHeaderFields(vprocore.FS882Header{}),
		vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}), vprocore.RegionHeaderFields(vprocore.FS882Header{}),
		vprocore.SoilHeaderFields(vprocore.FS882Header{}), vprocore.GeologyHeaderFields(vprocore.FS882Header{}),
		vprocore.ParentCodeHeaderFields(vprocore.FS882Header{}), vprocore.OrdinaryTextFields(vprocore.FS882Header{}),
	} {
		for _, field := range fields {
			if previous, exists := texts[field.Name]; exists && previous != field.Maximum {
				return nil, fmt.Errorf("two-page XL %s has conflicting accepted text bounds", field.Name)
			}
			texts[field.Name] = field.Maximum
		}
	}
	source := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		source[binding.Binding] = true
	}
	result := map[string]twoPageXLField{}
	header := reflect.TypeOf(vprocore.FS882Header{})
	for _, field := range vprocore.HeaderFields {
		if field.Column == "" || field.Column == "PlotNumber" || !source[field.Column] {
			continue
		}
		if _, inCommon := TwoPageParentCommonFields[field.Column]; inCommon {
			continue
		}
		if _, inExtra := sourceform.TwoPageParentExtraFields[field.Column]; inExtra {
			continue
		}
		member, exists := header.FieldByName(field.Member)
		if !exists || member.Type.Kind() != reflect.Pointer {
			return nil, fmt.Errorf("two-page XL %s has no accepted nullable header type", field.Column)
		}
		policy := twoPageXLField{owner: field.Table}
		switch member.Type.Elem().Kind() {
		case reflect.String:
			if field.Column == "Date" {
				policy.kind = "date"
			} else {
				var found bool
				policy.maximum, found = texts[field.Column]
				if !found {
					return nil, fmt.Errorf("two-page XL %s has no accepted text policy", field.Column)
				}
				policy.kind = "text"
			}
		case reflect.Int:
			policy.kind = "integer"
		case reflect.Float64:
			policy.kind = "single"
			if field.Column == "Latitude" || field.Column == "Longitude" {
				policy.kind = strings.ToLower(field.Column)
			}
		case reflect.Bool:
			policy.kind = "boolean"
		default:
			return nil, fmt.Errorf("two-page XL %s has no verified physical domain", field.Column)
		}
		result[field.Column] = policy
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

func validateTwoPageXLField(column string, field twoPageXLField, value vprocore.ProjectMetadataCell) error {
	if value.Storage == "null" {
		return nil
	}
	switch field.kind {
	case "text", "date":
		if value.Storage != "text" || value.Text == nil || strings.ContainsRune(*value.Text, '\x00') {
			return fmt.Errorf("two-page XL %s requires nullable literal TEXT without NUL", column)
		}
		if field.kind == "date" {
			return sourceform.ValidateSIVIParentDateTimestamp(value.Text)
		}
		return vprocore.ValidateOrdinaryText(vprocore.BecHeaderField{Name: column, Table: field.owner, Maximum: field.maximum, Value: value.Text})
	case "integer", "single", "latitude", "longitude":
		return sourceform.ValidateSIVIParentSharedNumber(column, field.kind, value)
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
func planTwoPageXLProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool) ([]sourceform.SiviParentScalarAssignment, error) {
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
	parent, err := ProjectTwoPageParent(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(fields))
	for column, field := range fields {
		owners[column] = field.owner
	}
	assignments, err := sourceform.PlanProjectedParentCells(ctx, original.ContextID, original.Project, parent, edits, owners, nil,
		func(column string, value vprocore.ProjectMetadataCell) error {
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
