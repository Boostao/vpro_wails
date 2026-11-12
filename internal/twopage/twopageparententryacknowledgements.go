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
	"reflect"
	"strconv"
	"strings"
)

type TwoPageEntryCodeAcknowledgementInternal struct {
	ContextID, Project, Plot, Form, Table, RowID, Column string
	Expected, Value                                      vprocore.ProjectMetadataCell
	Reference                                            sourceform.SIVIParentSharedReference
}

type TwoPageEntryCodeReader func(context.Context, *sql.Tx, TwoPageEntryReferenceField) (sourceform.SIVIParentSharedReference, error)

func TwoPageEntryReferenceCode(field TwoPageEntryReferenceField, cell vprocore.ProjectMetadataCell) (string, error) {
	if cell.Storage != field.storage {
		return "", fmt.Errorf("complete-entry %s reference needs %s storage", field.Column, field.storage)
	}
	if _, err := vprocore.MetadataCellValue(cell); err != nil {
		return "", err
	}
	if field.storage == "integer" {
		if err := sourceform.ValidateSIVIParentSharedNumber(field.Column, "integer", cell); err != nil {
			return "", err
		}
		value, err := strconv.ParseInt(*cell.Integer, 10, 64)
		if err != nil || strconv.FormatInt(value, 10) != *cell.Integer {
			return "", errors.New("complete-entry numeric reference requires canonical INTEGER transport")
		}
		return *cell.Integer, nil
	}
	if cell.Text == nil {
		return "", fmt.Errorf("complete-entry %s reference requires literal TEXT", field.Column)
	}
	if err := sqlutil.ValidateSiteCodeText(field.Column, cell.Text, field.Maximum); err != nil {
		return "", err
	}
	return *cell.Text, nil
}

func twoPageEntryOptionalReferenceMatch(field TwoPageEntryReferenceField, value string, choice sourceform.SIVIParentSharedReferenceChoice) bool {
	if !choice.Selectable || choice.Code == nil {
		return false
	}
	if field.storage == "integer" {
		candidate := vprocore.ProjectMetadataCell{Storage: "integer", Integer: choice.Code}
		code, err := TwoPageEntryReferenceCode(field, candidate)
		return err == nil && code == value
	}
	if sqlutil.ValidateSiteCodeText(field.Column, choice.Code, field.Maximum) != nil {
		return false
	}
	fold := func(value string) string {
		return strings.Map(func(character rune) rune {
			if character >= 'A' && character <= 'Z' {
				return character + ('a' - 'A')
			}
			return character
		}, value)
	}
	return fold(value) == fold(*choice.Code)
}

func ApproveTwoPageEntryOptionalReferences(ctx context.Context, tx *sql.Tx, original *sourceform.SiviParentProjection,
	assignments []sourceform.SiviParentScalarAssignment, acknowledgements []TwoPageEntryCodeAcknowledgementInternal, read TwoPageEntryCodeReader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || original == nil || len(original.Rows) != 1 || read == nil {
		return errors.New("complete-entry optional references require an owned transaction/original and independent reader")
	}
	fields, err := twoPageEntryReferenceFields(original.Form)
	if err != nil {
		return err
	}
	policies := map[string]TwoPageEntryReferenceField{}
	for _, field := range fields {
		policies[field.Column] = field
	}
	accepted := map[string]TwoPageEntryCodeAcknowledgementInternal{}
	for _, acknowledgement := range acknowledgements {
		field, exists := policies[acknowledgement.Column]
		if _, duplicate := accepted[acknowledgement.Column]; !exists || field.Required || duplicate {
			return errors.New("complete-entry acknowledgement has a foreign, mandatory or duplicate reference target")
		}
		accepted[acknowledgement.Column] = acknowledgement
	}
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return err
		}
		field, reference := policies[assignment.Column]
		if !reference || field.Required || assignment.After.Storage == "null" {
			continue
		}
		value, err := TwoPageEntryReferenceCode(field, assignment.After)
		if err != nil {
			return err
		}
		fresh, err := read(ctx, tx, field)
		if err != nil {
			return fmt.Errorf("complete-entry optional %s reference read: %w", field.Column, err)
		}
		if fresh.Column != field.Column || fresh.ListName != field.List || fresh.Required ||
			(!fresh.Available && fresh.Diagnostic == "") {
			return fmt.Errorf("complete-entry optional %s reference has a foreign policy or unexplained availability", field.Column)
		}
		matched := false
		if fresh.Available {
			for _, choice := range fresh.Choices {
				matched = matched || twoPageEntryOptionalReferenceMatch(field, value, choice)
			}
		}
		acknowledgement, acknowledged := accepted[field.Column]
		if acknowledged {
			if acknowledgement.ContextID != original.ContextID || acknowledgement.Project != original.Project ||
				acknowledgement.Plot != original.Plot || acknowledgement.Form != original.Form ||
				acknowledgement.ContextID != assignment.ContextID || acknowledgement.Table != assignment.Table ||
				acknowledgement.RowID != assignment.RowID || !reflect.DeepEqual(acknowledgement.Expected, assignment.Before) ||
				!reflect.DeepEqual(acknowledgement.Value, assignment.After) || !reflect.DeepEqual(acknowledgement.Reference, fresh) {
				return fmt.Errorf("complete-entry %s acknowledgement differs from the owned original, literal draft or fresh reference", field.Column)
			}
			delete(accepted, field.Column)
		}
		if !matched && !acknowledged {
			return fmt.Errorf("complete-entry %s unmatched or unavailable reference requires explicit current-value acknowledgement", field.Column)
		}
	}
	if len(accepted) != 0 {
		return errors.New("complete-entry acknowledgement contains unused or cleared reference targets")
	}
	return ctx.Err()
}
