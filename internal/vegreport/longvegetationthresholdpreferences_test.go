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
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func vegetationThresholdRequest(t *testing.T, expected, proposed LongVegetationThresholdPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationThresholdPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationThresholdPreferencesStrictWire(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationThresholdPreferencesService{nil, {}, NewLongVegetationThresholdPreferencesService(nil)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if got, err := service.GetLongVegetationThresholdPreferences(requestCtx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationThresholdPreferences(requestCtx, "id", "{}"); got.Committed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	var wire struct {
		Review  LongVegetationThresholdPreferencesReview  `json:"review"`
		Request LongVegetationThresholdPreferencesRequest `json:"request"`
		Outcome LongVegetationThresholdPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/tests/fixtures/longVegetationThresholdWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "threshold wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationThresholdPreferencesService(contexts)
	read, err := service.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID)
	if err != nil || read.ContextID != state.ContextID || read.ProjectPath != state.ProjectPath || read.SUPath != state.SUPath {
		t.Fatal(read, err)
	}
	read.ContextID, read.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("Go/client threshold wire differs", read)
	}
	outcome := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID,
		vegetationThresholdRequest(t, wire.Request.Expected, wire.Request.Proposed))
	outcome.ContextID = wire.Outcome.ContextID
	if *outcome != wire.Outcome {
		t.Fatal(outcome)
	}
	encoded, err := json.Marshal(struct {
		Review  LongVegetationThresholdPreferencesReview  `json:"review"`
		Request LongVegetationThresholdPreferencesRequest `json:"request"`
		Outcome LongVegetationThresholdPreferencesOutcome `json:"outcome"`
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
		t.Fatal("strict-Go threshold wire round trip changed")
	}
}

func TestLongVegetationThresholdPreferencesStrictNumbersAndMalformedRequests(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationThresholdPreferencesService(contexts)
	base := vegetationThresholdRequest(t, LongVegetationThresholdPreferenceValues{}, LongVegetationThresholdPreferenceValues{})
	before, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`, `null`, `[]`, strings.Replace(base, `"expected":`, `"Expected":`, 1),
		strings.Replace(base, `"expected":`, `"expected":null,"extra":`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":null`, 1),
		strings.Replace(base, `"meanCoverGreaterThan":0`, `"meanCoverGreaterThan":null`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":true`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":"0"`, 1),
		strings.Replace(base, `"presenceGreaterThan":0,`, "", 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":0,"presenceGreaterThan":1`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":0,"\u0070resenceGreaterThan":1`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"PresenceGreaterThan":0`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":1e400`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":1e-400`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":-1e-400`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":0,"\ud800":0`, 1),
		strings.Replace(base, `"presenceGreaterThan":0`, `"presenceGreaterThan":0,"`+string([]byte{0xff})+`":0`, 1),
	} {
		if got := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID, raw); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal("malformed threshold request accepted", raw, got)
		}
	}
	for _, raw := range []string{"0", "-0", "0e999", "0.0e-999", "5e-324", "-5e-324", "1.7976931348623157e308"} {
		if _, err := decodeLongVegetationThresholdNumber([]byte(raw), "test"); err != nil {
			t.Fatal("finite number rejected", raw, err)
		}
	}
	for _, raw := range []string{"", "null", " null ", `"0"`, "true", "[]", "{}", "1e400", "1e-400", "-1e-400"} {
		if _, err := decodeLongVegetationThresholdNumber([]byte(raw), "test"); err == nil {
			t.Fatal("invalid numeric boundary accepted", raw)
		}
	}
	after, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed YAML", err)
	}
}

func TestLongVegetationThresholdPreferencesFinitePairsNoopStaleAndHistoricalOmission(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationThresholdPreferencesService(contexts)
	store := contexts.Projects.Preferences
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, next := range []LongVegetationThresholdPreferenceValues{
		{-1, 0}, {-1, 1.5}, {101, -999}, {math.SmallestNonzeroFloat64, math.MaxFloat64},
		{-math.SmallestNonzeroFloat64, -math.MaxFloat64}, {0, 0},
	} {
		current, err := service.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		values, err := store.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		fields := values["ReportOptions"].(map[string]any)
		if current.Values.PresenceGreaterThan != next.PresenceGreaterThan {
			fields["LVPresenceGreaterThan"] = next.PresenceGreaterThan
		}
		if current.Values.MeanCoverGreaterThan != next.MeanCoverGreaterThan {
			fields["LVCoverGreaterThan"] = next.MeanCoverGreaterThan
		}
		expectedBytes, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		got := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID,
			vegetationThresholdRequest(t, current.Values, next))
		actualBytes, err := os.ReadFile(store.Path)
		if err != nil || !got.Committed || got.ErrorMessage != "" || !bytes.Equal(expectedBytes, actualBytes) {
			t.Fatal("unrelated YAML changed", next, got, err)
		}
		read, err := service.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID)
		if err != nil || read.Values != next {
			t.Fatal(next, read, err)
		}
		noop := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID,
			vegetationThresholdRequest(t, next, next))
		stale := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID,
			vegetationThresholdRequest(t, current.Values, next))
		after, err := os.ReadFile(store.Path)
		if err != nil || noop.Committed || noop.ErrorMessage != "" || stale.Committed ||
			!strings.Contains(stale.ErrorMessage, "reload") || !bytes.Equal(actualBytes, after) {
			t.Fatal(noop, stale, err)
		}
	}
	if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("threshold writes changed project data/audits")
	}
}

func TestLongVegetationThresholdPreferencesInvalidStorageAndNonfinitePlans(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationThresholdPreferencesService(contexts)
	store := contexts.Projects.Preferences
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	fields := values["ReportOptions"].(map[string]any)
	for _, key := range []string{"LVPresenceGreaterThan", "LVCoverGreaterThan"} {
		original := fields[key]
		delete(fields, key)
		missing, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(missing); err != nil {
			t.Fatal(err)
		}
		if got, err := service.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal("missing threshold defaulted", key, got, err)
		}
		for _, invalid := range []any{nil, "", "0", false, int64(9007199254740993), math.NaN(), math.Inf(1)} {
			fields[key] = invalid
			raw, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(raw); err != nil {
				t.Fatal(err)
			}
			if got, err := service.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID); got != nil || err == nil {
				t.Fatal(key, invalid, got, err)
			}
			got := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID,
				vegetationThresholdRequest(t, LongVegetationThresholdPreferenceValues{}, LongVegetationThresholdPreferenceValues{}))
			after, err := os.ReadFile(store.Path)
			if got.Committed || got.ErrorMessage == "" || err != nil || !bytes.Equal(raw, after) {
				t.Fatal(key, invalid, got, err)
			}
		}
		fields[key] = original
	}
	for _, number := range []float64{math.NaN(), math.Inf(-1), math.Inf(1)} {
		for _, invalid := range []LongVegetationThresholdPreferenceValues{{number, 0}, {0, number}} {
			if result, err := CompareAndSetLongVegetationThresholdPreferences(store, context.Background(), invalid,
				LongVegetationThresholdPreferenceValues{}); result.Committed || err == nil {
				t.Fatal(result, err)
			}
			if result, err := CompareAndSetLongVegetationThresholdPreferences(store, context.Background(),
				LongVegetationThresholdPreferenceValues{}, invalid); result.Committed || err == nil {
				t.Fatal(result, err)
			}
		}
	}
}

func TestLongVegetationThresholdPreferencesOwnershipCleanupCollisionCancellationAndRetry(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationThresholdPreferencesService(contexts)
	store := contexts.Projects.Preferences
	proposed := LongVegetationThresholdPreferenceValues{-1, 1.5}
	request := vegetationThresholdRequest(t, LongVegetationThresholdPreferenceValues{}, proposed)
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationThresholdPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationThresholdPreferences(context.Background(), id, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationThresholdPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationThresholdPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal(got, err, stageErr)
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
	got = service.SaveLongVegetationThresholdPreferences(ctx, state.ContextID, request)
	store.Replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	current, err := service.GetLongVegetationThresholdPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values != proposed {
		t.Fatal(current, err)
	}
}
