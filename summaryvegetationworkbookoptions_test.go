package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryDraftWorkbookRetainedShowOptionMatrix(t *testing.T) {
	source, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	service, err := NewSummaryVegetationWorkbookService(source, func(name string) (string, bool) {
		return "true", name == summaryVegetationWorkbookFeatureEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := source.environment.report.contexts.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	configPath := source.environment.report.contexts.projects.preferences.path
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, includeEnv := range []bool{false, true} {
		for _, average := range []int{1, 2} {
			for display := 1; display <= 6; display++ {
				var baseline *SummaryVegetationWorkbookReview
				approvals := map[string]bool{}
				for _, show := range []int{0, 1, 10, 20} {
					t.Run(fmt.Sprintf("env=%t/average=%d/display=%d/show=%d", includeEnv, average, display, show), func(t *testing.T) {
						options := SummaryVegetationWorkbookOptions{Title: "Retained options",
							Average: average, Display: display, ShowClass: show, IncludeEnv: includeEnv}
						request, err := json.Marshal(options)
						if err != nil {
							t.Fatal(err)
						}
						review, err := service.GetReview(t.Context(), state.ContextID, string(request))
						classMode := display == 4 || display == 5
						if classMode && show == 0 {
							if err == nil || review != nil {
								t.Fatal("missing class selection was silently repaired", review, err)
							}
							return
						}
						if err != nil || review == nil {
							t.Fatal("valid source options refused", options, err)
						}
						if review.Options != options || review.Preview.Average != average || review.Preview.Display != display ||
							(review.Environment != nil) != includeEnv {
							t.Fatal("explicit options or selected calculation changed", review)
						}
						expectedShow := 0
						if classMode {
							expectedShow = show
						}
						if review.Preview.ShowClass != expectedShow ||
							includeEnv && !reflect.DeepEqual(review.Environment.Vegetation, review.Preview) {
							t.Fatal("retained options changed the legacy calculation policy", review)
						}
						if approvals[review.ApprovalHash] {
							t.Fatal("distinct explicit Show selections reused approval")
						}
						approvals[review.ApprovalHash] = true
						if !classMode {
							if baseline != nil && (review.WorkbookSHA256 != baseline.WorkbookSHA256 ||
								!reflect.DeepEqual(review.Preview, baseline.Preview)) {
								t.Fatal("ignored Show changed source cells/provenance")
							}
							baseline = review
						}
					})
				}
			}
		}
	}
	if _, err := source.environment.report.PreviewClass(t.Context(), state.ContextID, 1, 6, 1); err == nil {
		t.Fatal("legacy class-only consumer was implicitly widened")
	}
	for _, show := range []int{-1, 2, 9, 21} {
		request, err := json.Marshal(SummaryVegetationWorkbookOptions{Average: 1, Display: 6, ShowClass: show})
		if err != nil {
			t.Fatal(err)
		}
		if review, err := service.GetReview(t.Context(), state.ContextID, string(request)); err == nil || review != nil {
			t.Fatal("invalid retained Show was silently ignored", show, review, err)
		}
	}
	afterConfig, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(config, afterConfig) || !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("option matrix changed physical data/config/audit", err)
	}
}

func TestSummaryDraftWorkbookIgnoredShowStillBindsPublicationApproval(t *testing.T) {
	source, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	service, err := NewSummaryVegetationWorkbookService(source, func(name string) (string, bool) {
		return "true", name == summaryVegetationWorkbookFeatureEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, ShowClass: 1}
	request, err := json.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.GetReview(t.Context(), state.ContextID, string(request))
	if err != nil {
		t.Fatal(err)
	}
	options.ShowClass = 20
	destination := filepath.Join(t.TempDir(), "not-authorized.xlsx")
	export, err := json.Marshal(summaryVegetationWorkbookExportRequest{
		Options: options, ApprovalHash: review.ApprovalHash, Destination: destination,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ExportReviewed(t.Context(), state.ContextID, string(export))
	if err == nil || !strings.Contains(err.Error(), "source/scope/options differ") || result != nil {
		t.Fatal("unchanged rendered bytes authorized different explicit options", result, err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("rejected option change created a destination", err)
	}
}
