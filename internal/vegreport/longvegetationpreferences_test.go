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
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func longVegetationTitleRequest(t *testing.T, expected, proposed string) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationPreferencesRequest{
		Expected: LongVegetationPreferenceValues{Title: expected}, Proposed: LongVegetationPreferenceValues{Title: proposed}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationPreferencesIndependentGate(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, true, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := LongVegetationPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_LONG_VEGETATION_PREFERENCES" {
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
	for _, service := range []*LongVegetationPreferencesService{nil, {}, NewLongVegetationPreferencesService(nil, true)} {
		for _, requestCtx := range []context.Context{nil, context.Background(), ctx} {
			if value, err := service.GetLongVegetationPreferences(requestCtx, "id"); value != nil || err == nil {
				t.Fatal(value, err)
			}
			if value := service.SaveLongVegetationPreferences(requestCtx, "id", "{}"); value.Committed || value.Changed || value.ErrorMessage == "" {
				t.Fatal(value)
			}
		}
	}
}

func TestLongVegetationPreferencesStrictWireAndYAMLOnly(t *testing.T) {
	var wire struct {
		Review  LongVegetationPreferencesReview  `json:"review"`
		Request LongVegetationPreferencesRequest `json:"request"`
		Outcome LongVegetationPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile("frontend/src/longVegetationPreferenceWire.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "title wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	if wire.Request.Expected.Title != "Long Vegetation Report" || wire.Request.Proposed.Title != " Literal <&\U0001f332\r\n title " {
		t.Fatal(wire.Request)
	}
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationPreferencesService(contexts, true)
	disabled := NewLongVegetationPreferencesService(contexts, false)
	if value, err := disabled.GetLongVegetationPreferences(context.Background(), state.ContextID); value != nil || err == nil {
		t.Fatal(value, err)
	}
	if value := disabled.SaveLongVegetationPreferences(context.Background(), state.ContextID, longVegetationTitleRequest(t, "Long Vegetation Report", "new")); value.Committed || value.ErrorMessage == "" {
		t.Fatal(value)
	}
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	before, err := contexts.Projects.Preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.GetLongVegetationPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if review.ContextID != state.ContextID || review.Project != "Sample" || review.ProjectPath != state.ProjectPath || review.SU != "None" || review.SUPath != state.SUPath {
		t.Fatal(review, state)
	}
	review.ContextID, review.ProjectPath = wire.Review.ContextID, wire.Review.ProjectPath
	if !reflect.DeepEqual(*review, wire.Review) {
		t.Fatal("production review differs from client wire", review, wire.Review)
	}
	request, err := json.Marshal(wire.Request)
	if err != nil {
		t.Fatal(err)
	}
	outcome := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, string(request))
	outcome.ContextID = wire.Outcome.ContextID
	if *outcome != wire.Outcome {
		t.Fatal("production outcome differs from client wire", outcome)
	}
	after, err := contexts.Projects.Preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	before["ReportOptions"].(map[string]any)["LVReportTitle"] = wire.Request.Proposed.Title
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated preferences changed")
	}
	encoded, err := json.Marshal(struct {
		Review  LongVegetationPreferencesReview  `json:"review"`
		Request LongVegetationPreferencesRequest `json:"request"`
		Outcome LongVegetationPreferencesOutcome `json:"outcome"`
	}{*review, wire.Request, *outcome})
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
		t.Fatal("wire shape differs", string(encoded))
	}
	if !reflect.DeepEqual(beforeDB, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("title persistence changed data or audits")
	}
}

