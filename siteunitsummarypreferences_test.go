package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func summaryPreferenceRequest(t *testing.T, expected, proposed SiteUnitSummaryPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(SiteUnitSummaryPreferencesRequest{expected, proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSiteUnitSummaryPreferencesGateStrictRequestAndOwnership(t *testing.T) {
	for _, value := range []string{"TRUE", "", "1", " true "} {
		if _, err := siteUnitSummaryPreferencesFeature(func(string) (string, bool) { return value, true }); err == nil {
			t.Fatal("invalid preference grant accepted", value)
		}
	}
	if enabled, err := siteUnitSummaryPreferencesFeature(func(string) (string, bool) { return "", false }); err != nil || enabled {
		t.Fatal("default enabled", enabled, err)
	}
	for _, raw := range []string{`{}`, `{"expected":null,"proposed":{"method":1,"siteUnitType":1}}`,
		`{"expected":{"method":1,"siteUnitType":1},"proposed":{"method":null,"siteUnitType":1}}`,
		`{"expected":{"method":1,"siteUnitType":1},"proposed":{"method":2,"method":1,"siteUnitType":1}}`,
		`{"expected":{"method":1,"siteUnitType":1},"proposed":{"method":"2","siteUnitType":1}}`,
		`{"expected":{"method":1,"siteUnitType":1},"proposed":{"method":2,"SiteUnitType":1}}`,
		`{"expected":{"method":1,"siteUnitType":1},"proposed":{"method":2,"siteUnitType":1},"\ud800":1}`} {
		var request SiteUnitSummaryPreferencesRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("malformed request accepted", raw)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*SiteUnitSummaryPreferencesService{nil, {}, NewSiteUnitSummaryPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if result, err := service.GetSiteUnitSummaryPreferences(requestCtx, "id"); err == nil || result != nil {
				t.Fatal("invalid preference read accepted", result, err)
			}
			result := service.SaveSiteUnitSummaryPreferences(requestCtx, "id", "{}")
			if result.Committed || result.Changed || result.ErrorMessage == "" {
				t.Fatal("invalid preference save accepted", result)
			}
		}
	}
}

func TestSiteUnitSummaryPreferencesOwnedCASNoopCollisionAndRestoration(t *testing.T) {
	contexts, state := reportServiceFixture(t, true)
	service := NewSiteUnitSummaryPreferencesService(contexts, true)
	disabled := NewSiteUnitSummaryPreferencesService(contexts, false)
	if result, err := disabled.GetSiteUnitSummaryPreferences(context.Background(), state.ContextID); err == nil || result != nil {
		t.Fatal("live owned context enabled preferences", result, err)
	}
	original, err := service.GetSiteUnitSummaryPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	before, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	expected := SiteUnitSummaryPreferenceValues{original.Method, original.SiteUnitType}
	proposed := SiteUnitSummaryPreferenceValues{2, 1}
	result := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, summaryPreferenceRequest(t, expected, proposed))
	if !result.Committed || !result.Changed || result.ErrorMessage != "" {
		t.Fatal("reviewed quartile save failed", result)
	}
	after, err := contexts.projects.preferences.snapshot()
	before["ReportOptions"].(map[string]any)["SEOptValueMethod"] = 2
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated YAML changed", err)
	}
	collision := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, summaryPreferenceRequest(t, expected, proposed))
	if collision.Committed || !strings.Contains(collision.ErrorMessage, "reload") {
		t.Fatal("stale expected option accepted", collision)
	}
	rawBefore, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	noOp := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, summaryPreferenceRequest(t, proposed, proposed))
	rawAfter, readErr := os.ReadFile(contexts.projects.preferences.path)
	if noOp.Committed || noOp.Changed || noOp.ErrorMessage != "" || readErr != nil || !bytes.Equal(rawBefore, rawAfter) {
		t.Fatal("no-op rewrote preferences", noOp, readErr)
	}
	for _, invalid := range []SiteUnitSummaryPreferenceValues{{0, 1}, {3, 1}, {2, 2}, {2, 3}} {
		result := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, summaryPreferenceRequest(t, proposed, invalid))
		if result.Committed || result.ErrorMessage == "" {
			t.Fatal("unsupported proposed option accepted", invalid, result)
		}
	}
	restored := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, summaryPreferenceRequest(t, proposed, expected))
	if !restored.Committed || restored.ErrorMessage != "" {
		t.Fatal("explicit restoration rejected", restored)
	}
	assertProfileSUFiles(t, contexts, beforeDB)
	next, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if result := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, summaryPreferenceRequest(t, expected, proposed)); result.Committed || result.ErrorMessage == "" {
		t.Fatal("stale context saved preference", result)
	}
	selection := contextSelection(next)
	selection.SU, selection.SUPath = "None", ""
	next, err = contexts.SwitchContext(next.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if result := service.SaveSiteUnitSummaryPreferences(context.Background(), next.ContextID, summaryPreferenceRequest(t, expected, proposed)); result.Committed || result.ErrorMessage == "" {
		t.Fatal("unselected SU saved preference", result)
	}
}

func TestSiteUnitSummaryPreferencesFailureRetryAndCommittedCancellation(t *testing.T) {
	contexts, state := reportServiceFixture(t, true)
	service := NewSiteUnitSummaryPreferencesService(contexts, true)
	store := contexts.projects.preferences
	expected, proposed := SiteUnitSummaryPreferenceValues{1, 1}, SiteUnitSummaryPreferenceValues{2, 1}
	request := summaryPreferenceRequest(t, expected, proposed)
	sentinel := errors.New("preference snapshot cleanup failed")
	service.snapshot.commitRead = func(*sql.Tx) error { return sentinel }
	if result := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, request); result.Committed || !strings.Contains(result.ErrorMessage, sentinel.Error()) {
		t.Fatal("cleanup failure saved preferences", result)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("injected replacement failure") }
	if result := service.SaveSiteUnitSummaryPreferences(context.Background(), state.ContextID, request); result.Committed || !strings.Contains(result.ErrorMessage, "replacement failure") {
		t.Fatal("replacement failure looked committed", result)
	}
	if options, err := service.GetSiteUnitSummaryPreferences(context.Background(), state.ContextID); err != nil || options.Method != 1 {
		t.Fatal("failure changed original", options, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(stage, destination string) error {
		if err := replace(stage, destination); err != nil {
			return err
		}
		cancel()
		return nil
	}
	result := service.SaveSiteUnitSummaryPreferences(ctx, state.ContextID, request)
	if !result.Committed || !result.Changed || !strings.Contains(result.ErrorMessage, "do not replay") {
		t.Fatal("irreversible receipt lost after cancellation", result)
	}
	store.replace = replace
	if options, err := service.GetSiteUnitSummaryPreferences(context.Background(), state.ContextID); err != nil || options.Method != 2 {
		t.Fatal("committed retry absent", options, err)
	}
}
