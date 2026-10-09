package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func twoPageWriteFixture(t *testing.T, form string, external bool) (*ContextService, ProjectState, *sql.DB, *siviParentProjection, []siviParentScalarEdit) {
	t.Helper()
	service, state, db, _, _ := siviParentWriteFixture(t, external, 3)
	parent, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	values := []struct {
		column string
		value  ProjectMetadataCell
	}{
		{"SV_WaterTableCM", siviReal(-3.25)}, {"ActiveLayerDepth", siviReal(0.1)},
		{"SV_FullCruiseCard", metadataText("  cArD  ")}, {"PlotSize", siviReal(12.5)},
		{"ProvinceStateTerritory", metadataText("  État  ")}, {"SiteUnitLongName", metadataText("e\u0301 Long Name")},
		{"GIS_BGC", metadataText("  MiXeD  ")}, {"GIS_BGC_VER", metadataInteger("-32768")},
		{"BEC_Use", metadataText("Case Sensitive")},
	}
	edits := []siviParentScalarEdit{}
	for _, field := range values {
		if form == "FS882-8x6XL-CHARS" && field.column == "BEC_Use" {
			continue
		}
		edits = append(edits, siviParentEdit(t, parent, field.column, field.value))
	}
	return service, state, db, parent, edits
}

func twoPageWriteTables(t *testing.T, db *sql.DB) map[string]ProjectMetadataTable {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	tables, err := environmentSiteUnitTables(context.Background(), tx)
	if err := errors.Join(err, tx.Rollback()); err != nil {
		t.Fatal(err)
	}
	return tables
}

