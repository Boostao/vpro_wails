package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func siviDeletionRestorationFixture(t *testing.T, external bool, strength int) (*ContextService, ProjectState, *sql.DB, siviDeletionRestorationRequest, *siviDeletionResult) {
	t.Helper()
	service, state, db, deletion := siviDeletionFixture(t, external, strength)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Flag=1,Other2=' Historical overlength Literal ',Cover5a=0 WHERE rowid=?`, deletion.Original.RowID); err != nil {
		t.Fatal(err)
	}
	original, err := service.readSIVIDeletionOriginal(context.Background(), state.ContextID, deletion.Plot, deletion.Form, deletion.Original.RowID)
	if err != nil {
		t.Fatal(err)
	}
	deletion.Original = original.Original
	deleted, err := service.deleteSIVIVegetation(context.Background(), state.ContextID, deletion)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, deletion.Plot, deletion.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db, siviDeletionRestorationRequest{
		RequestID: "00000000-0000-4000-8000-000000000011", ContextID: state.ContextID,
		Project: deletion.Project, Plot: deletion.Plot, HistoryID: deletion.RequestID, Action: AuditRestoreRetain,
		Expected: review.Expected,
	}, deleted
}

func assertSIVIDeletionRestorationRefused(t *testing.T, service *ContextService, contextID string, request siviDeletionRestorationRequest, lookup bool) {
	t.Helper()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.restoreSIVIDeletion(context.Background(), contextID, request); result != nil || err == nil {
		t.Fatal("invalid restoration mutated/replayed", result, err)
	}
	if lookup {
		if result, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), contextID, request); result != nil || err == nil {
			t.Fatal("invalid restoration receipt resolved", result, err)
		}
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIDeletionRestorationRetainPruneAllStrengthsAliasesAndReceipt(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, strength := range []int{0, 1, 2, 3} {
			for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
				t.Run(fmt.Sprintf("%t/%d/%s", external, strength, action), func(t *testing.T) {
					service, state, db, request, deletion := siviDeletionRestorationFixture(t, external, strength)
					request.Action = action
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='sAmPlE_vEg' WHERE ID=?`, deletion.ID); err != nil {
						t.Fatal(err)
					}
					before, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Audit", "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
					files := databaseBytes(t, service.projects.sqlite.attachments)
					review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID)
					if err != nil || review == nil || !reflect.DeepEqual(review.Original, deletion.Original) ||
						!reflect.DeepEqual(review.Columns, deletion.Columns) || len(review.Columns) != 44 ||
						review.RowID != deletion.RowID || review.ID != deletion.ID || review.HistoryID != request.HistoryID ||
						len(review.AuditsBefore) != len(deletion.Audits) || review.Expected != request.Expected {
						t.Fatal("read-only all44 review lost original/evidence", review, err)
					}
					assertProfileSUFiles(t, service, files)
					result, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request)
					if err != nil || result == nil || !result.DidCommit || result.Replayed || result.Cancelled ||
						result.RestoredRows != 1 || result.RestorationID != request.RequestID || result.HistoryID != request.HistoryID ||
						result.ContextID != state.ContextID || result.Project != request.Project || result.Plot != request.Plot ||
						result.Action != action || result.ID != deletion.ID || result.RowID != deletion.RowID || result.Actor == "" ||
						result.AuditStrength != strength || result.Form != deletion.Form ||
						result.Expected != request.Expected ||
						!reflect.DeepEqual(result.Request, request) || !reflect.DeepEqual(result.Columns, deletion.Columns) ||
						!reflect.DeepEqual(result.Original, deletion.Original) || !reflect.DeepEqual(result.Restored, &deletion.Original) ||
						!reflect.DeepEqual(result.AuditsBefore, review.AuditsBefore) {
						t.Fatal("complete exact restoration receipt failed", result, err)
					}
					if _, err := time.Parse("2006-01-02 15:04:05", result.EditWhen); err != nil {
						t.Fatal(err)
					}
					pruned := 0
					if action == AuditRestorePrune {
						pruned = len(deletion.Audits)
					}
					if result.PrunedAuditRows != pruned || len(result.AuditsAfter) != len(deletion.Audits)-pruned {
						t.Fatal("zero-audit/retain/prune counts differ", result)
					}
					restored, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, row := range restored.Rows {
						found = found || reflect.DeepEqual(row, deletion.Original)
					}
					if !found {
						t.Fatal("full raw original was not exactly reinserted, including Flag1")
					}
					after, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Audit", "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
					plan, err := planSIVIDeletionRestorationAudits(before, deletion.Audits, action)
					if err != nil || !reflect.DeepEqual(plan, after) {
						t.Fatal("source audit action changed unrelated rows or normalized aliases", err)
					}
					var reserved int
					if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=?`, deletion.ID).Scan(&reserved); err != nil || reserved != 1 {
						t.Fatal("restoration released the permanent reservation", reserved, err)
					}
					if _, err := db.Exec(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory" WHERE RequestID=?`, request.HistoryID); err != nil {
						t.Fatal(err)
					}
					raw, err := json.Marshal(result)
					if err != nil {
						t.Fatal(err)
					}
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(raw, &fields); err != nil {
						t.Fatal(err)
					}
					keys := []string{"requestId", "contextId", "project", "plot", "historyId", "expected", "restorationId", "form", "rowId", "id",
						"action", "actor", "auditStrength", "editWhen", "columns", "original", "restored", "deletion", "request",
						"auditColumns", "auditsBefore", "auditsAfter", "cancelled", "restoredRows", "prunedAuditRows", "didCommit", "replayed"}
					if len(fields) != len(keys) {
						t.Fatal("receipt wire shape differs", string(raw))
					}
					for _, key := range keys {
						if fields[key] == nil {
							t.Fatal("missing explicit lowerCamel receipt field", key)
						}
					}
					files = databaseBytes(t, service.projects.sqlite.attachments)
					expected := *result
					expected.DidCommit, expected.Replayed = false, true
					for i := 0; i < 2; i++ {
						replay, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request)
						if err != nil || !reflect.DeepEqual(replay, &expected) {
							t.Fatal("idempotent writer replay differs", replay, err)
						}
						receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), state.ContextID, request)
						if err != nil || !reflect.DeepEqual(receipt, &expected) {
							t.Fatal("read-only lost-acknowledgement receipt differs", receipt, err)
						}
						assertProfileSUFiles(t, service, files)
					}
					other := request
					other.RequestID = "00000000-0000-4000-8000-000000000012"
					assertSIVIDeletionRestorationRefused(t, service, state.ContextID, other, false)
					if review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID); review != nil || err == nil {
						t.Fatal("consumed deletion became available for a second restoration", review, err)
					}
					conflict := request
					if action == AuditRestoreRetain {
						conflict.Action = AuditRestorePrune
					} else {
						conflict.Action = AuditRestoreRetain
					}
					assertSIVIDeletionRestorationRefused(t, service, state.ContextID, conflict, true)
				})
			}
		}
	}
}

