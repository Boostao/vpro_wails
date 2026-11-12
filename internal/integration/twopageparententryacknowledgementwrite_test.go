package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestTwoPageEntryOptionalAcknowledgementActualCatalogueAtomicWriteAndRestore(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		contexts, state, db, parent, _ := twoPageWriteFixture(t, form, false)
		readers := sftest.TwoPageEntryBorrowedReferenceReaders(t)
		references := &sourceform.SIVIParentSharedService{References: readers}
		fresh, err := vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID,
			func(plots *vprocore.PlotService) (sourceform.SIVIParentSharedReference, error) {
				return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (sourceform.SIVIParentSharedReference, error) {
					return references.ReadReference(context.Background(), tx, "VLists",
						sourceform.SiviParentSharedReferencePolicy{Column: "RealmClass", List: "RealmClass", Reader: "parent", Maximum: 5, Required: false},
						vprocore.ProjectMetadataCell{Storage: "null"})
				})
			})
		if err != nil {
			t.Fatal(err)
		}
		for _, choice := range fresh.Choices {
			if choice.Code != nil && *choice.Code == "ZZZZZ" {
				t.Fatal("independent unlisted test code became a listed source Item")
			}
		}
		edits := []sourceform.SiviParentScalarEdit{
			sftest.SiviParentEdit(t, parent, "FieldNumber", vprotest.MetadataText("  Literal field  ")),
			sftest.SiviParentEdit(t, parent, "SV_PolygonNumber", vprotest.MetadataText("  Literal polygon  ")),
			sftest.SiviParentEdit(t, parent, "GIS_BGC", vprotest.MetadataText("  Literal GIS  ")),
			sftest.SiviParentEdit(t, parent, "RealmClass", vprotest.MetadataText("ZZZZZ")),
		}
		assignments, err := twopage.PlanTwoPageEntryProjection(context.Background(), parent, edits, false)
		if err != nil {
			t.Fatal(err)
		}
		var target *sourceform.SiviParentScalarAssignment
		for i := range assignments {
			if assignments[i].Column == "RealmClass" {
				target = &assignments[i]
			}
		}
		if target == nil {
			t.Fatal("expected independent optional reference assignment missing")
		}
		acknowledgement := twopage.TwoPageEntryCodeAcknowledgementInternal{ContextID: parent.ContextID, Project: parent.Project,
			Plot: parent.Plot, Form: parent.Form, Table: target.Table, RowID: target.RowID, Column: target.Column,
			Expected: target.Before, Value: target.After, Reference: fresh}
		reader := func(ctx context.Context, tx *sql.Tx, field twopage.TwoPageEntryReferenceField) (sourceform.SIVIParentSharedReference, error) {
			return references.ReadReference(ctx, tx, "two_page_entry_refs", field.SiviParentSharedReferencePolicy,
				vprocore.ProjectMetadataCell{Storage: "null"})
		}
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		for _, failure := range []string{"missing acknowledgement", "old draft", "old catalogue", "read failure"} {
			current := acknowledgement
			acknowledgements := []twopage.TwoPageEntryCodeAcknowledgementInternal{current}
			currentReader := twopage.TwoPageEntryCodeReader(reader)
			switch failure {
			case "missing acknowledgement":
				acknowledgements = nil
			case "old draft":
				acknowledgements[0].Value = vprotest.MetadataText("other")
			case "old catalogue":
				acknowledgements[0].Reference.Source = "old source context"
			case "read failure":
				currentReader = func(context.Context, *sql.Tx, twopage.TwoPageEntryReferenceField) (sourceform.SIVIParentSharedReference, error) {
					return sourceform.SIVIParentSharedReference{}, errors.New("independent reference read failed")
				}
			}
			approve := func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
				return twopage.ApproveTwoPageEntryOptionalReferences(context.Background(), tx, observed, assignments, acknowledgements, currentReader)
			}
			if got, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, context.Background(), state.ContextID, parent.Plot,
				form, parent, edits, false, readers, approve); err == nil || got != nil {
				t.Fatal("optional acknowledgement failure leaked a mixed commit", form, failure, got, err)
			}
			vprotest.AssertProfileSUFiles(t, contexts, before)
		}
		originalTables := vprotest.TwoPageWriteTables(t, db)
		approve := func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			return twopage.ApproveTwoPageEntryOptionalReferences(context.Background(), tx, observed, assignments,
				[]twopage.TwoPageEntryCodeAcknowledgementInternal{acknowledgement}, reader)
		}
		written, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, context.Background(), state.ContextID, parent.Plot,
			form, parent, edits, false, readers, approve)
		if err != nil || written == nil || written.ChangedCells != 4 || written.HistoryID == "" {
			t.Fatal("current explicit acknowledgement did not permit one literal mixed commit", form, written, err)
		}
		freshParent, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, parent.Plot, form)
		if err != nil {
			t.Fatal(err)
		}
		for _, edit := range edits {
			if !reflect.DeepEqual(sftest.SiviParentCell(t, freshParent, edit.Column), edit.Value) {
				t.Fatal("optional approval normalized literal source storage", edit)
			}
		}
		if restored, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot,
			form, written.HistoryID, vprocore.AuditRestorePrune, false); err != nil || restored == nil || restored.RestoredRows != 4 {
			t.Fatal("acknowledged unlisted source history could not restore", restored, err)
		}
		restoredTables := vprotest.TwoPageWriteTables(t, db)
		for _, table := range []string{"Sample_Env", "Sample_Admin"} {
			if !reflect.DeepEqual(originalTables[table], restoredTables[table]) {
				t.Fatal("optional acknowledgement restoration changed original parents", table)
			}
		}
	}
}
