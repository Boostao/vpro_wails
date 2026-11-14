package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func siviCollectedServiceFixture(t *testing.T, extended bool, strength int) (*SIVICollectedService, *vprocore.ContextService, vprocore.ProjectState, *sql.DB, []sourceform.SIVIVegetationProjection, []SIVICollectedEdit) {
	t.Helper()
	contexts, state, db, _, _ := sftest.SiviWriteFixture(t, extended, strength)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover6,Collected)
		VALUES ('108050','C',10000002,0,'C');
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover7,Collected)
		VALUES ('108050','D',10000003,0,'V')`); err != nil {
		t.Fatal(err)
	}
	service, err := NewSIVICollectedService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", extended)
	if err != nil {
		t.Fatal(err)
	}
	edits := []SIVICollectedEdit{}
	for i, id := range []string{"10000001", "10000002", "10000003"} {
		for _, row := range original[i].Rows {
			if row.Cells[0].Integer != nil && *row.Cells[0].Integer == id {
				edits = append(edits, SIVICollectedEdit{row.RowID, original[i].Form, row.Cells[len(row.Cells)-1], 1})
			}
		}
	}
	if len(edits) != 3 {
		t.Fatal("three controlled source groups missing", edits)
	}
	return service, contexts, state, db, original, edits
}

func siviCollectedRequestJSON(t *testing.T, original []sourceform.SIVIVegetationProjection, edits []SIVICollectedEdit) string {
	t.Helper()
	data, err := json.Marshal(SIVICollectedWrite{original, edits})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSIVICollectedServiceNilAndCanceled(t *testing.T) {
	service, contexts, state, _, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := NewSIVICollectedService(nil); err == nil || got != nil {
		t.Fatal("nil owner accepted", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, target := range []*SIVICollectedService{nil, {}, service} {
		for _, ctx := range []context.Context{nil, ctx} {
			if got, err := target.GetOriginal(ctx, state.ContextID, "108050", false); err == nil || got != nil {
				t.Fatal("nil/canceled read succeeded", got, err)
			}
			if got, err := target.SaveReviewed(ctx, state.ContextID, "108050", false, valid); err == nil || got != nil {
				t.Fatal("nil/canceled Save succeeded", got, err)
			}
			if got, err := target.RestoreReviewed(ctx, state.ContextID, "108050", "1", vprocore.AuditRestorePrune); err == nil || got != nil {
				t.Fatal("nil/canceled restore succeeded", got, err)
			}
		}
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestSIVICollectedServiceAtomicTypedAuditStrengthsAliasesAndRecovery(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for strength := 0; strength <= 3; strength++ {
			for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
				t.Run(strconv.FormatBool(extended)+"/"+strconv.Itoa(strength)+"/"+string(action), func(t *testing.T) {
					service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, extended, strength)
					unrelated := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
					for role, path := range contexts.Projects.Sqlite.Attachments {
						if path == contexts.Projects.Sqlite.Attachments["project"] {
							delete(unrelated, role)
						}
					}
					saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", extended, siviCollectedRequestJSON(t, original, edits))
					if err != nil || saved == nil || saved.ChangedCells != 3 || (saved.HistoryID == "") != (strength == 0) {
						t.Fatal("Collected transaction/audit strength differs", saved, err)
					}
					for i, want := range []*string{vprotest.MetadataText("C").Text, vprotest.MetadataText("V").Text, nil} {
						var value *string
						if err := db.QueryRow(`SELECT Collected FROM Sample_Veg WHERE ID=?`, 10000001+i).Scan(&value); err != nil || !reflect.DeepEqual(value, want) {
							t.Fatal("typed cycle differs", i, value, want, err)
						}
					}
					var count int
					if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE EditField='Collected'`).Scan(&count); err != nil || count != strength {
						t.Fatal("audit strengths differ", count, err)
					}
					for _, table := range []string{sourceform.SiviCoverHistoryTable, sourceform.SiviHeightHistoryTable, siviCombinedHistoryTable} {
						if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, table).Scan(&count); err != nil || count != 0 {
							t.Fatal("Collected wrote predecessor history", table, count, err)
						}
					}
					vprotest.AssertProfileSUFiles(t, contexts, unrelated)
					if strength == 0 {
						if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, siviCollectedHistoryTable).Scan(&count); err != nil || count != 0 {
							t.Fatal("unaudited Save invented history", count, err)
						}
						return
					}
					var proposal string
					if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICollectedHistory" WHERE ID=?`, saved.HistoryID).Scan(&proposal); err != nil {
						t.Fatal(err)
					}
					var history sourceform.SiviHeightHistory
					if err := json.Unmarshal([]byte(proposal), &history); err != nil || len(history.Changes) != strength || len(history.Committed) != strength {
						t.Fatal("typed history incomplete", history, err)
					}
					for _, change := range history.Changes {
						if change.Column != "Collected" || change.Audit.EditField != "Collected" || change.Audit.Restore || change.Audit.Flag {
							t.Fatal("unrelated column/phantom audit", change)
						}
					}
					before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
					for _, policy := range []sourceform.SiviVegetationWritePolicy{sourceform.SiviCoverWritePolicy(), sourceform.SiviHeightWritePolicy(), siviCombinedWritePolicy()} {
						if got, err := sourceform.RestoreSIVIVegetationCells(contexts, context.Background(), state.ContextID, "108050", saved.HistoryID, action, policy); err == nil || got != nil {
							t.Fatal("predecessor restored Collected history", got, err)
						}
					}
					if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreCancel); err != nil || got == nil || !got.Cancelled {
						t.Fatal("cancel restore failed", got, err)
					}
					vprotest.AssertProfileSUFiles(t, contexts, before)
					alias := "_vEg"
					if extended {
						alias = "sAmPlE_vEg"
					}
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE EditField='Collected'`, alias); err != nil {
						t.Fatal(err)
					}
					restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action)
					if err != nil || restored == nil || restored.RestoredRows != strength || restored.CleanedVegRows != 0 ||
						restored.PrunedAuditRows != map[bool]int{true: strength, false: 0}[action == vprocore.AuditRestorePrune] {
						t.Fatal("typed restoration/aliases failed", restored, err)
					}
					wants := []*string{vprotest.MetadataText("C").Text, vprotest.MetadataText("V").Text, nil}
					if strength >= 2 {
						wants[0] = nil
					}
					if strength >= 1 {
						wants[1] = vprotest.MetadataText("C").Text
					}
					if strength == 3 {
						wants[2] = vprotest.MetadataText("V").Text
					}
					for i, want := range wants {
						var value *string
						if err := db.QueryRow(`SELECT Collected FROM Sample_Veg WHERE ID=?`, 10000001+i).Scan(&value); err != nil || !reflect.DeepEqual(value, want) {
							t.Fatal("recovery changed unaudited cell or lost typed original", i, value, want, err)
						}
					}
					if action == vprocore.AuditRestoreRetain {
						if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE EditField='Collected' AND typeof(Restore)='integer' AND Restore=-1`).Scan(&count); err != nil || count != strength {
							t.Fatal("Access true=-1 was not preserved", count, err)
						}
					}
					before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
					if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || got != nil {
						t.Fatal("completed history replayed", got, err)
					}
					vprotest.AssertProfileSUFiles(t, contexts, before)
					vprotest.AssertProfileSUFiles(t, contexts, unrelated)
				})
			}
		}
	}
}

