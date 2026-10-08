package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func twoPageEntryBorrowedReferenceReaders(t *testing.T) siviParentSharedReferenceReaders {
	t.Helper()
	dir := t.TempDir()
	site, err := NewSiteCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := NewParentCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(site.Close(), parent.Close()); err != nil {
			t.Error(err)
		}
	})
	return siviParentSharedReferenceReaders{site: site, parent: parent}
}

func twoPageEntryConfiguredRequiredEdits(t *testing.T, contexts *ContextService, state ProjectState,
	parent *siviParentProjection, readers siviParentSharedReferenceReaders) []siviParentScalarEdit {
	t.Helper()
	fields, err := twoPageEntryRequiredReferenceFields(parent.Form)
	if err != nil {
		t.Fatal(err)
	}
	references := &SIVIParentSharedService{references: readers}
	edits, err := withContextPlotRequest(context.Background(), contexts, state.ContextID,
		func(plots *PlotService) ([]siviParentScalarEdit, error) {
			return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) ([]siviParentScalarEdit, error) {
				edits := []siviParentScalarEdit{}
				for _, field := range fields {
					reference, err := references.readReference(context.Background(), tx, "VLists", field, ProjectMetadataCell{Storage: "null"})
					if err != nil {
						return nil, err
					}
					found := false
					for _, choice := range reference.Choices {
						if choice.Selectable && choice.Code != nil {
							edits = append(edits, siviParentEdit(t, parent, field.column, metadataText(*choice.Code)))
							found = true
							break
						}
					}
					if !found {
						return nil, errors.New("configured mandatory reference has no selectable Item: " + field.column)
					}
				}
				return edits, nil
			})
		})
	if err != nil {
		t.Fatal(err)
	}
	return edits
}

func TestTwoPageEntryConfiguredRequiredReferencesOneOwnedWriteAndRestore(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			contexts, state, db, parent, _ := twoPageWriteFixture(t, form, external)
			readers := twoPageEntryBorrowedReferenceReaders(t)
			edits := append(twoPageEntryWriteEdits(t, parent),
				twoPageEntryConfiguredRequiredEdits(t, contexts, state, parent, readers)...)
			before := twoPageWriteTables(t, db)
			beforeReferences := databaseBytes(t, map[string]string{"VLists": contexts.projects.sqlite.attachments["VLists"]})
			written, err := contexts.writeTwoPageEntryWithRequiredReferences(context.Background(), state.ContextID,
				parent.Plot, form, parent, edits, false, readers, twoPageEntryApprovePhysical)
			if err != nil || written == nil || written.ChangedCells != 9 || written.HistoryID == "" {
				t.Fatal("required source references did not join one owned transaction", form, external, written, err)
			}
			if got := databaseBytes(t, map[string]string{"VLists": contexts.projects.sqlite.attachments["VLists"]}); !reflect.DeepEqual(beforeReferences, got) {
				t.Fatal("reference approval wrote the configured family")
			}
			if restored, err := contexts.restoreTwoPageEntry(context.Background(), state.ContextID, parent.Plot,
				form, written.HistoryID, AuditRestorePrune, false); err != nil || restored == nil || restored.RestoredRows != 9 {
				t.Fatal("source reference transaction could not restore", restored, err)
			}
			after := twoPageWriteTables(t, db)
			for _, table := range []string{"Sample_Env", "Sample_Admin"} {
				if !reflect.DeepEqual(before[table], after[table]) {
					t.Fatal("reference-backed restoration changed original parents", table)
				}
			}
		}
	}
}

