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
