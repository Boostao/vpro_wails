package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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

func summaryQualityFixture(t *testing.T) *vprocore.DesktopConfig {
	t.Helper()
	data, config := vprotest.ConfigFixture(t)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]any{
		"DataQualityFilterEnforce": false, "DataQualityFilterSite": "Poor", "DataQualityFilterVeg": "Poor",
		"DataQualityFilterSoil": "Poor", "DataQualityFilterBECUseLV": "True",
		"DataQualityFilterSiteNull": true, "DataQualityFilterVegNull": "True",
		"DataQualityFilterSoilNull": "True", "DataQualityFilterBECUse": 1,
	}
	if err := store.Update("ReportOptions", fields); err != nil {
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
	read, err := ReadSummaryVegetationQualityPreferences(store, ctx)
	if err != nil || read != expected {
		t.Fatal(read, err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, ctx, expected, expected); err != nil || result != (vprocore.ReportPreferenceUpdate{}) {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("no-op rewrote legacy storage", err)
	}
	original, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	proposed := expected
	proposed.SiteMinimum = "  literal nonmember longer than 15  "
	proposed.BECMinimum = ""
	proposed.BECIncludeNulls = false
	result, err := CompareAndSetSummaryVegetationQualityPreferences(store, ctx, expected, proposed)
	if err != nil || !result.Committed || !result.Changed {
		t.Fatal(result, err)
	}
	reopened, err := vprocore.OpenDesktopConfig(filepath.Dir(filepath.Dir(store.Path)), filepath.Dir(store.Path))
	if err != nil {
		t.Fatal(err)
	}
	read, err = ReadSummaryVegetationQualityPreferences(reopened, ctx)
	if err != nil || read != proposed {
		t.Fatal("independent reopen changed literal values", read, err)
	}
	persisted, err := reopened.Snapshot()
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
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, ctx, expected, proposed); err == nil || result.Committed || !strings.Contains(err.Error(), "changed") {
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
			value, err := summaryVegetationQualityPreferenceOption(vprocore.ConfigValues{"ReportOptions": map[string]any{key: test.value}}, key)
			if (err == nil) != test.valid || test.valid && value != test.want {
				t.Fatal(key, test, value, err)
			}
		}
	}
	if value, err := summaryVegetationQualityPreferenceOption(vprocore.ConfigValues{}, "LVGroupBy"); value != nil || err == nil {
		t.Fatal(value, err)
	}
}

func TestSummaryQualityPrivateRefusalRollbackCancellationAndCommittedReceipt(t *testing.T) {
	store := summaryQualityFixture(t)
	expected, err := ReadSummaryVegetationQualityPreferences(store, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"bad\x00new", string([]byte{0xff})} {
		proposed := expected
		proposed.VegMinimum = text
		if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, context.Background(), expected, proposed); err == nil || result.Committed {
			t.Fatal(result, err)
		}
	}
	proposed := expected
	proposed.Enforce = true
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, context.Background(), expected, proposed); err == nil || result.Committed {
		t.Fatal("mutating Load enforcement invented", result, err)
	}
	proposed = expected
	proposed.VegIncludeNulls = false
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, ctx, expected, proposed); !errors.Is(err, context.Canceled) || result.Committed {
		t.Fatal(result, err)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("replacement collision") }
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, context.Background(), expected, proposed); err == nil || result.Committed {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("refusal changed config or retained stages", err, stageErr)
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
	result, err := CompareAndSetSummaryVegetationQualityPreferences(store, ctx, expected, proposed)
	store.Replace = replace
	if !result.Committed || !result.Changed || err == nil || !strings.Contains(err.Error(), "do not replay") {
		t.Fatal(result, err)
	}
	read, err := ReadSummaryVegetationQualityPreferences(store, context.Background())
	if err != nil || read != proposed {
		t.Fatal(read, err)
	}
}

func TestSummaryQualityPrivateHistoricalNULOmissionAndCorrection(t *testing.T) {
	store := summaryQualityFixture(t)
	if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterSite": "old\x00literal"}); err != nil {
		t.Fatal(err)
	}
	expected, err := ReadSummaryVegetationQualityPreferences(store, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	proposed := expected
	proposed.SoilIncludeNulls = false
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, context.Background(), expected, proposed); err != nil || !result.Committed {
		t.Fatal(result, err)
	}
	if read, err := ReadSummaryVegetationQualityPreferences(store, context.Background()); err != nil || read != proposed {
		t.Fatal(read, err)
	}
	corrected := proposed
	corrected.SiteMinimum = "Good"
	if result, err := CompareAndSetSummaryVegetationQualityPreferences(store, context.Background(), proposed, corrected); err != nil || !result.Committed {
		t.Fatal(result, err)
	}
}
