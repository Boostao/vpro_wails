package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSummaryEnvironmentalNumericIndependentFeature(t *testing.T) {
	for _, base := range []string{"", "false", "true"} {
		for _, numeric := range []string{"", "false", "true", "TRUE", "1", " true"} {
			values := map[string]string{
				summaryVegetationEnvironmentalFeatureEnvironment:        base,
				summaryVegetationEnvironmentalNumericFeatureEnvironment: numeric,
			}
			service, err := NewSummaryVegetationEnvironmentalService(nil, func(key string) (string, bool) {
				value, exists := values[key]
				if !exists {
					t.Fatal("unexpected flag", key)
				}
				return value, value != ""
			})
			valid := numeric == "" || numeric == "false" || numeric == "true"
			if !valid {
				if err == nil || service != nil {
					t.Fatal("nonliteral numeric flag accepted", base, numeric, service, err)
				}
				continue
			}
			if err != nil || service.enabled != (base == "true") || service.numericEnabled != (numeric == "true") {
				t.Fatal(base, numeric, service, err)
			}
			got, err := service.PreviewEnvironmentalNumeric(t.Context(), "owned", 1, 6, 0)
			if got != nil || err == nil {
				t.Fatal("unowned numeric request dispatched", base, numeric, got, err)
			}
			if numeric != "true" && !strings.Contains(err.Error(), "numeric environmental preview is disabled") {
				t.Fatal("missing independent refusal", err)
			}
		}
	}
}

func TestSummaryEnvironmentalNumericGateMatrixNoWrites(t *testing.T) {
	for _, gates := range []struct{ numeric, environment, report bool }{
		{false, true, true}, {true, false, true}, {true, true, false}, {true, true, true},
	} {
		service, state := summaryEnvironmentalNumericServiceFixture(t, false)
		owner := service.report.contexts.projects.sqlite
		before := databaseBytes(t, owner.attachments)
		service.numericEnabled = gates.numeric
		service.enabled = gates.environment
		service.report.enabled = gates.report
		snapshots := 0
		service.snapshot.commitRead = func(*sql.Tx) error { snapshots++; return nil }
		result, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0)
		want := gates.numeric && gates.environment && gates.report
		if want != (result != nil && err == nil) || !want && snapshots != 0 || want && snapshots != 1 {
			t.Fatal(gates, result, err, snapshots)
		}
		if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
			t.Fatal("gate/read mutated source", gates)
		}
		if !want {
			projects := service.report.contexts.projects
			projects.operationMu.Lock()
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			blocked, blockedErr := service.PreviewEnvironmentalNumeric(ctx, state.ContextID, 1, 6, 0)
			cancel()
			projects.operationMu.Unlock()
			if blocked != nil || blockedErr == nil || errors.Is(blockedErr, context.DeadlineExceeded) {
				t.Fatal("disabled gate entered the read lease", gates, blocked, blockedErr)
			}
		}
	}
	service := &SummaryVegetationEnvironmentalService{}
	if result, err := service.PreviewEnvironmentalNumeric(nil, "owned", 1, 6, 0); result != nil || err == nil {
		t.Fatal("nil request context accepted", result, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := service.PreviewEnvironmentalNumeric(ctx, "owned", 1, 6, 0); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request lost cancellation", result, err)
	}
}