func TestLongVegetationPreferencesRefusalsNoopAndLiteralRetry(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationPreferencesService(contexts, true)
	store := contexts.Projects.Preferences
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	base := `{"title":"Long Vegetation Report"}`
	for _, raw := range []string{
		`{}`, `null`, `[]`, `{"expected":null,"proposed":` + base + `}`, `{"expected":` + base + `}`,
		`{"Expected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"\u0065xpected":` + base + `,"proposed":` + base + `}`,
		`{"expected":` + base + `,"proposed":{"title":"one","title":"two"}}`,
		`{"expected":` + base + `,"proposed":{"title":"one","\u0074itle":"two"}}`,
		`{"expected":` + base + `,"proposed":{"Title":"wrong"}}`,
		`{"expected":` + base + `,"proposed":{"title":null}}`,
		`{"expected":` + base + `,"proposed":{"title":42}}`,
		`{"expected":` + base + `,"proposed":{"title":"\ud800"}}`,
		`{"expected":` + base + `,"proposed":{"title":"` + string([]byte{0xff}) + `"}}`,
		`{"expected":` + base + `,"proposed":{"title":"a\u0000b"}}`,
		`{"expected":` + base + `,"proposed":{"title":"new","extra":0}}`,
		`{"expected":` + base + `,"proposed":` + base + `,"extra":0}`,
	} {
		if value := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, raw); value.Committed || value.Changed || value.ErrorMessage == "" {
			t.Fatal(raw, value)
		}
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationPreferences(context.Background(), id, longVegetationTitleRequest(t, "Long Vegetation Report", "new")); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	rejected, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, rejected) {
		t.Fatal("strict/foreign request changed YAML", err)
	}
	for _, proposed := range []string{"", "  \U0001f332 literal\r\n  ", strings.Repeat("x", 1000)} {
		read, err := service.GetLongVegetationPreferences(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		next := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, longVegetationTitleRequest(t, read.Values.Title, proposed))
		if !next.Committed || next.ErrorMessage != "" {
			t.Fatal(next)
		}
		stale := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, longVegetationTitleRequest(t, read.Values.Title, "stale"))
		if stale.Committed || !strings.Contains(stale.ErrorMessage, "reload") {
			t.Fatal(stale)
		}
	}
	// The rejection/no-op checkpoint uses an independently reopened YAML file.
	current, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, current) {
		t.Fatal("valid literal changes did not persist")
	}
	title, err := ReadLongVegetationPreference(store, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, longVegetationTitleRequest(t, title, title))
	if got.Committed || got.Changed || got.ErrorMessage != "" {
		t.Fatal(got)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(current, after) {
		t.Fatal("no-op rewrote YAML", err)
	}
}

func TestLongVegetationPreferencesHistoricalInvalidStorage(t *testing.T) {
	for _, historical := range []any{"old\x00title", nil, true, 42} {
		contexts, state := vprotest.PrepareKMLContextFixture(t)
		store := contexts.Projects.Preferences
		values, err := store.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		values["ReportOptions"].(map[string]any)["LVReportTitle"] = historical
		raw, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(raw); err != nil {
			t.Fatal(err)
		}
		service := NewLongVegetationPreferencesService(contexts, true)
		review, err := service.GetLongVegetationPreferences(context.Background(), state.ContextID)
		if title, valid := historical.(string); valid {
			if err != nil || review.Values.Title != title {
				t.Fatal(review, err)
			}
			got := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, longVegetationTitleRequest(t, title, title))
			if got.Committed || got.ErrorMessage != "" {
				t.Fatal(got)
			}
			got = service.SaveLongVegetationPreferences(context.Background(), state.ContextID, longVegetationTitleRequest(t, title, "corrected"))
			if !got.Committed || got.ErrorMessage != "" {
				t.Fatal(got)
			}
		} else if err == nil || review != nil {
			t.Fatal("invalid setting became an empty title", review, err)
		}
	}
}

func TestLongVegetationPreferencesCleanupOwnershipRollbackAndCommittedReceipt(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	service := NewLongVegetationPreferencesService(contexts, true)
	store := contexts.Projects.Preferences
	request := longVegetationTitleRequest(t, "Long Vegetation Report", "committed")
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, request); got.Committed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationPreferences(ctx, state.ContextID, request); got.Committed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	owner := contexts.Projects.Sqlite
	path := owner.Attachments["project"]
	owner.Attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = path
	if got.Committed || got.ErrorMessage == "" {
		t.Fatal("unowned context authorized title save", got)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	if got.Committed || !strings.Contains(got.ErrorMessage, "collision") {
		t.Fatal(got)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected cleanup/ownership/replacement changed config", err)
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
	got = service.SaveLongVegetationPreferences(ctx, state.ContextID, request)
	store.Replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("irreversible result lost", got)
	}
	saved, err := service.GetLongVegetationPreferences(context.Background(), state.ContextID)
	if err != nil || saved.Values.Title != "committed" {
		t.Fatal(saved, err)
	}
}
