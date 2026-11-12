package integration

import (
	referencespkg "github.com/boostao/vpro-wails/internal/references"
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

func twoPageEntryConfiguredRequiredEdits(t *testing.T, contexts *vprocore.ContextService, state vprocore.ProjectState,
	parent *sourceform.SiviParentProjection, readers sourceform.SiviParentSharedReferenceReaders) []sourceform.SiviParentScalarEdit {
	t.Helper()
	fields, err := twopage.TwoPageEntryRequiredReferenceFields(parent.Form)
	if err != nil {
		t.Fatal(err)
	}
	references := &sourceform.SIVIParentSharedService{References: readers}
	edits, err := vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID,
		func(plots *vprocore.PlotService) ([]sourceform.SiviParentScalarEdit, error) {
			return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) ([]sourceform.SiviParentScalarEdit, error) {
				edits := []sourceform.SiviParentScalarEdit{}
				for _, field := range fields {
					reference, err := references.ReadReference(context.Background(), tx, "VLists", field, vprocore.ProjectMetadataCell{Storage: "null"})
					if err != nil {
						return nil, err
					}
					found := false
					for _, choice := range reference.Choices {
						if choice.Selectable && choice.Code != nil {
							edits = append(edits, sftest.SiviParentEdit(t, parent, field.Column, vprotest.MetadataText(*choice.Code)))
							found = true
							break
						}
					}
					if !found {
						return nil, errors.New("configured mandatory reference has no selectable Item: " + field.Column)
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
			readers := sftest.TwoPageEntryBorrowedReferenceReaders(t)
			edits := append(twoPageEntryWriteEdits(t, parent),
				twoPageEntryConfiguredRequiredEdits(t, contexts, state, parent, readers)...)
			before := vprotest.TwoPageWriteTables(t, db)
			beforeReferences := vprotest.DatabaseBytes(t, map[string]string{"VLists": contexts.Projects.Sqlite.Attachments["VLists"]})
			written, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, context.Background(), state.ContextID,
				parent.Plot, form, parent, edits, false, readers, twoPageEntryApprovePhysical)
			if err != nil || written == nil || written.ChangedCells != 9 || written.HistoryID == "" {
				t.Fatal("required source references did not join one owned transaction", form, external, written, err)
			}
			if got := vprotest.DatabaseBytes(t, map[string]string{"VLists": contexts.Projects.Sqlite.Attachments["VLists"]}); !reflect.DeepEqual(beforeReferences, got) {
				t.Fatal("reference approval wrote the configured family")
			}
			if restored, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot,
				form, written.HistoryID, vprocore.AuditRestorePrune, false); err != nil || restored == nil || restored.RestoredRows != 9 {
				t.Fatal("source reference transaction could not restore", restored, err)
			}
			after := vprotest.TwoPageWriteTables(t, db)
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
	readers := sftest.TwoPageEntryBorrowedReferenceReaders(t)
	edits := twoPageEntryConfiguredRequiredEdits(t, contexts, state, parent, readers)
	site, ok := readers.Site.(*referencespkg.SiteCodeService)
	if !ok {
		t.Fatal("fixture has no actual borrowed site catalogue")
	}
	if err := site.Close(); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, context.Background(), state.ContextID,
		parent.Plot, parent.Form, parent, edits, false, readers, twoPageEntryApprovePhysical); err == nil || got != nil {
		t.Fatal("closed borrowed catalogue approved mandatory membership", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if got, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, context.Background(), state.ContextID,
		parent.Plot, parent.Form, parent, []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, "Exposure1", vprocore.ProjectMetadataCell{Storage: "null"})},
		false, readers, twoPageEntryApprovePhysical); err != nil || got == nil {
		t.Fatal("closed borrowed catalogue prevented explicit NULL clearing", got, err)
	}
}

