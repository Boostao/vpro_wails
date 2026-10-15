package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestTwoPageVegetationAttemptUnseenSealingAndCorrectedNewUUID(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
			t.Run(fmt.Sprintf("%s/external-%v", form, external), func(t *testing.T) {
				contexts, state, _, request := vegetationWriteFixture(t, external, form, 3)
				enabled := vegetationFacade(t, contexts, map[string]string{
					twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
				})
				readonly := vegetationFacade(t, contexts, map[string]string{twoPageVegetationReviewEnvironment: "true"})
				raw := vegetationAttemptJSON(t, request)
				before := databaseBytes(t, contexts.projects.sqlite.attachments)
				outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
				if err != nil || outcome == nil || outcome.Status != "notCommitted" || outcome.Receipt != nil ||
					outcome.ContextID != state.ContextID || outcome.RequestID != request.RequestID ||
					outcome.RequestJSON != raw || outcome.Reason == "" {
					t.Fatal("unseen matching RPC not safely sealed", outcome, err)
				}
				if result, err := enabled.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
					t.Fatal("late sealed request committed", result, err)
				}
				request.Edits[0].Value = siviReal(6)
				if result, err := enabled.Save(context.Background(), state.ContextID, vegetationAttemptJSON(t, request)); result != nil || err == nil {
					t.Fatal("sealed UUID adopted changed payload", result, err)
				}
				if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
					t.Fatal("absence sealing/refusal created data/audits/history")
				}
				request.RequestID = "b0cbfa00-1234-4123-8123-000000000002"
				if result, err := enabled.Save(context.Background(), state.ContextID, vegetationAttemptJSON(t, request)); result == nil || err != nil || !result.DidCommit {
					t.Fatal("explicit corrected new UUID could not commit", result, err)
				}
			})
		}
	}
}

func TestTwoPageVegetationAttemptSharedActiveOwnershipAndCommittedSealing(t *testing.T) {
	contexts, state, db, request := vegetationWriteFixture(t, true, "SubVegAXL", 3)
	first := vegetationFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
	})
	second := vegetationFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
	})
	readonly := vegetationFacade(t, contexts, map[string]string{twoPageVegetationReviewEnvironment: "true"})
	raw := vegetationAttemptJSON(t, request)
	type completion struct {
		result *TwoPageVegetationWriteResult
		err    error
	}
	completed := make(chan completion, 2)
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			owner.mu.Unlock()
		}
	}()
	for _, facade := range []*TwoPageVegetationService{first, second} {
		go func(service *TwoPageVegetationService) {
			result, err := service.Save(context.Background(), state.ContextID, raw)
			completed <- completion{result, err}
		}(facade)
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		contexts.twoPageVegetationAttempts.mu.Lock()
		entry := contexts.twoPageVegetationAttempts.requests[request.RequestID]
		active := 0
		if entry != nil {
			active = entry.active
		}
		contexts.twoPageVegetationAttempts.mu.Unlock()
		if active == 2 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("both actual facade writers did not register shared active ownership")
		case <-time.After(time.Millisecond):
		}
	}
	outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "running" || outcome.Receipt != nil {
		t.Fatal("active writer certified absent while physical writer was held", outcome, err)
	}
	owner.mu.Unlock()
	unlocked = true
	commits, replays := 0, 0
	for range 2 {
		select {
		case result := <-completed:
			if result.err != nil || result.result == nil {
				t.Fatal("shared facade completion failed", result.result, result.err)
			}
			if result.result.DidCommit {
				commits++
			}
			if result.result.Replayed {
				replays++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("shared facade completion timed out")
		}
	}
	if commits != 1 || replays != 1 || soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
		t.Fatal("shared facade lost one commit/exact replay", commits, replays)
	}
	before := databaseBytes(t, owner.attachments)
	outcome, err = readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "committed" || outcome.Receipt == nil ||
		outcome.Receipt.DidCommit || !outcome.Receipt.Replayed {
		t.Fatal("committed sealed request lost read-only provenance", outcome, err)
	}
	replay, err := first.Save(context.Background(), state.ContextID, raw)
	if err != nil || replay == nil || replay.DidCommit || !replay.Replayed ||
		!reflect.DeepEqual(replay.History, outcome.Receipt.History) ||
		!reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("sealed committed replay changed history/storage", replay, err)
	}
}

