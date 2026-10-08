package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestSummaryExtendedWorkbookServiceIndependentFlagAndMethods(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSiteUnitSummaryExtendedWorkbookService(nil, func(name string) (string, bool) {
			if name != siteUnitSummaryExtendedWorkbookFeatureEnvironment {
				t.Fatal("borrowed another feature gate", name)
			}
			return flag, flag != ""
		})
		if flag == "" || flag == "false" || flag == "true" {
			if err != nil || service.enabled != (flag == "true") {
				t.Fatal("independent default-off flag differs", service, err)
			}
		} else if err == nil || service != nil {
			t.Fatal("malformed flag accepted", flag)
		}
		if err == nil && !service.enabled {
			if value, err := service.GetReview(context.Background(), "unowned", "{}"); err == nil || value != nil {
				t.Fatal("disabled service returned review", value, err)
			}
			if value, err := service.ExportReviewed(context.Background(), "unowned", "{}"); err == nil || value != nil {
				t.Fatal("disabled service returned publication", value, err)
			}
		}
	}
	contexts, state := reportServiceFixture(t, false)
	contexts.siteUnitSummarySpeciesEnabled, contexts.siteUnitSummaryLifeformsEnabled = true, true
	service, err := NewSiteUnitSummaryExtendedWorkbookService(contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	request := summaryExtendedJSON(t, SiteUnitSummaryExtendedWorkbookOptions{1, 2, 1, 1, 1, 0, 0})
	if value, err := service.GetReview(context.Background(), state.ContextID, request); err != nil || value == nil {
		t.Fatal("registered review facade unavailable", value, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := service.GetReview(ctx, state.ContextID, request); !errors.Is(err, context.Canceled) || value != nil {
		t.Fatal("cancelled facade returned review", value, err)
	}
	if value, err := service.GetReview(nil, state.ContextID, request); err == nil || value != nil {
		t.Fatal("nil context facade returned review", value, err)
	}
	if value, err := service.ExportReviewed(ctx, state.ContextID, "{}"); !errors.Is(err, context.Canceled) || value != nil {
		t.Fatal("cancelled publication facade returned receipt", value, err)
	}
	if value, err := service.ExportReviewed(nil, state.ContextID, "{}"); err == nil || value != nil {
		t.Fatal("nil publication context returned receipt", value, err)
	}
}

func TestSummaryExtendedWorkbookServiceReviewedPublication(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	contexts.siteUnitSummaryLifeformsEnabled = true
	service, err := NewSiteUnitSummaryExtendedWorkbookService(contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	options := SiteUnitSummaryExtendedWorkbookOptions{1, 2, 0, 1, 1, 0, 0}
	review, err := service.GetReview(context.Background(), state.ContextID, summaryExtendedJSON(t, options))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "lifeforms.xlsx")
	request := siteUnitSummaryExtendedExportRequest{1, 2, 0, 1, 1, 0, 0, review.ApprovalHash, destination}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := service.ExportReviewed(context.Background(), state.ContextID, string(raw))
	if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
		t.Fatal("reviewed facade did not publish exact bytes", outcome, err)
	}
	collision, err := service.ExportReviewed(context.Background(), state.ContextID, string(raw))
	if err != nil || collision == nil || collision.Status != "not-published" {
		t.Fatal("registered facade replaced destination", collision, err)
	}
}
