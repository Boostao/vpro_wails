package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func soilAttemptJSON(t *testing.T, request twoPageSoilRequest) string {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestTwoPageSoilAttemptSealsUnseenLateDispatchWithoutWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
			t.Run(fmt.Sprintf("%s/external-%v", form, external), func(t *testing.T) {
				contexts, state, _, request := soilWriteFixture(t, external, form, 3)
				enabled := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
				readonly := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true"})
				raw := soilAttemptJSON(t, request)
				before := databaseBytes(t, contexts.projects.sqlite.attachments)
				outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
				if err != nil || outcome == nil || outcome.Status != "notCommitted" || outcome.Receipt != nil ||
					outcome.RequestID != request.RequestID || outcome.ContextID != state.ContextID || outcome.RequestJSON != raw || outcome.Reason == "" {
					t.Fatal("unseen exact request was not sealed against late dispatch", outcome, err)
				}
				if result, err := enabled.Save(context.Background(), state.ContextID, raw); err == nil || result != nil {
					t.Fatal("late dispatch committed after absence certification", result, err)
				}
				if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
					t.Fatal("sealing/recovery/late refusal wrote database data or schema")
				}
				request.Edits[0].Value = metadataText("LF")
				if result, err := readonly.RecoverOutcome(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err == nil || result != nil {
					t.Fatal("sealed UUID silently adopted a changed payload", result, err)
				}
				if result, err := enabled.Save(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err == nil || result != nil {
					t.Fatal("sealed UUID collision granted a new writer", result, err)
				}
				request.RequestID = "f4b1aa00-1234-4123-8123-000000000002"
				result, err := enabled.Save(context.Background(), state.ContextID, soilAttemptJSON(t, request))
				if err != nil || result == nil || !result.DidCommit {
					t.Fatal("explicit new UUID was refused after independently unchanged source recovery", result, err)
				}
			})
		}
	}
}

func TestTwoPageSoilAttemptRunningSharedFacadesAndCompletedReplay(t *testing.T) {
	contexts, state, db, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	first := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	second := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	readonly := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true"})
	raw := soilAttemptJSON(t, request)
	type completion struct {
		result *TwoPageSoilWriteResult
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
	for _, facade := range []*TwoPageSoilService{first, second} {
		go func(service *TwoPageSoilService) {
			result, err := service.Save(context.Background(), state.ContextID, raw)
			completed <- completion{result, err}
		}(facade)
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		contexts.twoPageSoilAttempts.mu.Lock()
		entry := contexts.twoPageSoilAttempts.requests[request.RequestID]
		active := 0
		if entry != nil {
			active = entry.active
		}
		contexts.twoPageSoilAttempts.mu.Unlock()
		if active == 2 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("both actual facade dispatches did not register before entering the owned writer")
		case <-time.After(time.Millisecond):
		}
	}
	outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "running" || outcome.Receipt != nil {
		t.Fatal("active owned attempts were certified while the source writer was held", outcome, err)
	}
	owner.mu.Unlock()
	unlocked = true
	commits, replays := 0, 0
	for range 2 {
		receipt := <-completed
		if receipt.err != nil || receipt.result == nil {
			t.Fatal("shared facade completion failed", receipt.result, receipt.err)
		}
		if receipt.result.DidCommit {
			commits++
		}
		if receipt.result.Replayed {
			replays++
		}
	}
	if commits != 1 || replays != 1 || soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
		t.Fatal("shared facade concurrency lost one-commit/exact-replay ownership", commits, replays)
	}
	before := databaseBytes(t, owner.attachments)
	outcome, err = readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "committed" || outcome.Receipt == nil ||
		outcome.Receipt.DidCommit || !outcome.Receipt.Replayed {
		t.Fatal("completed exact request lost read-only durable receipt", outcome, err)
	}
	if replay, err := first.Save(context.Background(), state.ContextID, raw); err != nil || replay == nil ||
		replay.DidCommit || !replay.Replayed || !reflect.DeepEqual(replay.History, outcome.Receipt.History) {
		t.Fatal("sealed committed UUID lost existing explicit replay behavior", replay, err)
	}
	if after := databaseBytes(t, owner.attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("outcome certification or sealed replay wrote source/audit/history")
	}
}

