package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func vegetationSwitchRequest(t *testing.T, expected, proposed LongVegetationSwitchPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationSwitchPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationSwitchPreferencesGateAndStrictWire(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := longVegetationSwitchPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_LONG_VEGETATION_SWITCH_PREFERENCES" {
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
	for _, service := range []*LongVegetationSwitchPreferencesService{nil, {}, NewLongVegetationSwitchPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if got, err := service.GetLongVegetationSwitchPreferences(requestCtx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationSwitchPreferences(requestCtx, "id", "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	var wire struct {
		Review  LongVegetationSwitchPreferencesReview  `json:"review"`
		Request LongVegetationSwitchPreferencesRequest `json:"request"`
		Outcome LongVegetationSwitchPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/src/longVegetationSwitchWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "switch wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationSwitchPreferencesService(contexts, true)
	disabled := NewLongVegetationSwitchPreferencesService(contexts, false)
	if got, err := disabled.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal(got, err)
	}
	if got := disabled.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
		vegetationSwitchRequest(t, wire.Request.Expected, wire.Request.Proposed)); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	read, err := service.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID)
	if err != nil || read.ContextID != state.ContextID || read.Project != "Sample" || read.SU != "None" ||
		read.ProjectPath != state.ProjectPath || read.SUPath != state.SUPath {
		t.Fatal(read, err)
	}
	read.ContextID, read.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("Go/client review wire differs", read)
	}
	outcome := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
		vegetationSwitchRequest(t, wire.Request.Expected, wire.Request.Proposed))
	outcome.ContextID = wire.Outcome.ContextID
	if *outcome != wire.Outcome {
		t.Fatal("Go/client outcome differs", outcome)
	}
	encoded, err := json.Marshal(struct {
		Review  LongVegetationSwitchPreferencesReview  `json:"review"`
		Request LongVegetationSwitchPreferencesRequest `json:"request"`
		Outcome LongVegetationSwitchPreferencesOutcome `json:"outcome"`
	}{*read, wire.Request, *outcome})
	if err != nil {
		t.Fatal(err)
	}
	var expectedJSON, actualJSON any
	if err := json.Unmarshal(raw, &expectedJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actualJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expectedJSON, actualJSON) {
		t.Fatal("strict-Go round trip changed client wire")
	}
}

func TestLongVegetationSwitchPreferencesRejectMalformedBooleanRequests(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationSwitchPreferencesService(contexts, true)
	values := LongVegetationSwitchPreferenceValues{true, false, true, true}
	base := vegetationSwitchRequest(t, values, values)
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`, `null`, `[]`, strings.Replace(base, `"expected":`, `"Expected":`, 1),
		strings.Replace(base, `"expected":`, `"expected":null,"extra":`, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":null`, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":1`, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":"true"`, 1),
		strings.Replace(base, `"quickReport":true,`, ``, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":true,"quickReport":false`, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":true,"\u0071uickReport":false`, 1),
		strings.Replace(base, `"quickReport":true`, `"QuickReport":true`, 1),
		strings.Replace(base, `"spaceBetweenGroups":false`, `"spaceBetweenGroups":null`, 1),
		strings.Replace(base, `"reportSummary":true`, `"reportSummary":null`, 1),
		strings.Replace(base, `"constantSpeciesList":true`, `"constantSpeciesList":null`, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":true,"\ud800":0`, 1),
		strings.Replace(base, `"quickReport":true`, `"quickReport":true,"`+string([]byte{0xff})+`":0`, 1),
	} {
		if got := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID, raw); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal("malformed switch request accepted", raw, got)
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed YAML", err)
	}
}

