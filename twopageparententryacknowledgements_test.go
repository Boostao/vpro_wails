package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func twoPageEntryOptionalFixture(column string, value ProjectMetadataCell) (*siviParentProjection, siviParentScalarAssignment, SIVIParentSharedReference, twoPageEntryCodeAcknowledgement) {
	original := &siviParentProjection{ContextID: "owned", Project: "Sample", Plot: "108050", Form: "FS882-8x6XL",
		Rows: []SIVIParentRow{{}}}
	assignment := siviParentScalarAssignment{ContextID: "owned", Table: "Sample_Env", RowID: "1", Column: column,
		Before: metadataText("historical"), After: value}
	reference := SIVIParentSharedReference{Column: column, ListName: column, Available: true,
		Source: "independent current source", Definitions: ProjectMetadataTable{
			Columns: []ProjectMetadataColumn{{Name: "Item", DeclaredType: "TEXT"}}, Rows: []ProjectMetadataRow{}},
		Choices: []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("XX")}}
	acknowledgement := twoPageEntryCodeAcknowledgement{ContextID: original.ContextID, Project: original.Project,
		Plot: original.Plot, Form: original.Form, Table: assignment.Table, RowID: assignment.RowID, Column: column,
		Expected: assignment.Before, Value: value, Reference: reference}
	return original, assignment, reference, acknowledgement
}

func TestTwoPageEntryOptionalAcknowledgementLiteralValueAndFreshIdentity(t *testing.T) {
	original, assignment, reference, acknowledgement := twoPageEntryOptionalFixture("Zone", metadataText("Ab"))
	before := *original
	tx := &sql.Tx{}
	reader := func(context.Context, *sql.Tx, twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
		return reference, nil
	}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original,
		[]siviParentScalarAssignment{assignment}, nil, reader); err == nil {
		t.Fatal("unmatched raw code was approved without acknowledgement")
	}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original,
		[]siviParentScalarAssignment{assignment}, []twoPageEntryCodeAcknowledgement{acknowledgement}, reader); err != nil {
		t.Fatal("exact original/draft/reference acknowledgement rejected", err)
	}
	if !reflect.DeepEqual(*original, before) || *assignment.After.Text != "Ab" {
		t.Fatal("approval normalized the original or literal case")
	}
	for _, failure := range []string{"context", "project", "plot", "form", "table", "row", "expected", "value",
		"source", "description", "NULL metadata", "empty metadata", "availability", "duplicate", "mandatory", "foreign", "unused"} {
		current := acknowledgement
		current.Reference = reference
		current.Reference.Choices = append([]SIVIParentSharedReferenceChoice(nil), reference.Choices...)
		currentAssignment := assignment
		switch failure {
		case "context":
			current.ContextID = "another editor"
		case "project":
			current.Project = "another project"
		case "plot":
			current.Plot = "another plot"
		case "form":
			current.Form = "FS882-8x6XL-CHARS"
		case "table":
			current.Table = "Sample_Admin"
		case "row":
			current.RowID = "2"
		case "expected":
			current.Expected = metadataText("another original")
		case "value":
			current.Value = metadataText("another draft")
		case "source":
			current.Reference.Source = "another source context"
		case "description":
			text := "changed description"
			current.Reference.Choices[0].Description = &text
		case "NULL metadata":
			current.Reference.Choices[0].Description = nil
			current.Reference.Definitions.Rows = []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{{Storage: "null"}}}}
		case "empty metadata":
			current.Reference.Definitions.Rows = []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{metadataText("")}}}
		case "availability":
			current.Reference.Available = false
			current.Reference.Diagnostic = "catalogue unavailable"
		case "mandatory":
			current.Column = "Exposure1"
		case "foreign":
			current.Column = "FieldNumber"
		case "unused":
			currentAssignment.Column = "FieldNumber"
		}
		acknowledgements := []twoPageEntryCodeAcknowledgement{current}
		if failure == "duplicate" {
			acknowledgements = append(acknowledgements, current)
		}
		if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original,
			[]siviParentScalarAssignment{currentAssignment}, acknowledgements, reader); err == nil {
			t.Fatal("stale/foreign acknowledgement accepted", failure)
		}
	}
	reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("aB"), twoPageEntryReferenceChoice("aB")}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original,
		[]siviParentScalarAssignment{assignment}, nil, reader); err != nil {
		t.Fatal("existing ASCII-insensitive optional matching changed or collapsed duplicates", err)
	}
}