func TestTwoPageEntryConfiguredClosedCatalogueAndNullClearing(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", false)
	readers := twoPageEntryBorrowedReferenceReaders(t)
	edits := twoPageEntryConfiguredRequiredEdits(t, contexts, state, parent, readers)
	site, ok := readers.site.(*SiteCodeService)
	if !ok {
		t.Fatal("fixture has no actual borrowed site catalogue")
	}
	if err := site.Close(); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := contexts.writeTwoPageEntryWithRequiredReferences(context.Background(), state.ContextID,
		parent.Plot, parent.Form, parent, edits, false, readers, twoPageEntryApprovePhysical); err == nil || got != nil {
		t.Fatal("closed borrowed catalogue approved mandatory membership", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if got, err := contexts.writeTwoPageEntryWithRequiredReferences(context.Background(), state.ContextID,
		parent.Plot, parent.Form, parent, []siviParentScalarEdit{siviParentEdit(t, parent, "Exposure1", ProjectMetadataCell{Storage: "null"})},
		false, readers, twoPageEntryApprovePhysical); err != nil || got == nil {
		t.Fatal("closed borrowed catalogue prevented explicit NULL clearing", got, err)
	}
}

func TestTwoPageEntryLifecycleUsesOwnedSnapshotAndRollsBackVerification(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	var prepared *PlotService
	prepareCount, verifyCount, releaseCount := 0, 0, 0
	lifecycle := twoPageEntryLifecycle{
		prepare: func(plots *PlotService, conn *sql.Conn) (func(), error) {
			prepareCount++
			if conn == nil || plots == contexts.plots || plots.projects.sqlite != contexts.projects.sqlite {
				t.Fatal("entry preparation lost the immutable request/owned connection", plots, conn)
			}
			prepared = plots
			return func() { releaseCount++ }, nil
		},
		verify: func(plots *PlotService, tx *sql.Tx) error {
			verifyCount++
			if plots != prepared || tx == nil {
				t.Fatal("entry verification lost its prepared owned snapshot", plots, tx)
			}
			return errors.New("reference ownership verification failed")
		},
	}
	if got, err := contexts.writeTwoPageEntryWithLifecycle(context.Background(), state.ContextID, parent.Plot,
		parent.Form, parent, twoPageEntryWriteEdits(t, parent), false, twoPageEntryApprovePhysical, lifecycle); err == nil || got != nil {
		t.Fatal("late reference ownership failure committed", got, err)
	}
	if prepareCount != 1 || verifyCount != 1 || releaseCount != 1 {
		t.Fatal("entry reference lifecycle lost cleanup or duplicated ownership", prepareCount, verifyCount, releaseCount)
	}
	assertProfileSUFiles(t, contexts, before)
	lifecycle.verify = nil
	if got, err := contexts.writeTwoPageEntryWithLifecycle(context.Background(), state.ContextID, parent.Plot,
		parent.Form, parent, twoPageEntryWriteEdits(t, parent), false, twoPageEntryApprovePhysical, lifecycle); err != nil || got == nil {
		t.Fatal("entry ownership verification failure prevented retry", got, err)
	}
	if prepareCount != 2 || releaseCount != 2 {
		t.Fatal("entry lifecycle leaked preparation on retry", prepareCount, releaseCount)
	}
}

func TestTwoPageEntryConfiguredRequiredReferenceFailuresRollbackAndRetry(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	readers := twoPageEntryBorrowedReferenceReaders(t)
	edits := append(twoPageEntryWriteEdits(t, parent), twoPageEntryConfiguredRequiredEdits(t, contexts, state, parent, readers)...)
	for _, failure := range []string{"nil approval", "missing site", "missing parent", "unlisted", "approval", "cancel"} {
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		ctx, cancel := context.WithCancel(context.Background())
		currentReaders := readers
		currentEdits := append([]siviParentScalarEdit(nil), edits...)
		approval := twoPageEntryApproval(twoPageEntryApprovePhysical)
		switch failure {
		case "nil approval":
			approval = nil
		case "missing site":
			currentReaders.site = nil
		case "missing parent":
			currentReaders.parent = nil
		case "unlisted":
			currentEdits[len(currentEdits)-1] = siviParentEdit(t, parent, "SurfaceShape", metadataText("ZZZ"))
		case "approval":
			approval = func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error {
				return errors.New("independent optional approval failed")
			}
		case "cancel":
			approval = func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error {
				cancel()
				return ctx.Err()
			}
		}
		got, err := contexts.writeTwoPageEntryWithRequiredReferences(ctx, state.ContextID, parent.Plot,
			parent.Form, parent, currentEdits, false, currentReaders, approval)
		cancel()
		if err == nil || got != nil {
			t.Fatal("configured reference/approval failure committed", failure, got, err)
		}
		if failure == "cancel" && !errors.Is(err, context.Canceled) {
			t.Fatal("configured reference cancellation lost its identity", err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	written, err := contexts.writeTwoPageEntryWithRequiredReferences(context.Background(), state.ContextID,
		parent.Plot, parent.Form, parent, edits, false, readers, twoPageEntryApprovePhysical)
	if err != nil || written == nil || written.ChangedCells != 9 {
		t.Fatal("reference-denied writes left an attachment/transaction that prevented retry", written, err)
	}
}
