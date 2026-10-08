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

	"gopkg.in/yaml.v3"
)

func preferenceRequest(t *testing.T, expected, proposed GoogleEarthPreferenceValues) string {
	t.Helper()
	data, err := json.Marshal(GoogleEarthPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGoogleEarthPreferencesFacadeGate(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := googleEarthPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_GOOGLE_EARTH_PREFERENCES" {
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
	for _, service := range []*GoogleEarthPreferencesService{nil, {}, NewGoogleEarthPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if value, err := service.GetGoogleEarthPreferences(requestCtx, "id"); value != nil || err == nil {
				t.Fatal(value, err)
			}
			value := service.SaveGoogleEarthPreferences(requestCtx, "id", "{}")
			if value.Committed || value.Changed || value.ErrorMessage == "" {
				t.Fatal(value)
			}
		}
	}
}

func TestGoogleEarthPreferencesFacadeOwnedReadCASAndNoDataWrites(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	got, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil || got.ContextID != state.ContextID || got.ProjectPath != state.ProjectPath || got.SU != "None" ||
		got.Values != (GoogleEarthPreferenceValues{"VPro Plot Locations", "PlotRepresenting"}) {
		t.Fatal(got, err)
	}
	before, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	next := GoogleEarthPreferenceValues{" Literal <&\U0001f332\r\n ", "Zone"}
	outcome := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, got.Values, next))
	if !outcome.Committed || !outcome.Changed || outcome.ErrorMessage != "" {
		t.Fatal(outcome)
	}
	after, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	before["ReportOptions"].(map[string]any)["GoogleEarthPlaceName"] = next.Title
	before["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = next.DescriptionField
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated preferences changed")
	}
	stale := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, got.Values, next))
	if stale.Committed || !strings.Contains(stale.ErrorMessage, "reload") {
		t.Fatal(stale)
	}
	noOp := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, next, next))
	if noOp.Committed || noOp.Changed || noOp.ErrorMessage != "" {
		t.Fatal(noOp)
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("data/audits changed")
	}
}

func TestGoogleEarthPreferencesFacadeStrictJSONAndMembership(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthPreferencesService(contexts, true)
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	base := `{"title":"VPro Plot Locations","descriptionField":"PlotRepresenting"}`
	for _, raw := range []string{
		`{}`, `{"expected":null,"proposed":` + base + `}`, `{"expected":` + base + `}`,
		`{"expected":` + base + `,"proposed":` + base + `,"extra":0}`,
		`{"expected":` + base + `,"Expected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"expected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"proposed":{"title":"\ud800","descriptionField":"Zone"}}`,
		`{"expected":` + base + `,"proposed":{"title":null,"descriptionField":"Zone"}}`,
		`{"expected":` + base + `,"proposed":{"title":"one","Title":"two","descriptionField":"Zone"}}`,
		`{"expected":` + base + `,"proposed":{"title":"one","\u0074itle":"two","descriptionField":"Zone"}}`,
		`{"expected":` + base + `,"proposed":{"title":"` + string([]byte{0xff}) + `","descriptionField":"Zone"}}`,
		`{"expected":` + base + `,"proposed":{"title":"changed","descriptionField":"zone"}}`,
		`{"expected":` + base + `,"proposed":{"title":"changed","descriptionField":" Zone "}}`,
		`{"expected":` + base + `,"proposed":{"title":"changed","descriptionField":"Missing"}}`,
	} {
		outcome := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, raw)
		if outcome.Committed || outcome.Changed || outcome.ErrorMessage == "" {
			t.Fatal(raw, outcome)
		}
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetGoogleEarthPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveGoogleEarthPreferences(context.Background(), id, `{"expected":`+base+`,"proposed":`+base+`}`); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected request wrote config", err)
	}
}

func TestGoogleEarthPreferencesFacadeHistoricalAndNonKMLField(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	historical := GoogleEarthPreferenceValues{"old\x01title", "absent\x00field"}
	values["ReportOptions"].(map[string]any)["GoogleEarthPlaceName"] = historical.Title
	values["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = historical.DescriptionField
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	service := NewGoogleEarthPreferencesService(contexts, true)
	got, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil || got.Values != historical {
		t.Fatal("historical strings repaired", got, err)
	}
	next := historical
	next.Title = "Corrected"
	outcome := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, historical, next))
	if !outcome.Committed || outcome.ErrorMessage != "" {
		t.Fatal("unchanged unavailable field rejected", outcome)
	}
	numeric := next
	numeric.DescriptionField = "Longitude"
	outcome = service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, next, numeric))
	if !outcome.Committed || outcome.ErrorMessage != "" {
		t.Fatal("physical nonKML field rejected", outcome)
	}
}

