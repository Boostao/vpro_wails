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
)

func vegetationGroupRequest(t *testing.T, expected, proposed int) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationGroupPreferencesRequest{
		Expected: LongVegetationGroupPreferenceValues{GroupBy: expected},
		Proposed: LongVegetationGroupPreferenceValues{GroupBy: proposed},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationGroupPreferencesGateAndStrictWire(t *testing.T) {
	for _, test := range []struct {
		raw              string
		present, enabled bool
		invalid          bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := longVegetationGroupPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_LONG_VEGETATION_GROUP_PREFERENCES" {
				t.Fatal(name)
			}
			return test.raw, test.present
		})
		if got != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationGroupPreferencesService{nil, {}, NewLongVegetationGroupPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if got, err := service.GetLongVegetationGroupPreferences(requestCtx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationGroupPreferences(requestCtx, "id", "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	contexts, state := prepareKMLContextFixture(t)
	contexts.longVegetationStrataEnabled, contexts.longVegetationLifeformEnabled, contexts.longVegetationNoneEnabled = true, true, true
	service := NewLongVegetationGroupPreferencesService(contexts, true)
	var wire struct {
		Review  LongVegetationGroupPreferencesReview  `json:"review"`
		Request LongVegetationGroupPreferencesRequest `json:"request"`
		Outcome LongVegetationGroupPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/src/longVegetationGroupWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "group wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	read, err := service.GetLongVegetationGroupPreferences(context.Background(), state.ContextID)
	if err != nil || read.ContextID != state.ContextID || read.ProjectPath != state.ProjectPath || read.SUPath != state.SUPath {
		t.Fatal(read, err)
	}
	read.ContextID, read.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("production grouping wire differs", read)
	}
	outcome := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, vegetationGroupRequest(t, 1, 2))
	outcome.ContextID = wire.Outcome.ContextID
	if *outcome != wire.Outcome {
		t.Fatal(outcome)
	}
	roundtrip, err := json.Marshal(struct {
		Review  LongVegetationGroupPreferencesReview  `json:"review"`
		Request LongVegetationGroupPreferencesRequest `json:"request"`
		Outcome LongVegetationGroupPreferencesOutcome `json:"outcome"`
	}{*read, wire.Request, *outcome})
	if err != nil {
		t.Fatal(err)
	}
	var expectedJSON, actualJSON any
	if err := json.Unmarshal(raw, &expectedJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roundtrip, &actualJSON); err != nil || !reflect.DeepEqual(expectedJSON, actualJSON) {
		t.Fatal("strict Go/client round trip differs", err)
	}
	before, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{`{}`, `null`, `[]`, `{"Expected":{"groupBy":2},"proposed":{"groupBy":1}}`,
		`{"expected":{"groupBy":2},"expected":{"groupBy":1},"proposed":{"groupBy":1}}`,
		`{"expected":{"groupBy":2},"proposed":{"groupBy":1,"\u0067roupBy":2}}`,
		`{"expected":{"groupBy":2},"proposed":{"GroupBy":1}}`,
		`{"expected":{"groupBy":2},"proposed":{"groupBy":1,"availability":true}}`,
		`{"expected":{"groupBy":2},"proposed":null}`, `{"expected":{"groupBy":2},"proposed":{}}`,
		`{"expected":{"groupBy":2},"proposed":{"groupBy":1},"\ud800":0}`,
		`{"expected":{"groupBy":2},"proposed":{"groupBy":1},"` + string([]byte{0xff}) + `":0}`} {
		got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, invalid)
		if got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(invalid, got)
		}
	}
	for _, invalid := range []string{`null`, `"1"`, `true`, `1.0`, `1e0`, `0`, `5`, `-1`, `9223372036854775808`} {
		for _, field := range []string{"expected", "proposed"} {
			raw := `{"expected":{"groupBy":2},"proposed":{"groupBy":1}}`
			if field == "expected" {
				raw = `{"expected":{"groupBy":` + invalid + `},"proposed":{"groupBy":1}}`
			} else {
				raw = `{"expected":{"groupBy":2},"proposed":{"groupBy":` + invalid + `}}`
			}
			got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, raw)
			if got.Committed || got.ErrorMessage == "" {
				t.Fatal(raw, got)
			}
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("malformed request changed YAML", err)
	}
}

