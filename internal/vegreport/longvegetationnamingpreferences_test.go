package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	if err := store.Update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": "retained field"}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationNamingPreferencesService(contexts)
	expected, proposed := LongVegetationNamingPreferenceValues{1, "retained field"}, LongVegetationNamingPreferenceValues{0, ""}
	request := vegetationNamingRequest(t, expected, proposed)
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got := service.SaveLongVegetationNamingPreferences(context.Background(), id, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
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
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationNamingPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "collision") || err != nil ||
		!bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed source or retained staging", got, err, stageErr)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.Replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got = service.SaveLongVegetationNamingPreferences(ctx, state.ContextID, request)
	store.Replace = replace
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
	raw, err := os.ReadFile(filepath.Join("frontend", "tests", "fixtures", "longVegetationNamingWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Review  LongVegetationNamingPreferencesReview  `json:"review"`
		Request LongVegetationNamingPreferencesRequest `json:"request"`
		Outcome LongVegetationNamingPreferencesOutcome `json:"outcome"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "naming client wire", "review", "request", "outcome"); err != nil {
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

func TestLongVegetationNamingPreferencesOwnedReadAndHistoricalAvailability(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationNamingPreferencesService{nil, {}, NewLongVegetationNamingPreferencesService(nil)} {
		for _, requestContext := range []context.Context{nil, ctx, context.Background()} {
			if read, err := service.GetLongVegetationNamingPreferences(requestContext, "id"); read != nil || err == nil {
				t.Fatal(read, err)
			}
			if saved := service.SaveLongVegetationNamingPreferences(requestContext, "id", "{}"); saved.Changed || saved.Committed || saved.ErrorMessage == "" {
				t.Fatal(saved)
			}
		}
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationNamingPreferencesService(contexts)
	for _, mode := range []int{0, 1, 2, 4} {
		if err := contexts.Projects.Preferences.Update("ReportOptions", map[string]any{
			"LVShowEnglishName": mode, "LVShowField": " historical\u0000field ",
		}); err != nil {
			t.Fatal(err)
		}
		read, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID)
		if err != nil || read.Values != (LongVegetationNamingPreferenceValues{mode, " historical\u0000field "}) ||
			read.ContextID != state.ContextID || read.ProjectPath != state.ProjectPath || read.Project != "Sample" ||
			read.Availability != (LongVegetationNamingAvailability{None: true, English: true, Code: true}) {
			t.Fatal(mode, read, err)
		}
		before, err := os.ReadFile(contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		saved := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
			vegetationNamingRequest(t, read.Values, read.Values))
		after, err := os.ReadFile(contexts.Projects.Preferences.Path)
		if saved.Changed || saved.Committed || saved.ErrorMessage != "" || err != nil || !bytes.Equal(before, after) {
			t.Fatal("historical unavailable/unused-field no-op changed bytes", mode, saved, err)
		}
	}
	for _, id := range []string{"", "foreign"} {
		if read, err := service.GetLongVegetationNamingPreferences(context.Background(), id); read != nil || err == nil {
			t.Fatal(read, err)
		}
	}
}

func TestLongVegetationNamingPreferencesSupportedChoicesAndAtomicClear(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationNamingPreferencesService(contexts)
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, originalMode := range []int{0, 1, 2, 4} {
		for _, nextMode := range []int{0, 1, 2, 4} {
			expected := LongVegetationNamingPreferenceValues{originalMode, "Old\u0000 literal"}
			proposed := LongVegetationNamingPreferenceValues{nextMode, ""}
			store := contexts.Projects.Preferences
			if err := store.Update("ReportOptions", map[string]any{"LVShowEnglishName": expected.Mode, "LVShowField": expected.SelectedField}); err != nil {
				t.Fatal(err)
			}
			before, err := store.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			result := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
				vegetationNamingRequest(t, expected, proposed))
			allowed := nextMode != 4
			after, err := store.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if allowed {
				if !result.Changed || !result.Committed || result.ErrorMessage != "" {
					t.Fatal(originalMode, nextMode, result)
				}
				before["ReportOptions"].(map[string]any)["LVShowEnglishName"] = proposed.Mode
				before["ReportOptions"].(map[string]any)["LVShowField"] = ""
			} else if result.Changed || result.Committed || result.ErrorMessage == "" {
				t.Fatal("unavailable new choice/clear authorized", originalMode, nextMode, result)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("coupled selection changed unrelated storage or refused request", originalMode, nextMode)
			}
			current, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID)
			if err != nil || allowed && current.Values != proposed || !allowed && current.Values != expected {
				t.Fatal(current, err)
			}
		}
	}
	if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("naming Save changed project data/audits")
	}
}

func TestLongVegetationNamingPreferencesStrictRequestsCoupledClearAndStaleRefusal(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	if err := store.Update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": "old field"}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationNamingPreferencesService(contexts)
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
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range requests {
		result := service.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID, raw)
		after, err := os.ReadFile(store.Path)
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
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	service := NewLongVegetationNamingPreferencesService(contexts)
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, key := range []string{"LVShowEnglishName", "LVShowField"} {
		invalid := []any{nil, true, []any{"unexpected"}}
		if key == "LVShowEnglishName" {
			invalid = append(invalid, "1", 1.5, 3, -1, 5)
		} else {
			invalid = append(invalid, 1)
		}
		for _, value := range invalid {
			if err := store.Update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": ""}); err != nil {
				t.Fatal(err)
			}
			values, err := store.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			values["ReportOptions"].(map[string]any)[key] = value
			raw, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if read, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID); read != nil || err == nil {
				t.Fatal("malformed storage gained a default", key, value, read, err)
			}
		}
	}
	if err := store.Update("ReportOptions", map[string]any{"LVShowEnglishName": 1, "LVShowField": ""}); err != nil {
		t.Fatal(err)
	}
	if read, err := service.GetLongVegetationNamingPreferences(context.Background(), state.ContextID); err != nil || read.Values != (LongVegetationNamingPreferenceValues{1, ""}) {
		t.Fatal("corrected storage retry failed", read, err)
	}
	if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("unavailable storage read changed data/audits")
	}
}
