package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type twoPageEntryCodeAcknowledgement struct {
	ContextID, Project, Plot, Form, Table, RowID, Column string
	Expected, Value                                      ProjectMetadataCell
	Reference                                            SIVIParentSharedReference
}

type twoPageEntryCodeReader func(context.Context, *sql.Tx, twoPageEntryReferenceField) (SIVIParentSharedReference, error)

func twoPageEntryReferenceCode(field twoPageEntryReferenceField, cell ProjectMetadataCell) (string, error) {
	if cell.Storage != field.storage {
		return "", fmt.Errorf("complete-entry %s reference needs %s storage", field.column, field.storage)
	}
	if _, err := metadataCellValue(cell); err != nil {
		return "", err
	}
	if field.storage == "integer" {
		if err := validateSIVIParentSharedNumber(field.column, "integer", cell); err != nil {
			return "", err
		}
		value, err := strconv.ParseInt(*cell.Integer, 10, 64)
		if err != nil || strconv.FormatInt(value, 10) != *cell.Integer {
			return "", errors.New("complete-entry numeric reference requires canonical INTEGER transport")
		}
		return *cell.Integer, nil
	}
	if cell.Text == nil {
		return "", fmt.Errorf("complete-entry %s reference requires literal TEXT", field.column)
	}
	if err := validateSiteCodeText(field.column, cell.Text, field.maximum); err != nil {
		return "", err
	}
	return *cell.Text, nil
}

func twoPageEntryOptionalReferenceMatch(field twoPageEntryReferenceField, value string, choice SIVIParentSharedReferenceChoice) bool {
	if !choice.Selectable || choice.Code == nil {
		return false
	}
	if field.storage == "integer" {
		candidate := ProjectMetadataCell{Storage: "integer", Integer: choice.Code}
		code, err := twoPageEntryReferenceCode(field, candidate)
		return err == nil && code == value
	}
	if validateSiteCodeText(field.column, choice.Code, field.maximum) != nil {
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

func approveTwoPageEntryOptionalReferences(ctx context.Context, tx *sql.Tx, original *siviParentProjection,
	assignments []siviParentScalarAssignment, acknowledgements []twoPageEntryCodeAcknowledgement, read twoPageEntryCodeReader) error {
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
	policies := map[string]twoPageEntryReferenceField{}
	for _, field := range fields {
		policies[field.column] = field
	}
	accepted := map[string]twoPageEntryCodeAcknowledgement{}
	for _, acknowledgement := range acknowledgements {
		field, exists := policies[acknowledgement.Column]
		if _, duplicate := accepted[acknowledgement.Column]; !exists || field.required || duplicate {
			return errors.New("complete-entry acknowledgement has a foreign, mandatory or duplicate reference target")
		}
		accepted[acknowledgement.Column] = acknowledgement
	}
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return err
		}
		field, reference := policies[assignment.Column]
		if !reference || field.required || assignment.After.Storage == "null" {
			continue
		}
		value, err := twoPageEntryReferenceCode(field, assignment.After)
		if err != nil {
			return err
		}
		fresh, err := read(ctx, tx, field)
		if err != nil {
			return fmt.Errorf("complete-entry optional %s reference read: %w", field.column, err)
		}
		if fresh.Column != field.column || fresh.ListName != field.list || fresh.Required ||
			(!fresh.Available && fresh.Diagnostic == "") {
			return fmt.Errorf("complete-entry optional %s reference has a foreign policy or unexplained availability", field.column)
		}
		matched := false
		if fresh.Available {
			for _, choice := range fresh.Choices {
				matched = matched || twoPageEntryOptionalReferenceMatch(field, value, choice)
			}
		}
		acknowledgement, acknowledged := accepted[field.column]
		if acknowledged {
			if acknowledgement.ContextID != original.ContextID || acknowledgement.Project != original.Project ||
				acknowledgement.Plot != original.Plot || acknowledgement.Form != original.Form ||
				acknowledgement.ContextID != assignment.ContextID || acknowledgement.Table != assignment.Table ||
				acknowledgement.RowID != assignment.RowID || !reflect.DeepEqual(acknowledgement.Expected, assignment.Before) ||
				!reflect.DeepEqual(acknowledgement.Value, assignment.After) || !reflect.DeepEqual(acknowledgement.Reference, fresh) {
				return fmt.Errorf("complete-entry %s acknowledgement differs from the owned original, literal draft or fresh reference", field.column)
			}
			delete(accepted, field.column)
		}
		if !matched && !acknowledged {
			return fmt.Errorf("complete-entry %s unmatched or unavailable reference requires explicit current-value acknowledgement", field.column)
		}
	}
	if len(accepted) != 0 {
		return errors.New("complete-entry acknowledgement contains unused or cleared reference targets")
	}
	return ctx.Err()
}
