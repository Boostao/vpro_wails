package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func plotLabelPDFExportFixture(t *testing.T) (*PlotLabelPDFExportService, ProjectState, PlotLabelPDFExportRequest) {
	t.Helper()
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	service := NewPlotLabelPDFExportService(contexts, true)
	service.runtime = runtime
	review, err := service.GetReview(t.Context(), state.ContextID, `{"plotNumber":"LB001"}`)
	if err != nil {
		t.Fatal(err)
	}
	if review.Preview.ContextID != state.ContextID || review.Project != state.ActiveProject ||
		review.Layout != "portable-letter-single-fixed-ascii-v1" || review.PDFSHA256 == "" {
		t.Fatalf("incomplete review: %+v", review)
	}
	return service, state, PlotLabelPDFExportRequest{
		PlotNumber: "LB001", ApprovalHash: review.ApprovalHash,
		Destination: filepath.Join(t.TempDir(), "reviewed.pdf"),
		RequestID:   "00000000-0000-4000-8000-000000000001",
		SessionID:   review.SessionID,
	}
}

func plotLabelPDFRequestJSON(t *testing.T, request PlotLabelPDFExportRequest) string {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPlotLabelPDFServiceGateAndOnceOnlyOutcome(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	raw := plotLabelPDFRequestJSON(t, request)
	service.enabled = false
	if review, err := service.GetReview(t.Context(), state.ContextID, `{"plotNumber":"LB001"}`); err == nil || review != nil {
		t.Fatalf("disabled review: %v", err)
	}
	if outcome, err := service.ExportReviewed(t.Context(), state.ContextID, raw); err == nil || outcome != nil {
		t.Fatalf("disabled export: %v", err)
	}
	service.enabled = true
	outcome, err := service.ExportReviewed(t.Context(), state.ContextID, raw)
	if err != nil || outcome.Status != "published" || outcome.SHA256 == "" {
		t.Fatalf("export: %+v, %v", outcome, err)
	}
	info, err := os.Stat(outcome.Path)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.ExportReviewed(t.Context(), state.ContextID, raw)
	after, statErr := os.Stat(outcome.Path)
	if err != nil || statErr != nil || *repeated != *outcome || !os.SameFile(info, after) {
		t.Fatalf("UUID repeated publication instead of immutable receipt: %+v, %v, %v", repeated, err, statErr)
	}
	service.enabled = false
	recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered.Status != "completed" || recovered.Outcome == nil || *recovered.Outcome != *outcome {
		t.Fatalf("readonly backend-off recovery: %+v, %v", recovered, err)
	}
}

func TestPlotLabelPDFServiceStaleApprovalAndCommittedErrors(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET PlotRepresenting='changed' WHERE PlotNumber='LB001'`)
	raw := plotLabelPDFRequestJSON(t, request)
	outcome, err := service.ExportReviewed(t.Context(), state.ContextID, raw)
	if err == nil || outcome == nil || outcome.Status != "not-published" {
		t.Fatalf("stale approval not explicit: %+v, %v", outcome, err)
	}
	recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered.Outcome == nil || recovered.Outcome.Status != "not-published" {
		t.Fatalf("failed attempt not sealed: %+v, %v", recovered, err)
	}
	service, state, request = plotLabelPDFExportFixture(t)
	raw = plotLabelPDFRequestJSON(t, request)
	hooks := plotLabelPDFPublicationHooks{runtime: service.runtime, publication: artifactPublicationHooks{
		remove: func(path string) error {
			if err := os.Remove(path); err != nil {
				return err
			}
			return errors.New("planned committed cleanup fault")
		},
	}}
	outcome, err = service.exportReviewed(t.Context(), state.ContextID, raw, hooks)
	if err != nil || outcome.Status != "published-with-errors" || outcome.SHA256 == "" || outcome.ErrorMessage == "" {
		t.Fatalf("committed cleanup acknowledgement: %+v, %v", outcome, err)
	}
	recovered, err = service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered.Outcome == nil || *recovered.Outcome != *outcome {
		t.Fatalf("irreversible receipt not retained: %+v, %v", recovered, err)
	}
}

func TestPlotLabelPDFServiceNilAndCanceledContext(t *testing.T) {
	var service *PlotLabelPDFExportService
	if outcome, err := service.ExportReviewed(context.Background(), "owner", "{}"); err == nil || outcome != nil {
		t.Fatal("nil service accepted")
	}
	service = NewPlotLabelPDFExportService(nil, true)
	if outcome, err := service.ExportReviewed(nil, "owner", "{}"); err == nil || outcome != nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetReview(ctx, "owner", "{}"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not surfaced: %v", err)
	}
}
