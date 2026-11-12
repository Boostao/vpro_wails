package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestTwoPageEntryRequiredReferenceSourceScope(t *testing.T) {
	want := []sourceform.SiviParentSharedReferencePolicy{
		{Column: "Exposure1", List: "Exposure", Reader: "site", Maximum: 2, Required: true},
		{Column: "Exposure2", List: "Exposure", Reader: "site", Maximum: 2, Required: true},
		{Column: "MesoSlopePosition", List: "MesoSlopePosition", Reader: "family", Maximum: 3, Required: true},
		{Column: "SoilDrainage", List: "SoilDrainage", Reader: "parent", Maximum: 5, Required: true},
		{Column: "SuccessionalStatus", List: "SuccessionalStatus", Reader: "family", Maximum: 3, Required: true},
		{Column: "SurfaceShape", List: "SurfaceShape", Reader: "family", Maximum: 3, Required: true},
	}
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		fields, err := twopage.TwoPageEntryRequiredReferenceFields(form)
		if err != nil || !reflect.DeepEqual(fields, want) {
			t.Fatal("source-specific mandatory scope changed", form, fields, err)
		}
	}
	for _, column := range []string{"MesoSlopePosition", "SurfaceShape"} {
		field, exists := sourceform.SiviParentSharedReferenceField(column)
		if !exists || field.Required {
			t.Fatal("two-page policy mutated the different SIVI source", column, field)
		}
	}
	if got, err := twopage.TwoPageEntryRequiredReferenceFields("FS882-6x4XL"); err == nil || got != nil {
		t.Fatal("foreign source inherited required reference scope", got, err)
	}
}

func TestTwoPageEntryRequiredReferenceApprovalExactMembershipAndFailures(t *testing.T) {
	parent := &sourceform.SiviParentProjection{Form: "FS882-8x6XL"}
	tx := &sql.Tx{}
	reader := func(context.Context, *sql.Tx, sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
		t.Fatal("unexpected reference read")
		return sourceform.SIVIParentSharedReference{}, nil
	}
	if err := twopage.ApproveTwoPageEntryRequiredReferences(context.Background(), tx, parent, []sourceform.SiviParentScalarAssignment{
		{Column: "SurfaceShape", Value: nil}, {Column: "FieldNumber", Value: "literal"},
	}, reader); err != nil {
		t.Fatal("NULL clearing or unrelated assignment needs no reference lookup", err)
	}
	for _, column := range []string{"Exposure1", "Exposure2", "MesoSlopePosition", "SoilDrainage", "SuccessionalStatus", "SurfaceShape"} {
		for _, failure := range []string{"", "prefix", "case", "unlisted", "NULL", "empty", "nonselectable",
			"overlength", "malformed", "column", "list", "policy", "unavailable", "read", "cancel", "integer"} {
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			reader := func(ctx context.Context, observed *sql.Tx, field sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
				calls++
				if observed != tx || field.Column != column || !field.Required {
					t.Fatal("reference read lost the owned transaction/source policy", observed, field)
				}
				reference := sourceform.SIVIParentSharedReference{Column: column, ListName: field.List, Required: true,
					Available: true, Choices: []sourceform.SIVIParentSharedReferenceChoice{
						sftest.TwoPageEntryReferenceChoice("AB"), sftest.TwoPageEntryReferenceChoice("AB"),
					}}
				switch failure {
				case "prefix":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice("ABC")}
				case "case":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice("ab")}
				case "unlisted":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice("XY")}
				case "NULL":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{{Selectable: true}}
				case "empty":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice("")}
				case "nonselectable":
					for i := range reference.Choices {
						reference.Choices[i].Selectable = false
					}
				case "overlength":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice("123456789")}
				case "malformed":
					reference.Choices = []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice(string([]byte{0xff}))}
				case "column":
					reference.Column = "another field"
				case "list":
					reference.ListName = "another list"
				case "policy":
					reference.Required = false
				case "unavailable":
					reference.Available, reference.Diagnostic = false, "catalogue closed"
				case "read":
					return sourceform.SIVIParentSharedReference{}, errors.New("reference read failed")
				case "cancel":
					cancel()
				}
				return reference, nil
			}
			var value any = "AB"
			if failure == "integer" {
				value = 1
			}
			err := twopage.ApproveTwoPageEntryRequiredReferences(ctx, tx, parent, []sourceform.SiviParentScalarAssignment{{Column: column, Value: value}}, reader)
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
		currentTx, original, currentReader := tx, parent, twopage.TwoPageEntryReferenceReader(reader)
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
		if err := twopage.ApproveTwoPageEntryRequiredReferences(ctx, currentTx, original, nil, currentReader); err == nil {
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
		edits := append(twoPageEntryWriteEdits(t, parent), sftest.SiviParentEdit(t, parent, "MesoSlopePosition", vprotest.MetadataText("AB")))
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		reader := func(ctx context.Context, tx *sql.Tx, field sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
			var code string
			if err := tx.QueryRowContext(ctx, `SELECT Code FROM EntryReferenceFixture`).Scan(&code); err != nil {
				return sourceform.SIVIParentSharedReference{}, err
			}
			return sourceform.SIVIParentSharedReference{Column: field.Column, ListName: field.List, Required: true, Available: true,
				Choices: []sourceform.SIVIParentSharedReferenceChoice{sftest.TwoPageEntryReferenceChoice(code)}}, nil
		}
		approval := func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			return twopage.ApproveTwoPageEntryRequiredReferences(context.Background(), tx, observed, assignments, reader)
		}
		if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
			parent, edits, false, approval); err == nil || got != nil {
			t.Fatal("required source membership permitted a partial mixed commit", got, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
		if _, err := db.Exec(`UPDATE EntryReferenceFixture SET Code='AB'`); err != nil {
			t.Fatal(err)
		}
		written, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
			parent, edits, false, approval)
		if err != nil || written == nil || written.ChangedCells != 4 {
			t.Fatal("independent transaction-held membership correction could not retry", written, err)
		}
	}
}
