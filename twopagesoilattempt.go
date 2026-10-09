package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
)

type twoPageSoilAttempt struct {
	request twoPageSoilRequest
	active  int
	sealed  bool
}

type twoPageSoilAttemptLedger struct {
	mu       sync.Mutex
	requests map[string]*twoPageSoilAttempt
}

type TwoPageSoilAttemptOutcome struct {
	ContextID   string                  `json:"contextId"`
	RequestID   string                  `json:"requestId"`
	RequestJSON string                  `json:"requestJSON"`
	Status      string                  `json:"status"`
	Reason      string                  `json:"reason"`
	Receipt     *TwoPageSoilWriteResult `json:"receipt"`
}

func (ledger *twoPageSoilAttemptLedger) entry(request twoPageSoilRequest) (*twoPageSoilAttempt, error) {
	if ledger.requests == nil {
		ledger.requests = map[string]*twoPageSoilAttempt{}
	}
	existing := ledger.requests[request.RequestID]
	if existing != nil {
		if !reflect.DeepEqual(existing.request, request) {
			return nil, errors.New("two-page soil UUID already belongs to another exact request payload")
		}
		return existing, nil
	}
	entry := &twoPageSoilAttempt{request: request}
	ledger.requests[request.RequestID] = entry
	return entry, nil
}

func (s *ContextService) beginTwoPageSoilAttempt(ctx context.Context, request twoPageSoilRequest) (bool, error) {
	ledger := &s.twoPageSoilAttempts
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

func (s *ContextService) finishTwoPageSoilAttempt(requestID string) {
	ledger := &s.twoPageSoilAttempts
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.requests[requestID].active--
}

func (s *TwoPageSoilService) RecoverOutcome(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilAttemptOutcome, error) {
	return s.recoverTwoPageSoilOutcome(ctx, contextID, requestJSON, nil)
}

func (s *TwoPageSoilService) recoverTwoPageSoilOutcome(ctx context.Context, contextID, requestJSON string,
	afterSeal func()) (*TwoPageSoilAttemptOutcome, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request twoPageSoilRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	ledger := &s.contexts.twoPageSoilAttempts
	if err := acquireMutexLease(ctx, &ledger.mu); err != nil {
		return nil, err
	}
	defer ledger.mu.Unlock()
	entry, err := ledger.entry(request)
	if err != nil {
		return nil, err
	}
	outcome := &TwoPageSoilAttemptOutcome{ContextID: contextID, RequestID: request.RequestID, RequestJSON: requestJSON}
	if entry.active != 0 {
		outcome.Status, outcome.Reason = "running", "An owned Save attempt is still active; retain the original request."
		return outcome, nil
	}
	// Sealing also prevents an unseen, late RPC from dispatching after this read.
	entry.sealed = true
	if afterSeal != nil {
		afterSeal()
	}
	receipt, err := s.contexts.lookupTwoPageSoilReceipt(ctx, contextID, request)
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
