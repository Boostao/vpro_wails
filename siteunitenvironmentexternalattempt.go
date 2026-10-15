package main

import (
	"context"
	"encoding/json"
	"errors"
)

type externalSUEnvironmentAttempt struct {
	contextID, payload string
	active, sealed     bool
	outcome            *ExternalSUEnvironmentOutcome
	source             *externalSUEnvironmentSource
}

type ExternalSUEnvironmentAttemptOutcome struct {
	ContextID string                        `json:"contextId"`
	RequestID string                        `json:"requestId"`
	Status    string                        `json:"status"`
	Reason    string                        `json:"reason"`
	Outcome   *ExternalSUEnvironmentOutcome `json:"outcome"`
}

func cloneExternalSUEnvironmentOutcome(outcome ExternalSUEnvironmentOutcome) *ExternalSUEnvironmentOutcome {
	if outcome.Receipt != nil {
		copy := *outcome.Receipt
		outcome.Receipt = &copy
	}
	return &outcome
}

func (s *ExternalSUEnvironmentService) externalSUEnvironmentAttempt(contextID string,
	request ExternalSUEnvironmentRequest) (*externalSUEnvironmentAttempt, error) {
	if request.SessionID != s.sessionID {
		return nil, errors.New("external SU/environment service owner changed; prior outcome is unknown here, never replay")
	}
	if contextID == "" {
		return nil, errors.New("external SU/environment attempt requires its original context")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if s.attempts == nil {
		s.attempts = make(map[string]*externalSUEnvironmentAttempt)
	}
	if existing := s.attempts[request.RequestID]; existing != nil {
		if existing.contextID != contextID || existing.payload != string(raw) {
			return nil, errors.New("external SU/environment UUID belongs to a different exact context/request payload")
		}
		return existing, nil
	}
	entry := &externalSUEnvironmentAttempt{contextID: contextID, payload: string(raw)}
	s.attempts[request.RequestID] = entry
	return entry, nil
}

func (s *ExternalSUEnvironmentService) beginExternalSUEnvironmentAttempt(ctx context.Context, contextID string,
	request ExternalSUEnvironmentRequest) (bool, *ExternalSUEnvironmentOutcome, *externalSUEnvironmentSource, error) {
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return false, nil, nil, err
	}
	defer s.mu.Unlock()
	entry, err := s.externalSUEnvironmentAttempt(contextID, request)
	if err != nil {
		return false, nil, nil, err
	}
	if entry.active {
		return false, nil, nil, errors.New("external SU/environment attempt is still running; recover readonly, never replay")
	}
	if entry.outcome != nil {
		return false, cloneExternalSUEnvironmentOutcome(*entry.outcome), nil, nil
	}
	if entry.sealed {
		return false, nil, nil, errors.New("external SU/environment attempt was sealed; prepare a new review")
	}
	source := s.reviews[request.ApprovalHash]
	if source == nil || source.Review.ContextID != contextID {
		return false, nil, nil, errors.New("external SU/environment approval is not owned by this exact context and service")
	}
	entry.active = true
	entry.source = source
	return true, nil, source, nil
}

func (s *ExternalSUEnvironmentService) finishExternalSUEnvironmentAttempt(requestID string, outcome ExternalSUEnvironmentOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.attempts[requestID]
	entry.active, entry.sealed, entry.outcome = false, true, cloneExternalSUEnvironmentOutcome(outcome)
	if outcome.Status != "unknown" {
		entry.source = nil
	}
}

func (s *ExternalSUEnvironmentService) RecoverOutcome(ctx context.Context, contextID, requestJSON string) (*ExternalSUEnvironmentAttemptOutcome, error) {
	if err := s.authorize(ctx, false); err != nil {
		return nil, err
	}
	var request ExternalSUEnvironmentRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	entry, err := s.externalSUEnvironmentAttempt(contextID, request)
	if err != nil {
		return nil, err
	}
	result := &ExternalSUEnvironmentAttemptOutcome{ContextID: contextID, RequestID: request.RequestID}
	if entry.active {
		result.Status, result.Reason = "running", "The exact owned transfer is still active; retain its request."
		return result, nil
	}
	if entry.outcome == nil {
		entry.sealed = true
		entry.outcome = externalSUEnvironmentOutcome(nil, errors.New("readonly recovery sealed the exact request before any transfer could begin"))
		entry.outcome.Status = "notCommitted"
	}
	if entry.outcome.Status == "unknown" {
		if entry.source == nil {
			return nil, errors.New("external SU/environment outcome lacks its original physical source; outcome remains unknown, never replay")
		}
		identity := externalSUEnvironmentAttemptIdentity{request.ApprovalHash, request.RequestID, request.SessionID}
		receipt, err := lookupExternalSUEnvironmentReceipt(ctx, entry.source, identity)
		if err != nil {
			return nil, err
		}
		copy := *entry.outcome
		copy.Receipt = receipt
		copy.Status = "notCommitted"
		if receipt != nil {
			copy.Status = "committed"
		}
		entry.outcome = &copy
	}
	result.Status, result.Reason, result.Outcome = "completed",
		"The exact request is sealed; this readonly recovery never transfers source values.",
		cloneExternalSUEnvironmentOutcome(*entry.outcome)
	return result, nil
}