func TestSIVICollectedServiceMalformedTransportAndForeignOwnership(t *testing.T) {
	service, contexts, state, _, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, body := range []string{
		`{}`, `null`, `[]`, valid + `{}`,
		strings.Replace(valid, `"original":`, `"Original":`, 1),
		strings.Replace(valid, `"original":`, `"extra":true,"original":`, 1),
		strings.Replace(valid, `"edits":`, `"edits":[],"edits":`, 1),
		strings.Replace(valid, `"rowId":`, `"value":null,"rowId":`, 1),
		strings.Replace(valid, `"clicks":1`, `"clicks":1,"clicks":2`, 1),
		strings.Replace(valid, `"text":"C"`, `"text":"\ud800"`, 1),
		strings.Replace(valid, `"text":"C"`, `"text":"\udc00"`, 1),
		strings.Replace(valid, `"text":"C"`, `"text":"`+"\xff"+`"`, 1),
		siviCollectedRequestJSON(t, original[:2], edits),
		siviCollectedRequestJSON(t, original, nil),
	} {
		if body == valid {
			t.Fatal("invalid case did not alter payload")
		}
		if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, body); err == nil || got != nil {
			t.Fatal("malformed wire accepted", got, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	for _, id := range []string{"foreign", state.ContextID} {
		plot := "108050"
		if id == state.ContextID {
			plot = "foreign"
		}
		got, err := service.GetOriginal(context.Background(), id, plot, false)
		if id != state.ContextID {
			if err == nil || got != nil {
				t.Fatal("foreign context read succeeded", got, err)
			}
		} else if err != nil || len(got) != 3 || len(got[0].Rows)+len(got[1].Rows)+len(got[2].Rows) != 0 {
			t.Fatal("unavailable plot invented source rows", got, err)
		}
		if got, err := service.SaveReviewed(context.Background(), id, plot, false, valid); err == nil || got != nil {
			t.Fatal("foreign Save succeeded", got, err)
		}
		if got, err := service.RestoreReviewed(context.Background(), id, plot, "1", vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("foreign restore succeeded", got, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
}

func TestSIVICollectedServiceNoopIsolationRollbackCASCollisionAndRetry(t *testing.T) {
	service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	if _, err := db.Exec(`CREATE TRIGGER SIVICollectedReject BEFORE UPDATE OF Collected ON Sample_Veg WHEN OLD.ID=10000003
		BEGIN SELECT RAISE(ABORT,'controlled late rejection'); END`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || got != nil {
		t.Fatal("late rejection committed earlier rows", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`DROP TRIGGER SIVICollectedReject;
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	collision, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, collision, edits)); err == nil || got != nil {
		t.Fatal("application ID collision accepted", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE ID=10000001 AND Species='DUP';
		UPDATE Sample_Veg SET Collected=? WHERE ID=10000001`, strings.Repeat("x", 256)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{strings.Repeat("x", 256), "", "?", nil, "C", "V"} {
		if _, err := db.Exec(`UPDATE Sample_Veg SET Collected=? WHERE ID=10000001`, value); err != nil {
			t.Fatal(err)
		}
		historical, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
		if err != nil {
			t.Fatal(err)
		}
		cell := vprocore.ProjectMetadataCell{Storage: "null"}
		if value != nil {
			cell = vprotest.MetadataText(value.(string))
		}
		noop := []SIVICollectedEdit{{edits[0].RowID, edits[0].Form, cell, 3}}
		before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, historical, noop)); err != nil || got == nil || got.ChangedCells != 0 || got.HistoryID != "" {
			t.Fatal("unchanged historical/full-cycle draft assigned or audited", got, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Collected=NULL WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	wrong := append([]SIVICollectedEdit{}, edits...)
	wrong[0].Expected = vprotest.MetadataText("C")
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, original, wrong)); err == nil || got != nil {
		t.Fatal("cell CAS ignored", got, err)
	}
	duplicate := append(append([]SIVICollectedEdit{}, edits...), SIVICollectedEdit{edits[0].RowID, "SubVegC-SIVI", edits[0].Expected, 1})
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, original, duplicate)); err == nil || got != nil {
		t.Fatal("multiple forms double-cycled physical Collected", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid)
	if err != nil || saved == nil || saved.ChangedCells != 3 {
		t.Fatal("failed transactions poisoned retry", saved, err)
	}
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || got != nil {
		t.Fatal("stale original replayed", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestSIVICollectedServiceRestoreDriftTamperRollbackAndOwnership(t *testing.T) {
	service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, true, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	owner := contexts.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	reference := owner.Attachments["VLists"]
	owner.Mu.Lock()
	owner.Attachments["VLists"] = owner.Attachments["project"]
	owner.Mu.Unlock()
	_, ownershipErr := service.SaveReviewed(context.Background(), state.ContextID, "108050", true, valid)
	owner.Mu.Lock()
	owner.Attachments["VLists"] = reference
	owner.Mu.Unlock()
	if ownershipErr == nil {
		t.Fatal("changed file ownership authorized writes")
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", true, valid)
	if err != nil || saved == nil {
		t.Fatal("ownership rejection poisoned retry", saved, err)
	}
	for _, change := range []struct{ setup, cleanup string }{
		{`UPDATE Sample_Veg SET Species='OTHER' WHERE ID=10000001`, `UPDATE Sample_Veg SET Species='RAW' WHERE ID=10000001`},
		{`UPDATE Sample_Audit SET Flag=-1 WHERE ID=10000003`, `UPDATE Sample_Audit SET Flag=0 WHERE ID=10000003`},
		{`UPDATE Sample_Audit SET Flag=0.5 WHERE ID=10000003`, `UPDATE Sample_Audit SET Flag=0 WHERE ID=10000003`},
		{`UPDATE Sample_Audit SET "Table"='Veg' WHERE EditField='Collected'`, `UPDATE Sample_Audit SET "Table"='_Veg' WHERE EditField='Collected'`},
		{`CREATE TRIGGER SIVICollectedRestoreSideEffect AFTER UPDATE OF Collected ON Sample_Veg
		  BEGIN UPDATE Sample_Veg SET Cover1=1 WHERE ID=10000001; END`, `DROP TRIGGER SIVICollectedRestoreSideEffect`},
	} {
		if _, err := db.Exec(change.setup); err != nil {
			t.Fatal(err)
		}
		before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
			if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || got != nil {
				t.Fatal("unsafe restoration/pruning committed", got, err)
			}
			vprotest.AssertProfileSUFiles(t, contexts, before)
		}
		if _, err := db.Exec(change.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 3 {
		t.Fatal("restore refusals poisoned retry", got, err)
	}
}

func TestSIVICollectedServiceBlockedCancellationAndRetry(t *testing.T) {
	service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if got, err := service.SaveReviewed(ctx, state.ContextID, "108050", false, valid); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked cancellation lost context or returned success", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid)
	if err != nil || saved == nil || saved.ChangedCells != 3 {
		t.Fatal("cancellation poisoned retry", saved, err)
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked = true
	before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer restoreCancel()
	if got, err := service.RestoreReviewed(restoreCtx, state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked restoration cancellation returned success", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 3 {
		t.Fatal("restoration cancellation poisoned retry", got, err)
	}
}

func TestSIVICollectedServiceMixedNoopsAndNonCyclableStorage(t *testing.T) {
	service, contexts, state, db, _, edits := siviCollectedServiceFixture(t, false, 3)
	historical := strings.Repeat("historical", 40)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Collected=? WHERE ID=10000001`, historical); err != nil {
		t.Fatal(err)
	}
	original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	edits[0].Expected = vprotest.MetadataText(historical)
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, original, edits))
	if err != nil || saved == nil || saved.ChangedCells != 2 {
		t.Fatal("historical noop did not isolate assignments", saved, err)
	}
	var value, proposal string
	if err := db.QueryRow(`SELECT Collected FROM Sample_Veg WHERE ID=10000001`).Scan(&value); err != nil || value != historical {
		t.Fatal("historical overlength text normalized", value, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&count); err != nil || count != 0 {
		t.Fatal("noop inherited phantom audit", count, err)
	}
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICollectedHistory" WHERE ID=?`, saved.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var history sourceform.SiviHeightHistory
	if err := json.Unmarshal([]byte(proposal), &history); err != nil || len(history.Changes) != 2 || len(history.Committed) != 2 {
		t.Fatal("noop leaked into typed history", history, err)
	}
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreRetain); err != nil || got == nil || got.RestoredRows != 2 {
		t.Fatal("mixed noop restoration failed", got, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Collected=x'43' WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	original, err = service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	cell := vprocore.ProjectMetadataCell{Storage: "blob", BlobHex: vprotest.MetadataText("43").Text}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false,
		siviCollectedRequestJSON(t, original, []SIVICollectedEdit{{edits[0].RowID, edits[0].Form, cell, 3}})); err == nil || got != nil || !strings.Contains(err.Error(), "not cyclable") {
		t.Fatal("owned non-text Collected coerced", got, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}

func TestSIVICollectedServiceRejectsForeignTypedHistoryCells(t *testing.T) {
	service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, false, 3)
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, original, edits))
	if err != nil || saved == nil {
		t.Fatal(saved, err)
	}
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICollectedHistory" WHERE ID=?`, saved.HistoryID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	for _, tamper := range []func(*sourceform.SiviHeightHistory){
		func(h *sourceform.SiviHeightHistory) { h.Changes[0].Column = "HeightA" },
		func(h *sourceform.SiviHeightHistory) { h.Changes[0].Column = "Cover1" },
		func(h *sourceform.SiviHeightHistory) { h.Plot = "foreign" },
		func(h *sourceform.SiviHeightHistory) { h.Project = "foreign" },
		func(h *sourceform.SiviHeightHistory) { h.Committed = nil },
		func(h *sourceform.SiviHeightHistory) { h.Changes = append(h.Changes, h.Changes[0]) },
		func(h *sourceform.SiviHeightHistory) { h.Committed = append(h.Committed, h.Committed[0]) },
	} {
		var history sourceform.SiviHeightHistory
		if err := json.Unmarshal([]byte(raw), &history); err != nil {
			t.Fatal(err)
		}
		tamper(&history)
		data, err := json.Marshal(history)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVICollectedHistory" SET Proposal=? WHERE ID=?`, string(data), saved.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("foreign/tampered history restored or pruned", got, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICollectedHistory" SET Proposal=? WHERE ID=?`, raw, saved.HistoryID); err != nil {
		t.Fatal(err)
	}
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 3 {
		t.Fatal("history rejection poisoned retry", got, err)
	}
}