func TestGoogleEarthPreferencesFacadeMissingNullAndWrongType(t *testing.T) {
	for _, invalid := range []any{nil, true, 42} {
		contexts, state := prepareKMLContextFixture(t)
		store := contexts.projects.preferences
		values, err := store.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		values["ReportOptions"].(map[string]any)["GoogleEarthPlaceName"] = invalid
		raw, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.commit(raw); err != nil {
			t.Fatal(err)
		}
		service := NewGoogleEarthPreferencesService(contexts, true)
		if got, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal("silently defaulted", got, err)
		}
	}
}

func TestGoogleEarthPreferencesFacadeReadCleanupAndCancellation(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthPreferencesService(contexts, true)
	review, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next := review.Values
	next.Title = "Corrected"
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("read cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		got := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, review.Values, next))
		if got.Committed || got.ErrorMessage == "" {
			t.Fatal("cleanup failure committed preferences", got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	got := service.SaveGoogleEarthPreferences(ctx, state.ContextID, preferenceRequest(t, review.Values, next))
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed read/cancel changed config", err)
	}
}

func TestGoogleEarthPreferencesFacadeDisabledOwnedServiceAndReplacementRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	disabled := NewGoogleEarthPreferencesService(contexts, false)
	if got, err := disabled.GetGoogleEarthPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal("live context enabled preferences implicitly", got, err)
	}
	if got := disabled.SaveGoogleEarthPreferences(context.Background(), state.ContextID, "{}"); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service := NewGoogleEarthPreferencesService(contexts, true)
	review, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next := review.Values
	next.Title = "Retry"
	store := contexts.projects.preferences
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("config target collision") }
	got := service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, review.Values, next))
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "collision") {
		t.Fatal(got)
	}
	store.replace = replace
	got = service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, review.Values, next))
	if !got.Committed || got.ErrorMessage != "" {
		t.Fatal("explicit retry unavailable", got)
	}
}

func TestGoogleEarthPreferencesFacadeQueuedCancelledOwnerLeaseAndRestoration(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthPreferencesService(contexts, true)
	review, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next := review.Values
	next.Title = "Queued"
	ctx, cancel := context.WithCancel(context.Background())
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	done := make(chan *GoogleEarthPreferencesOutcome, 1)
	go func() {
		done <- service.SaveGoogleEarthPreferences(ctx, state.ContextID, preferenceRequest(t, review.Values, next))
	}()
	cancel()
	got := <-done
	owner.mu.Unlock()
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	saved, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil || saved.Values != review.Values {
		t.Fatal("cancel left lease or modified config", saved, err)
	}
	originalPath := owner.attachments["project"]
	owner.attachments["project"] = originalPath + ".unowned"
	got = service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, review.Values, next))
	owner.attachments["project"] = originalPath
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned source authorized config write", got)
	}
	got = service.SaveGoogleEarthPreferences(context.Background(), state.ContextID, preferenceRequest(t, review.Values, next))
	if !got.Committed || got.ErrorMessage != "" {
		t.Fatal("ownership restoration not usable", got)
	}
}
func TestGoogleEarthPreferencesFacadeCommittedCancellationTransport(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewGoogleEarthPreferencesService(contexts, true)
	review, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next := review.Values
	next.Title = "Committed"
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
	got := service.SaveGoogleEarthPreferences(ctx, state.ContextID, preferenceRequest(t, review.Values, next))
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal(got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var transported GoogleEarthPreferencesOutcome
	if err := json.Unmarshal(raw, &transported); err != nil || transported != *got {
		t.Fatal("committed receipt lost", transported, err)
	}
	saved, err := service.GetGoogleEarthPreferences(context.Background(), state.ContextID)
	if err != nil || saved.Values != next {
		t.Fatal(saved, err)
	}
}
