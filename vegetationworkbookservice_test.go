package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLongVegetationWorkbookPublicServiceShapeAndReviewedExport(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, true)
	service := &VegetationWorkbookService{private: private}
	review, err := service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil || review == nil || review.Preview.ContextID != state.ContextID ||
		review.Summary == nil || !review.Options.ReportSummary || !review.Options.QuickReport ||
		review.Summary.CreatedDate != review.CreatedDate || review.Scope != "unlumped" ||
		review.Sheets == nil || review.SkippedUnits == nil {
		t.Fatal("public review lost owned/default/summary/array contract", review, err)
	}
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(data, &shape); err != nil || len(shape) != 10 {
		t.Fatal("public review shape differs", string(data), err)
	}
	for _, key := range []string{"preview", "options", "summary", "scope", "createdDate",
		"sheets", "skippedUnits", "approvalHash", "workbookSHA256", "bytes"} {
		if shape[key] == nil {
			t.Fatal("public model field missing", key)
		}
	}
	destination := filepath.Join(t.TempDir(), "public-vegetation.xlsx")
	outcome, err := service.ExportReviewed(context.Background(), state.ContextID,
		workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination}))
	if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
		t.Fatal("public model cannot publish its exact reviewed approval", outcome, err)
	}
	if data, err := os.ReadFile(destination); err != nil || len(data) != review.Bytes {
		t.Fatal("public receipt does not match actual output", len(data), err)
	}
	if err := private.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVReportSummary": 0}); err != nil {
		t.Fatal(err)
	}
	review, err = service.GetReview(context.Background(), state.ContextID, `{"scope":"unlumped"}`)
	if err != nil || review == nil || review.Options.ReportSummary || review.Summary != nil {
		t.Fatal("source summary-off preference silently overridden", review, err)
	}
}

func TestLongVegetationWorkbookPublicServiceUnavailable(t *testing.T) {
	var nilService *VegetationWorkbookService
	for _, service := range []*VegetationWorkbookService{nilService, {}} {
		if review, err := service.GetReview(context.Background(), "owned", `{"scope":"unlumped"}`); err == nil || review != nil {
			t.Fatal("unavailable public service invented review", review, err)
		}
		if outcome, err := service.ExportReviewed(context.Background(), "owned", `{}`); err == nil || outcome != nil {
			t.Fatal("unavailable public service invented receipt", outcome, err)
		}
	}
	service, err := NewVegetationWorkbookService(nil, func(string) (string, bool) { return "", false })
	if err != nil || service.private.enabled {
		t.Fatal("public service is not default-off", err)
	}
	if _, err := service.GetReview(context.Background(), "owned", `{"scope":"unlumped"}`); err == nil {
		t.Fatal("default-off public service exposed review")
	}
}
