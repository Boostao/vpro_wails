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

func TestLongVegetationSelectedFieldGatePhysicalDefinitionsAndOwnedCleanup(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}} {
		enabled, err := longVegetationSelectedFieldPreferencesFeature(func(key string) (string, bool) {
			if key != "VPRO_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES" {
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
	for _, service := range []*LongVegetationSelectedFieldPreferencesService{nil, {}, NewLongVegetationSelectedFieldPreferencesService(nil, true)} {
		for _, requestContext := range []context.Context{nil, ctx, context.Background()} {
			if got, err := service.GetLongVegetationSelectedFieldPreferences(requestContext, "id"); got != nil || err == nil {
				t.Fatal(got, err)
			}
			if got := service.SaveLongVegetationSelectedFieldPreferences(requestContext, "id", "{}"); got.Changed || got.Committed || got.ErrorMessage == "" {
				t.Fatal(got)
			}
		}
	}
	contexts, state := prepareKMLContextFixture(t)
	owner, store := contexts.projects.sqlite, contexts.projects.preferences
	lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lists.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := lists.Exec(`DROP TABLE USysAllSpecs;
		CREATE TABLE USysAllSpecs (
		Code TEXT, EnglishName TEXT, " Native " TEXT, ID INTEGER, IsAttribute BOOLEAN)`); err != nil {
		t.Fatal(err)
	}
	if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": 4, "LVShowField": " absent field "}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationSelectedFieldPreferencesService(contexts, true)
	review, err := service.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID)
	definitions := []ProjectMetadataColumn{{"Code", "TEXT"}, {"EnglishName", "TEXT"}, {" Native ", "TEXT"}, {"ID", "INTEGER"}, {"IsAttribute", "BOOLEAN"}}
	if err != nil || review.Values != (LongVegetationSelectedFieldPreferenceValues{4, " absent field "}) ||
		review.ContextID != state.ContextID || review.ProjectPath != state.ProjectPath ||
		review.Source != longVegetationSelectedFieldDefinitionsSource || !reflect.DeepEqual(review.Definitions, definitions) {
		t.Fatal("literal saved identity or physical declared metadata changed", review, err)
	}
	for _, id := range []string{"", "foreign"} {
		if got, err := service.GetLongVegetationSelectedFieldPreferences(context.Background(), id); got != nil || err == nil {
			t.Fatal(got, err)
		}
	}
	expected, proposed := review.Values, LongVegetationSelectedFieldPreferenceValues{4, ""}
	request := vegetationSelectedFieldRequest(t, expected, proposed)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	ctx, cancel = context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got := service.SaveLongVegetationSelectedFieldPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal(got)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	path := owner.attachments["project"]
	owner.attachments["project"] = path + ".unowned"
	got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = path
	if got.Committed || got.Changed || got.ErrorMessage == "" {
		t.Fatal("unowned attachment authorized field save", got)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got = service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if got.Committed || got.Changed || !strings.Contains(got.ErrorMessage, "collision") ||
		err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed field persistence changed source or staging", got, err, stageErr)
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
	got = service.SaveLongVegetationSelectedFieldPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !got.Committed || !got.Changed || !strings.Contains(got.ErrorMessage, "do not replay") {
		t.Fatal("committed field result lost", got)
	}
	review, err = service.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID)
	if err != nil || review.Values != proposed {
		t.Fatal("cancelled cleanup poisoned ownership or lost committed pair", review, err)
	}
	if got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID,
		vegetationSelectedFieldRequest(t, proposed, proposed)); got.Committed || got.Changed || got.ErrorMessage != "" {
		t.Fatal("observed committed retry replayed mutation", got)
	}
	if _, err := lists.Exec(`DROP TABLE USysAllSpecs;
		CREATE VIEW USysAllSpecs AS SELECT 'phantom' AS Code`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal("view became physical definitions", got, err)
	}
	if _, err := lists.Exec(`DROP VIEW USysAllSpecs;
		CREATE TABLE USysAllSpecs (Code TEXT, "rowid" INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal("shadowed identities became physical definitions", got, err)
	}
}

func vegetationSelectedFieldRequest(t *testing.T, expected, proposed LongVegetationSelectedFieldPreferenceValues) string {
	t.Helper()
	data, err := json.Marshal(LongVegetationSelectedFieldPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLongVegetationSelectedFieldStrictGoClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationSelectedFieldWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Review  LongVegetationSelectedFieldPreferencesReview  `json:"review"`
		Request LongVegetationSelectedFieldPreferencesRequest `json:"request"`
		Outcome LongVegetationSelectedFieldPreferencesOutcome `json:"outcome"`
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "selected-field client wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	if wire.Review.Values != (LongVegetationSelectedFieldPreferenceValues{1, " historical\u0000field "}) ||
		wire.Request.Expected != wire.Review.Values ||
		wire.Request.Proposed != (LongVegetationSelectedFieldPreferenceValues{4, "Native"}) ||
		wire.Review.Source != longVegetationSelectedFieldDefinitionsSource ||
		!reflect.DeepEqual(wire.Review.Definitions, []ProjectMetadataColumn{
			{"Code", "TEXT"}, {"Native", "TEXT"}, {"ID", "INTEGER"}, {"IsAttribute", "BOOLEAN"},
		}) || wire.Outcome != (LongVegetationSelectedFieldPreferencesOutcome{
		ContextID: "selected-field-wire", Changed: true, Committed: true,
	}) {
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
		t.Fatal("production Go selected-field wire differs from client fixture", err)
	}
}

func TestLongVegetationSelectedFieldSourceReachablePairsHistoricalNoopAndNamingCollision(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	service := NewLongVegetationSelectedFieldPreferencesService(contexts, true)
	beforeDB := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, mode := range []int{0, 1, 2, 4} {
		for _, oldField := range []string{"", " historical\u0000field ", "EnglishName"} {
			for _, field := range []string{"", "Native", " nonmember ", strings.Repeat("x", 300), "SQL ]; DROP TABLE x;--"} {
				if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": mode, "LVShowField": oldField}); err != nil {
					t.Fatal(err)
				}
				expected := LongVegetationSelectedFieldPreferenceValues{mode, oldField}
				for _, proposed := range []LongVegetationSelectedFieldPreferenceValues{
					{mode, oldField}, {mode, field}, {4, field},
				} {
					allowed := proposed == expected || proposed.SelectedField != "" && proposed.Mode == 4 ||
						proposed.SelectedField == "" && (proposed.Mode == mode || proposed.Mode == 4)
					got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID,
						vegetationSelectedFieldRequest(t, expected, proposed))
					if allowed {
						if got.ErrorMessage != "" || got.Changed != (expected != proposed) || got.Committed != (expected != proposed) {
							t.Fatal(mode, oldField, proposed, got)
						}
					} else if got.Committed || got.Changed || got.ErrorMessage == "" {
						t.Fatal("unreachable source pair accepted", expected, proposed, got)
					}
					current, err := store.readLongVegetationSelectedFieldPreferences(context.Background())
					if err != nil || allowed && current != proposed || !allowed && current != expected {
						t.Fatal(current, proposed, allowed, err)
					}
					if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": mode, "LVShowField": oldField}); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	expected, proposed := LongVegetationSelectedFieldPreferenceValues{1, ""}, LongVegetationSelectedFieldPreferenceValues{4, "Native"}
	if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": expected.Mode, "LVShowField": expected.SelectedField}); err != nil {
		t.Fatal(err)
	}
	request := vegetationSelectedFieldRequest(t, expected, proposed)
	got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, request)
	if !got.Committed || got.ErrorMessage != "" {
		t.Fatal(got)
	}
	stale := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, request)
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "reload") {
		t.Fatal("selected-field stale pair accepted", stale)
	}
	naming := NewLongVegetationNamingPreferencesService(contexts, true)
	namingStale := naming.SaveLongVegetationNamingPreferences(context.Background(), state.ContextID,
		vegetationNamingRequest(t, LongVegetationNamingPreferenceValues(expected), LongVegetationNamingPreferenceValues{0, ""}))
	if namingStale.Committed || namingStale.Changed || !strings.Contains(namingStale.ErrorMessage, "reload") {
		t.Fatal("naming overwrote selected-field sibling", namingStale)
	}
	if !reflect.DeepEqual(beforeDB, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("field preference changed physical data or audits")
	}
}

func TestLongVegetationSelectedFieldStrictRequestsAndUnchangedInvalidOmission(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	service := NewLongVegetationSelectedFieldPreferencesService(contexts, true)
	expected := LongVegetationSelectedFieldPreferenceValues{1, " historical\u0000field "}
	if err := store.update("ReportOptions", map[string]any{"LVShowEnglishName": expected.Mode, "LVShowField": expected.SelectedField}); err != nil {
		t.Fatal(err)
	}
	base := vegetationSelectedFieldRequest(t, expected, expected)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{"", "null", "[]", "{}", base + "{}",
		strings.Replace(base, `"expected":`, `"foreign":`, 1),
		strings.Replace(base, `"mode":1`, `"mode":1,"\u006dode":4`, 1),
		strings.Replace(base, `"selectedField":`, `"selectedField":"x","selectedField":`, 1),
		strings.Replace(base, `"mode":1`, `"mode":3`, 1),
		strings.Replace(base, `"mode":1`, `"mode":null`, 1),
		strings.Replace(base, `"mode":1`, `"mode":"1"`, 1),
		strings.Replace(base, `"mode":1`, `"mode":1.0`, 1),
		strings.Replace(base, `"selectedField":`, `"missing":`, 1),
		strings.Replace(base, `" historical\u0000field "`, `"\ud800"`, 1),
	}
	for _, field := range []string{"new\u0000field", string([]byte{0xff})} {
		if result, err := store.compareAndSetLongVegetationSelectedFieldPreferences(context.Background(), expected,
			LongVegetationSelectedFieldPreferenceValues{4, field}); result.Committed || result.Changed || err == nil {
			t.Fatal("invalid new field survived direct boundary", result, err)
		}
	}
	for _, raw := range invalid {
		got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, raw)
		after, err := os.ReadFile(store.path)
		if got.Committed || got.Changed || got.ErrorMessage == "" || err != nil || !bytes.Equal(before, after) {
			t.Fatal(raw, got, err)
		}
	}
	noOp := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID, base)
	after, err := os.ReadFile(store.path)
	if noOp.Committed || noOp.Changed || noOp.ErrorMessage != "" || err != nil || !bytes.Equal(before, after) {
		t.Fatal("historical no-op changed file bytes", noOp, err)
	}
	modeOnly := LongVegetationSelectedFieldPreferenceValues{4, expected.SelectedField}
	got := service.SaveLongVegetationSelectedFieldPreferences(context.Background(), state.ContextID,
		vegetationSelectedFieldRequest(t, expected, modeOnly))
	if !got.Committed || got.ErrorMessage != "" {
		t.Fatal("unchanged invalid field blocked independent source mode assignment", got)
	}
	current, err := store.readLongVegetationSelectedFieldPreferences(context.Background())
	if err != nil || current != modeOnly {
		t.Fatal(current, err)
	}
}
