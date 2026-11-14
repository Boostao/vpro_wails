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

func TestLongVegetationGroupPreferencesStrictWire(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*LongVegetationGroupPreferencesService{nil, {}, NewLongVegetationGroupPreferencesService(nil)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if got, err := service.GetLongVegetationGroupPreferences(requestCtx, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationGroupPreferences(requestCtx, "id", "{}"); got.Committed || got.Changed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationGroupPreferencesService(contexts)
	var wire struct {
		Review  LongVegetationGroupPreferencesReview  `json:"review"`
		Request LongVegetationGroupPreferencesRequest `json:"request"`
		Outcome LongVegetationGroupPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/tests/fixtures/longVegetationGroupWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "group wire", "review", "request", "outcome"); err != nil {
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
	before, err := os.ReadFile(contexts.Projects.Preferences.Path)
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
	after, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("malformed request changed YAML", err)
	}
}

func TestLongVegetationGroupPreferencesAllTransitions(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	service := NewLongVegetationGroupPreferencesService(contexts)
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for original := 1; original <= 4; original++ {
		for proposed := 1; proposed <= 4; proposed++ {
			if err := store.Update("ReportOptions", map[string]any{"LVGroupBy": original}); err != nil {
				t.Fatal(err)
			}
			read, err := service.GetLongVegetationGroupPreferences(context.Background(), state.ContextID)
			expectedAvailability := LongVegetationGroupAvailability{true, true, true, true}
			if err != nil || read.Values.GroupBy != original || read.Availability != expectedAvailability {
				t.Fatal(original, read, err)
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			values, err := store.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			changed := original != proposed
			got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID,
				vegetationGroupRequest(t, original, proposed))
			if got.Committed != changed || got.Changed != got.Committed || got.ErrorMessage != "" {
				t.Fatal(original, proposed, got)
			}
			after, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Committed && !bytes.Equal(before, after) {
				t.Fatal("no-op rewrote historical storage", original, proposed)
			}
			if got.Committed {
				values["ReportOptions"].(map[string]any)["LVGroupBy"] = proposed
			}
			current, err := store.Snapshot()
			if err != nil || !reflect.DeepEqual(values, current) {
				t.Fatal("unrelated preference changed", original, proposed, err)
			}
		}
	}
	if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("grouping changed source data/audits")
	}
}

func TestLongVegetationGroupPreferencesStorageStaleAndOwnership(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	service := NewLongVegetationGroupPreferencesService(contexts)
	values, err := store.Snapshot()
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
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationGroupPreferencesService(contexts)
	store := contexts.Projects.Preferences
	request := vegetationGroupRequest(t, 1, 2)
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationGroupPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized Save", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationGroupPreferences(context.Background(), state.ContextID, request)
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
	got = service.SaveLongVegetationGroupPreferences(ctx, state.ContextID, request)
	store.Replace = replace
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
