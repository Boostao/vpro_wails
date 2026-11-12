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

	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTwoPageParentExtraOwnedAtomicAuditsAndRestorationAliases(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			t.Run(form+map[bool]string{false: "-local", true: "-external"}[external], func(t *testing.T) {
				service, state, db, parent, edits := twoPageWriteFixture(t, form, external)
				before := vprotest.TwoPageWriteTables(t, db)
				written, err := twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", form, parent, edits)
				if err != nil || written == nil || written.ChangedCells != len(edits) || written.HistoryID == "" {
					t.Fatal("mixed-table additional fields did not commit atomically", written, err)
				}
				history, err := twopage.TwoPageParentExtraHistory(form)
				if err != nil {
					t.Fatal(err)
				}
				var proposal string
				if err := db.QueryRow(`SELECT Proposal FROM `+vprocore.QuoteHeaderIdentifier(history.Table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
					t.Fatal(err)
				}
				var event sourceform.SiviParentHistory
				if err := json.Unmarshal([]byte(proposal), &event); err != nil ||
					len(event.Changes) != len(edits) || !reflect.DeepEqual(event.Original, parent) || event.Committed.Form != form {
					t.Fatal("history lost source variant/original identity", event, err)
				}
				fresh, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
				if err != nil || !reflect.DeepEqual(fresh, event.Committed) {
					t.Fatal("committed physical bytes differ from history", err)
				}
				for _, edit := range edits {
					if !reflect.DeepEqual(sftest.SiviParentCell(t, fresh, edit.Column), edit.Value) {
						t.Fatal("typed committed storage changed", edit.Column)
					}
				}
				for _, change := range event.Changes {
					suffix := "_Env"
					alias := "sAmPlE_EnV"
					if change.Table == "Sample_Admin" {
						suffix, alias = "_Admin", "_aDmIn"
					}
					value, err := vprocore.MetadataCellValue(change.After)
					if err != nil || change.Audit.Table != suffix || change.Audit.Project != "Sample" ||
						change.Audit.PlotNumber != "108050" || change.Audit.User != service.Plots.CurrentUser ||
						change.Audit.ID != nil || change.Audit.Restore || change.Audit.Flag ||
						!vprocore.MetadataAuditTextEqual(change.Audit.BeforeEdit, nil) || !vprocore.MetadataAuditTextEqual(change.Audit.AfterEdit, value) {
						t.Fatal("additional-field audit storage/ownership differs", change, err)
					}
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE rowid=?`, alias, change.Audit.RowID); err != nil {
						t.Fatal(err)
					}
				}
				frozen := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				cancelled, err := twopage.RestoreTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", form, written.HistoryID, vprocore.AuditRestoreCancel)
				if err != nil || cancelled == nil || !cancelled.Cancelled {
					t.Fatal("cancel did not remain read-only", cancelled, err)
				}
				vprotest.AssertProfileSUFiles(t, service, frozen)
				action := vprocore.AuditRestorePrune
				if external {
					action = vprocore.AuditRestoreRetain
				}
				restored, err := twopage.RestoreTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", form, written.HistoryID, action)
				if err != nil || restored == nil || restored.RestoredRows != len(edits) || restored.CleanedVegRows != 0 ||
					restored.PrunedAuditRows != map[bool]int{true: len(edits), false: 0}[action == vprocore.AuditRestorePrune] {
					t.Fatal("typed restoration widened its scope", restored, err)
				}
				retry, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
				if err != nil || !reflect.DeepEqual(retry, parent) {
					t.Fatal("restoration did not recover exact physical originals", err)
				}
				after := vprotest.TwoPageWriteTables(t, db)
				master := after["sqlite_master"]
				priorMaster := before["sqlite_master"]
				if len(master.Rows) != len(priorMaster.Rows)+1 {
					t.Fatal("history changed an unexpected number of schema entries")
				}
				historyRow := master.Rows[len(priorMaster.Rows)]
				if historyRow.Cells[0].Text == nil || *historyRow.Cells[0].Text != "table" ||
					historyRow.Cells[1].Text == nil || *historyRow.Cells[1].Text != history.Table ||
					historyRow.Cells[2].Text == nil || *historyRow.Cells[2].Text != history.Table ||
					historyRow.Cells[4].Text == nil || *historyRow.Cells[4].Text != history.Schema {
					t.Fatal("created schema is not the exact isolated history table", historyRow)
				}
				master.Rows = master.Rows[:len(priorMaster.Rows)]
				if !reflect.DeepEqual(master, priorMaster) {
					t.Fatal("original physical schema entries changed")
				}
				for table, original := range before {
					if table != "Sample_Audit" && table != "sqlite_master" && !reflect.DeepEqual(original, after[table]) {
						t.Fatal("unrelated table changed", table)
					}
				}
				if action == vprocore.AuditRestorePrune && !reflect.DeepEqual(before["Sample_Audit"], after["Sample_Audit"]) {
					t.Fatal("prune failed to recover all original audits")
				}
				for _, change := range event.Changes {
					var count, flag int
					if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, change.Audit.RowID).Scan(&count, &flag); err != nil ||
						(action == vprocore.AuditRestoreRetain && (count != 1 || flag != -1)) || (action == vprocore.AuditRestorePrune && count != 0) {
						t.Fatal("retained Access true=-1 or pruned audit changed", count, flag, err)
					}
				}
				var consumed *string
				if err := db.QueryRow(`SELECT Restored FROM `+vprocore.QuoteHeaderIdentifier(history.Table)+` WHERE ID=?`, written.HistoryID).Scan(&consumed); err != nil || consumed == nil {
					t.Fatal("consumed history marker lost", consumed, err)
				}
				frozen = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				if got, err := twopage.RestoreTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", form, written.HistoryID, action); err == nil || got != nil {
					t.Fatal("history replay accepted", got, err)
				}
				vprotest.AssertProfileSUFiles(t, service, frozen)
			})
		}
	}
}

func TestTwoPageParentExtraWriterCollisionRollbackAndRetry(t *testing.T) {
	service, state, db, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	for _, kind := range []string{"context", "form", "expected", "duplicate", "bad-tail", "original"} {
		drafts := append([]sourceform.SiviParentScalarEdit(nil), edits...)
		contextID, form, original := state.ContextID, "FS882-8x6XL", parent
		switch kind {
		case "context":
			contextID = "stale"
		case "form":
			form = "FS882-8x6XL-CHARS"
		case "expected":
			drafts[len(drafts)-1].Expected = vprotest.MetadataText("stale")
		case "duplicate":
			drafts = append(drafts, drafts[0])
		case "bad-tail":
			drafts[len(drafts)-1].Value = vprotest.MetadataText(strings.Repeat("x", 256))
		case "original":
			original = nil
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := twopage.WriteTwoPageParentExtra(service, context.Background(), contextID, "108050", form, original, drafts); err == nil || got != nil {
			t.Fatal("foreign or invalid request committed", kind, got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`CREATE TRIGGER two_page_audit_failure BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='GIS_BGC_VER' BEGIN SELECT RAISE(ABORT,'additional audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", "FS882-8x6XL", parent, edits); err == nil || got != nil {
		t.Fatal("late audit failure committed earlier assignments", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DROP TRIGGER two_page_audit_failure; UPDATE Sample_Admin SET GIS_BGC_VER=7 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", "FS882-8x6XL", parent, edits); err == nil || got != nil {
		t.Fatal("independent physical collision accepted", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	fresh, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	for i := range edits {
		edits[i] = sftest.SiviParentEdit(t, fresh, edits[i].Column, edits[i].Value)
	}
	got, err := twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", "FS882-8x6XL", fresh, edits)
	if err != nil || got == nil || got.ChangedCells != 9 {
		t.Fatal("reload retry did not recheck exact owners", got, err)
	}
}

func TestTwoPageParentExtraOwnedBlockedCancellationNoopAndRetry(t *testing.T) {
	service, state, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", true)
	noops := append([]sourceform.SiviParentScalarEdit(nil), edits...)
	for i := range noops {
		noops[i].Value = noops[i].Expected
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	got, err := twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", parent.Form, parent, noops)
	if err != nil || got == nil || got.ChangedCells != 0 || got.HistoryID != "" {
		t.Fatal("no-op generated assignments or provenance", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	owner := service.Projects.Sqlite
	owner.Mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	got, err = twopage.WriteTwoPageParentExtra(service, ctx, state.ContextID, "108050", parent.Form, parent, edits)
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked write ignored cancellation", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	got, err = twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", parent.Form, parent, edits)
	if err != nil || got == nil || got.ChangedCells != 8 {
		t.Fatal("cancelled request retired owner or leaked a commit", got, err)
	}
}

func TestTwoPageParentExtraRestorationRejectsTamperedHistoryAndCollision(t *testing.T) {
	service, state, db, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	written, err := twopage.WriteTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", parent.Form, parent, edits)
	if err != nil {
		t.Fatal(err)
	}
	history, err := twopage.TwoPageParentExtraHistory(parent.Form)
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM `+vprocore.QuoteHeaderIdentifier(history.Table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"foreign-form", "foreign-field", "duplicate", "committed-form"} {
		var event sourceform.SiviParentHistory
		if err := json.Unmarshal([]byte(proposal), &event); err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "foreign-form":
			event.Original.Form = "frmSIVIsite"
		case "foreign-field":
			event.Changes[0].Column = "SV_StandHeight"
		case "duplicate":
			event.Changes = append(event.Changes, event.Changes[0])
		case "committed-form":
			event.Committed.Form = "FS882-8x6XL-CHARS"
		}
		bad, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE `+vprocore.QuoteHeaderIdentifier(history.Table)+` SET Proposal=? WHERE ID=?`, string(bad), written.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := twopage.RestoreTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("tampered typed history authorized restoration", kind, got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE `+vprocore.QuoteHeaderIdentifier(history.Table)+` SET Proposal=? WHERE ID=?`, proposal, written.HistoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET GIS_BGC_VER=7 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := twopage.RestoreTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
		t.Fatal("collision authorized destructive restoration", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}
