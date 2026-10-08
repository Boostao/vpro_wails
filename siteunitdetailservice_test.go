package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSiteUnitSummaryOwnedPreviewMethodsAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			if _, err := service.PreviewSiteUnitSummary(context.Background(), state.ContextID, SiteUnitSummaryRequest{1}); err == nil {
				t.Fatal("default gate enabled")
			}
			service.siteUnitSummaryEnabled = true
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []int{1, 2} {
				result, err := service.PreviewSiteUnitSummary(context.Background(), state.ContextID, SiteUnitSummaryRequest{method})
				if err != nil || result.ContextID != state.ContextID || result.ProjectPath != state.ProjectPath || result.SUPath != state.SUPath ||
					result.Report.Method != method || len(result.Report.Fields) != 39 || len(result.Report.Units) != 1 ||
					len(result.Report.Units[0].Plots) != 1 || len(result.Report.Memberships) != 3 {
					t.Fatal("owned full summary differs", result, err)
				}
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("summary wrote preferences", err)
			}
		})
	}
}

func TestSiteUnitSummaryCancellationCleanupStaleAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	service.siteUnitSummaryEnabled = true
	sentinel := errors.New("summary snapshot cleanup failed")
	for _, hooks := range []publicationReadSnapshotHooks{{commitRead: func(*sql.Tx) error { return sentinel }},
		{rollbackRead: func(*sql.Tx) error { return sentinel }}} {
		result, err := service.readSiteUnitSummary(context.Background(), state.ContextID, SiteUnitSummaryRequest{1}, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(result, SiteUnitSummaryPreview{}) {
			t.Fatal("cleanup yielded success", result, err)
		}
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.PreviewSiteUnitSummary(ctx, state.ContextID, SiteUnitSummaryRequest{1})
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, SiteUnitSummaryPreview{}) {
		t.Fatal("lease cancellation lost", result, err)
	}
	if _, err := service.PreviewSiteUnitSummary(context.Background(), state.ContextID, SiteUnitSummaryRequest{1}); err != nil {
		t.Fatal("retry leaked lease", err)
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewSiteUnitSummary(context.Background(), state.ContextID, SiteUnitSummaryRequest{1}); err == nil {
		t.Fatal("stale context accepted")
	}
	if _, err := service.PreviewSiteUnitSummary(context.Background(), next.ContextID, SiteUnitSummaryRequest{1}); err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(next)
	selection.SU = "None"
	selection.SUPath = ""
	next, err = service.SwitchContext(next.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewSiteUnitSummary(context.Background(), next.ContextID, SiteUnitSummaryRequest{1}); err == nil {
		t.Fatal("None guessed project scope")
	}
}

func TestSiteUnitSummaryStrictRequestAndFeature(t *testing.T) {
	for _, raw := range []string{`{}`, `{"method":null}`, `{"method":1,"method":2}`, `{"Method":1}`, `{"method":1,"species":true}`, `{"method":"1"}`} {
		var request SiteUnitSummaryRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("incomplete or ambiguous request accepted", raw)
		}
	}
	if _, err := siviFeature(siteUnitSummaryFeatureEnvironment, func(string) (string, bool) { return "", false }); err != nil {
		t.Fatal(err)
	}
	if enabled, err := siviFeature(siteUnitSummaryFeatureEnvironment, func(string) (string, bool) { return "", false }); err != nil || enabled {
		t.Fatal("default not off", err)
	}
	if _, err := siviFeature(siteUnitSummaryFeatureEnvironment, func(string) (string, bool) { return "TRUE", true }); err == nil {
		t.Fatal("malformed flag accepted")
	}
}

func TestSiteUnitSummarySavedOptionsOwnedAndReadOnly(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			if _, err := service.GetSiteUnitSummaryOptions(context.Background(), state.ContextID); err == nil {
				t.Fatal("default options gate enabled")
			}
			service.siteUnitSummaryEnabled = true
			for _, unitType := range []int{1, 2, 3} {
				if err := service.projects.preferences.update("ReportOptions", map[string]any{
					"SEOptValueMethod": 2, "SESuType": unitType,
				}); err != nil {
					t.Fatal(err)
				}
				before := databaseBytes(t, service.projects.sqlite.attachments)
				config, err := os.ReadFile(service.projects.preferences.path)
				if err != nil {
					t.Fatal(err)
				}
				options, err := service.GetSiteUnitSummaryOptions(context.Background(), state.ContextID)
				selection := contextSelection(state)
				expected := SiteUnitSummaryOptions{state.ContextID, selection.Project, state.ProjectPath, selection.SU, state.SUPath, 2, unitType}
				if err != nil || !reflect.DeepEqual(options, expected) {
					t.Fatal("saved method/scope/ownership differs", options, expected, err)
				}
				assertProfileSUFiles(t, service, before)
				after, err := os.ReadFile(service.projects.preferences.path)
				if err != nil || !bytes.Equal(config, after) {
					t.Fatal("options read rewrote preferences", err)
				}
			}
		})
	}
}