func TestSIVIDeletionRestorationCancelAndUnresolvedAreReadOnly(t *testing.T) {
	service, state, db, request, _ := siviDeletionRestorationFixture(t, true, 3)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
		t.Fatal("missing history not unresolved", receipt, err)
	}
	cancel := request
	cancel.Action = AuditRestoreCancel
	receipt, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, cancel)
	if err != nil || receipt == nil || !receipt.Cancelled || receipt.DidCommit || receipt.Replayed ||
		receipt.Restored != nil || receipt.RestorationID != "" || receipt.RestoredRows != 0 {
		t.Fatal("cancel became a durable mutation/success-shaped restored row", receipt, err)
	}
	if receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), state.ContextID, cancel); receipt != nil || err != nil {
		t.Fatal("cancel created a durable receipt", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
	var technical int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, siviDeletionRestorationTable).Scan(&technical); err != nil || technical != 0 {
		t.Fatal("cancel/unresolved created technical history", technical, err)
	}
	if receipt, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal("cancel consumed restoration authority", receipt, err)
	}
	unknown := request
	unknown.RequestID = "00000000-0000-4000-8000-000000000012"
	files = databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), state.ContextID, unknown); receipt != nil || err != nil {
		t.Fatal("unmatched receipt was not unresolved", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
	if _, err := db.Exec(`DROP TABLE "__VPRO_SIVIDeletionRestorationHistory"`); err != nil {
		t.Fatal(err)
	}
	files = databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
		t.Fatal("lost history was recreated or inferred", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
}

func TestSIVIDeletionRestorationStrictRawRequest(t *testing.T) {
	request := siviDeletionRestorationRequest{RequestID: "abcdef00-0000-4000-8000-000000000011",
		ContextID: "owner", Project: "Sample", Plot: "108050", HistoryID: "00000000-0000-4000-8000-000000000001",
		Action: AuditRestoreRetain, Expected: strings.Repeat("ab", 32)}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip siviDeletionRestorationRequest
	if err := json.Unmarshal(raw, &roundtrip); err != nil || roundtrip != request {
		t.Fatal("valid raw request failed", err)
	}
	text := string(raw)
	for _, invalid := range []string{
		strings.Replace(text, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(text, `"contextId":`, `"confirmed":true,"contextId":`, 1),
		strings.Replace(text, `"contextId":`, `"contextId":"duplicate","contextId":`, 1),
		strings.Replace(text, `"owner"`, `"\ud800"`, 1),
		strings.Replace(text, `"owner"`, "\"\xff\"", 1),
		strings.Replace(text, `"action":"retain"`, `"action":"Retain"`, 1),
		strings.Replace(text, `"action":"retain"`, `"action":null`, 1),
		strings.Replace(text, `"action":"retain"`, `"action":" retain "`, 1),
		strings.Replace(text, request.RequestID, strings.ToUpper(request.RequestID), 1),
		strings.Replace(text, request.RequestID, request.HistoryID, 1),
		strings.Replace(text, `"expected":`, `"Expected":`, 1),
		strings.Replace(text, `"expected":`, `"expected":"duplicate","expected":`, 1),
		strings.Replace(text, request.Expected, "", 1),
		strings.Replace(text, request.Expected, strings.ToUpper(request.Expected), 1),
		strings.Replace(text, request.Expected, strings.Repeat("g", 64), 1),
		strings.Replace(text, request.Expected, request.Expected[:63], 1),
		strings.Replace(text, `"expected":"`+request.Expected+`"`, `"expected":null`, 1),
		strings.Replace(text, `,"expected":"`+request.Expected+`"`, "", 1),
		text + `{}`,
	} {
		if err := json.Unmarshal([]byte(invalid), &roundtrip); err == nil {
			t.Fatal("malformed/unknown/Unicode authority accepted", invalid)
		}
	}
}

func TestSIVIDeletionRestorationDriftCollisionOwnershipAndAuditRejections(t *testing.T) {
	for _, change := range []string{
		`INSERT INTO Sample_Veg(rowid,PlotNumber,Species,ID,Cover1) VALUES(%s,'108050','new',9,0)`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','new',10000001,0)`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('outside','new',10000001,0)`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','new','10000001.0',0)`,
		`UPDATE Sample_Admin SET Plot='outside' WHERE Plot='108050'`,
		`UPDATE Sample_Env SET PlotNumber='outside' WHERE PlotNumber='108050'`,
		`ALTER TABLE Sample_Veg ADD COLUMN Changed TEXT`,
		`DELETE FROM "__VPRO_ChildIdentity"`,
		`UPDATE "__VPRO_ChildIdentity" SET ChildTable='"Other_Veg"'`,
		`DELETE FROM Sample_Audit WHERE ID=10000001`,
		`UPDATE Sample_Audit SET BeforeEdit='drift' WHERE ID=10000001`,
		`UPDATE Sample_Audit SET "Table"='Other_Veg' WHERE ID=10000001`,
		`UPDATE Sample_Audit SET "Table"='_Veg ' WHERE ID=10000001`,
		`UPDATE Sample_Audit SET Restore=-1 WHERE ID=10000001`,
		`UPDATE Sample_Audit SET Flag=-1 WHERE ID=10000001`,
		`UPDATE Sample_Audit SET Project='Other' WHERE ID=10000001`,
		`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=Proposal||' '`,
	} {
		t.Run(change, func(t *testing.T) {
			service, state, db, request, deletion := siviDeletionRestorationFixture(t, false, 3)
			query := strings.ReplaceAll(change, "%s", deletion.RowID)
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			if review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID); review != nil || err == nil {
				t.Fatal("drift/collision review accepted", review, err)
			}
			assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
			assertProfileSUFiles(t, service, files)
		})
	}
	service, state, _, request, _ := siviDeletionRestorationFixture(t, true, 3)
	for _, mutate := range []func(*siviDeletionRestorationRequest){
		func(r *siviDeletionRestorationRequest) { r.ContextID = "stale" },
		func(r *siviDeletionRestorationRequest) { r.Project = "Other" },
		func(r *siviDeletionRestorationRequest) { r.Plot = "999999" },
		func(r *siviDeletionRestorationRequest) { r.HistoryID = "00000000-0000-4000-8000-000000000099" },
		func(r *siviDeletionRestorationRequest) { r.Action = "undelete" },
		func(r *siviDeletionRestorationRequest) { r.Expected = strings.Repeat("0", 64) },
	} {
		draft := request
		mutate(&draft)
		assertSIVIDeletionRestorationRefused(t, service, state.ContextID, draft, false)
	}
	mutateContextFixture(t, state.SUPath, `UPDATE Report_SU SET PlotNumber='outside' WHERE PlotNumber='108050'`)
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, true)
}

func TestSIVIDeletionRestorationRollbackCancellationAndRetry(t *testing.T) {
	service, state, db, request, _ := siviDeletionRestorationFixture(t, true, 3)
	for _, trigger := range []string{
		`CREATE TRIGGER fail_restore AFTER INSERT ON Sample_Veg BEGIN UPDATE Sample_Veg SET Flag=-1 WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_restore AFTER INSERT ON Sample_Veg BEGIN DELETE FROM Sample_Admin; END`,
		`CREATE TRIGGER fail_restore BEFORE INSERT ON Sample_Veg BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER fail_restore AFTER UPDATE OF Restore ON Sample_Audit BEGIN UPDATE Sample_Audit SET BeforeEdit='WRONG' WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_restore BEFORE UPDATE OF Restore ON Sample_Audit BEGIN SELECT RAISE(ABORT,'retain rejected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
		if _, err := db.Exec(`DROP TRIGGER fail_restore`); err != nil {
			t.Fatal(err)
		}
	}
	request.Action = AuditRestorePrune
	if _, err := db.Exec(`CREATE TRIGGER fail_prune AFTER DELETE ON Sample_Audit BEGIN DELETE FROM "__VPRO_ChildIdentity"; END`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TRIGGER fail_prune`); err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, action := range []AuditRestoreAction{AuditRestoreCancel, AuditRestoreRetain, AuditRestorePrune} {
		draft := request
		draft.Action = action
		if result, err := service.restoreSIVIDeletion(ctx, state.ContextID, draft); result != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled context ignored", result, err)
		}
	}
	if result, err := service.lookupSIVIDeletionRestorationReceipt(ctx, state.ContextID, request); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled receipt lookup ignored", result, err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	result, err := service.restoreSIVIDeletion(ctx, state.ContextID, request)
	cancel()
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("writer-wait cancellation ignored", result, err)
	}
	assertProfileSUFiles(t, service, files)
	ctx, cancel = context.WithCancel(context.Background())
	staged := &siviCreationJournalCancellation{Context: ctx, journal: service.projects.sqlite.attachments["project"] + "-journal", cancel: cancel}
	result, err = service.restoreSIVIDeletion(staged, state.ContextID, request)
	cancel()
	if result != nil || !errors.Is(err, context.Canceled) || !staged.staged {
		t.Fatal("staged mutation did not cancel and roll back", result, err, staged.staged)
	}
	assertProfileSUFiles(t, service, files)
	if result, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request); result == nil || err != nil {
		t.Fatal("rollback consumed the deletion or blocked retry", result, err)
	}
}

func TestSIVIDeletionRestorationConcurrentOneWinnerAndReopen(t *testing.T) {
	service, state, db, request, _ := siviDeletionRestorationFixture(t, true, 3)
	type outcome struct {
		receipt *siviDeletionRestorationResult
		err     error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			draft := request
			if i == 1 {
				draft.RequestID = "00000000-0000-4000-8000-000000000012"
				draft.Action = AuditRestorePrune
			}
			receipt, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, draft)
			results <- outcome{receipt, err}
		}(i)
	}
	close(start)
	workers.Wait()
	close(results)
	var winner *siviDeletionRestorationResult
	wins, rejects := 0, 0
	for result := range results {
		if result.err == nil && result.receipt != nil && result.receipt.DidCommit {
			wins++
			winner = result.receipt
		} else if result.err != nil && result.receipt == nil {
			rejects++
		} else {
			t.Fatal("unexpected concurrent restoration outcome", result)
		}
	}
	if wins != 1 || rejects != 1 {
		t.Fatal("deletion not consumed by exactly one winner", wins, rejects)
	}
	reopened, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), reopened.ContextID, winner.Request)
	if err != nil || receipt == nil || receipt.ContextID != reopened.ContextID ||
		receipt.Request.ContextID != state.ContextID || receipt.DidCommit || !receipt.Replayed {
		t.Fatal("reopened receipt did not retain durable owner/current caller", receipt, err)
	}
	replay, err := service.restoreSIVIDeletion(context.Background(), reopened.ContextID, winner.Request)
	if err != nil || !reflect.DeepEqual(replay, receipt) {
		t.Fatal("old durable context retry reinserted or lost receipt", replay, err)
	}
	assertProfileSUFiles(t, service, files)
	var entries int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_SIVIDeletionRestorationHistory" WHERE DeletionID=?`, request.HistoryID).Scan(&entries); err != nil || entries != 1 {
		t.Fatal("multiple immutable consumptions", entries, err)
	}
}

func TestSIVIDeletionRestorationHistoryTamperAndReplayDrift(t *testing.T) {
	for _, change := range []string{
		`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET Created='invalid'`,
		`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET DeletionID='00000000-0000-4000-8000-000000000099'`,
		`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET RequestID='00000000-0000-4000-8000-000000000099'`,
		`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET Proposal=Proposal||' '`,
		`DELETE FROM "__VPRO_ChildIdentity"`,
		`UPDATE Sample_Veg SET Flag=-1 WHERE ID=10000001`,
		`DELETE FROM Sample_Veg WHERE ID=10000001`,
		`UPDATE Sample_Audit SET Restore=0 WHERE ID=10000001`,
		`UPDATE Sample_Audit SET "Table"='_Veg' WHERE ID=10000001`,
		`DELETE FROM "__VPRO_SIVIDeletionHistory"`,
	} {
		t.Run(change, func(t *testing.T) {
			service, state, db, request, _ := siviDeletionRestorationFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='Sample_Veg' WHERE ID=10000001`); err != nil {
				t.Fatal(err)
			}
			if receipt, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request); receipt == nil || err != nil {
				t.Fatal(receipt, err)
			}
			if _, err := db.Exec(change); err != nil {
				t.Fatal(err)
			}
			assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, true)
		})
	}
	service, state, db, request, _ := siviDeletionRestorationFixture(t, false, 3)
	if receipt, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal(receipt, err)
	}
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionRestorationHistory"`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []func(*siviDeletionRestorationHistory){
		func(h *siviDeletionRestorationHistory) { h.Request.Project = "Other" },
		func(h *siviDeletionRestorationHistory) { h.Request.Action = AuditRestoreCancel },
		func(h *siviDeletionRestorationHistory) { h.Deletion.Result.ID++ },
		func(h *siviDeletionRestorationHistory) { h.Restored.RowID = "999" },
		func(h *siviDeletionRestorationHistory) { h.Restored.Cells[0] = metadataText("changed") },
		func(h *siviDeletionRestorationHistory) { h.Actor = "" },
		func(h *siviDeletionRestorationHistory) { h.AuditStrength = 4 },
		func(h *siviDeletionRestorationHistory) { h.Request.Expected = strings.Repeat("0", 64) },
		func(h *siviDeletionRestorationHistory) { h.AuditsBefore = h.AuditsBefore[:len(h.AuditsBefore)-1] },
		func(h *siviDeletionRestorationHistory) { h.AuditsAfter[0].Cells[0] = metadataText("wrong") },
	} {
		var history siviDeletionRestorationHistory
		if err := json.Unmarshal([]byte(raw), &history); err != nil {
			t.Fatal(err)
		}
		corrupt(&history)
		broken, err := json.Marshal(history)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET Proposal=?`, string(broken)); err != nil {
			t.Fatal(err)
		}
		assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, true)
	}
}

func TestSIVIDeletionRestorationSharedProjectsAndForeignCorruption(t *testing.T) {
	service, state, db, deletionRequest := siviDeletionFixture(t, false, 3)
	for _, suffix := range coreTables {
		if _, err := db.Exec(`CREATE TABLE ` + quoteHeaderIdentifier("Second_"+suffix) + ` AS SELECT * FROM ` + quoteHeaderIdentifier("Sample_"+suffix)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO _table_metadata(table_name,description) VALUES('Second_Env','VP08')`); err != nil {
		t.Fatal(err)
	}
	if receipt, err := service.deleteSIVIVegetation(context.Background(), state.ContextID, deletionRequest); receipt == nil || err != nil {
		t.Fatal(receipt, err)
	}
	request := siviDeletionRestorationRequest{RequestID: "00000000-0000-4000-8000-000000000011",
		ContextID: state.ContextID, Project: "Sample", Plot: deletionRequest.Plot, HistoryID: deletionRequest.RequestID, Action: AuditRestorePrune}
	review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID)
	if err != nil {
		t.Fatal(err)
	}
	request.Expected = review.Expected
	if receipt, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal(receipt, err)
	}
	selection := contextSelection(state)
	selection.Project = "Second"
	secondState, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	secondDeletion := deletionRequest
	secondDeletion.Project, secondDeletion.ContextID, secondDeletion.RequestID = "Second", secondState.ContextID, "00000000-0000-4000-8000-000000000002"
	original, err := service.readSIVIDeletionOriginal(context.Background(), secondState.ContextID, secondDeletion.Plot, secondDeletion.Form, secondDeletion.Original.RowID)
	if err != nil {
		t.Fatal(err)
	}
	secondDeletion.Columns, secondDeletion.Original = original.Columns, original.Original
	if receipt, err := service.deleteSIVIVegetation(context.Background(), secondState.ContextID, secondDeletion); receipt == nil || err != nil {
		t.Fatal("foreign restored deletion history blocked unrelated deletion", receipt, err)
	}
	second := request
	second.ContextID, second.Project, second.RequestID, second.HistoryID = secondState.ContextID, "Second",
		"00000000-0000-4000-8000-000000000012", secondDeletion.RequestID
	secondReview, err := service.reviewSIVIDeletionRestoration(context.Background(), secondState.ContextID, second.Plot, second.HistoryID)
	if err != nil {
		t.Fatal(err)
	}
	second.Expected = secondReview.Expected
	if receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), secondState.ContextID, second); receipt != nil || err != nil {
		t.Fatal("foreign valid restoration blocked unresolved lookup", receipt, err)
	}
	if receipt, err := service.restoreSIVIDeletion(context.Background(), secondState.ContextID, second); receipt == nil || err != nil {
		t.Fatal("foreign valid restoration blocked new project commit", receipt, err)
	}
	cross := second
	cross.RequestID = request.RequestID
	assertSIVIDeletionRestorationRefused(t, service, secondState.ContextID, cross, true)
	reopened, err := service.SwitchContext(secondState.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVIDeletionRestorationReceipt(context.Background(), reopened.ContextID, request); receipt == nil || err != nil {
		t.Fatal("second project blocked first durable receipt", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionRestorationHistory" WHERE RequestID=?`, second.RequestID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var corrupt siviDeletionRestorationHistory
	if err := json.Unmarshal([]byte(raw), &corrupt); err != nil {
		t.Fatal(err)
	}
	corrupt.Request.Project = "Sample"
	broken, err := json.Marshal(corrupt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET Proposal=? WHERE RequestID=?`, string(broken), second.RequestID); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRestorationRefused(t, service, reopened.ContextID, request, true)
}

func TestSIVIDeletionRestorationAll44HistoricalCellsAndPhysicalExtremes(t *testing.T) {
	for _, rowID := range []string{"-9223372036854775808", "9223372036854775807"} {
		service, state, db, deletion := siviDeletionFixture(t, false, 3)
		if _, err := db.Exec(`UPDATE Sample_Veg SET rowid=? WHERE rowid=?`, rowID, deletion.Original.RowID); err != nil {
			t.Fatal(err)
		}
		for _, column := range deletion.Columns {
			if column.Name == "ID" || column.Name == "PlotNumber" || column.Name == "Flag" {
				continue
			}
			if _, err := db.Exec(`UPDATE Sample_Veg SET `+quoteHeaderIdentifier(column.Name)+`=? WHERE rowid=?`,
				" Invalid overlength historical Literal ", rowID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`UPDATE Sample_Veg SET Flag=1,ID=-2147483648 WHERE rowid=?`, rowID); err != nil {
			t.Fatal(err)
		}
		original, err := service.readSIVIDeletionOriginal(context.Background(), state.ContextID, deletion.Plot, deletion.Form, rowID)
		if err != nil {
			t.Fatal(err)
		}
		deletion.Original = original.Original
		deleted, err := service.deleteSIVIVegetation(context.Background(), state.ContextID, deletion)
		if err != nil || deleted == nil || len(deleted.Audits) != 42 {
			t.Fatal("all44 fixture deletion failed", deleted, err)
		}
		request := siviDeletionRestorationRequest{RequestID: "00000000-0000-4000-8000-000000000011",
			ContextID: state.ContextID, Project: deletion.Project, Plot: deletion.Plot, HistoryID: deletion.RequestID, Action: AuditRestorePrune}
		review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID)
		if err != nil {
			t.Fatal(err)
		}
		request.Expected = review.Expected
		result, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request)
		if err != nil || result == nil || result.RowID != rowID || result.ID != -2147483648 ||
			!reflect.DeepEqual(result.Restored, &deleted.Original) || result.PrunedAuditRows != 42 {
			t.Fatal("raw historical cells/signed identities were coerced or implicitly allocated", result, err)
		}
	}
}

func TestSIVIDeletionRestorationNewContextAuthorityAndReadonlyLeaseCancellation(t *testing.T) {
	service, state, _, request, _ := siviDeletionRestorationFixture(t, true, 3)
	reopened, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRestorationRefused(t, service, reopened.ContextID, request, false)
	owner := service.projects.sqlite
	files := databaseBytes(t, owner.attachments)
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	review, err := service.reviewSIVIDeletionRestoration(ctx, reopened.ContextID, request.Plot, request.HistoryID)
	cancel()
	owner.mu.Unlock()
	if review != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("held read-only review ignored lease cancellation", review, err)
	}
	assertProfileSUFiles(t, service, files)
	request.ContextID = reopened.ContextID
	if result, err := service.restoreSIVIDeletion(context.Background(), reopened.ContextID, request); result == nil || err != nil {
		t.Fatal("explicit new context could not restore", result, err)
	}
	files = databaseBytes(t, owner.attachments)
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	receipt, err := service.lookupSIVIDeletionRestorationReceipt(ctx, reopened.ContextID, request)
	cancel()
	owner.mu.Unlock()
	if receipt != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("held receipt lookup ignored lease cancellation", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIDeletionRestorationTechnicalSchemaAndPrunedAuditReuse(t *testing.T) {
	service, state, db, request, deletion := siviDeletionRestorationFixture(t, false, 3)
	if _, err := db.Exec(`CREATE TABLE "__VPRO_SIVIDeletionRestorationHistory"(RequestID TEXT NOT NULL PRIMARY KEY,DeletionID TEXT NOT NULL,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TABLE "__VPRO_SIVIDeletionRestorationHistory"`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(siviDeletionRestorationSQL + `;
		CREATE TRIGGER unsafe_history AFTER INSERT ON "__VPRO_SIVIDeletionRestorationHistory" BEGIN DELETE FROM Sample_Admin; END`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TRIGGER unsafe_history`); err != nil {
		t.Fatal(err)
	}
	request.Action = AuditRestorePrune
	if result, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request); result == nil || err != nil {
		t.Fatal("provenance repair in fixture did not permit retry", result, err)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Audit(rowid,Project,PlotNumber,"Table",EditField,ID) VALUES(?,'Sample','108050','_Veg','unrelated',123)`,
		deletion.Audits[0].RowID); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, true)
}

func TestSIVIDeletionRestorationReviewCASRejectsValidReplacedHistoricalEvidence(t *testing.T) {
	for _, strength := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprint(strength), func(t *testing.T) {
			service, state, db, request, _ := siviDeletionRestorationFixture(t, true, strength)
			var proposal string
			if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory" WHERE RequestID=?`, request.HistoryID).Scan(&proposal); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte(proposal))
			if request.Expected != hex.EncodeToString(sum[:]) {
				t.Fatal("read-only review token is not SHA256 of the complete canonical immutable proposal")
			}
			var changed siviDeletionHistory
			if err := json.Unmarshal([]byte(proposal), &changed); err != nil {
				t.Fatal(err)
			}
			columns, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: changed.Columns}, siviCreationColumns...)
			if err != nil {
				t.Fatal(err)
			}
			field, value := "Other2", metadataText(" historical LITERAL replaced after review ")
			if strength == 3 {
				// The source BOOLEAN audit remains -1, but raw Flag1 must not
				// silently become Flag7 in the reviewed restoration.
				field, value = "Flag", metadataInteger("7")
			}
			changed.Original.Cells[columns[field]] = value
			changed.Request.Original.Cells[columns[field]] = value
			if err := validateSIVIDeletionHistory(context.Background(), changed); err != nil {
				t.Fatal("replacement must be internally valid to exercise full-row review CAS", err)
			}
			replacement, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=? WHERE RequestID=?`, string(replacement), request.HistoryID); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			newReview, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID)
			if err != nil || newReview == nil || newReview.Expected == request.Expected ||
				!reflect.DeepEqual(newReview.Original.Cells[columns[field]], value) {
				t.Fatal("replacement was not a valid independent new full44 review", newReview, err)
			}
			assertProfileSUFiles(t, service, before)
			for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune, AuditRestoreCancel} {
				stale := request
				stale.Action = action
				if result, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, stale); result != nil || err == nil ||
					!strings.Contains(err.Error(), "reviewed evidence changed") {
					t.Fatal("valid replaced historical evidence bypassed the explicit review CAS", result, err)
				}
				assertProfileSUFiles(t, service, before)
			}
			request.Expected = newReview.Expected
			result, err := service.restoreSIVIDeletion(context.Background(), state.ContextID, request)
			if err != nil || result == nil || !result.DidCommit || result.Expected != newReview.Expected ||
				!reflect.DeepEqual(result.Restored, &newReview.Original) {
				t.Fatal("independently re-reviewed exact evidence could not restore", result, err)
			}
			wrongToken := request
			wrongToken.Expected = strings.Repeat("0", 64)
			assertSIVIDeletionRestorationRefused(t, service, state.ContextID, wrongToken, true)
		})
	}
}

func TestSIVIDeletionRestorationReviewCASBindsEveryTypedCellAndLiteralCase(t *testing.T) {
	service, state, db, request, _ := siviDeletionRestorationFixture(t, false, 0)
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory" WHERE RequestID=?`, request.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 44; i++ {
		var changed siviDeletionHistory
		if err := json.Unmarshal([]byte(proposal), &changed); err != nil {
			t.Fatal(err)
		}
		changed.Original.Cells[i] = metadataText(" changed raw typed cell ")
		changed.Request.Original.Cells[i] = changed.Original.Cells[i]
		actual, err := siviDeletionRestorationExpected(changed)
		if err != nil || actual == request.Expected {
			t.Fatal("review SHA256 omitted a physical original cell", i, err)
		}
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=replace(Proposal,'Historical','HISTORICAL') WHERE RequestID=?`, request.HistoryID); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if review, err := service.reviewSIVIDeletionRestoration(context.Background(), state.ContextID, request.Plot, request.HistoryID); err != nil ||
		review == nil || review.Expected == request.Expected {
		t.Fatal("valid literal recasing was normalized out of the full evidence token", review, err)
	}
	assertSIVIDeletionRestorationRefused(t, service, state.ContextID, request, false)
	assertProfileSUFiles(t, service, before)
}