func TestLongVegetationGroupPreferencesAllTransitionsAndIndependentAvailability(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	service := NewLongVegetationGroupPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	for mask := 0; mask < 8; mask++ {
		contexts.longVegetationStrataEnabled = mask&1 != 0
		contexts.longVegetationLifeformEnabled = mask&2 != 0
		contexts.longVegetationNoneEnabled = mask&4 != 0
		for original := 1; original <= 4; original++ {
			for proposed := 1; proposed <= 4; proposed++ {
				if err := store.update("ReportOptions", map[string]any{"LVGroupBy": original}); err != nil {
					t.Fatal(err)
				}
				read, err := service.GetLongVegetationGroupPreferences(context.Background(), state.ContextID)
				expectedAvailability := LongVegetationGroupAvailability{true, mask&1 != 0, mask&2 != 0, mask&4 != 0}
				if err != nil || read.Values.GroupBy != original || read.Availability != expectedAvailability {
					t.Fatal(mask, original, read, err)
				}
				before, err := os.ReadFile(store.path)
				if err != nil {
					t.Fatal(err)
				}
				values, err := store.snapshot()
				if err != nil {
					t.Fatal(err)
				}
				allowed := proposed == 1 || proposed == 2 && mask&1 != 0 ||
					proposed == 3 && mask&2 != 0 || proposed == 4 && mask&4 != 0
				changed := original != proposed
				got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID,
					vegetationGroupRequest(t, original, proposed))
				if got.Committed != (changed && allowed) || got.Changed != got.Committed ||
					(got.ErrorMessage != "") != (changed && !allowed) {
					t.Fatal(mask, original, proposed, got)
				}
				after, err := os.ReadFile(store.path)
				if err != nil {
					t.Fatal(err)
				}
				if !got.Committed && !bytes.Equal(before, after) {
					t.Fatal("refusal/no-op rewrote historical storage", mask, original, proposed)
				}
				if got.Committed {
					values["ReportOptions"].(map[string]any)["LVGroupBy"] = proposed
				}
				current, err := store.snapshot()
				if err != nil || !reflect.DeepEqual(values, current) {
					t.Fatal("unrelated preference changed", mask, original, proposed, err)
				}
			}
		}
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("grouping changed source data/audits")
	}
}

func TestLongVegetationGroupPreferencesStorageStaleAndOwnership(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	service := NewLongVegetationGroupPreferencesService(contexts, true)
	disabled := NewLongVegetationGroupPreferencesService(contexts, false)
	if got, err := disabled.GetLongVegetationGroupPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal(got, err)
	}
	if got := disabled.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, vegetationGroupRequest(t, 1, 1)); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []any{nil, 0, 5, -1, 1.0, "1", true} {
		values["ReportOptions"].(map[string]any)["LVGroupBy"] = invalid
		if _, err := longVegetationGroupOption(values, "LVGroupBy"); err == nil {
			t.Fatal("invalid historical storage accepted", invalid)
		}
	}
	delete(values["ReportOptions"].(map[string]any), "LVGroupBy")
	if _, err := longVegetationGroupOption(values, "LVGroupBy"); err == nil {
		t.Fatal("missing historical storage defaulted")
	}
	if _, err := longVegetationGroupOption(values, "LVOrderBy"); err == nil {
		t.Fatal("foreign key accepted")
	}
	for _, pair := range [][2]int{{0, 1}, {1, 5}, {2, 1}} {
		got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, vegetationGroupRequest(t, pair[0], pair[1]))
		if got.Committed || got.ErrorMessage == "" {
			t.Fatal(pair, got)
		}
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationGroupPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationGroupPreferences(context.Background(), id, vegetationGroupRequest(t, 1, 1)); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
}

func TestLongVegetationGroupPreferencesCleanupCollisionCancellationAndCommittedRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	contexts.longVegetationStrataEnabled = true
	service := NewLongVegetationGroupPreferencesService(contexts, true)
	store := contexts.projects.preferences
	request := vegetationGroupRequest(t, 1, 2)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationGroupPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	owner := contexts.projects.sqlite
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") || err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal(got, err, stageErr)
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
	got = service.SaveLongVegetationGroupPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed result lost", got)
	}
	current, err := service.GetLongVegetationGroupPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values.GroupBy != 2 {
		t.Fatal(current, err)
	}
	got = service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, vegetationGroupRequest(t, 2, 2))
	if got.Committed || got.Changed || got.ErrorMessage != "" {
		t.Fatal("committed retry was not a no-op", got)
	}
}
