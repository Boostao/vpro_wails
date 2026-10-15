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

func vegetationNamingRequest(t *testing.T, expected, proposed LongVegetationNamingPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationNamingPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationNamingPreferencesOwnershipCleanupCancellationCollisionAndRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": "retained field"}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationNamingPreferencesService(contexts, true)
	expected, proposed := LongVegetationNamingPreferenceValues{1, "retained field"}, LongVegetationNamingPreferenceValues{0, ""}
	request := vegetationNamingRequest(t, expected, proposed)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got := service.SaveLongVegetationNamingPreferences(context.Background(), id, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationNamingPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "collision") || err != nil ||
		!bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
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
	got = service.SaveLongVegetationNamingPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	current, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values != proposed {
		t.Fatal("cancelled snapshot/persistence poisoned coordinator or lost atomic clear", current, err)
	}
	stale := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, request)
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "changed") {
		t.Fatal("stale naming pair accepted", stale)
	}
	retry := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, vegetationNamingRequest(t, proposed, proposed))
	if retry.Committed || retry.Changed || retry.ErrorMessage != "" {
		t.Fatal("observed committed retry was not a no-op", retry)
	}
}
func TestLongVegetationNamingPreferencesStrictGoClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationNamingWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Review  LongVegetationNamingPreferencesReview  `json:"review"`
		Request LongVegetationNamingPreferencesRequest `json:"request"`
		Outcome LongVegetationNamingPreferencesOutcome `json:"outcome"`
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "naming client wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	if wire.Review.ContextID != "naming-wire" || wire.Review.Values != wire.Request.Expected ||
		wire.Review.Values != (LongVegetationNamingPreferenceValues{1, " historical\u0000field "}) ||
		wire.Request.Proposed != (LongVegetationNamingPreferenceValues{2, ""}) ||
		wire.Review.Availability != (LongVegetationNamingAvailability{None: true, English: true, Code: true}) ||
		wire.Outcome != (LongVegetationNamingPreferencesOutcome{ContextID: "naming-wire", Changed: true, Committed: true}) {
		t.Fatal(wire)
	}
	roundtrip, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var independent, returned any
	if err := json.Unmarshal(raw, &independent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roundtrip, &returned); err != nil || !reflect.DeepEqual(independent, returned) {
		t.Fatal("production Go wire differs from client fixture", err)
	}
}

func TestLongVegetationNamingPreferencesGateOwnedReadAndHistoricalAvailability(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}} {
		enabled, err := longVegetationNamingPreferencesFeature(func(key string) (string, bool) {
			if key != "VPRO_LONG_VEGETATION_NAMING_PREFERENCES" {
				t.Fatal(key)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationNamingPreferencesService{nil, {}, NewLongVegetationNamingPreferencesService(nil, true)} {
		for _, requestContext := range []context.Context{nil, ctx, context.Background()} {
			if read, err := service.GetLongVegetationNamingPreferences(requestContext, "id"); read != nil || err == nil {
				t.Fatal(read, err)
			}
			if saved := service.SaveLongVegetationNamingPreferences(requestContext, "id", "{}"); saved.Changed || saved.Committed || saved.ErrorMessage == "" {
				t.Fatal(saved)
			}
		}
	}
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationNamingPreferencesService(contexts, true)
	for _, code := range []bool{false, true} {
		contexts.longVegetationCodeEnabled = code
		for _, mode := range []int{0, 1, 2, 4} {
			if err := contexts.projects.preferences.update("ReportOptions", map[string]any{
				"LVShowEnglishName": mode, "LVShowField": " historical\u0000field ",
			}); err != nil {
				t.Fatal(err)
			}
			read, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID)
			if err != nil || read.Values != (LongVegetationNamingPreferenceValues{mode, " historical\u0000field "}) ||
				read.ContextID != state.ContextID || read.ProjectPath != state.ProjectPath || read.Project != "Sample" ||
				read.Availability != (LongVegetationNamingAvailability{None: true, English: true, Code: code}) {
				t.Fatal(mode, code, read, err)
			}
			before, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			saved := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
				vegetationNamingRequest(t, read.Values, read.Values))
			after, err := os.ReadFile(contexts.projects.preferences.path)
			if saved.Changed || saved.Committed || saved.ErrorMessage != "" || err != nil || !bytes.Equal(before, after) {
				t.Fatal("historical unavailable/unused-field no-op changed bytes", mode, code, saved, err)
			}
		}
	}
	for _, id := range []string{"", "foreign"} {
		if read, err := service.GetLongVegetationNamingPreferences(context.Background(), id); read != nil || err == nil {
			t.Fatal(read, err)
		}
	}
}

func TestLongVegetationNamingPreferencesSupportedChoicesAtomicClearAndCodeAvailability(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	service := NewLongVegetationNamingPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, code := range []bool{false, true} {
		contexts.longVegetationCodeEnabled = code
		for _, originalMode := range []int{0, 1, 2, 4} {
			for _, nextMode := range []int{0, 1, 2, 4} {
				expected := LongVegetationNamingPreferenceValues{originalMode, "Old\u0000 literal"}
				proposed := LongVegetationNamingPreferenceValues{nextMode, ""}
				store := contexts.projects.preferences
				if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": expected.Mode, "LVShowField": expected.SelectedField}); err != nil {
					t.Fatal(err)
				}
				before, err := store.snapshot()
				if err != nil {
					t.Fatal(err)
				}
				result := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
					vegetationNamingRequest(t, expected, proposed))
				allowed := nextMode != 4 && (nextMode != 2 || code)
				after, err := store.snapshot()
				if err != nil {
					t.Fatal(err)
				}
				if allowed {
					if !result.Changed || !result.Committed || result.ErrorMessage != "" {
						t.Fatal(originalMode, nextMode, code, result)
					}
					before["ReportOptions"].(map[string]any)["LVShowEnglishName"] = proposed.Mode
					before["ReportOptions"].(map[string]any)["LVShowField"] = ""
				} else if result.Changed || result.Committed || result.ErrorMessage == "" {
					t.Fatal("unavailable new choice/clear authorized", originalMode, nextMode, code, result)
				}
				if !reflect.DeepEqual(before, after) {
					t.Fatal("coupled selection changed unrelated storage or refused request", originalMode, nextMode, code)
				}
				current, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID)
				if err != nil || allowed && current.Values != proposed || !allowed && current.Values != expected {
					t.Fatal(current, err)
				}
			}
		}
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("naming Save changed project data/audits")
	}
}

