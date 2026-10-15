package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestExternalSUEnvironmentReadonlyExactHistoryRecovery(t *testing.T) {
	for _, variant := range []string{"committed", "retired-context-disabled", "duplicate", "absent"} {
		t.Run(variant, func(t *testing.T) {
			service, state, request, raw := externalSUEnvironmentServiceFixture(t)
			if variant == "absent" {
				if execute, _, _, err := service.beginExternalSUEnvironmentAttempt(t.Context(), state.ContextID, request); !execute || err != nil {
					t.Fatal(err)
				}
			} else {
				if result, err := service.TransferReviewed(t.Context(), state.ContextID, raw); err != nil || result.Status != "committed" {
					t.Fatal(result, err)
				}
			}
			service.mu.Lock()
			entry := service.attempts[request.RequestID]
			entry.active, entry.sealed = false, true
			entry.source = service.reviews[request.ApprovalHash]
			entry.outcome = &ExternalSUEnvironmentOutcome{Status: "unknown", ErrorMessage: "Independent uncertain response"}
			service.mu.Unlock()
			if variant == "duplicate" {
				db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
				if err != nil {
					t.Fatal(err)
				}
				_, writeErr := db.Exec(`INSERT INTO __VPRO_SUEnvironmentHistory(Created,Proposal)
					SELECT Created,Proposal FROM __VPRO_SUEnvironmentHistory WHERE ID=1`)
				if err := errors.Join(writeErr, db.Close()); err != nil {
					t.Fatal(err)
				}
			}
			if variant == "retired-context-disabled" {
				selection := contextSelection(state)
				selection.SU, selection.SUPath = "None", ""
				next, err := service.contexts.SwitchContext(state.ContextID, selection)
				if err != nil || next.ContextID == state.ContextID {
					t.Fatal("old context was not retired", next, err)
				}
				service.enabled = false
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			result, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
			if variant == "absent" || variant == "duplicate" {
				if err == nil || result != nil || entry.outcome.Status != "unknown" {
					t.Fatal("absence or duplicate history invented an outcome", result, err)
				}
			} else {
				if err != nil || result.Status != "completed" || result.Outcome.Status != "committed" ||
					result.Outcome.Receipt.HistoryID != "1" || result.Outcome.Receipt.ChangedCells != 4 ||
					result.Outcome.Receipt.ChangedRows != 2 || result.Outcome.ErrorMessage != "Independent uncertain response" {
					t.Fatal("readonly exact physical history recovery failed", result, err)
				}
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
}

func TestExternalSUEnvironmentUnknownOutcomeCannotClaimRollback(t *testing.T) {
	failure := errors.New("independent driver commit failure")
	for _, variant := range []struct {
		err    error
		status string
	}{
		{nil, "unknown"},
		{failure, "unknown"},
		{&externalSUEnvironmentCommitUncertainError{failure}, "unknown"},
		{&externalSUEnvironmentNotCommittedError{failure}, "notCommitted"},
	} {
		result := externalSUEnvironmentOutcome(nil, variant.err)
		if result.Status != variant.status || result.Receipt != nil || result.ErrorMessage == "" {
			t.Fatal("unproven empty result changed outcome certainty", variant.status, result)
		}
	}
}

func TestExternalSUEnvironmentUnknownHistoryCancelledAndCorrectedRetry(t *testing.T) {
	service, state, request, raw := externalSUEnvironmentServiceFixture(t)
	committed, err := service.TransferReviewed(t.Context(), state.ContextID, raw)
	if err != nil || committed.Status != "committed" {
		t.Fatal(committed, err)
	}
	service.mu.Lock()
	entry := service.attempts[request.RequestID]
	entry.source = service.reviews[request.ApprovalHash]
	entry.outcome = &ExternalSUEnvironmentOutcome{Status: "unknown", ErrorMessage: "Independent lost receipt"}
	service.mu.Unlock()
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := service.RecoverOutcome(ctx, state.ContextID, raw); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled recovery invented outcome", result, err)
	}
	if entry.outcome.Status != "unknown" {
		t.Fatal("cancelled recovery erased uncertainty")
	}
	result, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || !reflect.DeepEqual(result.Outcome.Receipt, committed.Receipt) {
		t.Fatal("readonly retry failed exact original receipt", result, err)
	}
	assertProfileSUFiles(t, service.contexts, before)
}
