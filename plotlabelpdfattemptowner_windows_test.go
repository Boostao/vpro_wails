package main

import (
	"os"
	"strings"
	"testing"
)

func TestPlotLabelPDFAttemptNewServiceCannotCertifyOldPublicationAbsent(t *testing.T) {
	service, state, request := plotLabelPDFExportFixture(t)
	raw := plotLabelPDFRequestJSON(t, request)
	committed, err := service.ExportReviewed(t.Context(), state.ContextID, raw)
	if err != nil || committed.Status != "published" {
		t.Fatalf("original publication: %+v, %v", committed, err)
	}
	restarted := NewPlotLabelPDFExportService(service.contexts, true)
	restarted.runtime = service.runtime
	if restarted.sessionID == service.sessionID {
		t.Fatal("fresh service did not establish independent attempt ownership")
	}
	for _, action := range []string{"recover", "export"} {
		if action == "recover" {
			outcome, err := restarted.RecoverOutcome(t.Context(), state.ContextID, raw)
			if err == nil || outcome != nil || !strings.Contains(err.Error(), "outcome is unknown") {
				t.Fatalf("new owner invented no publication: %+v, %v", outcome, err)
			}
		} else {
			outcome, err := restarted.ExportReviewed(t.Context(), state.ContextID, raw)
			if err == nil || outcome != nil {
				t.Fatalf("retired service request replayed: %+v, %v", outcome, err)
			}
		}
	}
	if len(restarted.attempts) != 0 {
		t.Fatal("foreign owner request established attempt authority")
	}
	if _, err := os.Stat(committed.Path); err != nil {
		t.Fatalf("old committed file changed: %v", err)
	}
}