func TestLongVegetationNamingPreferencesStrictRequestsCoupledClearAndStaleRefusal(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": "old field"}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationNamingPreferencesService(contexts, true)
	expected := LongVegetationNamingPreferenceValues{1, "old field"}
	base := vegetationNamingRequest(t, expected, expected)
	requests := []string{"", "{}", "null", "[]", base + "{}",
		strings.Replace(base, `"mode":1`, `"mode":1,"mode":0`, 1),
		strings.Replace(base, `"mode":1`, `"mode":1,"\u006dode":0`, 1),
		strings.Replace(base, `"expected":`, `"foreign":`, 1),
		strings.Replace(base, `"old field"`, `"\ud800"`, 1)}
	for _, token := range []string{"null", `"1"`, "1.0", "true", "3", "-1", "5", "{}", "[]"} {
		requests = append(requests, strings.Replace(base, `"mode":1`, `"mode":`+token, 1))
	}
	for _, token := range []string{"null", "1", "true", "{}", "[]"} {
		requests = append(requests, strings.Replace(base, `"selectedField":"old field"`, `"selectedField":`+token, 1))
	}
	for _, proposed := range []LongVegetationNamingPreferenceValues{{0, "old field"}, {1, "new field"}, {1, "new\u0000field"}, {4, ""}} {
		requests = append(requests, vegetationNamingRequest(t, expected, proposed))
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range requests {
		result := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, raw)
		after, err := os.ReadFile(store.path)
		if result.Changed || result.Committed || result.ErrorMessage == "" || err != nil || !bytes.Equal(before, after) {
			t.Fatal("strict/refused request changed storage", raw, result, err)
		}
	}
	proposed := LongVegetationNamingPreferenceValues{1, ""}
	result := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
		vegetationNamingRequest(t, expected, proposed))
	if !result.Committed || !result.Changed || result.ErrorMessage != "" {
		t.Fatal("changed-away/back final mode could not clear the source-coupled field", result)
	}
	stale := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
		vegetationNamingRequest(t, expected, proposed))
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "reload") {
		t.Fatal("stale paired field witness accepted", stale)
	}
}

func TestLongVegetationNamingPreferencesMalformedStorageAndCorrection(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	service := NewLongVegetationNamingPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, key := range []string{"LVShowEnglishName", "LVShowField"} {
		invalid := []any{nil, true, []any{"unexpected"}}
		if key == "LVShowEnglishName" {
			invalid = append(invalid, "1", 1.5, 3, -1, 5)
		} else {
			invalid = append(invalid, 1)
		}
		for _, value := range invalid {
			if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": ""}); err != nil {
				t.Fatal(err)
			}
			values, err := store.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			values["ReportOptions"].(map[string]any)[key] = value
			raw, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if read, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID); read != nil || err == nil {
				t.Fatal("malformed storage gained a default", key, value, read, err)
			}
		}
	}
	if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": ""}); err != nil {
		t.Fatal(err)
	}
	if read, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID); err != nil || read.Values != (LongVegetationNamingPreferenceValues{1, ""}) {
		t.Fatal("corrected storage retry failed", read, err)
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("unavailable storage read changed data/audits")
	}
}