func TestSiteUnitSummarySavedOptionsRefuseMalformedAndMissing(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	service.siteUnitSummaryEnabled = true
	for _, invalid := range []struct {
		key   string
		value any
	}{{"SEOptValueMethod", nil}, {"SEOptValueMethod", "2"}, {"SEOptValueMethod", 0},
		{"SEOptValueMethod", 3}, {"SEOptValueMethod", 1.5},
		{"SESuType", nil}, {"SESuType", "1"}, {"SESuType", 0}, {"SESuType", 4}} {
		changes := map[string]any{"SEOptValueMethod": 1, "SESuType": 1}
		changes[invalid.key] = invalid.value
		if err := service.projects.preferences.update("ReportOptions", changes); err != nil {
			t.Fatal(err)
		}
		result, err := service.GetSiteUnitSummaryOptions(context.Background(), state.ContextID)
		if err == nil || result != (SiteUnitSummaryOptions{}) {
			t.Fatal("malformed saved option defaulted", invalid, result, err)
		}
	}
	for _, key := range []string{"SEOptValueMethod", "SESuType"} {
		values := configValues{"ReportOptions": map[string]any{"SEOptValueMethod": 1, "SESuType": 1}}
		delete(values["ReportOptions"].(map[string]any), key)
		if _, err := configInt(values, "ReportOptions", key, 1, 2); err == nil {
			t.Fatal("missing option defaulted", key)
		}
	}
}

func TestSiteUnitSummarySavedOptionsCancellationCleanupAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	service.siteUnitSummaryEnabled = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetSiteUnitSummaryOptions(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled options succeeded", err)
	}
	service.projects.preferences.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err := service.GetSiteUnitSummaryOptions(ctx, state.ContextID)
	cancel()
	service.projects.preferences.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("configuration lease cancellation lost", err)
	}
	sentinel := errors.New("saved options cleanup failed")
	for _, hooks := range []publicationReadSnapshotHooks{{commitRead: func(*sql.Tx) error { return sentinel }},
		{rollbackRead: func(*sql.Tx) error { return sentinel }}} {
		result, err := service.readSiteUnitSummaryOptions(context.Background(), state.ContextID, hooks)
		if !errors.Is(err, sentinel) || result != (SiteUnitSummaryOptions{}) {
			t.Fatal("options cleanup yielded success", result, err)
		}
	}
	if _, err := service.GetSiteUnitSummaryOptions(context.Background(), state.ContextID); err != nil {
		t.Fatal("options retry leaked lease", err)
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetSiteUnitSummaryOptions(context.Background(), state.ContextID); err == nil {
		t.Fatal("stale options context accepted")
	}
	selection := contextSelection(next)
	selection.SU, selection.SUPath = "None", ""
	next, err = service.SwitchContext(next.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetSiteUnitSummaryOptions(context.Background(), next.ContextID); err == nil {
		t.Fatal("options guessed unselected SU")
	}
}