func TestTwoPageEntryLifecycleUsesOwnedSnapshotAndRollsBackVerification(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	var prepared *vprocore.PlotService
	prepareCount, verifyCount, releaseCount := 0, 0, 0
	lifecycle := twopage.TwoPageEntryLifecycle{
		Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
			prepareCount++
			if conn == nil || plots == contexts.Plots || plots.Projects.Sqlite != contexts.Projects.Sqlite {
				t.Fatal("entry preparation lost the immutable request/owned connection", plots, conn)
			}
			prepared = plots
			return func() { releaseCount++ }, nil
		},
		Verify: func(plots *vprocore.PlotService, tx *sql.Tx) error {
			verifyCount++
			if plots != prepared || tx == nil {
				t.Fatal("entry verification lost its prepared owned snapshot", plots, tx)
			}
			return errors.New("reference ownership verification failed")
		},
	}
	if got, err := twopage.WriteTwoPageEntryWithLifecycle(contexts, context.Background(), state.ContextID, parent.Plot,
		parent.Form, parent, twoPageEntryWriteEdits(t, parent), false, twoPageEntryApprovePhysical, lifecycle); err == nil || got != nil {
		t.Fatal("late reference ownership failure committed", got, err)
	}
	if prepareCount != 1 || verifyCount != 1 || releaseCount != 1 {
		t.Fatal("entry reference lifecycle lost cleanup or duplicated ownership", prepareCount, verifyCount, releaseCount)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	lifecycle.Verify = nil
	if got, err := twopage.WriteTwoPageEntryWithLifecycle(contexts, context.Background(), state.ContextID, parent.Plot,
		parent.Form, parent, twoPageEntryWriteEdits(t, parent), false, twoPageEntryApprovePhysical, lifecycle); err != nil || got == nil {
		t.Fatal("entry ownership verification failure prevented retry", got, err)
	}
	if prepareCount != 2 || releaseCount != 2 {
		t.Fatal("entry lifecycle leaked preparation on retry", prepareCount, releaseCount)
	}
}

func TestTwoPageEntryConfiguredRequiredReferenceFailuresRollbackAndRetry(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	readers := sftest.TwoPageEntryBorrowedReferenceReaders(t)
	edits := append(twoPageEntryWriteEdits(t, parent), twoPageEntryConfiguredRequiredEdits(t, contexts, state, parent, readers)...)
	for _, failure := range []string{"nil approval", "missing site", "missing parent", "unlisted", "approval", "cancel"} {
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		ctx, cancel := context.WithCancel(context.Background())
		currentReaders := readers
		currentEdits := append([]sourceform.SiviParentScalarEdit(nil), edits...)
		approval := twopage.TwoPageEntryApproval(twoPageEntryApprovePhysical)
		switch failure {
		case "nil approval":
			approval = nil
		case "missing site":
			currentReaders.Site = nil
		case "missing parent":
			currentReaders.Parent = nil
		case "unlisted":
			currentEdits[len(currentEdits)-1] = sftest.SiviParentEdit(t, parent, "SurfaceShape", vprotest.MetadataText("ZZZ"))
		case "approval":
			approval = func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error {
				return errors.New("independent optional approval failed")
			}
		case "cancel":
			approval = func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error {
				cancel()
				return ctx.Err()
			}
		}
		got, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, ctx, state.ContextID, parent.Plot,
			parent.Form, parent, currentEdits, false, currentReaders, approval)
		cancel()
		if err == nil || got != nil {
			t.Fatal("configured reference/approval failure committed", failure, got, err)
		}
		if failure == "cancel" && !errors.Is(err, context.Canceled) {
			t.Fatal("configured reference cancellation lost its identity", err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	written, err := twopage.WriteTwoPageEntryWithRequiredReferences(contexts, context.Background(), state.ContextID,
		parent.Plot, parent.Form, parent, edits, false, readers, twoPageEntryApprovePhysical)
	if err != nil || written == nil || written.ChangedCells != 9 {
		t.Fatal("reference-denied writes left an attachment/transaction that prevented retry", written, err)
	}
}
