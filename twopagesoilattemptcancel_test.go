package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTwoPageSoilAttemptCancellationAfterSealingRetainsLateDispatchRefusal(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	raw := soilAttemptJSON(t, request)
	owner := contexts.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	owner.mu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			owner.mu.Unlock()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	sealedNotice := make(chan struct{})
	go func() {
		result, err := service.recoverTwoPageSoilOutcome(ctx, state.ContextID, raw, func() { close(sealedNotice) })
		if result != nil {
			done <- errors.New("cancelled sealed recovery returned an outcome")
			return
		}
		done <- err
	}()
	select {
	case <-sealedNotice:
	case <-time.After(10 * time.Second):
		t.Fatal("recovery did not seal its request before waiting for the owned SQLite snapshot")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("owned in-flight recovery did not surface cancellation", err)
	}
	contexts.twoPageSoilAttempts.mu.Lock()
	sealed := contexts.twoPageSoilAttempts.requests[request.RequestID].sealed
	contexts.twoPageSoilAttempts.mu.Unlock()
	if !sealed {
		t.Fatal("recovery cancellation before its snapshot unsealed a future late request")
	}
	owner.mu.Unlock()
	unlocked = true
	if result, err := service.Save(context.Background(), state.ContextID, raw); err == nil || result != nil {
		t.Fatal("late dispatch was accepted after cancelled sealed recovery", result, err)
	}
	outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" {
		t.Fatal("corrected read-only retry could not finish sealed certification", outcome, err)
	}
	if after := databaseBytes(t, owner.attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("cancelled recovery or its correction wrote data/schema")
	}
}

func TestTwoPageSoilAttemptFacadeStagedCancellationAndPostcommitCompletionRecovery(t *testing.T) {
	for _, postcommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "precommit", true: "postcommit"}[postcommit], func(t *testing.T) {
			contexts, state, db, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
			service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
			raw := soilAttemptJSON(t, request)
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			witness := func(want float64) {
				t.Helper()
				var depth float64
				if err := db.QueryRow(`SELECT UpperDepth FROM Sample_Humus WHERE rowid=-7`).Scan(&depth); err != nil || depth != want {
					t.Fatal("independent physical visibility differs", depth, want, err)
				}
				contexts.twoPageSoilAttempts.mu.Lock()
				active := contexts.twoPageSoilAttempts.requests[request.RequestID].active
				contexts.twoPageSoilAttempts.mu.Unlock()
				if active != 1 {
					t.Fatal("actual facade observer lost its registered active request", active)
				}
			}
			var beforeCommit func(*sql.Tx) error
			var afterCommit func(*twoPageSoilWriteResult) error
			if postcommit {
				afterCommit = func(result *twoPageSoilWriteResult) error {
					witness(5)
					if result == nil || !result.DidCommit {
						t.Fatal("postcommit observer did not retain the committed receipt", result)
					}
					cancel()
					return errors.Join(ctx.Err(), errors.New("owned completion failure"))
				}
			} else {
				beforeCommit = func(*sql.Tx) error {
					witness(-0.25)
					cancel()
					return ctx.Err()
				}
			}
			result, err := service.saveTwoPageSoilObserved(ctx, state.ContextID, raw, beforeCommit, afterCommit)
			if !errors.Is(err, context.Canceled) || !postcommit && result != nil ||
				postcommit && (result == nil || !result.DidCommit) {
				t.Fatal("facade completion did not preserve the actual commit boundary", result, err)
			}
			contexts.twoPageSoilAttempts.mu.Lock()
			active := contexts.twoPageSoilAttempts.requests[request.RequestID].active
			contexts.twoPageSoilAttempts.mu.Unlock()
			if active != 0 {
				t.Fatal("failed completion leaked an active facade attempt", active)
			}
			outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
			if err != nil || outcome == nil {
				t.Fatal(outcome, err)
			}
			if postcommit && (outcome.Status != "committed" || outcome.Receipt == nil ||
				!reflect.DeepEqual(outcome.Receipt.History, result.History)) ||
				!postcommit && (outcome.Status != "notCommitted" || outcome.Receipt != nil) {
				t.Fatal("read-only recovery confused pre/postcommit failure provenance", outcome)
			}
			if !postcommit {
				if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
					t.Fatal("staged cancellation or read-only certification wrote data/schema")
				}
			}
		})
	}
}
