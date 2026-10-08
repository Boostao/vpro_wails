package main

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

func siviCollectedServiceFixture(t *testing.T, extended bool, strength int) (*SIVICollectedService, *ContextService, ProjectState, *sql.DB, []SIVIVegetationProjection, []SIVICollectedEdit) {
	t.Helper()
	contexts, state, db, _, _ := siviWriteFixture(t, extended, strength)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover6,Collected)
		VALUES ('108050','C',10000002,0,'C');
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover7,Collected)
		VALUES ('108050','D',10000003,0,'V')`); err != nil {
		t.Fatal(err)
	}
	service, err := NewSIVICollectedService(contexts, func(name string) (string, bool) {
		if name != siviCollectedFeatureEnvironment {
			t.Fatal("Collected consulted a predecessor permission", name)
		}
		return "true", true
	})
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

func siviCollectedRequestJSON(t *testing.T, original []SIVIVegetationProjection, edits []SIVICollectedEdit) string {
	t.Helper()
	data, err := json.Marshal(SIVICollectedWrite{original, edits})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSIVICollectedServiceIndependentStrictGateNilAndCanceled(t *testing.T) {
	service, contexts, state, _, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	contexts.siviHeightEnabled = true
	_ = siviCoverServiceFixture(t, contexts, "true", true)
	_ = siviCombinedServiceFixture(t, contexts)
	for _, present := range []bool{false, true} {
		disabled, err := NewSIVICollectedService(contexts, func(name string) (string, bool) {
			if name != siviCollectedFeatureEnvironment {
				t.Fatal("gate consulted predecessor", name)
			}
			return "false", present
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := disabled.GetOriginal(context.Background(), state.ContextID, "108050", false); err == nil || got != nil {
			t.Fatal("disabled read succeeded", got, err)
		}
		if got, err := disabled.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || got != nil {
			t.Fatal("disabled Save succeeded", got, err)
		}
		if got, err := disabled.RestoreReviewed(context.Background(), state.ContextID, "108050", "1", AuditRestoreCancel); err == nil || got != nil {
			t.Fatal("disabled restore succeeded", got, err)
		}
	}
	for _, value := range []string{"", "TRUE", "1", " true ", "yes"} {
		if got, err := NewSIVICollectedService(contexts, func(string) (string, bool) { return value, true }); err == nil || got != nil {
			t.Fatal("malformed feature accepted", value, got, err)
		}
	}
	if got, err := NewSIVICollectedService(nil, func(string) (string, bool) { return "true", true }); err == nil || got != nil {
		t.Fatal("nil owner accepted", got, err)
	}
	if got, err := NewSIVICollectedService(contexts, nil); err == nil || got != nil {
		t.Fatal("nil lookup accepted", got, err)
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
			if got, err := target.RestoreReviewed(ctx, state.ContextID, "108050", "1", AuditRestorePrune); err == nil || got != nil {
				t.Fatal("nil/canceled restore succeeded", got, err)
			}
		}
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVICollectedServiceAtomicTypedAuditStrengthsAliasesAndRecovery(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for strength := 0; strength <= 3; strength++ {
			for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
				t.Run(strconv.FormatBool(extended)+"/"+strconv.Itoa(strength)+"/"+string(action), func(t *testing.T) {
					service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, extended, strength)
					unrelated := databaseBytes(t, contexts.projects.sqlite.attachments)
					for role, path := range contexts.projects.sqlite.attachments {
						if path == contexts.projects.sqlite.attachments["project"] {
							delete(unrelated, role)
						}
					}
					saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", extended, siviCollectedRequestJSON(t, original, edits))
					if err != nil || saved == nil || saved.ChangedCells != 3 || (saved.HistoryID == "") != (strength == 0) {
						t.Fatal("Collected transaction/audit strength differs", saved, err)
					}
					for i, want := range []*string{metadataText("C").Text, metadataText("V").Text, nil} {
						var value *string
						if err := db.QueryRow(`SELECT Collected FROM Sample_Veg WHERE ID=?`, 10000001+i).Scan(&value); err != nil || !reflect.DeepEqual(value, want) {
							t.Fatal("typed cycle differs", i, value, want, err)
						}
					}
					var count int
					if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE EditField='Collected'`).Scan(&count); err != nil || count != strength {
						t.Fatal("audit strengths differ", count, err)
					}
					for _, table := range []string{siviCoverHistoryTable, siviHeightHistoryTable, siviCombinedHistoryTable} {
						if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, table).Scan(&count); err != nil || count != 0 {
							t.Fatal("Collected wrote predecessor history", table, count, err)
						}
					}
					assertProfileSUFiles(t, contexts, unrelated)
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
					var history siviHeightHistory
					if err := json.Unmarshal([]byte(proposal), &history); err != nil || len(history.Changes) != strength || len(history.Committed) != strength {
						t.Fatal("typed history incomplete", history, err)
					}
					for _, change := range history.Changes {
						if change.Column != "Collected" || change.Audit.EditField != "Collected" || change.Audit.Restore || change.Audit.Flag {
							t.Fatal("unrelated column/phantom audit", change)
						}
					}
					before := databaseBytes(t, contexts.projects.sqlite.attachments)
					for _, policy := range []siviVegetationWritePolicy{siviCoverWritePolicy(), siviHeightWritePolicy(), siviCombinedWritePolicy()} {
						if got, err := contexts.restoreSIVIVegetationCells(context.Background(), state.ContextID, "108050", saved.HistoryID, action, policy); err == nil || got != nil {
							t.Fatal("predecessor restored Collected history", got, err)
						}
					}
					if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestoreCancel); err != nil || got == nil || !got.Cancelled {
						t.Fatal("cancel restore failed", got, err)
					}
					assertProfileSUFiles(t, contexts, before)
					alias := "_vEg"
					if extended {
						alias = "sAmPlE_vEg"
					}
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE EditField='Collected'`, alias); err != nil {
						t.Fatal(err)
					}
					restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action)
					if err != nil || restored == nil || restored.RestoredRows != strength || restored.CleanedVegRows != 0 ||
						restored.PrunedAuditRows != map[bool]int{true: strength, false: 0}[action == AuditRestorePrune] {
						t.Fatal("typed restoration/aliases failed", restored, err)
					}
					wants := []*string{metadataText("C").Text, metadataText("V").Text, nil}
					if strength >= 2 {
						wants[0] = nil
					}
					if strength >= 1 {
						wants[1] = metadataText("C").Text
					}
					if strength == 3 {
						wants[2] = metadataText("V").Text
					}
					for i, want := range wants {
						var value *string
						if err := db.QueryRow(`SELECT Collected FROM Sample_Veg WHERE ID=?`, 10000001+i).Scan(&value); err != nil || !reflect.DeepEqual(value, want) {
							t.Fatal("recovery changed unaudited cell or lost typed original", i, value, want, err)
						}
					}
					if action == AuditRestoreRetain {
						if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE EditField='Collected' AND typeof(Restore)='integer' AND Restore=-1`).Scan(&count); err != nil || count != strength {
							t.Fatal("Access true=-1 was not preserved", count, err)
						}
					}
					before = databaseBytes(t, contexts.projects.sqlite.attachments)
					if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || got != nil {
						t.Fatal("completed history replayed", got, err)
					}
					assertProfileSUFiles(t, contexts, before)
					assertProfileSUFiles(t, contexts, unrelated)
				})
			}
		}
	}
}