func TestTwoPageVegetationAttemptRollbackCancellationAndOwnerCorrection(t *testing.T) {
	contexts, state, db, request := vegetationWriteFixture(t, false, "SubVegCXL", 3)
	service := vegetationFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
	})
	if _, err := db.Exec(`CREATE TRIGGER reject_vegetation BEFORE UPDATE ON Sample_Veg BEGIN SELECT RAISE(ABORT,'attempt rollback'); END`); err != nil {
		t.Fatal(err)
	}
	raw := vegetationAttemptJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if result, err := service.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
		t.Fatal("actual rejected mutation did not roll back", result, err)
	}
	outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" ||
		!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("rolled back attempt was not safely retired without storage changes", outcome, err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_vegetation`); err != nil {
		t.Fatal(err)
	}
	if result, err := service.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
		t.Fatal("retired UUID became writable after correction", result, err)
	}
	request.RequestID = "b0cbfa00-1234-4123-8123-000000000002"
	raw = vegetationAttemptJSON(t, request)
	ctx, cancel := context.WithCancel(context.Background())
	outcome, err = service.recoverTwoPageVegetationOutcome(ctx, state.ContextID, raw, cancel)
	if outcome != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled sealed read returned absence certificate", outcome, err)
	}
	if result, err := service.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
		t.Fatal("cancelled recovery unsealed a late dispatch", result, err)
	}
	outcome, err = service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" {
		t.Fatal("cancelled sealed recovery could not be retried readonly", outcome, err)
	}
	request.RequestID = "b0cbfa00-1234-4123-8123-000000000003"
	raw = vegetationAttemptJSON(t, request)
	owner := contexts.projects.sqlite
	path := owner.attachments["VUser"]
	owner.attachments["VUser"] = owner.attachments["project"]
	if outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw); outcome != nil || err == nil {
		t.Fatal("retargeted owner certified original absence", outcome, err)
	}
	owner.attachments["VUser"] = path
	outcome, err = service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" {
		t.Fatal("original physical owner correction did not permit readonly certification", outcome, err)
	}
	if result, err := service.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
		t.Fatal("failed owner observation unsealed the UUID", result, err)
	}
	request.RequestID = "b0cbfa00-1234-4123-8123-000000000004"
	if result, err := service.Save(context.Background(), state.ContextID, vegetationAttemptJSON(t, request)); result == nil || err != nil || !result.DidCommit {
		t.Fatal("new UUID failed after exact source/owner correction", result, err)
	}
}

func TestTwoPageVegetationAttemptCanonicalPayloadIsImmutableAndPreservesRealBits(t *testing.T) {
	contexts, _, _, request := vegetationWriteFixture(t, false, "SubVegAXL", 3)
	*vegetationPlanCell(request.Original, "3", "Cover1") = siviReal(0)
	started, err := contexts.beginTwoPageVegetationAttempt(context.Background(), request)
	if err != nil || !started {
		t.Fatal("initial exact typed payload refused", started, err)
	}
	contexts.finishTwoPageVegetationAttempt(request.RequestID)
	*vegetationPlanCell(request.Original, "3", "Cover1") = siviReal(math.Copysign(0, -1))
	if started, err := contexts.beginTwoPageVegetationAttempt(context.Background(), request); started || err == nil {
		t.Fatal("caller mutation or signed-zero alias changed the frozen request", started, err)
	}
	*vegetationPlanCell(request.Original, "3", "Cover1") = siviReal(0)
	started, err = contexts.beginTwoPageVegetationAttempt(context.Background(), request)
	if err != nil || !started {
		t.Fatal("exact original payload retry no longer matches", started, err)
	}
	contexts.finishTwoPageVegetationAttempt(request.RequestID)
}

func TestTwoPageVegetationAttemptRotatedAbsenceIsUnknownAndLedgerCancellation(t *testing.T) {
	contexts, state, db := twoPageSoilReadFixture(t, true, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Veg(ID,PlotNumber,Species,Cover7) VALUES(1299001,'108050','TEST',0)`); err != nil {
		t.Fatal(err)
	}
	var rowID string
	if err := db.QueryRow(`SELECT rowid FROM Sample_Veg WHERE ID=1299001`).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	original, err := contexts.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", "SubVegDXL")
	if err != nil {
		t.Fatal(err)
	}
	request := twoPageVegetationRequest{RequestID: vegetationWriteUUID, Original: original, Edits: []twoPageVegetationEdit{
		{ContextID: state.ContextID, Form: "SubVegDXL", RowID: rowID, Column: "Cover7",
			Expected: *vegetationPlanCell(original, rowID, "Cover7"), Value: siviReal(5)},
	}}
	service := vegetationFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
	})
	raw := vegetationAttemptJSON(t, request)
	next, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil || next.ContextID == state.ContextID {
		t.Fatal("actual source context did not rotate", next, err)
	}
	outcome, err := service.RecoverOutcome(context.Background(), next.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "unknown" || outcome.Receipt != nil {
		t.Fatal("absent history in another context certified original no-commit", outcome, err)
	}
	request.RequestID = "b0cbfa00-1234-4123-8123-000000000002"
	raw = vegetationAttemptJSON(t, request)
	ledger := &contexts.twoPageVegetationAttempts
	ledger.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	result, err := service.Save(ctx, next.ContextID, raw)
	cancel()
	ledger.mu.Unlock()
	if result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("queued ledger acquisition ignored cancellation", result, err)
	}
	if _, found := ledger.requests[request.RequestID]; found {
		t.Fatal("cancelled ledger acquisition created an active attempt")
	}
}

