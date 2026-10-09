package main

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
		readers := twoPageEntryBorrowedReferenceReaders(t)
		references := &SIVIParentSharedService{references: readers}
		fresh, err := withContextPlotRequest(context.Background(), contexts, state.ContextID,
			func(plots *PlotService) (SIVIParentSharedReference, error) {
				return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (SIVIParentSharedReference, error) {
					return references.readReference(context.Background(), tx, "VLists",
						siviParentSharedReferencePolicy{"RealmClass", "RealmClass", "parent", 5, false},
						ProjectMetadataCell{Storage: "null"})
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
		edits := []siviParentScalarEdit{
			siviParentEdit(t, parent, "FieldNumber", metadataText("  Literal field  ")),
			siviParentEdit(t, parent, "SV_PolygonNumber", metadataText("  Literal polygon  ")),
			siviParentEdit(t, parent, "GIS_BGC", metadataText("  Literal GIS  ")),
			siviParentEdit(t, parent, "RealmClass", metadataText("ZZZZZ")),
		}
		assignments, err := planTwoPageEntryProjection(context.Background(), parent, edits, false)
		if err != nil {
			t.Fatal(err)
		}
		var target *siviParentScalarAssignment
		for i := range assignments {
			if assignments[i].Column == "RealmClass" {
				target = &assignments[i]
			}
		}
		if target == nil {
			t.Fatal("expected independent optional reference assignment missing")
		}
		acknowledgement := twoPageEntryCodeAcknowledgement{ContextID: parent.ContextID, Project: parent.Project,
			Plot: parent.Plot, Form: parent.Form, Table: target.Table, RowID: target.RowID, Column: target.Column,
			Expected: target.Before, Value: target.After, Reference: fresh}
		reader := func(ctx context.Context, tx *sql.Tx, field twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
			return references.readReference(ctx, tx, "two_page_entry_refs", field.siviParentSharedReferencePolicy,
				ProjectMetadataCell{Storage: "null"})
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		for _, failure := range []string{"missing acknowledgement", "old draft", "old catalogue", "read failure"} {
			current := acknowledgement
			acknowledgements := []twoPageEntryCodeAcknowledgement{current}
			currentReader := twoPageEntryCodeReader(reader)
			switch failure {
			case "missing acknowledgement":
				acknowledgements = nil
			case "old draft":
				acknowledgements[0].Value = metadataText("other")
			case "old catalogue":
				acknowledgements[0].Reference.Source = "old source context"
			case "read failure":
				currentReader = func(context.Context, *sql.Tx, twoPageEntryReferenceField) (SIVIParentSharedReference, error) {
					return SIVIParentSharedReference{}, errors.New("independent reference read failed")
				}
			}
			approve := func(tx *sql.Tx, observed *siviParentProjection, assignments []siviParentScalarAssignment) error {
				return approveTwoPageEntryOptionalReferences(context.Background(), tx, observed, assignments, acknowledgements, currentReader)
			}
			if got, err := contexts.writeTwoPageEntryWithRequiredReferences(context.Background(), state.ContextID, parent.Plot,
				form, parent, edits, false, readers, approve); err == nil || got != nil {
				t.Fatal("optional acknowledgement failure leaked a mixed commit", form, failure, got, err)
			}
			assertProfileSUFiles(t, contexts, before)
		}
		originalTables := twoPageWriteTables(t, db)
		approve := func(tx *sql.Tx, observed *siviParentProjection, assignments []siviParentScalarAssignment) error {
			return approveTwoPageEntryOptionalReferences(context.Background(), tx, observed, assignments,
				[]twoPageEntryCodeAcknowledgement{acknowledgement}, reader)
		}
		written, err := contexts.writeTwoPageEntryWithRequiredReferences(context.Background(), state.ContextID, parent.Plot,
			form, parent, edits, false, readers, approve)
		if err != nil || written == nil || written.ChangedCells != 4 || written.HistoryID == "" {
			t.Fatal("current explicit acknowledgement did not permit one literal mixed commit", form, written, err)
		}
		freshParent, err := contexts.readTwoPageParent(context.Background(), state.ContextID, parent.Plot, form)
		if err != nil {
			t.Fatal(err)
		}
		for _, edit := range edits {
			if !reflect.DeepEqual(siviParentCell(t, freshParent, edit.Column), edit.Value) {
				t.Fatal("optional approval normalized literal source storage", edit)
			}
		}
		if restored, err := contexts.restoreTwoPageEntry(context.Background(), state.ContextID, parent.Plot,
			form, written.HistoryID, AuditRestorePrune, false); err != nil || restored == nil || restored.RestoredRows != 4 {
			t.Fatal("acknowledged unlisted source history could not restore", restored, err)
		}
		restoredTables := twoPageWriteTables(t, db)
		for _, table := range []string{"Sample_Env", "Sample_Admin"} {
			if !reflect.DeepEqual(originalTables[table], restoredTables[table]) {
				t.Fatal("optional acknowledgement restoration changed original parents", table)
			}
		}
	}
}
