package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestTwoPageEntryRequiredReferenceSourceScope(t *testing.T) {
	want := []siviParentSharedReferencePolicy{
		{"Exposure1", "Exposure", "site", 2, true},
		{"Exposure2", "Exposure", "site", 2, true},
		{"MesoSlopePosition", "MesoSlopePosition", "family", 3, true},
		{"SoilDrainage", "SoilDrainage", "parent", 5, true},
		{"SuccessionalStatus", "SuccessionalStatus", "family", 3, true},
		{"SurfaceShape", "SurfaceShape", "family", 3, true},
	}
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		fields, err := twoPageEntryRequiredReferenceFields(form)
		if err != nil || !reflect.DeepEqual(fields, want) {
			t.Fatal("source-specific mandatory scope changed", form, fields, err)
		}
	}
	for _, column := range []string{"MesoSlopePosition", "SurfaceShape"} {
		field, exists := siviParentSharedReferenceField(column)
		if !exists || field.required {
			t.Fatal("two-page policy mutated the different SIVI source", column, field)
		}
	}
	if got, err := twoPageEntryRequiredReferenceFields("FS882-6x4XL"); err == nil || got != nil {
		t.Fatal("foreign source inherited required reference scope", got, err)
	}
}

func twoPageEntryReferenceChoice(value string) SIVIParentSharedReferenceChoice {
	return SIVIParentSharedReferenceChoice{Code: &value, Selectable: true}
}

func TestTwoPageEntryRequiredReferenceApprovalExactMembershipAndFailures(t *testing.T) {
	parent := &siviParentProjection{Form: "FS882-8x6XL"}
	tx := &sql.Tx{}
	reader := func(context.Context, *sql.Tx, siviParentSharedReferencePolicy) (SIVIParentSharedReference, error) {
		t.Fatal("unexpected reference read")
		return SIVIParentSharedReference{}, nil
	}
	if err := approveTwoPageEntryRequiredReferences(context.Background(), tx, parent, []siviParentScalarAssignment{
		{Column: "SurfaceShape", Value: nil}, {Column: "FieldNumber", Value: "literal"},
	}, reader); err != nil {
		t.Fatal("NULL clearing or unrelated assignment needs no reference lookup", err)
	}
	for _, column := range []string{"Exposure1", "Exposure2", "MesoSlopePosition", "SoilDrainage", "SuccessionalStatus", "SurfaceShape"} {
		for _, failure := range []string{"", "prefix", "case", "unlisted", "NULL", "empty", "nonselectable",
			"overlength", "malformed", "column", "list", "policy", "unavailable", "read", "cancel", "integer"} {
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			reader := func(ctx context.Context, observed *sql.Tx, field siviParentSharedReferencePolicy) (SIVIParentSharedReference, error) {
				calls++
				if observed != tx || field.column != column || !field.required {
					t.Fatal("reference read lost the owned transaction/source policy", observed, field)
				}
				reference := SIVIParentSharedReference{Column: column, ListName: field.list, Required: true,
					Available: true, Choices: []SIVIParentSharedReferenceChoice{
						twoPageEntryReferenceChoice("AB"), twoPageEntryReferenceChoice("AB"),
					}}
				switch failure {
				case "prefix":
					reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("ABC")}
				case "case":
					reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("ab")}
				case "unlisted":
					reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("XY")}
				case "NULL":
					reference.Choices = []SIVIParentSharedReferenceChoice{{Selectable: true}}
				case "empty":
					reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("")}
				case "nonselectable":
					for i := range reference.Choices {
						reference.Choices[i].Selectable = false
					}
				case "overlength":
					reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice("123456789")}
				case "malformed":
					reference.Choices = []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice(string([]byte{0xff}))}
				case "column":
					reference.Column = "another field"
				case "list":
					reference.ListName = "another list"
				case "policy":
					reference.Required = false
				case "unavailable":
					reference.Available, reference.Diagnostic = false, "catalogue closed"
				case "read":
					return SIVIParentSharedReference{}, errors.New("reference read failed")
				case "cancel":
					cancel()
				}
				return reference, nil
			}
			var value any = "AB"
			if failure == "integer" {
				value = 1
			}
			err := approveTwoPageEntryRequiredReferences(ctx, tx, parent, []siviParentScalarAssignment{{Column: column, Value: value}}, reader)
			cancel()
			if failure == "" && (err != nil || calls != 1) {
				t.Fatal("exact duplicate definitions should remain selectable", column, calls, err)
			}
			if failure != "" && err == nil {
				t.Fatal("required membership accepted invalid reference", column, failure)
			}
			if failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("reference cancellation lost its identity", err)
			}
		}
	}
	for _, missing := range []string{"tx", "original", "reader", "cancel"} {
		ctx, cancel := context.WithCancel(context.Background())
		currentTx, original, currentReader := tx, parent, twoPageEntryReferenceReader(reader)
		switch missing {
		case "tx":
			currentTx = nil
		case "original":
			original = nil
		case "reader":
			currentReader = nil
		case "cancel":
			cancel()
		}
		if err := approveTwoPageEntryRequiredReferences(ctx, currentTx, original, nil, currentReader); err == nil {
			t.Fatal("missing authority boundary was accepted", missing)
		}
		cancel()
	}
}

func TestTwoPageEntryRequiredReferenceApprovalSharesAtomicRollbackAndRetry(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		contexts, state, db, parent, _ := twoPageWriteFixture(t, form, false)
		if _, err := db.Exec(`CREATE TABLE EntryReferenceFixture (Code TEXT); INSERT INTO EntryReferenceFixture VALUES('XY')`); err != nil {
			t.Fatal(err)
		}
		edits := append(twoPageEntryWriteEdits(t, parent), siviParentEdit(t, parent, "MesoSlopePosition", metadataText("AB")))
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		reader := func(ctx context.Context, tx *sql.Tx, field siviParentSharedReferencePolicy) (SIVIParentSharedReference, error) {
			var code string
			if err := tx.QueryRowContext(ctx, `SELECT Code FROM EntryReferenceFixture`).Scan(&code); err != nil {
				return SIVIParentSharedReference{}, err
			}
			return SIVIParentSharedReference{Column: field.column, ListName: field.list, Required: true, Available: true,
				Choices: []SIVIParentSharedReferenceChoice{twoPageEntryReferenceChoice(code)}}, nil
		}
		approval := func(tx *sql.Tx, observed *siviParentProjection, assignments []siviParentScalarAssignment) error {
			return approveTwoPageEntryRequiredReferences(context.Background(), tx, observed, assignments, reader)
		}
		if got, err := contexts.writeTwoPageEntry(context.Background(), state.ContextID, parent.Plot, form,
			parent, edits, false, approval); err == nil || got != nil {
			t.Fatal("required source membership permitted a partial mixed commit", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
		if _, err := db.Exec(`UPDATE EntryReferenceFixture SET Code='AB'`); err != nil {
			t.Fatal(err)
		}
		written, err := contexts.writeTwoPageEntry(context.Background(), state.ContextID, parent.Plot, form,
			parent, edits, false, approval)
		if err != nil || written == nil || written.ChangedCells != 4 {
			t.Fatal("independent transaction-held membership correction could not retry", written, err)
		}
	}
}
