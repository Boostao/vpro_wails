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
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func twoPageEntryApprovePhysical(tx *sql.Tx, original *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
	if tx == nil || original == nil {
		return errors.New("missing owned transaction/original")
	}
	return nil
}

func twoPageEntryWriteEdits(t *testing.T, parent *sourceform.SiviParentProjection) []sourceform.SiviParentScalarEdit {
	t.Helper()
	return []sourceform.SiviParentScalarEdit{
		sftest.SiviParentEdit(t, parent, "FieldNumber", vprotest.MetadataText("  Literal field  ")),
		sftest.SiviParentEdit(t, parent, "PlotType", vprotest.MetadataText("  Type  ")),
		sftest.SiviParentEdit(t, parent, "GIS_BGC_VER", vprotest.MetadataInteger("-32768")),
	}
}

func TestTwoPageEntryOneAtomicWriteHistoryAndRestoration(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			contexts, state, db, parent, _ := twoPageWriteFixture(t, form, external)
			before := vprotest.TwoPageWriteTables(t, db)
			edits := twoPageEntryWriteEdits(t, parent)
			written, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
				parent, edits, false, twoPageEntryApprovePhysical)
			if err != nil || written == nil || written.ChangedCells != 3 || written.HistoryID == "" {
				t.Fatal("mixed scopes did not share one audited transaction", form, external, written, err)
			}
			history, err := twopage.TwoPageEntryHistory(form, false)
			if err != nil {
				t.Fatal(err)
			}
			var proposal string
			if err := db.QueryRow(`SELECT Proposal FROM `+vprocore.QuoteHeaderIdentifier(history.Table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
				t.Fatal(err)
			}
			var event sourceform.SiviParentHistory
			if err := json.Unmarshal([]byte(proposal), &event); err != nil || len(event.Changes) != 3 || !reflect.DeepEqual(event.Original, parent) {
				t.Fatal("whole-entry history split or lost source identity", event, err)
			}
			current, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, parent.Plot, form)
			if err != nil || !reflect.DeepEqual(current, event.Committed) {
				t.Fatal("committed source differs from whole-entry history", err)
			}
			for _, edit := range edits {
				if !reflect.DeepEqual(sftest.SiviParentCell(t, current, edit.Column), edit.Value) {
					t.Fatal("mixed committed value changed", edit)
				}
			}
			for _, change := range event.Changes {
				alias := "_eNv"
				if change.Table == parent.AdminTable {
					alias = "sAmPlE_aDmIn"
				}
				if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE rowid=?`, alias, change.Audit.RowID); err != nil {
					t.Fatal(err)
				}
			}
			cancelled, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
				written.HistoryID, vprocore.AuditRestoreCancel, false)
			if err != nil || cancelled == nil || !cancelled.Cancelled {
				t.Fatal("complete-entry restoration cancellation", cancelled, err)
			}
			action := vprocore.AuditRestorePrune
			if external {
				action = vprocore.AuditRestoreRetain
			}
			restored, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
				written.HistoryID, action, false)
			if err != nil || restored == nil || restored.RestoredRows != 3 {
				t.Fatal("complete-entry typed restoration", form, external, restored, err)
			}
			after := vprotest.TwoPageWriteTables(t, db)
			for _, table := range []string{"Sample_Env", "Sample_Admin"} {
				if !reflect.DeepEqual(before[table], after[table]) {
					t.Fatal("complete-entry restoration changed physical parents", table)
				}
			}
			for _, domain := range []func(string) (sourceform.SiviParentHistoryDomain, error){twopage.TwoPageParentExtraHistory, twopage.TwoPageParentCommonHistory} {
				separate, err := domain(form)
				if err != nil {
					t.Fatal(err)
				}
				var count int
				if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, separate.Table).Scan(&count); err != nil || count != 0 {
					t.Fatal("complete entry sequenced existing scope writers", separate.Table, count, err)
				}

			}
		}
	}
}

func TestTwoPageEntryCancellationAndVariantHistoryIsolation(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	edits := twoPageEntryWriteEdits(t, parent)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithCancel(context.Background())
	if got, err := twopage.WriteTwoPageEntry(contexts, ctx, state.ContextID, parent.Plot, parent.Form, parent, edits, false,
		func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error {
			cancel()
			return ctx.Err()
		}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("complete-entry approval cancellation did not roll back", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	written, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, twoPageEntryApprovePhysical)
	if err != nil || written == nil || written.HistoryID == "" {
		t.Fatal("cancelled complete-entry attempt could not retry", written, err)
	}
	current := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, "FS882-8x6XL-CHARS",
		written.HistoryID, vprocore.AuditRestorePrune, false); err == nil || got != nil {
		t.Fatal("CHARS inherited normal complete-entry history", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, current)
}

func TestTwoPageEntryMasterAuthorityUsesCurrentOwnedUserForWriteAndRestore(t *testing.T) {
	contexts, state, _, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	setUser := func(user string) {
		contexts.Plots.Mu.Lock()
		contexts.Plots.CurrentUser = user
		contexts.Plots.Mu.Unlock()
	}
	edits := append(twoPageEntryWriteEdits(t, parent), sftest.SiviParentEdit(t, parent, "BECSiteUnit", vprotest.MetadataText("Master")))
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	setUser("ordinary user")
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, true, twoPageEntryApprovePhysical); err == nil || got != nil {
		t.Fatal("requested Master option or approval callback bypassed current source-user authority", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	setUser("Will MacKenzie")
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, twoPageEntryApprovePhysical); err == nil || got != nil {
		t.Fatal("authorized user bypassed disabled Master editing", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	written, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, true, func(tx *sql.Tx, original *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			setUser("ordinary user")
			return twoPageEntryApprovePhysical(tx, original, assignments)
		})
	if err != nil || written == nil || written.ChangedCells != 4 || written.HistoryID == "" {
		t.Fatal("current source-authorized mixed Master transaction", written, err)
	}
	current := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	setUser("ordinary user")
	if got, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		written.HistoryID, vprocore.AuditRestoreCancel, true); err != nil || got == nil || !got.Cancelled {
		t.Fatal("read-only cancellation inherited Master write restrictions", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, current)
	if got, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		written.HistoryID, vprocore.AuditRestorePrune, true); err == nil || got != nil {
		t.Fatal("current ordinary user inherited Master restoration authority", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, current)
	setUser("Will MacKenzie")
	if got, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		written.HistoryID, vprocore.AuditRestorePrune, false); err == nil || got != nil {
		t.Fatal("authorized restoration bypassed disabled Master editing", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, current)
	if got, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		written.HistoryID, vprocore.AuditRestorePrune, true); err != nil || got == nil || got.RestoredRows != 4 {
		t.Fatal("current source-authorized mixed Master restoration", got, err)
	}
}

func TestTwoPageEntryOrdinaryFieldsDoNotRequireMasterAuthority(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		contexts, state, _, parent, _ := twoPageWriteFixture(t, form, false)
		contexts.Plots.Mu.Lock()
		contexts.Plots.CurrentUser = "ordinary user"
		contexts.Plots.Mu.Unlock()
		written, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
			parent, twoPageEntryWriteEdits(t, parent), true, twoPageEntryApprovePhysical)
		if err != nil || written == nil || written.ChangedCells != 3 {
			t.Fatal("Master intent incorrectly blocked unrelated source fields", form, written, err)
		}
		if got, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, form,
			written.HistoryID, vprocore.AuditRestorePrune, true); err != nil || got == nil || got.RestoredRows != 3 {
			t.Fatal("Master intent incorrectly blocked unrelated restoration", form, got, err)
		}
	}
}

func TestTwoPageEntryApprovalStalenessRollbackAndNoPhantomHistory(t *testing.T) {
	contexts, state, db, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	edits := twoPageEntryWriteEdits(t, parent)
	frozen := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, nil); err == nil || got != nil {
		t.Fatal("missing reference/role approval enabled a write", got, err)
	}
	denied := errors.New("reference decision denied")
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error {
			return denied
		}); !errors.Is(err, denied) || got != nil {
		t.Fatal("rejected source approval committed or hid its error", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, frozen)
	for i := range edits {
		edits[i].Value = edits[i].Expected
	}
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, twoPageEntryApprovePhysical); err != nil || got == nil || got.ChangedCells != 0 || got.HistoryID != "" {
		t.Fatal("no-op created a phantom whole-entry event", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, frozen)
	edits = twoPageEntryWriteEdits(t, parent)
	if _, err := db.Exec(`CREATE TRIGGER reject_entry_audit BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT,'rejected audit'); END`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.TwoPageWriteTables(t, db)
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, twoPageEntryApprovePhysical); err == nil || got != nil {
		t.Fatal("audit failure committed mixed parent changes", got, err)
	}
	if after := vprotest.TwoPageWriteTables(t, db); !reflect.DeepEqual(before, after) {
		t.Fatal("mixed-scope rollback left a partial transaction")
	}
	if _, err := db.Exec(`DROP TRIGGER reject_entry_audit; UPDATE Sample_Env SET FieldNumber='independent' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.TwoPageWriteTables(t, db)
	calls := 0
	if got, err := twopage.WriteTwoPageEntry(contexts, context.Background(), state.ContextID, parent.Plot, parent.Form,
		parent, edits, false, func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error {
			calls++
			return nil
		}); err == nil || got != nil || calls != 0 {
		t.Fatal("stale physical source reached approval or committed", got, calls, err)
	}
	if after := vprotest.TwoPageWriteTables(t, db); !reflect.DeepEqual(before, after) {
		t.Fatal("stale complete-entry write changed data")
	}
}
