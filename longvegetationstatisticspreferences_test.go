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

func vegetationStatisticsRequest(t *testing.T, expected, proposed LongVegetationStatisticsPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationStatisticsPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationStatisticsPreferencesIndependentGateAndStrictRequests(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := longVegetationStatisticsPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_LONG_VEGETATION_STATISTICS_PREFERENCES" {
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
	for _, service := range []*LongVegetationStatisticsPreferencesService{nil, {}, NewLongVegetationStatisticsPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if got, err := service.GetLongVegetationStatisticsPreferences(requestCtx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationStatisticsPreferences(requestCtx, "id", "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationStatisticsPreferencesService(contexts, true)
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	base := `{"avgType":10,"orderBy":20}`
	for _, raw := range []string{
		`{}`, `null`, `[]`, `{"expected":null,"proposed":` + base + `}`,
		`{"Expected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"\u0065xpected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"proposed":{"avgType":20,"avgType":10,"orderBy":20}}`,
		`{"expected":` + base + `,"proposed":{"avgType":20,"\u0061vgType":10,"orderBy":20}}`,
		`{"expected":` + base + `,"proposed":{"AvgType":20,"orderBy":10}}`,
		`{"expected":` + base + `,"proposed":{"avgType":null,"orderBy":10}}`,
		`{"expected":` + base + `,"proposed":{"avgType":"20","orderBy":10}}`,
		`{"expected":` + base + `,"proposed":{"avgType":20.0,"orderBy":10}}`,
		`{"expected":` + base + `,"proposed":{"avgType":20}}`,
		`{"expected":` + base + `,"proposed":{"avgType":20,"orderBy":10,"title":"foreign"}}`,
		`{"expected":` + base + `,"proposed":` + base + `,"\ud800":0}`,
		`{"expected":` + base + `,"proposed":` + base + `,"` + string([]byte{0xff}) + `":0}`,
	} {
		got := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID, raw)
		if got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(raw, got)
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("malformed request wrote preferences", err)
	}
}

func TestLongVegetationStatisticsPreferencesSharedWireAllCombinationsAndYAMLOnly(t *testing.T) {
	var wire struct {
		Review  LongVegetationStatisticsPreferencesReview  `json:"review"`
		Request LongVegetationStatisticsPreferencesRequest `json:"request"`
		Outcome LongVegetationStatisticsPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/src/longVegetationStatisticsWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "statistics wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationStatisticsPreferencesService(contexts, true)
	disabled := NewLongVegetationStatisticsPreferencesService(contexts, false)
	if got, err := disabled.GetLongVegetationStatisticsPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal(got, err)
	}
	if got := disabled.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
		vegetationStatisticsRequest(t, wire.Request.Expected, wire.Request.Proposed)); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	read, err := service.GetLongVegetationStatisticsPreferences(context.Background(), state.ContextID)
	if err != nil || read.ContextID != state.ContextID || read.Project != "Sample" || read.SU != "None" ||
		read.ProjectPath != state.ProjectPath || read.SUPath != state.SUPath {
		t.Fatal(read, err)
	}
	read.ContextID, read.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("production wire review differs", read)
	}
	got := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
		vegetationStatisticsRequest(t, wire.Request.Expected, wire.Request.Proposed))
	got.ContextID = wire.Outcome.ContextID
	if *got != wire.Outcome {
		t.Fatal("production wire outcome differs", got)
	}
	encoded, err := json.Marshal(struct {
		Review  LongVegetationStatisticsPreferencesReview  `json:"review"`
		Request LongVegetationStatisticsPreferencesRequest `json:"request"`
		Outcome LongVegetationStatisticsPreferencesOutcome `json:"outcome"`
	}{*read, wire.Request, *got})
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
		t.Fatal("Go/client round trip changed wire shape")
	}
	// Adjacent transitions cover independent Average and Order assignments.
	for _, next := range []LongVegetationStatisticsPreferenceValues{{10, 10}, {10, 20}, {20, 20}, {20, 10}} {
		original, err := service.GetLongVegetationStatisticsPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		before, err := contexts.projects.preferences.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		got := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
			vegetationStatisticsRequest(t, original.Values, next))
		if !got.Committed || !got.Changed || got.ErrorMessage != "" {
			t.Fatal(next, got)
		}
		before["ReportOptions"].(map[string]any)["LVAvgType"] = next.AvgType
		before["ReportOptions"].(map[string]any)["LVOrderBy"] = next.OrderBy
		after, err := contexts.projects.preferences.snapshot()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("unrelated YAML changed", err)
		}
		stale := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
			vegetationStatisticsRequest(t, original.Values, next))
		if stale.Committed || !strings.Contains(stale.ErrorMessage, "reload") {
			t.Fatal(stale)
		}
		rawBefore, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		noop := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
			vegetationStatisticsRequest(t, next, next))
		rawAfter, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || noop.Committed || noop.Changed || noop.ErrorMessage != "" || !bytes.Equal(rawBefore, rawAfter) {
			t.Fatal("no-op changed bytes", noop, err)
		}
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("statistics preferences changed data/audits")
	}
}

func TestLongVegetationStatisticsPreferencesUnavailableStorageNoDefaults(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationStatisticsPreferencesService(contexts, true)
	store := contexts.projects.preferences
	base, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	expected := LongVegetationStatisticsPreferenceValues{10, 20}
	for _, key := range []string{"LVAvgType", "LVOrderBy"} {
		for _, invalid := range []any{nil, "", "10", 0, 11, 19, 21, 10.5, true} {
			base["ReportOptions"].(map[string]any)[key] = invalid
			raw, err := yaml.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.commit(raw); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetLongVegetationStatisticsPreferences(context.Background(), state.ContextID); got != nil || err == nil {
				t.Fatal(key, invalid, got, err)
			}
			result := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
				vegetationStatisticsRequest(t, expected, LongVegetationStatisticsPreferenceValues{20, 10}))
			after, err := os.ReadFile(store.path)
			if result.Committed || result.ErrorMessage == "" || err != nil || !bytes.Equal(raw, after) {
				t.Fatal("unavailable historical choice repaired", key, invalid, result, err)
			}
		}
		base["ReportOptions"].(map[string]any)[key] = map[string]int{"LVAvgType": 10, "LVOrderBy": 20}[key]
	}
}

func TestLongVegetationStatisticsPreferencesOwnershipCleanupReplacementCancellationAndRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationStatisticsPreferencesService(contexts, true)
	store := contexts.projects.preferences
	request := vegetationStatisticsRequest(t, LongVegetationStatisticsPreferenceValues{10, 20}, LongVegetationStatisticsPreferenceValues{20, 10})
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationStatisticsPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationStatisticsPreferences(context.Background(), id, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, invalid := range []LongVegetationStatisticsPreferenceValues{{0, 20}, {11, 20}, {10, 19}, {20, 21}} {
		got := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID,
			vegetationStatisticsRequest(t, LongVegetationStatisticsPreferenceValues{10, 20}, invalid))
		if got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(invalid, got)
		}
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationStatisticsPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("wrong attachment authorized statistics Save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationStatisticsPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed persistence changed either key", got, err)
	}
	stages, err := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if err != nil || len(stages) != 0 {
		t.Fatal("failed persistence retained a stage", stages, err)
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
	got = service.SaveLongVegetationStatisticsPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost on cancellation", got)
	}
	saved, err := service.GetLongVegetationStatisticsPreferences(context.Background(), state.ContextID)
	if err != nil || saved.Values != (LongVegetationStatisticsPreferenceValues{20, 10}) {
		t.Fatal(saved, err)
	}
}
