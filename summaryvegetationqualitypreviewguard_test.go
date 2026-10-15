package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryVegetationQualityOptInRefusesUnqualifiedPreviewWithoutChangingFeatureOff(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, false)
	baseline, err := service.Preview(context.Background(), state.ContextID, 1, 6)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforce": true}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := service.Preview(context.Background(), state.ContextID, 1, 6)
	if err != nil || !reflect.DeepEqual(baseline, unchanged) {
		t.Fatal("new quality gate changed existing feature-off behavior", unchanged, err)
	}
	service.qualityPreferences = true
	for _, preview := range []func() (SummaryVegetationPreview, error){
		func() (SummaryVegetationPreview, error) {
			return service.Preview(context.Background(), state.ContextID, 1, 6)
		},
		func() (SummaryVegetationPreview, error) {
			return service.PreviewClass(context.Background(), state.ContextID, 1, 4, 10)
		},
		func() (SummaryVegetationPreview, error) {
			return service.PreviewGrouped(context.Background(), state.ContextID, 1, 14)
		},
		func() (SummaryVegetationPreview, error) {
			return service.PreviewGroupedDisplay(context.Background(), state.ContextID, 1, 14, 1, 0)
		},
	} {
		got, err := preview()
		if err == nil || !strings.Contains(err.Error(), "no unfiltered report") || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
			t.Fatal("saved enforced QC returned unqualified preview", got, err)
		}
	}
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforce": "malformed"}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Preview(context.Background(), state.ContextID, 1, 6); err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal("malformed quality preferences silently defaulted", got, err)
	}
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforce": false}); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Preview(context.Background(), state.ContextID, 1, 6)
	if err != nil || !reflect.DeepEqual(baseline, recovered) {
		t.Fatal("corrected inactive quality changed original preview", recovered, err)
	}
}