func TestLongVegetationSwitchPreferencesAllSixteenCombinationsAndNoop(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationSwitchPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	for combination := 0; combination < 16; combination++ {
		next := LongVegetationSwitchPreferenceValues{combination&1 != 0, combination&2 != 0, combination&4 != 0, combination&8 != 0}
		current, err := service.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		before, err := contexts.projects.preferences.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		changed := current.Values != next
		result := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
			vegetationSwitchRequest(t, current.Values, next))
		if result.Committed != changed || result.Changed != changed || result.ErrorMessage != "" {
			t.Fatal(combination, result)
		}
		fields := before["ReportOptions"].(map[string]any)
		if current.Values.QuickReport != next.QuickReport {
			fields["LVQuickReport"] = next.QuickReport
		}
		if current.Values.SpaceBetweenGroups != next.SpaceBetweenGroups {
			fields["LVSpaceBetweenGroups"] = next.SpaceBetweenGroups
		}
		if current.Values.ReportSummary != next.ReportSummary {
			fields["LVReportSummary"] = next.ReportSummary
		}
		if current.Values.ConstantSpeciesList != next.ConstantSpeciesList {
			fields["LVConstantSppList"] = next.ConstantSpeciesList
		}
		after, err := contexts.projects.preferences.snapshot()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("unrelated YAML changed", combination, err)
		}
		rawBefore, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		noop := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID, vegetationSwitchRequest(t, next, next))
		rawAfter, err := os.ReadFile(contexts.projects.preferences.path)
		if noop.Committed || noop.Changed || noop.ErrorMessage != "" || err != nil || !bytes.Equal(rawBefore, rawAfter) {
			t.Fatal("no-op rewrote YAML", combination, noop, err)
		}
		if changed {
			stale := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
				vegetationSwitchRequest(t, current.Values, next))
			if stale.Committed || !strings.Contains(stale.ErrorMessage, "reload") {
				t.Fatal(stale)
			}
		}
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("switch Save changed data/audits")
	}
}

func TestLongVegetationSwitchPreferencesHistoricalStoragePreservedAndInvalidUnavailable(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationSwitchPreferencesService(contexts, true)
	store := contexts.projects.preferences
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	fields := values["ReportOptions"].(map[string]any)
	fields["LVQuickReport"], fields["LVSpaceBetweenGroups"] = -1, 0
	fields["LVReportSummary"], fields["LVConstantSppList"] = -1, -1
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	current, err := service.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values != (LongVegetationSwitchPreferenceValues{true, false, true, true}) {
		t.Fatal(current, err)
	}
	result := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
		vegetationSwitchRequest(t, current.Values, current.Values))
	after, err := os.ReadFile(store.path)
	if err != nil || result.Committed || result.ErrorMessage != "" || !bytes.Equal(raw, after) {
		t.Fatal("normalized read rewrote historical storage", result, err)
	}
	proposed := current.Values
	proposed.QuickReport = false
	result = service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
		vegetationSwitchRequest(t, current.Values, proposed))
	fields["LVQuickReport"] = false
	saved, err := store.snapshot()
	if err != nil || !result.Committed || result.ErrorMessage != "" || !reflect.DeepEqual(values, saved) {
		t.Fatal("unchanged historical assignments were normalized", result, err)
	}
	for _, key := range []string{"LVQuickReport", "LVSpaceBetweenGroups", "LVReportSummary", "LVConstantSppList"} {
		original := fields[key]
		for _, invalid := range []any{nil, "", "true", "False", 1, -2, float64(1)} {
			fields[key] = invalid
			raw, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.commit(raw); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID); got != nil || err == nil {
				t.Fatal(key, invalid, got, err)
			}
			got := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID,
				vegetationSwitchRequest(t, proposed, proposed))
			after, err := os.ReadFile(store.path)
			if got.Committed || got.ErrorMessage == "" || err != nil || !bytes.Equal(raw, after) {
				t.Fatal("malformed saved switch became a default", key, invalid, got, err)
			}
		}
		fields[key] = original
	}
}

func TestLongVegetationSwitchPreferencesOwnershipCleanupReplacementCancellationAndRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationSwitchPreferencesService(contexts, true)
	store := contexts.projects.preferences
	expected, proposed := LongVegetationSwitchPreferenceValues{true, false, true, true}, LongVegetationSwitchPreferenceValues{false, true, false, false}
	request := vegetationSwitchRequest(t, expected, proposed)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationSwitchPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationSwitchPreferences(context.Background(), id, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationSwitchPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationSwitchPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed source or retained staging", got, err, stageErr)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got = service.SaveLongVegetationSwitchPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	current, err := service.GetLongVegetationSwitchPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values != proposed {
		t.Fatal(current, err)
	}
}
