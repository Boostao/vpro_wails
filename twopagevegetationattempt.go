package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

type twoPageVegetationAttempt struct {
	payload string
	active  int
	sealed  bool
}

type twoPageVegetationAttemptLedger struct {
	mu       sync.Mutex
	requests map[string]*twoPageVegetationAttempt
}

type TwoPageVegetationAttemptOutcome struct {
	ContextID   string                        `json:"contextId"`
	RequestID   string                        `json:"requestId"`
	RequestJSON string                        `json:"requestJSON"`
	Status      string                        `json:"status"`
	Reason      string                        `json:"reason"`
	Receipt     *TwoPageVegetationWriteResult `json:"receipt"`
}

func (ledger *twoPageVegetationAttemptLedger) entry(request twoPageVegetationRequest) (*twoPageVegetationAttempt, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	payload := string(raw)
	if ledger.requests == nil {
		ledger.requests = map[string]*twoPageVegetationAttempt{}
	}
	if existing := ledger.requests[request.RequestID]; existing != nil {
		if existing.payload != payload {
			return nil, errors.New("two-page vegetation UUID already belongs to another exact request payload")
		}
		return existing, nil
	}
	entry := &twoPageVegetationAttempt{payload: payload}
	ledger.requests[request.RequestID] = entry
	return entry, nil
}

func (s *ContextService) beginTwoPageVegetationAttempt(ctx context.Context, request twoPageVegetationRequest) (bool, error) {
	ledger := &s.twoPageVegetationAttempts
	if err := acquireMutexLease(ctx, &ledger.mu); err != nil {
		return false, err
	}
	defer ledger.mu.Unlock()
	entry, err := ledger.entry(request)
	if err != nil {
		return false, err
	}
	if entry.sealed {
		return false, nil
	}
	entry.active++
	return true, nil
}

func (s *ContextService) finishTwoPageVegetationAttempt(requestID string) {
	ledger := &s.twoPageVegetationAttempts
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.requests[requestID].active--
}

func (s *TwoPageVegetationService) RecoverOutcome(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationAttemptOutcome, error) {
	return s.recoverTwoPageVegetationOutcome(ctx, contextID, requestJSON, nil)
}

func (s *TwoPageVegetationService) recoverTwoPageVegetationOutcome(ctx context.Context, contextID, requestJSON string,
	afterSeal func()) (*TwoPageVegetationAttemptOutcome, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageVegetationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	ledger := &s.contexts.twoPageVegetationAttempts
	if err := acquireMutexLease(ctx, &ledger.mu); err != nil {
		return nil, err
	}
	defer ledger.mu.Unlock()
	entry, err := ledger.entry(request)
	if err != nil {
		return nil, err
	}
	outcome := &TwoPageVegetationAttemptOutcome{ContextID: contextID, RequestID: request.RequestID, RequestJSON: requestJSON}
	if entry.active != 0 {
		outcome.Status, outcome.Reason = "running", "An owned Save attempt is still active; retain the original request."
		return outcome, nil
	}
	// Sealing excludes a matching RPC that has not yet reached the writer.
	entry.sealed = true
	if afterSeal != nil {
		afterSeal()
	}
	receipt, err := s.contexts.lookupTwoPageVegetationReceipt(ctx, contextID, request)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if receipt != nil {
		outcome.Status, outcome.Reason, outcome.Receipt = "committed", "Exact durable history proves the committed source plan.", receipt
	} else if contextID != request.Original.ContextID {
		outcome.Status, outcome.Reason = "unknown", "Absent history in another context cannot certify the original attempt."
	} else {
		outcome.Status, outcome.Reason = "notCommitted", "No active or future matching Save can commit; the original owned history is absent."
	}
	return outcome, nil
}
