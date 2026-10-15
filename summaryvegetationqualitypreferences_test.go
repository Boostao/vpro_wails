package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func summaryQualityFixture(t *testing.T) *desktopConfig {
	t.Helper()
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]any{
		"DataQualityFilterEnforce": false, "DataQualityFilterSite": "Poor", "DataQualityFilterVeg": "Poor",
		"DataQualityFilterSoil": "Poor", "DataQualityFilterBECUseLV": "True",
		"DataQualityFilterSiteNull": true, "DataQualityFilterVegNull": "True",
		"DataQualityFilterSoilNull": "True", "DataQualityFilterBECUse": 1,
	}
	if err := store.update("ReportOptions", fields); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSummaryQualityPrivateDefaultsOmissionSharedCASAndReopen(t *testing.T) {
	store := summaryQualityFixture(t)
	ctx := context.Background()
	expected := summaryVegetationQualityPreferenceValues{
		SiteMinimum: "Poor", VegMinimum: "Poor", SoilMinimum: "Poor", BECMinimum: "True",
		SiteIncludeNulls: true, VegIncludeNulls: true, SoilIncludeNulls: true, BECIncludeNulls: true,
	}
	read, err := store.readSummaryVegetationQualityPreferences(ctx)
	if err != nil || read != expected {
		t.Fatal(read, err)
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(ctx, expected, expected); err != nil || result != (reportPreferenceUpdate{}) {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("no-op rewrote legacy storage", err)
	}
	original, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	proposed := expected
	proposed.SiteMinimum = "  literal nonmember longer than 15  "
	proposed.BECMinimum = ""
	proposed.BECIncludeNulls = false
	result, err := store.compareAndSetSummaryVegetationQualityPreferences(ctx, expected, proposed)
	if err != nil || !result.Committed || !result.Changed {
		t.Fatal(result, err)
	}
	reopened, err := openDesktopConfig(filepath.Dir(filepath.Dir(store.path)), filepath.Dir(store.path))
	if err != nil {
		t.Fatal(err)
	}
	read, err = reopened.readSummaryVegetationQualityPreferences(ctx)
	if err != nil || read != proposed {
		t.Fatal("independent reopen changed literal values", read, err)
	}
	persisted, err := reopened.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	fields := original["ReportOptions"].(map[string]any)
	fields["DataQualityFilterSite"] = proposed.SiteMinimum
	fields["DataQualityFilterBECUseLV"] = ""
	fields["DataQualityFilterBECUse"] = false
	if !reflect.DeepEqual(original, persisted) {
		t.Fatal("unrelated/legacy storage changed", original, persisted)
	}
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(ctx, expected, proposed); err == nil || result.Committed || !strings.Contains(err.Error(), "changed") {
		t.Fatal("shared expected values authorized stale Save", result, err)
	}
}

func TestSummaryQualityPrivateBooleanStorageAndInvalidTypes(t *testing.T) {
	for _, key := range []string{"DataQualityFilterEnforce", "DataQualityFilterSiteNull", "DataQualityFilterVegNull", "DataQualityFilterSoilNull", "DataQualityFilterBECUse"} {
		for _, test := range []struct {
			value any
			want  bool
			valid bool
		}{
			{true, true, true}, {false, false, true}, {-1, true, true}, {0, false, true},
			{1, true, key == "DataQualityFilterBECUse"},
			{"True", true, key != "DataQualityFilterEnforce"}, {"False", false, key != "DataQualityFilterEnforce"},
			{"true", false, false}, {" True ", false, false}, {nil, false, false},
			{2, false, false}, {1.0, false, false}, {[]any{1}, false, false}, {map[string]any{"a": 1}, false, false},
		} {
			value, err := summaryVegetationQualityPreferenceOption(configValues{"ReportOptions": map[string]any{key: test.value}}, key)
			if (err == nil) != test.valid || test.valid && value != test.want {
				t.Fatal(key, test, value, err)
			}
		}
	}
	if value, err := summaryVegetationQualityPreferenceOption(configValues{}, "LVGroupBy"); value != nil || err == nil {
		t.Fatal(value, err)
	}
}

func TestSummaryQualityPrivateRefusalRollbackCancellationAndCommittedReceipt(t *testing.T) {
	store := summaryQualityFixture(t)
	expected, err := store.readSummaryVegetationQualityPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"bad\x00new", string([]byte{0xff})} {
		proposed := expected
		proposed.VegMinimum = text
		if result, err := store.compareAndSetSummaryVegetationQualityPreferences(context.Background(), expected, proposed); err == nil || result.Committed {
			t.Fatal(result, err)
		}
	}
	proposed := expected
	proposed.Enforce = true
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(context.Background(), expected, proposed); err == nil || result.Committed {
		t.Fatal("mutating Load enforcement invented", result, err)
	}
	proposed = expected
	proposed.VegIncludeNulls = false
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(ctx, expected, proposed); !errors.Is(err, context.Canceled) || result.Committed {
		t.Fatal(result, err)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("replacement collision") }
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(context.Background(), expected, proposed); err == nil || result.Committed {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("refusal changed config or retained stages", err, stageErr)
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
	result, err := store.compareAndSetSummaryVegetationQualityPreferences(ctx, expected, proposed)
	store.replace = replace
	if !result.Committed || !result.Changed || err == nil || !strings.Contains(err.Error(), "do not replay") {
		t.Fatal(result, err)
	}
	read, err := store.readSummaryVegetationQualityPreferences(context.Background())
	if err != nil || read != proposed {
		t.Fatal(read, err)
	}
}

func TestSummaryQualityPrivateHistoricalNULOmissionAndCorrection(t *testing.T) {
	store := summaryQualityFixture(t)
	if err := store.update("ReportOptions", map[string]any{"DataQualityFilterSite": "old\x00literal"}); err != nil {
		t.Fatal(err)
	}
	expected, err := store.readSummaryVegetationQualityPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	proposed := expected
	proposed.SoilIncludeNulls = false
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(context.Background(), expected, proposed); err != nil || !result.Committed {
		t.Fatal(result, err)
	}
	if read, err := store.readSummaryVegetationQualityPreferences(context.Background()); err != nil || read != proposed {
		t.Fatal(read, err)
	}
	corrected := proposed
	corrected.SiteMinimum = "Good"
	if result, err := store.compareAndSetSummaryVegetationQualityPreferences(context.Background(), proposed, corrected); err != nil || !result.Committed {
		t.Fatal(result, err)
	}
}
