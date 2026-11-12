package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type TwoPageEntryReferenceReader func(context.Context, *sql.Tx, sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error)

func TwoPageEntryRequiredReferenceFields(form string) ([]sourceform.SiviParentSharedReferencePolicy, error) {
	sources, err := twoPageParentSources()
	if err != nil {
		return nil, err
	}
	source, exists := sources[form]
	if !exists {
		return nil, errors.New("complete-entry references require an exact normal or CHARS source variant")
	}
	fields := []sourceform.SiviParentSharedReferencePolicy{}
	seen := map[string]bool{}
	for _, control := range source.Forms[0].Fields {
		limit, present := control.Properties["LimitToList"]
		if control.Type != "ComboBox" || control.Binding == "" || !present {
			continue
		}
		if limit.Value != "NotDefault" || seen[control.Binding] {
			return nil, errors.New("complete-entry required references contain an unreviewed source property or duplicate owner")
		}
		field, exists := sourceform.SiviParentSharedReferenceField(control.Binding)
		if !exists {
			return nil, fmt.Errorf("complete-entry %s has no verified reference reader", control.Binding)
		}
		field.Required = true
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
	slices.SortFunc(fields, func(left, right sourceform.SiviParentSharedReferencePolicy) int {
		return strings.Compare(left.Column, right.Column)
	})
	return fields, nil
}

// This checks mandatory membership only, not optional-code acknowledgements or publication.
func ApproveTwoPageEntryRequiredReferences(ctx context.Context, tx *sql.Tx, original *sourceform.SiviParentProjection,
	assignments []sourceform.SiviParentScalarAssignment, read TwoPageEntryReferenceReader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || original == nil || read == nil {
		return errors.New("complete-entry required references need an owned transaction, original and independent reader")
	}
	fields, err := TwoPageEntryRequiredReferenceFields(original.Form)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return err
		}
		index := slices.IndexFunc(fields, func(field sourceform.SiviParentSharedReferencePolicy) bool { return field.Column == assignment.Column })
		if index < 0 || assignment.Value == nil {
			continue
		}
		field := fields[index]
		value, text := assignment.Value.(string)
		if !text {
			return fmt.Errorf("complete-entry %s requires nullable TEXT for reference approval", field.Column)
		}
		if err := sqlutil.ValidateSiteCodeText(field.Column, &value, field.Maximum); err != nil {
			return err
		}
		reference, err := read(ctx, tx, field)
		if err != nil {
			return fmt.Errorf("complete-entry %s reference read: %w", field.Column, err)
		}
		if reference.Column != field.Column || reference.ListName != field.List || !reference.Required {
			return fmt.Errorf("complete-entry %s reference has a different source identity or membership policy", field.Column)
		}
		if !reference.Available {
			return fmt.Errorf("complete-entry %s reference is unavailable: %s", field.Column, reference.Diagnostic)
		}
		membership := map[string]bool{}
		for _, choice := range reference.Choices {
			if choice.Selectable && choice.Code != nil && sqlutil.ValidateSiteCodeText(field.Column, choice.Code, field.Maximum) == nil {
				membership[*choice.Code] = true
			}
		}
		if err := vprocore.CanonicalItemError(field.Column, field.List, value, membership); err != nil {
			return err
		}
	}
	return ctx.Err()
}