func TestSIVICollectedServiceMalformedTransportAndForeignOwnership(t *testing.T) {
	service, contexts, state, _, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
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
		assertProfileSUFiles(t, contexts, before)
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
		if got, err := service.RestoreReviewed(context.Background(), id, plot, "1", AuditRestorePrune); err == nil || got != nil {
			t.Fatal("foreign restore succeeded", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestSIVICollectedServiceNoopIsolationRollbackCASCollisionAndRetry(t *testing.T) {
	service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, false, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	if _, err := db.Exec(`CREATE TRIGGER SIVICollectedReject BEFORE UPDATE OF Collected ON Sample_Veg WHEN OLD.ID=10000003
		BEGIN SELECT RAISE(ABORT,'controlled late rejection'); END`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || got != nil {
		t.Fatal("late rejection committed earlier rows", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`DROP TRIGGER SIVICollectedReject;
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	collision, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, collision, edits)); err == nil || got != nil {
		t.Fatal("application ID collision accepted", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
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
		cell := ProjectMetadataCell{Storage: "null"}
		if value != nil {
			cell = metadataText(value.(string))
		}
		noop := []SIVICollectedEdit{{edits[0].RowID, edits[0].Form, cell, 3}}
		before = databaseBytes(t, contexts.projects.sqlite.attachments)
		if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, historical, noop)); err != nil || got == nil || got.ChangedCells != 0 || got.HistoryID != "" {
			t.Fatal("unchanged historical/full-cycle draft assigned or audited", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Collected=NULL WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	wrong := append([]SIVICollectedEdit{}, edits...)
	wrong[0].Expected = metadataText("C")
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, original, wrong)); err == nil || got != nil {
		t.Fatal("cell CAS ignored", got, err)
	}
	duplicate := append(append([]SIVICollectedEdit{}, edits...), SIVICollectedEdit{edits[0].RowID, "SubVegC-SIVI", edits[0].Expected, 1})
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCollectedRequestJSON(t, original, duplicate)); err == nil || got != nil {
		t.Fatal("multiple forms double-cycled physical Collected", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid)
	if err != nil || saved == nil || saved.ChangedCells != 3 {
		t.Fatal("failed transactions poisoned retry", saved, err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || got != nil {
		t.Fatal("stale original replayed", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVICollectedServiceRestoreDriftTamperRollbackAndOwnership(t *testing.T) {
	service, contexts, state, db, original, edits := siviCollectedServiceFixture(t, true, 3)
	valid := siviCollectedRequestJSON(t, original, edits)
	owner := contexts.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	reference := owner.attachments["VLists"]
	owner.mu.Lock()
	owner.attachments["VLists"] = owner.attachments["project"]
	owner.mu.Unlock()
	_, ownershipErr := service.SaveReviewed(context.Background(), state.ContextID, "108050", true, valid)
	owner.mu.Lock()
	owner.attachments["VLists"] = reference
	owner.mu.Unlock()
	if ownershipErr == nil {
		t.Fatal("changed file ownership authorized writes")
	}
	assertProfileSUFiles(t, contexts, before)
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
		before = databaseBytes(t, contexts.projects.sqlite.attachments)
		for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
			if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || got != nil {
				t.Fatal("unsafe restoration/pruning committed", got, err)
			}
			assertProfileSUFiles(t, contexts, before)
		}
		if _, err := db.Exec(change.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 3 {
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
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if got, err := service.SaveReviewed(ctx, state.ContextID, "108050", false, valid); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked cancellation lost context or returned success", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
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
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer restoreCancel()
	if got, err := service.RestoreReviewed(restoreCtx, state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked restoration cancellation returned success", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 3 {
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
	edits[0].Expected = metadataText(historical)
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
	var history siviHeightHistory
	if err := json.Unmarshal([]byte(proposal), &history); err != nil || len(history.Changes) != 2 || len(history.Committed) != 2 {
		t.Fatal("noop leaked into typed history", history, err)
	}
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestoreRetain); err != nil || got == nil || got.RestoredRows != 2 {
		t.Fatal("mixed noop restoration failed", got, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Collected=x'43' WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	original, err = service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	cell := ProjectMetadataCell{Storage: "blob", BlobHex: metadataText("43").Text}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false,
		siviCollectedRequestJSON(t, original, []SIVICollectedEdit{{edits[0].RowID, edits[0].Form, cell, 3}})); err == nil || got != nil || !strings.Contains(err.Error(), "not cyclable") {
		t.Fatal("owned non-text Collected coerced", got, err)
	}
	assertProfileSUFiles(t, contexts, before)
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
	for _, tamper := range []func(*siviHeightHistory){
		func(h *siviHeightHistory) { h.Changes[0].Column = "HeightA" },
		func(h *siviHeightHistory) { h.Changes[0].Column = "Cover1" },
		func(h *siviHeightHistory) { h.Plot = "foreign" },
		func(h *siviHeightHistory) { h.Project = "foreign" },
		func(h *siviHeightHistory) { h.Committed = nil },
		func(h *siviHeightHistory) { h.Changes = append(h.Changes, h.Changes[0]) },
		func(h *siviHeightHistory) { h.Committed = append(h.Committed, h.Committed[0]) },
	} {
		var history siviHeightHistory
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
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); err == nil || got != nil {
			t.Fatal("foreign/tampered history restored or pruned", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICollectedHistory" SET Proposal=? WHERE ID=?`, raw, saved.HistoryID); err != nil {
		t.Fatal(err)
	}
	if got, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 3 {
		t.Fatal("history rejection poisoned retry", got, err)
	}
}
