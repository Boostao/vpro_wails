package twopage

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
)

type TwoPageSoilAttemptOutcome struct {
	ContextID   string                  `json:"contextId"`
	RequestID   string                  `json:"requestId"`
	RequestJSON string                  `json:"requestJSON"`
	Status      string                  `json:"status"`
	Reason      string                  `json:"reason"`
	Receipt     *TwoPageSoilWriteResult `json:"receipt"`
}

func BeginTwoPageSoilAttempt(s *vprocore.ContextService, ctx context.Context, request twoPageSoilRequest) (bool, error) {
	ledger := &s.TwoPageSoilAttempts
	if err := sqlutil.AcquireMutexLease(ctx, &ledger.Mu); err != nil {
		return false, err
	}
	defer ledger.Mu.Unlock()
	entry, err := ledger.Entry(request.RequestID, request)
	if err != nil {
		return false, err
	}
	if entry.Sealed {
		return false, nil
	}
	entry.Active++
	return true, nil
}

func FinishTwoPageSoilAttempt(s *vprocore.ContextService, requestID string) {
	ledger := &s.TwoPageSoilAttempts
	ledger.Mu.Lock()
	defer ledger.Mu.Unlock()
	ledger.Requests[requestID].Active--
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
	ledger := &s.contexts.TwoPageSoilAttempts
	if err := sqlutil.AcquireMutexLease(ctx, &ledger.Mu); err != nil {
		return nil, err
	}
	defer ledger.Mu.Unlock()
	entry, err := ledger.Entry(request.RequestID, request)
	if err != nil {
		return nil, err
	}
	outcome := &TwoPageSoilAttemptOutcome{ContextID: contextID, RequestID: request.RequestID, RequestJSON: requestJSON}
	if entry.Active != 0 {
		outcome.Status, outcome.Reason = "running", "An owned Save attempt is still active; retain the original request."
		return outcome, nil
	}
	// Sealing also prevents an unseen, late RPC from dispatching after this read.
	entry.Sealed = true
	if afterSeal != nil {
		afterSeal()
	}
	receipt, err := LookupTwoPageSoilReceipt(s.contexts, ctx, contextID, request)
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
