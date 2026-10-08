package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type twoPageEntryReferenceReader func(context.Context, *sql.Tx, siviParentSharedReferencePolicy) (SIVIParentSharedReference, error)

func twoPageEntryRequiredReferenceFields(form string) ([]siviParentSharedReferencePolicy, error) {
	sources, err := twoPageParentSources()
	if err != nil {
		return nil, err
	}
	source, exists := sources[form]
	if !exists {
		return nil, errors.New("complete-entry references require an exact normal or CHARS source variant")
	}
	fields := []siviParentSharedReferencePolicy{}
	seen := map[string]bool{}
	for _, control := range source.Forms[0].Fields {
		limit, present := control.Properties["LimitToList"]
		if control.Type != "ComboBox" || control.Binding == "" || !present {
			continue
		}
		if limit.Value != "NotDefault" || seen[control.Binding] {
			return nil, errors.New("complete-entry required references contain an unreviewed source property or duplicate owner")
		}
		field, exists := siviParentSharedReferenceField(control.Binding)
		if !exists {
			return nil, fmt.Errorf("complete-entry %s has no verified reference reader", control.Binding)
		}
		field.required = true
		fields = append(fields, field)
		seen[control.Binding] = true
	}
	expected := []string{"Exposure1", "Exposure2", "MesoSlopePosition", "SoilDrainage", "SuccessionalStatus", "SurfaceShape"}
	if len(fields) != len(expected) {
		return nil, errors.New("complete-entry required reference scope changed")
	}
	for _, column := range expected {
		if !seen[column] {
			return nil, fmt.Errorf("complete-entry required reference %s is missing", column)
		}
	}
	slices.SortFunc(fields, func(left, right siviParentSharedReferencePolicy) int {
		return strings.Compare(left.column, right.column)
	})
	return fields, nil
}

// This checks mandatory membership only, not optional-code acknowledgements or publication.
func approveTwoPageEntryRequiredReferences(ctx context.Context, tx *sql.Tx, original *siviParentProjection,
	assignments []siviParentScalarAssignment, read twoPageEntryReferenceReader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || original == nil || read == nil {
		return errors.New("complete-entry required references need an owned transaction, original and independent reader")
	}
	fields, err := twoPageEntryRequiredReferenceFields(original.Form)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return err
		}
		index := slices.IndexFunc(fields, func(field siviParentSharedReferencePolicy) bool { return field.column == assignment.Column })
		if index < 0 || assignment.Value == nil {
			continue
		}
		field := fields[index]
		value, text := assignment.Value.(string)
		if !text {
			return fmt.Errorf("complete-entry %s requires nullable TEXT for reference approval", field.column)
		}
		if err := validateSiteCodeText(field.column, &value, field.maximum); err != nil {
			return err
		}
		reference, err := read(ctx, tx, field)
		if err != nil {
			return fmt.Errorf("complete-entry %s reference read: %w", field.column, err)
		}
		if reference.Column != field.column || reference.ListName != field.list || !reference.Required {
			return fmt.Errorf("complete-entry %s reference has a different source identity or membership policy", field.column)
		}
		if !reference.Available {
			return fmt.Errorf("complete-entry %s reference is unavailable: %s", field.column, reference.Diagnostic)
		}
		membership := map[string]bool{}
		for _, choice := range reference.Choices {
			if choice.Selectable && choice.Code != nil && validateSiteCodeText(field.column, choice.Code, field.maximum) == nil {
				membership[*choice.Code] = true
			}
		}
		if err := canonicalItemError(field.column, field.list, value, membership); err != nil {
			return err
		}
	}
	return ctx.Err()
}
