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

func enforcementRequest(t *testing.T, expected, proposed LongVegetationEnforcementPreferenceValues) string {
	t.Helper()
	raw, err := json.Marshal(LongVegetationEnforcementPreferencesRequest{Expected: expected, Proposed: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLongVegetationEnforcementSourcePairsRepresentationsOmissionAndCAS(t *testing.T) {
	for _, literal := range []any{false, true, 0, -1} {
		for _, groups := range []int{1, 2, 3} {
			contexts, _ := prepareKMLContextFixture(t)
			store := contexts.projects.preferences
			if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": literal, "LVUnitGroups": groups}); err != nil {
				t.Fatal(err)
			}
			wantEnforce := literal == true || literal == -1
			expected := LongVegetationEnforcementPreferenceValues{wantEnforce, groups}
			got, err := store.readLongVegetationEnforcementPreferences(context.Background())
			if err != nil || got != expected {
				t.Fatal(got, expected, err)
			}
			before, err := os.ReadFile(store.path)
			if err != nil {
				t.Fatal(err)
			}
			if update, err := store.compareAndSetLongVegetationEnforcementPreferences(context.Background(), expected, expected); err != nil || update.Changed || update.Committed {
				t.Fatal("historical no-op reset unit groups", update, err)
			}
			after, err := os.ReadFile(store.path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("no-op normalized original bool/int YAML", err)
			}
			proposed := LongVegetationEnforcementPreferenceValues{!wantEnforce, 1}
			if update, err := store.compareAndSetLongVegetationEnforcementPreferences(context.Background(), expected, proposed); err != nil || !update.Changed || !update.Committed {
				t.Fatal("enforcement callback failed coupled group1 reset", update, err)
			}
			values, err := store.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if values["ReportOptions"].(map[string]any)["LVUnitGroups"] != 1 || values["ReportOptions"].(map[string]any)["DataQualityFilterEnforceLV"] != !wantEnforce {
				t.Fatal("paired source plan lost", values)
			}
			if _, err := store.compareAndSetLongVegetationEnforcementPreferences(context.Background(), expected, proposed); err == nil {
				t.Fatal("stale paired plan overwrote current values")
			}
			for _, bad := range []LongVegetationEnforcementPreferenceValues{{wantEnforce, 0}, {wantEnforce, 4}, {wantEnforce, 2}, {wantEnforce, 3}} {
				if _, err := store.compareAndSetLongVegetationEnforcementPreferences(context.Background(), proposed, bad); err == nil {
					t.Fatal("enforcement save invented unit-group operation", bad)
				}
			}
			// Changed-away/back retains source callback provenance via the group1 plan.
			if groups != 1 {
				if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": literal, "LVUnitGroups": groups}); err != nil {
					t.Fatal(err)
				}
				sameFlag := LongVegetationEnforcementPreferenceValues{wantEnforce, 1}
				if update, err := store.compareAndSetLongVegetationEnforcementPreferences(context.Background(), expected, sameFlag); err != nil || !update.Committed {
					t.Fatal("final same enforcement skipped callback unit-group reset", update, err)
				}
				values, err = store.snapshot()
				if err != nil || !reflect.DeepEqual(values["ReportOptions"].(map[string]any)["DataQualityFilterEnforceLV"], literal) {
					t.Fatal("unchanged flag assignment normalized historic storage", values, err)
				}
			}
		}
	}
}

func TestLongVegetationEnforcementStrictRequestsMalformedStorageAndFeature(t *testing.T) {
	valid := `{"expected":{"enforce":false,"unitGroups":2},"proposed":{"enforce":true,"unitGroups":1}}`
	var request LongVegetationEnforcementPreferencesRequest
	if err := json.Unmarshal([]byte(valid), &request); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`, `null`, valid + `{}`, strings.Replace(valid, `"enforce":false`, `"enforce":null`, 1),
		strings.Replace(valid, `"enforce":false`, `"enforce":-1`, 1),
		strings.Replace(valid, `"unitGroups":2`, `"unitGroups":2.0`, 1),
		strings.Replace(valid, `"unitGroups":2`, `"unitGroups":4`, 1),
		strings.Replace(valid, `"enforce":false`, `"enforce":false,"enforce":true`, 1),
		strings.Replace(valid, `"enforce":false`, `"enforce":false,"\u0065nforce":true`, 1),
		strings.Replace(valid, `"enforce":false`, `"enforce":false,"other":true`, 1),
	} {
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("strict enforcement wire accepted malformed request", raw)
		}
	}
	for _, test := range []struct {
		literal string
		present bool
		valid   bool
		enabled bool
	}{{"", false, true, false}, {"true", true, true, true}, {"false", true, true, false}, {"TRUE", true, false, false}, {"1", true, false, false}} {
		got, err := longVegetationEnforcementPreferencesFeature(func(key string) (string, bool) {
			if key != "VPRO_LONG_VEGETATION_ENFORCEMENT_PREFERENCES" {
				t.Fatal(key)
			}
			return test.literal, test.present
		})
		if got != test.enabled || (err == nil) != test.valid {
			t.Fatal(test, got, err)
		}
	}
	contexts, _ := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	for _, bad := range []any{nil, 1, -2, "False", "0", []any{false}} {
		if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": bad, "LVUnitGroups": 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.readLongVegetationEnforcementPreferences(context.Background()); err == nil {
			t.Fatal("invalid flag storage became default", bad)
		}
	}
	if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": 0}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.path)
	if err != nil || bytes.Count(raw, []byte("DataQualityFilterEnforceLV: 0")) != 1 {
		t.Fatal("explicit malformed float fixture boundary unavailable", err)
	}
	raw = bytes.Replace(raw, []byte("DataQualityFilterEnforceLV: 0"), []byte("DataQualityFilterEnforceLV: 0.0"), 1)
	if err := os.WriteFile(store.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.readLongVegetationEnforcementPreferences(context.Background()); err == nil {
		t.Fatal("actual YAML floating flag became integer default")
	}
}

func TestLongVegetationEnforcementStrictGoClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationEnforcementWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Review  LongVegetationEnforcementPreferencesReview  `json:"review"`
		Request LongVegetationEnforcementPreferencesRequest `json:"request"`
		Outcome LongVegetationEnforcementPreferencesOutcome `json:"outcome"`
	}
	if err := decodeStrictRequiredJSON(raw, &wire, "enforcement client wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	if wire.Review.Values != (LongVegetationEnforcementPreferenceValues{false, 2}) ||
		wire.Request.Expected != wire.Review.Values || wire.Request.Proposed != (LongVegetationEnforcementPreferenceValues{true, 1}) ||
		wire.Outcome.ContextID != "enforcement-wire" || !wire.Outcome.Changed || !wire.Outcome.Committed ||
		wire.Outcome.ErrorMessage != "" || wire.Outcome.Printers == nil ||
		!reflect.DeepEqual(wire.Review.Printers.Names, []string{" literal printer ", "Printer", "Printer"}) ||
		wire.Review.Printers.Default == nil || *wire.Review.Printers.Default != "Other printer" {
		t.Fatal("wire lost source pairs or literal host observations", wire)
	}
	for _, observation := range []PrinterObservation{wire.Review.Printers, *wire.Outcome.Printers} {
		if err := validatePrinterObservation(context.Background(), observation); err != nil {
			t.Fatal(err)
		}
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
		t.Fatal("production Go enforcement wire differs from client fixture", err)
	}
}

func TestLongVegetationEnforcementOwnedCleanupPrinterFailureAndCommittedRetry(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	expected, proposed := LongVegetationEnforcementPreferenceValues{false, 2}, LongVegetationEnforcementPreferenceValues{true, 1}
	if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": 0, "LVUnitGroups": 2}); err != nil {
		t.Fatal(err)
	}

	service := NewLongVegetationEnforcementPreferencesService(contexts, true)
	names := PrinterObservation{Names: []string{" literal printer "}, Source: printerObservationSource}
	service.printers = func(context.Context) (PrinterObservation, error) { return names, nil }
	request := enforcementRequest(t, expected, proposed)
	for _, bad := range []*LongVegetationEnforcementPreferencesService{nil, {}, NewLongVegetationEnforcementPreferencesService(nil, true)} {
		if got, err := bad.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got := bad.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	review, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID)
	if err != nil || review.Values != expected || review.ProjectPath != state.ProjectPath || !reflect.DeepEqual(review.Printers, names) {
		t.Fatal(review, err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("read cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	service.snapshot = publicationReadSnapshotHooks{}
	for _, id := range []string{"", "foreign"} {
		if got := service.SaveLongVegetationEnforcementPreferences(context.Background(), id, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("target collision") }
	got := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if got.Committed || got.Changed || got.ErrorMessage == "" || err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence modified config/staging", got, err, stageErr)
	}
	service.printers = func(context.Context) (PrinterObservation, error) {
		return PrinterObservation{}, errors.New("owned printer read failure")
	}
	if review, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); review != nil || err == nil {
		t.Fatal("printer error silently defaulted", review, err)
	}
	got = service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request)
	if !got.Committed || !got.Changed || got.Printers != nil || !strings.Contains(got.ErrorMessage, "do not replay") || !strings.Contains(got.ErrorMessage, "printer") {
		t.Fatal("postcommit printer failure erased persistence receipt", got)
	}
	service.printers = func(context.Context) (PrinterObservation, error) { return names, nil }
	review, err = service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID)
	if err != nil || review.Values != proposed {
		t.Fatal("failed printer refresh lost committed pair/retry", review, err)
	}
	got = service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, enforcementRequest(t, proposed, proposed))
	if got.Committed || got.Changed || got.ErrorMessage != "" || got.Printers == nil {
		t.Fatal("confirmed committed retry replayed save", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		if got := service.SaveLongVegetationEnforcementPreferences(ctx, state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal("invalid context authorized paired save", got)
		}
	}
}
