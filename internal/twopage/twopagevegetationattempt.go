package twopage

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
)

type TwoPageVegetationAttemptOutcome struct {
	ContextID   string                        `json:"contextId"`
	RequestID   string                        `json:"requestId"`
	RequestJSON string                        `json:"requestJSON"`
	Status      string                        `json:"status"`
	Reason      string                        `json:"reason"`
	Receipt     *TwoPageVegetationWriteResult `json:"receipt"`
}

func BeginTwoPageVegetationAttempt(s *vprocore.ContextService, ctx context.Context, request twoPageVegetationRequest) (bool, error) {
	ledger := &s.TwoPageVegetationAttempts
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

func FinishTwoPageVegetationAttempt(s *vprocore.ContextService, requestID string) {
	ledger := &s.TwoPageVegetationAttempts
	ledger.Mu.Lock()
	defer ledger.Mu.Unlock()
	ledger.Requests[requestID].Active--
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
	ledger := &s.contexts.TwoPageVegetationAttempts
	if err := sqlutil.AcquireMutexLease(ctx, &ledger.Mu); err != nil {
		return nil, err
	}
	defer ledger.Mu.Unlock()
	entry, err := ledger.Entry(request.RequestID, request)
	if err != nil {
		return nil, err
	}
	outcome := &TwoPageVegetationAttemptOutcome{ContextID: contextID, RequestID: request.RequestID, RequestJSON: requestJSON}
	if entry.Active != 0 {
		outcome.Status, outcome.Reason = "running", "An owned Save attempt is still active; retain the original request."
		return outcome, nil
	}
	// Sealing excludes a matching RPC that has not yet reached the writer.
	entry.Sealed = true
	if afterSeal != nil {
		afterSeal()
	}
	receipt, err := LookupTwoPageVegetationReceipt(s.contexts, ctx, contextID, request)
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