func TestTwoPageSoilAttemptRollbackCancellationAndCorrectedNewUUID(t *testing.T) {
	contexts, state, db, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
	service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	if _, err := db.Exec(`CREATE TRIGGER reject_soil BEFORE UPDATE ON Sample_Humus BEGIN SELECT RAISE(ABORT,'attempt rollback'); END`); err != nil {
		t.Fatal(err)
	}
	raw := soilAttemptJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if result, err := service.Save(context.Background(), state.ContextID, raw); err == nil || result != nil {
		t.Fatal("fixture rejection did not fail the actual facade writer", result, err)
	}
	outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" || outcome.Receipt != nil {
		t.Fatal("completed rolled-back attempt could not be safely retired", outcome, err)
	}
	if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("rejected mutation or its retirement changed source/schema")
	}
	if _, err := db.Exec(`DROP TRIGGER reject_soil`); err != nil {
		t.Fatal(err)
	}
	if result, err := service.Save(context.Background(), state.ContextID, raw); err == nil || result != nil {
		t.Fatal("old retired UUID retried after correction", result, err)
	}
	request.RequestID = "f4b1aa00-1234-4123-8123-000000000002"
	raw = soilAttemptJSON(t, request)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.Save(cancelled, state.ContextID, raw); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled facade call changed state", result, err)
	}
	if outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw); err != nil ||
		outcome == nil || outcome.Status != "notCommitted" {
		t.Fatal("cancelled/unseen attempt did not seal against later delivery", outcome, err)
	}
	request.RequestID = "f4b1aa00-1234-4123-8123-000000000003"
	if result, err := service.Save(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err != nil ||
		result == nil || !result.DidCommit {
		t.Fatal("explicit corrected new request could not commit", result, err)
	}
}

func TestTwoPageSoilAttemptContextRotationNeverCertifiesAbsentOldHistory(t *testing.T) {
	for _, commit := range []bool{false, true} {
		t.Run(fmt.Sprint(commit), func(t *testing.T) {
			contexts, state, _, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
			service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
			raw := soilAttemptJSON(t, request)
			if commit {
				if _, err := service.Save(context.Background(), state.ContextID, raw); err != nil {
					t.Fatal(err)
				}
			}
			selection := contexts.projects.sqlite.selection
			next, err := contexts.SwitchContext(state.ContextID, ContextSelection{Project: selection.Project, ProjectPath: selection.ProjectPath,
				SU: selection.SU, SUPath: selection.SUPath, Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath})
			if err != nil || next.ContextID == state.ContextID {
				t.Fatal("actual owned context did not rotate", next, err)
			}
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			if outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw); err == nil || outcome != nil {
				t.Fatal("stale read context retained outcome authority", outcome, err)
			}
			outcome, err := service.RecoverOutcome(context.Background(), next.ContextID, raw)
			if err != nil || outcome == nil {
				t.Fatal(outcome, err)
			}
			if commit && (outcome.Status != "committed" || outcome.Receipt == nil ||
				outcome.Receipt.History.Request.Original.ContextID != state.ContextID) ||
				!commit && (outcome.Status != "unknown" || outcome.Receipt != nil) {
				t.Fatal("rotation changed committed provenance or certified absence in another owner", outcome)
			}
			if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
				t.Fatal("rotated outcome recovery wrote physical data")
			}
		})
	}
}

func TestTwoPageSoilAttemptRecoveryGatesStrictTransportAndLeaseCancellation(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
	raw := soilAttemptJSON(t, request)
	for _, values := range []map[string]string{{}, {twoPageSoilWritingEnvironment: "true"}} {
		service := soilFacade(t, contexts, values)
		if outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw); err == nil || outcome != nil {
			t.Fatal("outcome recovery bypassed independent review", outcome, err)
		}

	}
	service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true"})
	if outcome, err := service.RecoverOutcome(nil, state.ContextID, raw); err == nil || outcome != nil {
		t.Fatal("nil recovery context accepted", outcome, err)
	}
	if outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, `{"requestId":"bad"}`); err == nil || outcome != nil {
		t.Fatal("invalid strict outcome request accepted", outcome, err)
	}
	contexts.twoPageSoilAttempts.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	outcome, err := service.RecoverOutcome(ctx, state.ContextID, raw)
	contexts.twoPageSoilAttempts.mu.Unlock()
	if !errors.Is(err, context.Canceled) || outcome != nil {
		t.Fatal("cancelled recovery waited for or crossed the attempt lease", outcome, err)
	}
}
