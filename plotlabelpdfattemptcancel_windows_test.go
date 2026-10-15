package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPlotLabelPDFAttemptCancellationBeforeAndAfterPublication(t *testing.T) {
	for _, phase := range []string{"precommit", "published"} {
		service, state, request := plotLabelPDFExportFixture(t)
		raw := plotLabelPDFRequestJSON(t, request)
		ctx, cancel := context.WithCancel(t.Context())
		hooks := plotLabelPDFPublicationHooks{runtime: service.runtime, observe: func(actual string) error {
			if actual == phase {
				cancel()
			}
			return nil
		}}
		outcome, err := service.exportReviewed(ctx, state.ContextID, raw, hooks)
		cancel()
		if err != nil || outcome == nil {
			t.Fatalf("%s lost typed publication result: %+v, %v", phase, outcome, err)
		}
		want := "not-published"
		if phase == "published" {
			want = "published-with-errors"
		}
		if outcome.Status != want || outcome.ErrorMessage == "" {
			t.Fatalf("%s contradictory result: %+v", phase, outcome)
		}
		recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
		if err != nil || recovered.Outcome == nil || *recovered.Outcome != *outcome {
			t.Fatalf("%s cancellation lost ledger receipt: %+v, %v", phase, recovered, err)
		}
		if phase == "published" && (outcome.Path == "" || outcome.SHA256 == "" ||
			!strings.Contains(outcome.ErrorMessage, "do not replay")) {
			t.Fatalf("committed cancellation erased file identity: %+v", outcome)
		}
	}
}

func TestPlotLabelPDFAttemptFreshSourceFailuresRemainRecoverable(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	service.runtime.format = func(_ time.Time) (plotLabelNativeDate, error) {
		return plotLabelNativeDate{}, errors.New("planned native calendar failure")
	}
	raw := plotLabelPDFRequestJSON(t, request)
	outcome, err := service.ExportReviewed(t.Context(), state.ContextID, raw)
	if err == nil || outcome == nil || outcome.Status != "not-published" ||
		!strings.Contains(outcome.ErrorMessage, "native calendar failure") {
		t.Fatalf("failed source preparation hidden: %+v, %v", outcome, err)
	}
	recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered.Outcome == nil || *recovered.Outcome != *outcome {
		t.Fatalf("source preparation failure not recoverable: %+v, %v", recovered, err)
	}
}
