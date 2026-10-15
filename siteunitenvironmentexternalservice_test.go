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

func externalSUEnvironmentServiceFixture(t *testing.T) (*ExternalSUEnvironmentService, ProjectState, ExternalSUEnvironmentRequest, string) {
	t.Helper()
	contexts, state, _ := externalSUEnvironmentFixture(t)
	service := NewExternalSUEnvironmentService(contexts, true)
	review, err := service.GetReview(t.Context(), state.ContextID)
	if err != nil || review == nil || review.ProjectPath != state.ProjectPath || review.SUPath != state.SUPath ||
		review.Review.SU != "External" || len(review.Review.Changes) != 4 {
		t.Fatal("owned public external review unavailable", review, err)
	}
	request := ExternalSUEnvironmentRequest{review.ApprovalHash, "96e9f951-e521-4efb-a42e-437da0f9e81c", review.SessionID, true}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, request, string(raw)
}

func TestExternalSUEnvironmentServiceDefaultOffAndStrictRequests(t *testing.T) {
	service, state, request, raw := externalSUEnvironmentServiceFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	disabled := NewExternalSUEnvironmentService(service.contexts, false)
	if _, err := disabled.GetReview(t.Context(), state.ContextID); err == nil {
		t.Fatal("disabled service granted source review authority")
	}
	if _, err := disabled.TransferReviewed(t.Context(), state.ContextID, raw); err == nil {
		t.Fatal("disabled service granted write authority")
	}
	for _, malformed := range []string{
		`{}`, `null`, `[]`, raw + `{}`,
		strings.Replace(raw, `"confirmed":true`, `"confirmed":false`, 1),
		strings.Replace(raw, `"confirmed":true`, `"confirmed":null`, 1),
		strings.Replace(raw, `"confirmed":true`, `"confirmed":"true"`, 1),
		strings.Replace(raw, `"confirmed":true`, `"confirmed":true,"confirmed":false`, 1),
		strings.Replace(raw, `"confirmed":true`, `"confirmed":true,"\u0063onfirmed":true`, 1),
		strings.Replace(raw, `"confirmed":true`, `"confirmed":true,"unknown":1`, 1),
		strings.Replace(raw, request.RequestID, strings.ToUpper(request.RequestID), 1),
		strings.Replace(raw, request.RequestID, "96e9f951-e521-3efb-a42e-437da0f9e81c", 1),
		strings.Replace(raw, request.ApprovalHash, strings.ToUpper(request.ApprovalHash), 1),
		strings.Replace(raw, request.SessionID, `\ud800`, 1),
		strings.Replace(raw, request.SessionID, string([]byte{0xff}), 1),
	} {
		var decoded ExternalSUEnvironmentRequest
		if err := json.Unmarshal([]byte(malformed), &decoded); err == nil {
			t.Fatal("malformed request decoded", malformed)
		}
		if result, err := service.TransferReviewed(t.Context(), state.ContextID, malformed); result != nil || err == nil {
			t.Fatal("malformed transfer accepted", result, err)
		}
		if result, err := service.RecoverOutcome(t.Context(), state.ContextID, malformed); result != nil || err == nil {
			t.Fatal("malformed recovery accepted", result, err)
		}
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func TestExternalSUEnvironmentServiceCommittedOnceAndImmutableRecovery(t *testing.T) {
	service, state, request, raw := externalSUEnvironmentServiceFixture(t)
	result, err := service.TransferReviewed(t.Context(), state.ContextID, raw)
	if err != nil || result == nil || result.Status != "committed" || result.Receipt == nil ||
		result.Receipt.ChangedCells != 4 || result.Receipt.HistoryID != "1" || result.ErrorMessage != "" {
		t.Fatal("exact service transfer did not commit", result, err)
	}
	var proposal string
	if err := service.contexts.projects.sqlite.conn.QueryRowContext(t.Context(),
		`SELECT Proposal FROM project.__VPRO_SUEnvironmentHistory WHERE ID=1`).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var history struct {
		Attempt externalSUEnvironmentAttemptIdentity `json:"attempt"`
	}
	if err := json.Unmarshal([]byte(proposal), &history); err != nil ||
		history.Attempt != (externalSUEnvironmentAttemptIdentity{request.ApprovalHash, request.RequestID, request.SessionID}) {
		t.Fatal("atomic history lost exact service/request ownership", history, err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	result.Receipt.HistoryID = "Mutated return"
	result.Status = "Mutated return"
	replay, err := service.TransferReviewed(t.Context(), state.ContextID, raw)
	if err != nil || replay.Status != "committed" || replay.Receipt.HistoryID != "1" {
		t.Fatal("completed request replayed or mutable receipt changed ledger", replay, err)
	}
	recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered == nil || recovered.Status != "completed" || recovered.Outcome.Receipt.HistoryID != "1" {
		t.Fatal("readonly recovery lost original receipt", recovered, err)
	}
	recovered.Outcome.Receipt.HistoryID = "Mutated recovery"
	again, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || again.Outcome.Receipt.HistoryID != "1" {
		t.Fatal("returned recovery mutated retained receipt", again, err)
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func TestExternalSUEnvironmentServiceApprovalAndUUIDOwnership(t *testing.T) {
	for _, variant := range []string{"foreign-approval", "context", "session", "payload"} {
		t.Run(variant, func(t *testing.T) {
			service, state, request, raw := externalSUEnvironmentServiceFixture(t)
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if variant == "payload" || variant == "context" {
				if _, err := service.RecoverOutcome(t.Context(), state.ContextID, raw); err != nil {
					t.Fatal(err)
				}
			}
			contextID := state.ContextID
			switch variant {
			case "foreign-approval", "payload":
				request.ApprovalHash = strings.Repeat("0", 64)
			case "context":
				contextID += "-foreign"
			case "session":
				request.SessionID += "-foreign"
			}
			bytes, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := service.TransferReviewed(t.Context(), contextID, string(bytes)); result != nil || err == nil {
				t.Fatal("foreign ownership acquired transfer", result, err)
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
}

func TestExternalSUEnvironmentServiceHeldCommitRecoveryAndSealing(t *testing.T) {
	service, state, _, raw := externalSUEnvironmentServiceFixture(t)
	written, release := make(chan struct{}), make(chan struct{})
	type response struct {
		result *ExternalSUEnvironmentOutcome
		err    error
	}
	done := make(chan response, 1)
	go func() {
		result, err := service.transferReviewed(t.Context(), state.ContextID, raw, externalSUEnvironmentWriteHooks{}, func() {
			close(written)
			<-release
		})
		done <- response{result, err}
	}()
	select {
	case <-written:
	case <-time.After(20 * time.Second):
		close(release)
		t.Fatal("held actual commit never reached response boundary")
	}
	var history int
	if err := service.contexts.projects.sqlite.conn.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM project.__VPRO_SUEnvironmentHistory`).Scan(&history); err != nil || history != 1 {
		close(release)
		t.Fatal("hold did not follow actual committed history", history, err)
	}
	recovery, recoverErr := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	concurrent, concurrentErr := service.TransferReviewed(t.Context(), state.ContextID, raw)
	close(release)
	finished := <-done
	if recoverErr != nil || recovery.Status != "running" || recovery.Outcome != nil || concurrent != nil || concurrentErr == nil {
		t.Fatal("held active mutation was sealed or replayed", recovery, recoverErr, concurrent, concurrentErr)
	}
	if finished.err != nil || finished.result.Status != "committed" {
		t.Fatal("held committed result lost", finished)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	recovery, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovery.Status != "completed" || recovery.Outcome.Status != "committed" {
		t.Fatal("lost response not recovered readonly", recovery, err)
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func TestExternalSUEnvironmentServiceRecoveryBeforeArrivalAndRetiredOwner(t *testing.T) {
	service, state, request, raw := externalSUEnvironmentServiceFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	recovery, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovery.Status != "completed" || recovery.Outcome.Status != "notCommitted" ||
		recovery.Outcome.Receipt != nil || recovery.Outcome.ErrorMessage == "" {
		t.Fatal("prearrival sealing did not establish exact noncommit", recovery, err)
	}
	arrival, err := service.TransferReviewed(t.Context(), state.ContextID, raw)
	if err != nil || !reflect.DeepEqual(arrival, recovery.Outcome) {
		t.Fatal("sealed delayed request was allowed to write", arrival, err)
	}
	freshService := NewExternalSUEnvironmentService(service.contexts, true)
	if result, err := freshService.RecoverOutcome(t.Context(), state.ContextID, raw); result != nil || err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatal("fresh service invented retired-owner noncommit", result, err)
	}
	request.RequestID = "38ff2478-d562-41a5-903e-fa99a2cb695c"
	bytes, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	service.enabled = false
	if result, err := service.RecoverOutcome(t.Context(), state.ContextID, string(bytes)); result == nil || err != nil {
		t.Fatal("disabled writer prevented readonly sealing", result, err)
	}
	if _, err := service.TransferReviewed(t.Context(), state.ContextID, string(bytes)); err == nil {
		t.Fatal("disabled writer accepted sealed new request")
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func TestExternalSUEnvironmentServiceSnapshotDriftCancellationAndCleanup(t *testing.T) {
	for _, variant := range []string{"returned-review", "source-drift", "late-cancel", "cleanup"} {
		t.Run(variant, func(t *testing.T) {
			service, state, _, raw := externalSUEnvironmentServiceFixture(t)
			if variant == "returned-review" {
				review, err := service.GetReview(t.Context(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				source := service.reviews[review.ApprovalHash]
				original, err := json.Marshal(source.Review)
				if err != nil {
					t.Fatal(err)
				}
				review.Review.Admin.Rows[0].RowID = "Mutated return"
				review.Review.Changes[0].Field = "Mutated return"
				after, err := json.Marshal(source.Review)
				if err != nil || string(original) != string(after) {
					t.Fatal("public review mutated private source", err)
				}
			}
			if variant == "source-drift" {
				db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["su"], "rw"))
				if err != nil {
					t.Fatal(err)
				}
				_, writeErr := db.Exec(`UPDATE External_SU SET SiteUnit='Drift' WHERE PlotNumber='8229723'`)
				if err := errors.Join(writeErr, db.Close()); err != nil {
					t.Fatal(err)
				}
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			hooks := externalSUEnvironmentWriteHooks{}
			if variant == "late-cancel" {
				hooks.beforeCommit = func() error {
					cancel()
					return nil
				}
			}
			cleanupFailure := errors.New("independent facade cleanup failure")
			if variant == "cleanup" {
				hooks.afterCleanup = func() error { return cleanupFailure }
			}
			result, err := service.transferReviewed(ctx, state.ContextID, raw, hooks, nil)
			if err != nil || result == nil {
				t.Fatal("exact attempt did not return explicit sealed outcome", result, err)
			}
			if variant == "source-drift" || variant == "late-cancel" {
				if result.Status != "notCommitted" || result.Receipt != nil || result.ErrorMessage == "" {
					t.Fatal("rejected write claimed success", result)
				}
				assertProfileSUFiles(t, service.contexts, before)
			} else if result.Status != "committed" || result.Receipt == nil || result.Receipt.HistoryID != "1" {
				t.Fatal("immutable/cleanup commit lost receipt", result)
			}
			if variant == "cleanup" && !strings.Contains(result.ErrorMessage, cleanupFailure.Error()) {
				t.Fatal("committed cleanup failure hidden", result)
			}
			recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
			expected := cloneExternalSUEnvironmentOutcome(*result)
			if err != nil || !reflect.DeepEqual(recovered.Outcome, expected) {
				t.Fatal("exact attempt outcome not retained", recovered, result, err)
			}
		})
	}
}