func TestTwoPageVegetationAttemptInFlightCancelAndPostcommitResult(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint("committed-", committed), func(t *testing.T) {
			contexts, state, db, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
			service := vegetationFacade(t, contexts, map[string]string{
				twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
			})
			raw := vegetationAttemptJSON(t, request)
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var beforeCommit func(*sql.Tx) error
			var afterCommit func(*twoPageVegetationWriteResult) error
			if committed {
				afterCommit = func(*twoPageVegetationWriteResult) error {
					cancel()
					return ctx.Err()
				}
			} else {
				beforeCommit = func(*sql.Tx) error {
					cancel()
					return ctx.Err()
				}
			}
			result, err := service.saveTwoPageVegetationObserved(ctx, state.ContextID, raw, beforeCommit, afterCommit)
			if !errors.Is(err, context.Canceled) || committed && (result == nil || !result.DidCommit) ||
				!committed && result != nil {
				t.Fatal("completion cancellation erased/misrepresented irreversible result", result, err)
			}
			ledger := &contexts.twoPageVegetationAttempts
			ledger.mu.Lock()
			active := ledger.requests[request.RequestID].active
			ledger.mu.Unlock()
			if active != 0 {
				t.Fatal("completed cancellation retained active ownership", active)
			}
			outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
			want := "notCommitted"
			if committed {
				want = "committed"
			}
			if err != nil || outcome == nil || outcome.Status != want {
				t.Fatal("completion recovery status differs from physical commit", outcome, err)
			}
			if committed {
				if outcome.Receipt == nil || !reflect.DeepEqual(outcome.Receipt.History, result.History) ||
					soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
					t.Fatal("postcommit cancelled response lost complete durable receipt")
				}
			} else if outcome.Receipt != nil || !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("precommit cancellation/certification left data/audit/history")
			}
		})
	}
}