func TestTwoPageEntryOptionalAcknowledgementAvailabilityCancellationAndClearing(t *testing.T) {
	original, assignment, reference, acknowledgement := twoPageEntryOptionalFixture("Zone", metadataText("Ab"))
	tx := &sql.Tx{}
	for _, failure := range []string{"read", "identity", "policy", "unexplained availability", "cancel"} {
		ctx, cancel := context.WithCancel(context.Background())
		reader := func(context.Context, *sql.Tx, twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
			current := reference
			switch failure {
			case "read":
				return SIVIParentSharedReference{}, errors.New("reference read failed")
			case "identity":
				current.ListName = "another source"
			case "policy":
				current.Required = true
			case "unexplained availability":
				current.Available = false
			case "cancel":
				cancel()
			}
			return current, nil
		}
		err := approveTwoPageEntryOptionalReferences(ctx, tx, original, []siviParentScalarAssignment{assignment},
			[]twoPageEntryCodeAcknowledgement{acknowledgement}, reader)
		cancel()
		if err == nil || (failure == "cancel" && !errors.Is(err, context.Canceled)) {
			t.Fatal("optional reference error or cancellation was hidden", failure, err)
		}
	}
	reference.Available, reference.Diagnostic = false, "choices unavailable; raw value not checked"
	acknowledgement.Reference = reference
	reader := func(context.Context, *sql.Tx, twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
		return reference, nil
	}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original, []siviParentScalarAssignment{assignment},
		[]twoPageEntryCodeAcknowledgement{acknowledgement}, reader); err != nil {
		t.Fatal("explicitly acknowledged unavailable optional catalogue rejected", err)
	}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original, []siviParentScalarAssignment{assignment},
		nil, reader); err == nil {
		t.Fatal("unavailable optional code silently approved")
	}
	cleared := assignment
	cleared.After = ProjectMetadataCell{Storage: "null"}
	reads := 0
	reader = func(context.Context, *sql.Tx, twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
		reads++
		return reference, nil
	}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original, []siviParentScalarAssignment{cleared},
		nil, reader); err != nil || reads != 0 {
		t.Fatal("NULL clearing unnecessarily read an optional catalogue", reads, err)
	}
	if err := approveTwoPageEntryOptionalReferences(context.Background(), tx, original, []siviParentScalarAssignment{cleared},
		[]twoPageEntryCodeAcknowledgement{acknowledgement}, reader); err == nil {
		t.Fatal("cleared draft reused an old acknowledgement")
	}
	for _, missing := range []string{"tx", "original", "rows", "reader"} {
		currentTx, currentOriginal, currentReader := tx, original, twoPageEntryCodeReader(reader)
		if missing == "tx" {
			currentTx = nil
		} else if missing == "original" {
			currentOriginal = nil
		} else if missing == "rows" {
			copy := *original
			copy.Rows = nil
			currentOriginal = &copy
		} else {
			currentReader = nil
		}
		if err := approveTwoPageEntryOptionalReferences(context.Background(), currentTx, currentOriginal, nil, nil, currentReader); err == nil {
			t.Fatal("optional boundary accepted missing ownership", missing)
		}
	}
}

func TestTwoPageEntryOptionalReferenceNumberAndTextDomains(t *testing.T) {
	fields, err := twoPageEntryReferenceFields("FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range fields {
		if field.column == "LocationAccuracy" {
			for _, raw := range []string{"-32768", "0", "32767"} {
				if got, err := twoPageEntryReferenceCode(field, metadataInteger(raw)); err != nil || got != raw {
					t.Fatal("accepted INTEGER reference changed representation", raw, got, err)
				}
			}
			for _, raw := range []string{"32768", "-32769", "01", "+1", "1.0"} {
				if _, err := twoPageEntryReferenceCode(field, metadataInteger(raw)); err == nil {
					t.Fatal("invalid/noncanonical numeric reference accepted", raw)
				}
			}
			if !twoPageEntryOptionalReferenceMatch(field, "1", twoPageEntryReferenceChoice("1")) ||
				twoPageEntryOptionalReferenceMatch(field, "1", twoPageEntryReferenceChoice("01")) {
				t.Fatal("numeric catalogue representation silently rewritten")
			}
		}
		if field.column == "Zone" {
			for _, value := range []ProjectMetadataCell{metadataText(""), metadataText("12345"), metadataText(string([]byte{0xff})),
				metadataInteger("1")} {
				if _, err := twoPageEntryReferenceCode(field, value); err == nil {
					t.Fatal("optional text physical failure bypassed approval", value)
				}
			}
			if twoPageEntryOptionalReferenceMatch(field, "k", twoPageEntryReferenceChoice("\u212a")) {
				t.Fatal("Unicode folding widened established ASCII matching")
			}
		}
	}
}