func TestTwoPageParentExtraOwnedAtomicAuditsAndRestorationAliases(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			t.Run(form+map[bool]string{false: "-local", true: "-external"}[external], func(t *testing.T) {
				service, state, db, parent, edits := twoPageWriteFixture(t, form, external)
				before := twoPageWriteTables(t, db)
				written, err := service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", form, parent, edits)
				if err != nil || written == nil || written.ChangedCells != len(edits) || written.HistoryID == "" {
					t.Fatal("mixed-table additional fields did not commit atomically", written, err)
				}
				history, err := twoPageParentExtraHistory(form)
				if err != nil {
					t.Fatal(err)
				}
				var proposal string
				if err := db.QueryRow(`SELECT Proposal FROM `+quoteHeaderIdentifier(history.table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
					t.Fatal(err)
				}
				var event siviParentHistory
				if err := json.Unmarshal([]byte(proposal), &event); err != nil ||
					len(event.Changes) != len(edits) || !reflect.DeepEqual(event.Original, parent) || event.Committed.Form != form {
					t.Fatal("history lost source variant/original identity", event, err)
				}
				fresh, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
				if err != nil || !reflect.DeepEqual(fresh, event.Committed) {
					t.Fatal("committed physical bytes differ from history", err)
				}
				for _, edit := range edits {
					if !reflect.DeepEqual(siviParentCell(t, fresh, edit.Column), edit.Value) {
						t.Fatal("typed committed storage changed", edit.Column)
					}
				}
				for _, change := range event.Changes {
					suffix := "_Env"
					alias := "sAmPlE_EnV"
					if change.Table == "Sample_Admin" {
						suffix, alias = "_Admin", "_aDmIn"
					}
					value, err := metadataCellValue(change.After)
					if err != nil || change.Audit.Table != suffix || change.Audit.Project != "Sample" ||
						change.Audit.PlotNumber != "108050" || change.Audit.User != service.plots.currentUser ||
						change.Audit.ID != nil || change.Audit.Restore || change.Audit.Flag ||
						!metadataAuditTextEqual(change.Audit.BeforeEdit, nil) || !metadataAuditTextEqual(change.Audit.AfterEdit, value) {
						t.Fatal("additional-field audit storage/ownership differs", change, err)
					}
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE rowid=?`, alias, change.Audit.RowID); err != nil {
						t.Fatal(err)
					}
				}
				frozen := databaseBytes(t, service.projects.sqlite.attachments)
				cancelled, err := service.restoreTwoPageParentExtra(context.Background(), state.ContextID, "108050", form, written.HistoryID, AuditRestoreCancel)
				if err != nil || cancelled == nil || !cancelled.Cancelled {
					t.Fatal("cancel did not remain read-only", cancelled, err)
				}
				assertProfileSUFiles(t, service, frozen)
				action := AuditRestorePrune
				if external {
					action = AuditRestoreRetain
				}
				restored, err := service.restoreTwoPageParentExtra(context.Background(), state.ContextID, "108050", form, written.HistoryID, action)
				if err != nil || restored == nil || restored.RestoredRows != len(edits) || restored.CleanedVegRows != 0 ||
					restored.PrunedAuditRows != map[bool]int{true: len(edits), false: 0}[action == AuditRestorePrune] {
					t.Fatal("typed restoration widened its scope", restored, err)
				}
				retry, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
				if err != nil || !reflect.DeepEqual(retry, parent) {
					t.Fatal("restoration did not recover exact physical originals", err)
				}
				after := twoPageWriteTables(t, db)
				master := after["sqlite_master"]
				priorMaster := before["sqlite_master"]
				if len(master.Rows) != len(priorMaster.Rows)+1 {
					t.Fatal("history changed an unexpected number of schema entries")
				}
				historyRow := master.Rows[len(priorMaster.Rows)]
				if historyRow.Cells[0].Text == nil || *historyRow.Cells[0].Text != "table" ||
					historyRow.Cells[1].Text == nil || *historyRow.Cells[1].Text != history.table ||
					historyRow.Cells[2].Text == nil || *historyRow.Cells[2].Text != history.table ||
					historyRow.Cells[4].Text == nil || *historyRow.Cells[4].Text != history.schema {
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
				if action == AuditRestorePrune && !reflect.DeepEqual(before["Sample_Audit"], after["Sample_Audit"]) {
					t.Fatal("prune failed to recover all original audits")
				}
				for _, change := range event.Changes {
					var count, flag int
					if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, change.Audit.RowID).Scan(&count, &flag); err != nil ||
						(action == AuditRestoreRetain && (count != 1 || flag != -1)) || (action == AuditRestorePrune && count != 0) {
						t.Fatal("retained Access true=-1 or pruned audit changed", count, flag, err)
					}
				}
				var consumed *string
				if err := db.QueryRow(`SELECT Restored FROM `+quoteHeaderIdentifier(history.table)+` WHERE ID=?`, written.HistoryID).Scan(&consumed); err != nil || consumed == nil {
					t.Fatal("consumed history marker lost", consumed, err)
				}
				frozen = databaseBytes(t, service.projects.sqlite.attachments)
				if got, err := service.restoreTwoPageParentExtra(context.Background(), state.ContextID, "108050", form, written.HistoryID, action); err == nil || got != nil {
					t.Fatal("history replay accepted", got, err)
				}
				assertProfileSUFiles(t, service, frozen)
			})
		}
	}
}

func TestTwoPageParentExtraWriterCollisionRollbackAndRetry(t *testing.T) {
	service, state, db, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	for _, kind := range []string{"context", "form", "expected", "duplicate", "bad-tail", "original"} {
		drafts := append([]siviParentScalarEdit(nil), edits...)
		contextID, form, original := state.ContextID, "FS882-8x6XL", parent
		switch kind {
		case "context":
			contextID = "stale"
		case "form":
			form = "FS882-8x6XL-CHARS"
		case "expected":
			drafts[len(drafts)-1].Expected = metadataText("stale")
		case "duplicate":
			drafts = append(drafts, drafts[0])
		case "bad-tail":
			drafts[len(drafts)-1].Value = metadataText(strings.Repeat("x", 256))
		case "original":
			original = nil
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.writeTwoPageParentExtra(context.Background(), contextID, "108050", form, original, drafts); err == nil || got != nil {
			t.Fatal("foreign or invalid request committed", kind, got, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`CREATE TRIGGER two_page_audit_failure BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='GIS_BGC_VER' BEGIN SELECT RAISE(ABORT,'additional audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", "FS882-8x6XL", parent, edits); err == nil || got != nil {
		t.Fatal("late audit failure committed earlier assignments", got, err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DROP TRIGGER two_page_audit_failure; UPDATE Sample_Admin SET GIS_BGC_VER=7 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", "FS882-8x6XL", parent, edits); err == nil || got != nil {
		t.Fatal("independent physical collision accepted", got, err)
	}
	assertProfileSUFiles(t, service, before)
	fresh, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	for i := range edits {
		edits[i] = siviParentEdit(t, fresh, edits[i].Column, edits[i].Value)
	}
	got, err := service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", "FS882-8x6XL", fresh, edits)
	if err != nil || got == nil || got.ChangedCells != 9 {
		t.Fatal("reload retry did not recheck exact owners", got, err)
	}
}

func TestTwoPageParentExtraOwnedBlockedCancellationNoopAndRetry(t *testing.T) {
	service, state, _, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", true)
	noops := append([]siviParentScalarEdit(nil), edits...)
	for i := range noops {
		noops[i].Value = noops[i].Expected
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	got, err := service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", parent.Form, parent, noops)
	if err != nil || got == nil || got.ChangedCells != 0 || got.HistoryID != "" {
		t.Fatal("no-op generated assignments or provenance", got, err)
	}
	assertProfileSUFiles(t, service, before)
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	got, err = service.writeTwoPageParentExtra(ctx, state.ContextID, "108050", parent.Form, parent, edits)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked write ignored cancellation", got, err)
	}
	assertProfileSUFiles(t, service, before)
	got, err = service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", parent.Form, parent, edits)
	if err != nil || got == nil || got.ChangedCells != 8 {
		t.Fatal("cancelled request retired owner or leaked a commit", got, err)
	}
}

func TestTwoPageParentExtraRestorationRejectsTamperedHistoryAndCollision(t *testing.T) {
	service, state, db, parent, edits := twoPageWriteFixture(t, "FS882-8x6XL", false)
	written, err := service.writeTwoPageParentExtra(context.Background(), state.ContextID, "108050", parent.Form, parent, edits)
	if err != nil {
		t.Fatal(err)
	}
	history, err := twoPageParentExtraHistory(parent.Form)
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM `+quoteHeaderIdentifier(history.table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"foreign-form", "foreign-field", "duplicate", "committed-form"} {
		var event siviParentHistory
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
		if _, err := db.Exec(`UPDATE `+quoteHeaderIdentifier(history.table)+` SET Proposal=? WHERE ID=?`, string(bad), written.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.restoreTwoPageParentExtra(context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, AuditRestorePrune); err == nil || got != nil {
			t.Fatal("tampered typed history authorized restoration", kind, got, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE `+quoteHeaderIdentifier(history.table)+` SET Proposal=? WHERE ID=?`, proposal, written.HistoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET GIS_BGC_VER=7 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.restoreTwoPageParentExtra(context.Background(), state.ContextID, "108050", parent.Form, written.HistoryID, AuditRestorePrune); err == nil || got != nil {
		t.Fatal("collision authorized destructive restoration", got, err)
	}
	assertProfileSUFiles(t, service, before)
}
