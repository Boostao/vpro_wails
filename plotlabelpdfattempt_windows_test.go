package main

import (
	"context"
	"testing"
	"time"
)

func TestPlotLabelPDFAttemptRunningRecoveryAndNoConcurrentReplay(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	raw := plotLabelPDFRequestJSON(t, request)
	started, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	type result struct {
		outcome *PlotLabelPDFExportOutcome
		err     error
	}
	finished := make(chan result, 1)
	hooks := plotLabelPDFPublicationHooks{runtime: service.runtime, observe: func(phase string) error {
		if phase == "validated" {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}}
	go func() {
		outcome, err := service.exportReviewed(ctx, state.ContextID, raw, hooks)
		finished <- result{outcome, err}
	}()
	select {
	case <-started:
	case early := <-finished:
		t.Fatalf("export failed before held publication: %+v, %v", early.outcome, early.err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	recovered, err := service.RecoverOutcome(ctx, state.ContextID, raw)
	if err != nil || recovered.Status != "running" || recovered.Outcome != nil {
		t.Fatalf("in-flight recovery: %+v, %v", recovered, err)
	}
	if outcome, err := service.ExportReviewed(ctx, state.ContextID, raw); err == nil || outcome != nil {
		t.Fatalf("concurrent matching request replayed: %+v, %v", outcome, err)
	}
	close(release)
	completed := <-finished
	if completed.err != nil || completed.outcome.Status != "published" {
		t.Fatalf("held publication: %+v, %v", completed.outcome, completed.err)
	}
	recovered, err = service.RecoverOutcome(ctx, state.ContextID, raw)
	if err != nil || recovered.Outcome == nil || *recovered.Outcome != *completed.outcome {
		t.Fatalf("completed recovery: %+v, %v", recovered, err)
	}
	recovered.Outcome.Path = "mutated caller copy"
	retry, err := service.RecoverOutcome(ctx, state.ContextID, raw)
	if err != nil || retry.Outcome.Path != completed.outcome.Path {
		t.Fatalf("immutable recovery receipt changed: %+v, %v", retry, err)
	}
}

func TestPlotLabelPDFAttemptReadonlySealExcludesDelayedRPC(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	raw := plotLabelPDFRequestJSON(t, request)
	service.enabled = false
	recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered.Status != "completed" || recovered.Outcome == nil ||
		recovered.Outcome.Status != "not-published" {
		t.Fatalf("readonly pending-arrival seal: %+v, %v", recovered, err)
	}
	service.enabled = true
	service.runtime.fonts = func(context.Context) (plotLabelPDFFonts, error) {
		t.Fatal("sealed delayed RPC reached fresh source/font preparation")
		return plotLabelPDFFonts{}, nil
	}
	outcome, err := service.ExportReviewed(t.Context(), state.ContextID, raw)
	if err != nil || outcome.Status != "not-published" || *outcome != *recovered.Outcome {
		t.Fatalf("delayed sealed request executed: %+v, %v", outcome, err)
	}
}

func TestPlotLabelPDFAttemptUUIDOwnershipAndOldContextRecovery(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	raw := plotLabelPDFRequestJSON(t, request)
	if _, err := service.RecoverOutcome(t.Context(), state.ContextID, raw); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.Destination += ".pdf"
	if _, err := service.RecoverOutcome(t.Context(), state.ContextID, plotLabelPDFRequestJSON(t, changed)); err == nil {
		t.Fatal("UUID ownership lost across changed destination")
	}
	next, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecoverOutcome(t.Context(), next.ContextID, raw); err == nil {
		t.Fatal("UUID transferred to another context")
	}
	recovered, err := service.RecoverOutcome(t.Context(), state.ContextID, raw)
	if err != nil || recovered.Outcome.Status != "not-published" {
		t.Fatalf("old-context readonly receipt was discarded: %+v, %v", recovered, err)
	}
}
