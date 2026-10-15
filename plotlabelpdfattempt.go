package main

import (
	"context"
	"encoding/json"
	"errors"
)

type plotLabelPDFAttempt struct {
	contextID, payload string
	active, sealed     bool
	outcome            *PlotLabelPDFExportOutcome
}

type PlotLabelPDFAttemptOutcome struct {
	ContextID string                     `json:"contextId"`
	RequestID string                     `json:"requestId"`
	Status    string                     `json:"status"`
	Reason    string                     `json:"reason"`
	Outcome   *PlotLabelPDFExportOutcome `json:"outcome"`
}

func (s *PlotLabelPDFExportService) attempt(contextID string, request PlotLabelPDFExportRequest) (*plotLabelPDFAttempt, error) {
	if request.SessionID != s.sessionID {
		return nil, errors.New("label PDF service owner changed; prior outcome is unknown here, never replay")
	}
	if contextID == "" {
		return nil, errors.New("label PDF attempt requires its original context")
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if s.attempts == nil {
		s.attempts = map[string]*plotLabelPDFAttempt{}
	}
	if existing := s.attempts[request.RequestID]; existing != nil {
		if existing.contextID != contextID || existing.payload != string(data) {
			return nil, errors.New("label PDF UUID belongs to a different exact context/request payload")
		}
		return existing, nil
	}
	entry := &plotLabelPDFAttempt{contextID: contextID, payload: string(data)}
	s.attempts[request.RequestID] = entry
	return entry, nil
}

func (s *PlotLabelPDFExportService) beginAttempt(ctx context.Context, contextID string, request PlotLabelPDFExportRequest) (bool, *PlotLabelPDFExportOutcome, error) {
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return false, nil, err
	}
	defer s.mu.Unlock()
	entry, err := s.attempt(contextID, request)
	if err != nil {
		return false, nil, err
	}
	if entry.active {
		return false, nil, errors.New("label PDF attempt is still running; recover readonly, never replay")
	}
	if entry.outcome != nil {
		copy := *entry.outcome
		return false, &copy, nil
	}
	if entry.sealed {
		return false, nil, errors.New("label PDF attempt was sealed before publication; review a new request")
	}
	entry.active = true
	return true, nil, nil
}

func (s *PlotLabelPDFExportService) finishAttempt(requestID string, outcome PlotLabelPDFExportOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.attempts[requestID]
	entry.active, entry.sealed, entry.outcome = false, true, &outcome
}

func (s *PlotLabelPDFExportService) RecoverOutcome(ctx context.Context, contextID, requestJSON string) (*PlotLabelPDFAttemptOutcome, error) {
	if err := s.authorize(ctx, false); err != nil {
		return nil, err
	}
	var request PlotLabelPDFExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	entry, err := s.attempt(contextID, request)
	if err != nil {
		return nil, err
	}
	result := &PlotLabelPDFAttemptOutcome{ContextID: contextID, RequestID: request.RequestID}
	if entry.active {
		result.Status, result.Reason = "running", "The exact owned publication is still active; retain its request."
		return result, nil
	}
	if entry.outcome == nil {
		entry.sealed = true
		entry.outcome = plotLabelPDFOutcome(request.Destination, artifactPublication{}, errors.New("readonly recovery sealed the exact request before any publication could begin"))
	}
	copy := *entry.outcome
	result.Status, result.Reason, result.Outcome = "completed", "The exact request is sealed; this recovery never republishes a file.", &copy
	return result, nil
}
