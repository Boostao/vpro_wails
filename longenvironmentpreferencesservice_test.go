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
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

func longEnvironmentPreferenceRequest(t *testing.T, expected, proposed string) string {
	t.Helper()
	raw, err := json.Marshal(LongEnvironmentPreferencesRequest{
		Expected: LongEnvironmentPreferenceValues{Title: expected}, Proposed: LongEnvironmentPreferenceValues{Title: proposed}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongEnvironmentPreferencesFacadeGate(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := longEnvironmentPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_LONG_ENVIRONMENT_PREFERENCES" {
				t.Fatal(name)
			}
			return test.value, test.present
		})
		if got != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongEnvironmentPreferencesService{nil, {}, NewLongEnvironmentPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if value, err := service.GetLongEnvironmentPreferences(requestCtx, "id"); value != nil || err == nil {
				t.Fatal(value, err)
			}
			if value := service.SaveLongEnvironmentPreferences(requestCtx, "id", "{}"); value.Committed || value.Changed || value.ErrorMessage == "" {
				t.Fatal(value)
			}
		}
	}
}

func TestLongEnvironmentPreferencesFacadeOwnedYAMLOnlyCAS(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongEnvironmentPreferencesService(contexts, true)
	disabled := NewLongEnvironmentPreferencesService(contexts, false)
	if value, err := disabled.GetLongEnvironmentPreferences(context.Background(), state.ContextID); value != nil || err == nil {
		t.Fatal("live context implicitly enabled preferences", value, err)
	}
	if value := disabled.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, "{}"); value.Committed || value.ErrorMessage == "" {
		t.Fatal(value)
	}
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	got, err := service.GetLongEnvironmentPreferences(context.Background(), state.ContextID)
	if err != nil || got.ContextID != state.ContextID || got.Project != contexts.projects.sqlite.selection.Project || got.ProjectPath != state.ProjectPath ||
		got.SU != "None" || got.SUPath != state.SUPath || got.Values.Title != "Long Evironment Report" {
		t.Fatal(got, err)
	}
	before, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	next := " Literal <&\x01\U0001f332\r\n "
	outcome := service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, longEnvironmentPreferenceRequest(t, got.Values.Title, next))
	if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" {
		t.Fatal(outcome)
	}
	after, err := contexts.projects.preferences.snapshot()
	before["ReportOptions"].(map[string]any)["LEReportTitle"] = next
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated preferences changed", err)
	}
	stale := service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, longEnvironmentPreferenceRequest(t, got.Values.Title, next))
	if stale.Committed || !strings.Contains(stale.ErrorMessage, "reload") {
		t.Fatal(stale)
	}
	noOp := service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, longEnvironmentPreferenceRequest(t, next, next))
	if noOp.Committed || noOp.Changed || noOp.ErrorMessage != "" {
		t.Fatal(noOp)
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("data/audits changed")
	}
}

func TestLongEnvironmentPreferencesFacadeStrictTransport(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongEnvironmentPreferencesService(contexts, true)
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	base := `{"title":"Long Evironment Report"}`
	for _, raw := range []string{
		`{}`, `null`, `[]`, `{"expected":null,"proposed":` + base + `}`, `{"expected":` + base + `}`,
		`{"expected":` + base + `,"proposed":` + base + `,"extra":0}`,
		`{"Expected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"expected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"proposed":{"title":"\ud800"}}`,
		`{"expected":` + base + `,"proposed":{"title":null}}`,
		`{"expected":` + base + `,"proposed":{"title":42}}`,
		`{"expected":` + base + `,"proposed":{"title":"one","Title":"two"}}`,
		`{"expected":` + base + `,"proposed":{"title":"one","\u0074itle":"two"}}`,
		`{"expected":` + base + `,"proposed":{"title":"` + string([]byte{0xff}) + `"}}`,
		`{"expected":` + base + `,"proposed":{"title":"a\u0000b"}}`,
		`{"expected":` + base + `,"proposed":{"title":"new","other":0}}`,
		`{"expected":` + base + `,"proposed":{}}`,
	} {
		if outcome := service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, raw); outcome.Committed || outcome.Changed || outcome.ErrorMessage == "" {
			t.Fatal(raw, outcome)
		}
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongEnvironmentPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongEnvironmentPreferences(context.Background(), id, longEnvironmentPreferenceRequest(t, "Long Evironment Report", "new")); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected request wrote config", err)
	}
}

