package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func vegetationUnitRequest(t *testing.T, expected, proposed int) string {
	t.Helper()
	data, err := json.Marshal(LongVegetationUnitPreferencesRequest{
		Expected: LongVegetationUnitPreferenceValues{UnitGroups: expected},
		Proposed: LongVegetationUnitPreferenceValues{UnitGroups: proposed},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLongVegetationUnitPreferencesIndependentGateAndStrictRequest(t *testing.T) {
	for _, test := range []struct {
		raw     string
		present bool
		want    bool
		invalid bool
	}{{"", false, true, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		enabled, err := LongVegetationUnitPreferencesFeature(func(name string) (string, bool) {
			if name != "VPRO_LONG_VEGETATION_UNIT_PREFERENCES" {
				t.Fatal("unit preference borrowed report/species grouping gate", name)
			}
			return test.raw, test.present
		})
		if enabled != test.want || (err != nil) != test.invalid {
			t.Fatal("nonliteral preference gate accepted", test, enabled, err)
		}
	}
	for _, raw := range []string{
		`{}`, `{"expected":{"unitGroups":1}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":null}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":0}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":4}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":3.0}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":"3"}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":true}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":3,"extra":0}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":1,"unitGroups":3}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":1,"\u0075nitGroups":3}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":3},"proposed":{"unitGroups":1}}`,
		`{"expected":{"unitGroups":1},"proposed":{"unitGroups":3},"extra":"\ud800"}`,
	} {
		var request LongVegetationUnitPreferencesRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("malformed/duplicate/new unknown request accepted", raw, request)
		}
	}
	for _, service := range []*LongVegetationUnitPreferencesService{nil, {}, NewLongVegetationUnitPreferencesService(nil, true)} {
		for _, ctx := range []context.Context{nil, context.Background()} {
			if value, err := service.GetLongVegetationUnitPreferences(ctx, "id"); value != nil || err == nil {
				t.Fatal("unavailable unit editor returned review", value, err)
			}
			if value := service.SaveLongVegetationUnitPreferences(ctx, "id", "{}"); value.Committed || value.Changed || value.ErrorMessage == "" {
				t.Fatal("unavailable unit editor returned successful receipt", value)
			}
		}
	}
}

func TestLongVegetationUnitPreferencesChoiceMatrixRetainsHistoryAndOtherOptions(t *testing.T) {
	contexts, state := vprotest.ReportServiceFixture(t, true)
	service := NewLongVegetationUnitPreferencesService(contexts, true)
	store := contexts.Projects.Preferences
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, enabled := range []bool{false, true} {
		contexts.LongVegetationNoneUnitsEnabled = enabled
		for _, current := range []int{1, 2, 3} {
			for _, proposed := range []int{1, 2, 3} {
				if err := store.Update("ReportOptions", map[string]any{"LVUnitGroups": current}); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(store.Path)
				if err != nil {
					t.Fatal(err)
				}
				values, err := store.Snapshot()
				if err != nil {
					t.Fatal(err)
				}
				review, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID)
				if err != nil || review.Values.UnitGroups != current || review.ContextID != state.ContextID ||
					review.Project != state.ActiveProject || review.ProjectPath != state.ProjectPath ||
					review.SU != state.ActiveSU || review.SUPath != state.SUPath ||
					review.Availability != (LongVegetationUnitAvailability{SiteUnitTable: true, None: enabled}) {
					t.Fatal("owned saved unit choice or independent availability lost", review, err)
				}
				result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID,
					vegetationUnitRequest(t, current, proposed))
				allowed := current == proposed || proposed == 1 || proposed == 3 && enabled
				changed := allowed && current != proposed
				if result.Committed != changed || result.Changed != changed ||
					(result.ErrorMessage != "") == allowed || result.ContextID != state.ContextID {
					t.Fatal("choice/no-op availability differs", enabled, current, proposed, result)
				}
				after, err := os.ReadFile(store.Path)
				if err != nil {
					t.Fatal(err)
				}
				if !allowed || current == proposed {
					if !bytes.Equal(before, after) {
						t.Fatal("rejected or historical unavailable no-op rewrote YAML", current, proposed)
					}
				} else {
					values["ReportOptions"].(map[string]any)["LVUnitGroups"] = proposed
				}
				actual, err := store.Snapshot()
				if err != nil || !reflect.DeepEqual(values, actual) {
					t.Fatal("unit Save changed enforcement/sibling/unrelated options or lost type", actual, err)
				}
			}
		}
	}
	vprotest.AssertProfileSUFiles(t, contexts, beforeDB)
}

func TestLongVegetationUnitPreferencesMalformedStorageConflictAndRecovery(t *testing.T) {
	contexts, state := vprotest.ReportServiceFixture(t, false)
	service := NewLongVegetationUnitPreferencesService(contexts, true)
	contexts.LongVegetationNoneUnitsEnabled = true
	store := contexts.Projects.Preferences
	for _, bad := range []any{nil, true, "1", 0, 4} {
		if err := store.Update("ReportOptions", map[string]any{"LVUnitGroups": bad}); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(store.Path)
		if err != nil {
			t.Fatal(err)
		}
		if value, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID); err == nil || value != nil {
			t.Fatal("malformed storage guessed unit choice", bad, value, err)
		}
		result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3))
		if result.Committed || result.Changed || result.ErrorMessage == "" {
			t.Fatal("invalid original storage overwritten", bad, result)
		}
		after, err := os.ReadFile(store.Path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("invalid-storage refusal rewrote config", err)
		}
	}
	if err := store.Update("ReportOptions", map[string]any{"LVUnitGroups": 1}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	malformed := bytes.Replace(before, []byte("LVUnitGroups: 1"), []byte("LVUnitGroups: 1.0"), 1)
	if bytes.Equal(before, malformed) {
		t.Fatal("floating storage fixture was not changed")
	}
	if err := os.WriteFile(store.Path, malformed, 0600); err != nil {
		t.Fatal(err)
	}
	if value, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID); value != nil || err == nil {
		t.Fatal("actual floating YAML storage silently normalized", value, err)
	}
	if err := os.WriteFile(store.Path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Update("ReportOptions", map[string]any{"LVUnitGroups": 3}); err != nil {
		t.Fatal(err)
	}
	result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 1))
	if result.Committed || result.Changed || !strings.Contains(result.ErrorMessage, "changed") {
		t.Fatal("stale expected choice overwrote external/source callback change", result)
	}
	result = service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 3, 1))
	if !result.Committed || !result.Changed || result.ErrorMessage != "" {
		t.Fatal("owned correction/retry failed", result)
	}
}
