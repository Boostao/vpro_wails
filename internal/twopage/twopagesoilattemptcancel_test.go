package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	service := soilFacade(t, contexts)
	raw := soilAttemptJSON(t, request)
	owner := contexts.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	owner.Mu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			owner.Mu.Unlock()
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
	contexts.TwoPageSoilAttempts.Mu.Lock()
	sealed := contexts.TwoPageSoilAttempts.Requests[request.RequestID].Sealed
	contexts.TwoPageSoilAttempts.Mu.Unlock()
	if !sealed {
		t.Fatal("recovery cancellation before its snapshot unsealed a future late request")
	}
	owner.Mu.Unlock()
	unlocked = true
	if result, err := service.Save(context.Background(), state.ContextID, raw); err == nil || result != nil {
		t.Fatal("late dispatch was accepted after cancelled sealed recovery", result, err)
	}
	outcome, err := service.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "notCommitted" {
		t.Fatal("corrected read-only retry could not finish sealed certification", outcome, err)
	}
	if after := vprotest.DatabaseBytes(t, owner.Attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("cancelled recovery or its correction wrote data/schema")
	}
}

func TestTwoPageSoilAttemptFacadeStagedCancellationAndPostcommitCompletionRecovery(t *testing.T) {
	for _, postcommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "precommit", true: "postcommit"}[postcommit], func(t *testing.T) {
			contexts, state, db, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
			service := soilFacade(t, contexts)
			raw := soilAttemptJSON(t, request)
			before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			witness := func(want float64) {
				t.Helper()
				var depth float64
				if err := db.QueryRow(`SELECT UpperDepth FROM Sample_Humus WHERE rowid=-7`).Scan(&depth); err != nil || depth != want {
					t.Fatal("independent physical visibility differs", depth, want, err)
				}
				contexts.TwoPageSoilAttempts.Mu.Lock()
				active := contexts.TwoPageSoilAttempts.Requests[request.RequestID].Active
				contexts.TwoPageSoilAttempts.Mu.Unlock()
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
			contexts.TwoPageSoilAttempts.Mu.Lock()
			active := contexts.TwoPageSoilAttempts.Requests[request.RequestID].Active
			contexts.TwoPageSoilAttempts.Mu.Unlock()
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
				if after := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments); !reflect.DeepEqual(before, after) {
					t.Fatal("staged cancellation or read-only certification wrote data/schema")
				}
			}
		})
	}
}