func TestLongEnvironmentPreferencesFacadeHistoricalAndInvalidStorage(t *testing.T) {
	for _, historical := range []any{"old\x00title", string([]byte{0xff}), nil, true, 42} {
		contexts, state := prepareKMLContextFixture(t)
		store := contexts.projects.preferences
		values, err := store.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		values["ReportOptions"].(map[string]any)["LEReportTitle"] = historical
		raw, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.commit(raw); err != nil {
			t.Fatal(err)
		}
		service := NewLongEnvironmentPreferencesService(contexts, true)
		got, err := service.GetLongEnvironmentPreferences(context.Background(), state.ContextID)
		title, valid := historical.(string)
		if !valid || !utf8.ValidString(title) {
			if got != nil || err == nil {
				t.Fatal("silently defaulted", got, err)
			}
			continue
		}
		if err != nil || got.Values.Title != title {
			t.Fatal("historical title repaired", got, err)
		}
		outcome := service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, longEnvironmentPreferenceRequest(t, title, title))
		after, err := os.ReadFile(store.path)
		if err != nil || !bytes.Equal(raw, after) || outcome.Committed || outcome.Changed || outcome.ErrorMessage != "" {
			t.Fatal("historical no-op changed config", outcome, err)
		}
		outcome = service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, longEnvironmentPreferenceRequest(t, title, ""))
		if !outcome.Committed || outcome.ErrorMessage != "" {
			t.Fatal("correction to empty rejected", outcome)
		}
	}
}

func TestLongEnvironmentPreferencesFacadeCleanupCancellationRetryAndOwnership(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongEnvironmentPreferencesService(contexts, true)
	review, err := service.GetLongEnvironmentPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request := longEnvironmentPreferenceRequest(t, review.Values.Title, "Retry")
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if value, err := service.GetLongEnvironmentPreferences(context.Background(), state.ContextID); value != nil || err == nil {
			t.Fatal(value, err)
		}
		if got := service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal("cleanup authorized config write", got)
		}
	}
	duringRead, cancelRead := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancelRead(); return nil }}
	if got := service.SaveLongEnvironmentPreferences(duringRead, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal("read cancellation authorized config write", got)
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("cleanup/cancellation wrote config", err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	ctx, cancel := context.WithCancel(context.Background())
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	done := make(chan *LongEnvironmentPreferencesOutcome, 1)
	go func() { done <- service.SaveLongEnvironmentPreferences(ctx, state.ContextID, request) }()
	cancel()
	got := <-done
	owner.mu.Unlock()
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	originalPath := owner.attachments["project"]
	owner.attachments["project"] = originalPath + ".unowned"
	got = service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = originalPath
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned source authorized write", got)
	}
	store := contexts.projects.preferences
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("config target collision") }
	got = service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, request)
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") {
		t.Fatal(got)
	}
	store.replace = replace
	got = service.SaveLongEnvironmentPreferences(context.Background(), state.ContextID, request)
	if !got.Committed || got.ErrorMessage != "" {
		t.Fatal("explicit retry unavailable", got)
	}
}

func TestLongEnvironmentPreferencesFacadeCommittedReceipt(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongEnvironmentPreferencesService(contexts, true)
	review, err := service.GetLongEnvironmentPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := contexts.projects.preferences
	replace := store.replace
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got := service.SaveLongEnvironmentPreferences(ctx, state.ContextID, longEnvironmentPreferenceRequest(t, review.Values.Title, "Committed"))
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal(got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var transported LongEnvironmentPreferencesOutcome
	if err := json.Unmarshal(raw, &transported); err != nil || transported != *got {
		t.Fatal("committed receipt lost", transported, err)
	}
	saved, err := service.GetLongEnvironmentPreferences(context.Background(), state.ContextID)
	if err != nil || saved.Values.Title != "Committed" {
		t.Fatal(saved, err)
	}
}
