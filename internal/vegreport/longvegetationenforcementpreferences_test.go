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
			contexts, _ := vprotest.PrepareKMLContextFixture(t)
			store := contexts.Projects.Preferences
			if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": literal, "LVUnitGroups": groups}); err != nil {
				t.Fatal(err)
			}
			wantEnforce := literal == true || literal == -1
			expected := LongVegetationEnforcementPreferenceValues{wantEnforce, groups}
			got, err := ReadLongVegetationEnforcementPreferences(store, context.Background())
			if err != nil || got != expected {
				t.Fatal(got, expected, err)
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if update, err := CompareAndSetLongVegetationEnforcementPreferences(store, context.Background(), expected, expected); err != nil || update.Changed || update.Committed {
				t.Fatal("historical no-op reset unit groups", update, err)
			}
			after, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("no-op normalized original bool/int YAML", err)
			}
			proposed := LongVegetationEnforcementPreferenceValues{!wantEnforce, 1}
			if update, err := CompareAndSetLongVegetationEnforcementPreferences(store, context.Background(), expected, proposed); err != nil || !update.Changed || !update.Committed {
				t.Fatal("enforcement callback failed coupled group1 reset", update, err)
			}
			values, err := store.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if values["ReportOptions"].(map[string]any)["LVUnitGroups"] != 1 || values["ReportOptions"].(map[string]any)["DataQualityFilterEnforceLV"] != !wantEnforce {
				t.Fatal("paired source plan lost", values)
			}
			if _, err := CompareAndSetLongVegetationEnforcementPreferences(store, context.Background(), expected, proposed); err == nil {
				t.Fatal("stale paired plan overwrote current values")
			}
			for _, bad := range []LongVegetationEnforcementPreferenceValues{{wantEnforce, 0}, {wantEnforce, 4}, {wantEnforce, 2}, {wantEnforce, 3}} {
				if _, err := CompareAndSetLongVegetationEnforcementPreferences(store, context.Background(), proposed, bad); err == nil {
					t.Fatal("enforcement save invented unit-group operation", bad)
				}
			}
			// Changed-away/back retains source callback provenance via the group1 plan.
			if groups != 1 {
				if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": literal, "LVUnitGroups": groups}); err != nil {
					t.Fatal(err)
				}
				sameFlag := LongVegetationEnforcementPreferenceValues{wantEnforce, 1}
				if update, err := CompareAndSetLongVegetationEnforcementPreferences(store, context.Background(), expected, sameFlag); err != nil || !update.Committed {
					t.Fatal("final same enforcement skipped callback unit-group reset", update, err)
				}
				values, err = store.Snapshot()
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
	contexts, _ := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	for _, bad := range []any{nil, 1, -2, "False", "0", []any{false}} {
		if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": bad, "LVUnitGroups": 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadLongVegetationEnforcementPreferences(store, context.Background()); err == nil {
			t.Fatal("invalid flag storage became default", bad)
		}
	}
	if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": 0}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.Path)
	if err != nil || bytes.Count(raw, []byte("DataQualityFilterEnforceLV: 0")) != 1 {
		t.Fatal("explicit malformed float fixture boundary unavailable", err)
	}
	raw = bytes.Replace(raw, []byte("DataQualityFilterEnforceLV: 0"), []byte("DataQualityFilterEnforceLV: 0.0"), 1)
	if err := os.WriteFile(store.Path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLongVegetationEnforcementPreferences(store, context.Background()); err == nil {
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
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "enforcement client wire", "review", "request", "outcome"); err != nil {
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
	for _, observation := range []vprocore.PrinterObservation{wire.Review.Printers, *wire.Outcome.Printers} {
		if err := vprocore.ValidatePrinterObservation(context.Background(), observation); err != nil {
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
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	expected, proposed := LongVegetationEnforcementPreferenceValues{false, 2}, LongVegetationEnforcementPreferenceValues{true, 1}
	if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": 0, "LVUnitGroups": 2}); err != nil {
		t.Fatal(err)
	}

	service := NewLongVegetationEnforcementPreferencesService(contexts)
	names := vprocore.PrinterObservation{Names: []string{" literal printer "}, Source: vprocore.PrinterObservationSource}
	service.printers = func(context.Context) (vprocore.PrinterObservation, error) { return names, nil }
	request := enforcementRequest(t, expected, proposed)
	for _, bad := range []*LongVegetationEnforcementPreferencesService{nil, {}, NewLongVegetationEnforcementPreferencesService(nil)} {
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
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("read cleanup failure") }},
	} {
		service.snapshot = hooks
		if got := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	for _, id := range []string{"", "foreign"} {
		if got := service.SaveLongVegetationEnforcementPreferences(context.Background(), id, request); got.Committed || got.Changed || got.ErrorMessage == "" {
			t.Fatal(got)
		}
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("target collision") }
	got := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if got.Committed || got.Changed || got.ErrorMessage == "" || err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence modified config/staging", got, err, stageErr)
	}
	service.printers = func(context.Context) (vprocore.PrinterObservation, error) {
		return vprocore.PrinterObservation{}, errors.New("owned printer read failure")
	}
	if review, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); review != nil || err == nil {
		t.Fatal("printer error silently defaulted", review, err)
	}
	got = service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request)
	if !got.Committed || !got.Changed || got.Printers != nil || !strings.Contains(got.ErrorMessage, "do not replay") || !strings.Contains(got.ErrorMessage, "printer") {
		t.Fatal("postcommit printer failure erased persistence receipt", got)
	}
	service.printers = func(context.Context) (vprocore.PrinterObservation, error) { return names, nil }
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
