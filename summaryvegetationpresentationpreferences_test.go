package main

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func summaryPresentationFixture(t *testing.T, values map[string]any) *desktopConfig {
	t.Helper()
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.update("ReportOptions", values); err != nil {
		t.Fatal(err)
	}
	return store
}

func summaryPresentationBytes(t *testing.T, store *desktopConfig) []byte {
	t.Helper()
	value, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestSummaryVegetationPresentationDefaultsAnd128HistoricalStates(t *testing.T) {
	store := summaryPresentationFixture(t, nil)
	got, err := store.readSummaryVegetationPresentationPreferences(context.Background())
	defaults := summaryVegetationPresentationPreferenceValues{Title: "Short veg report title", CreateSummary: true, GroupBy: 1, Display: 1}
	if err != nil || got != defaults {
		t.Fatal("independent source defaults", got, err)
	}
	keys := []string{"SVShowEnglishName", "SVShowGuideName", "SVShowHierarchyHeader", "SVIncludeEnvFields",
		"SVReportSummary", "SVSubSppAsSpp", "SVConstancyFormat"}
	for state := 0; state < 128; state++ {
		values := map[string]any{"SVGroupBy": 4, "SVValue": 6, "SVShowValOpt": 7.25,
			"SVReportTitle": " historical \x00 title ", "unrelated": []any{"retain", 42, nil}}
		flags := [7]bool{}
		for index, key := range keys {
			flags[index] = state&(1<<index) != 0
			if index%2 == 0 {
				values[key] = flags[index]
			} else if flags[index] {
				values[key] = -1
			} else {
				values[key] = 0
			}
		}
		store := summaryPresentationFixture(t, values)
		expected := summaryVegetationPresentationPreferenceValues{
			Title: " historical \x00 title ", ShowCommon: flags[0], ShowGuide: flags[1],
			HierarchyHeader: flags[2], IncludeEnv: flags[3], CreateSummary: flags[4],
			CombineVariants: flags[5], Constancy: flags[6], GroupBy: 4, Display: 6, ValueLimit: 7.25,
		}
		read, err := store.readSummaryVegetationPresentationPreferences(context.Background())
		if err != nil || read != expected {
			t.Fatal(state, read, expected, err)
		}
		before := summaryPresentationBytes(t, store)
		result, err := store.compareAndSetSummaryVegetationPresentationPreferences(context.Background(), expected, expected, false)
		if err != nil || result.Committed || result.Changed || !bytes.Equal(before, summaryPresentationBytes(t, store)) {
			t.Fatal("historical inconsistent state/no-op rewritten", state, result, err)
		}
		proposed := expected
		proposed.Title = " updated literal " + strings.Repeat("long", 256)
		proposed.ShowGuide = !expected.ShowGuide
		proposed.IncludeEnv = !expected.IncludeEnv
		result, err = store.compareAndSetSummaryVegetationPresentationPreferences(context.Background(), expected, proposed, false)
		if err != nil || !result.Changed || !result.Committed {
			t.Fatal("unrelated edit silently normalized a historical constancy combination", state, result, err)
		}
		reopened := &desktopConfig{path: store.path}
		read, err = reopened.readSummaryVegetationPresentationPreferences(context.Background())
		if err != nil || read != proposed {
			t.Fatal("independent reopened values", state, read, err)
		}
		full, err := reopened.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		section, err := configSection(full, "ReportOptions")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{}
		for key, value := range values {
			want[key] = value
		}
		want["SVReportTitle"], want["SVShowGuideName"], want["SVIncludeEnvFields"] =
			proposed.Title, proposed.ShowGuide, proposed.IncludeEnv
		for key, value := range want {
			if !reflect.DeepEqual(section[key], value) {
				t.Fatal("original storage/unrelated value changed", state, key, section[key], value)
			}
		}
	}
}

func TestSummaryVegetationPresentationConstancySourceCallbackPlan(t *testing.T) {
	ctx := context.Background()
	for _, finalConstancy := range []bool{false, true} {
		for _, finalCommon := range []bool{false, true} {
			store := summaryPresentationFixture(t, map[string]any{
				"SVGroupBy": 4, "SVValue": 6, "SVShowValOpt": 7.25, "SVShowEnglishName": -1,
			})
			expected, err := store.readSummaryVegetationPresentationPreferences(ctx)
			if err != nil {
				t.Fatal(err)
			}
			proposed := expected
			proposed.Constancy, proposed.ShowCommon, proposed.GroupBy, proposed.Display =
				finalConstancy, finalCommon, 3, 1
			before := summaryPresentationBytes(t, store)
			got, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, true)
			if finalConstancy && finalCommon {
				if err == nil || got.Committed || !bytes.Equal(before, summaryPresentationBytes(t, store)) {
					t.Fatal("common names accepted while constancy control disabled", got, err)
				}
				continue
			}
			if err != nil || !got.Committed || !got.Changed {
				t.Fatal("constancy-on/off retained forced fields", finalConstancy, finalCommon, got, err)
			}
			reopened, err := (&desktopConfig{path: store.path}).readSummaryVegetationPresentationPreferences(ctx)
			if err != nil || reopened != proposed || reopened.ValueLimit != 7.25 {
				t.Fatal("Display1 callback cleared limit or toggling off restored prior fields", reopened, err)
			}
			off := proposed
			off.Constancy = false
			if _, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, proposed, off, false); err != nil {
				t.Fatal("constancy-off must not force or restore fields", err)
			}
		}
	}
	store := summaryPresentationFixture(t, nil)
	expected, err := store.readSummaryVegetationPresentationPreferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before := summaryPresentationBytes(t, store)
	for _, change := range []func(*summaryVegetationPresentationPreferenceValues){
		func(v *summaryVegetationPresentationPreferenceValues) { v.Constancy = true },
		func(v *summaryVegetationPresentationPreferenceValues) { v.GroupBy = 3 },
		func(v *summaryVegetationPresentationPreferenceValues) { v.Display = 4 },
		func(v *summaryVegetationPresentationPreferenceValues) { v.ValueLimit = 1 },
		func(v *summaryVegetationPresentationPreferenceValues) { v.ValueLimit = math.NaN() },
		func(v *summaryVegetationPresentationPreferenceValues) { v.ValueLimit = math.Inf(1) },
	} {
		proposed := expected
		change(&proposed)
		result, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, false)
		if err == nil || result.Committed {
			t.Fatal("shared/coupled mutation accepted without source callback plan", proposed, result, err)
		}
	}
	for _, change := range []func(*summaryVegetationPresentationPreferenceValues){
		func(v *summaryVegetationPresentationPreferenceValues) { v.GroupBy = 1 },
		func(v *summaryVegetationPresentationPreferenceValues) { v.Display = 2 },
		func(v *summaryVegetationPresentationPreferenceValues) { v.Constancy, v.ShowCommon = true, true },
	} {
		proposed := expected
		proposed.GroupBy, proposed.Display, proposed.Constancy = 3, 1, true
		change(&proposed)
		result, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, true)
		if err == nil || result.Committed {
			t.Fatal("incomplete source callback plan accepted", proposed, result, err)
		}
	}
	if !bytes.Equal(before, summaryPresentationBytes(t, store)) {
		t.Fatal("rejected callback plans wrote configuration")
	}
}

func TestSummaryVegetationPresentationTitleLiteralCorrectionAndRefusal(t *testing.T) {
	ctx := context.Background()
	for _, title := range []string{"", "  literal  ", "lowerCase", "O'Brien", "日本語😀", strings.Repeat("x", 10000)} {
		store := summaryPresentationFixture(t, map[string]any{"SVReportTitle": "historical\x00title"})
		expected, err := store.readSummaryVegetationPresentationPreferences(ctx)
		if err != nil {
			t.Fatal(err)
		}
		proposed := expected
		proposed.Title = title
		got, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, false)
		if err != nil || !got.Committed || !got.Changed {
			t.Fatal("literal title normalized or invented bounds", got, err)
		}
		reopened, err := (&desktopConfig{path: store.path}).readSummaryVegetationPresentationPreferences(ctx)
		if err != nil || reopened.Title != title {
			t.Fatal(reopened, err)
		}
	}
	store := summaryPresentationFixture(t, nil)
	expected, err := store.readSummaryVegetationPresentationPreferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before := summaryPresentationBytes(t, store)
	for _, title := range []string{"new\x00title", string([]byte{0xff}), string([]byte{0xed, 0xa0, 0x80})} {
		proposed := expected
		proposed.Title = title
		got, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, false)
		if err == nil || got.Committed {
			t.Fatal("new malformed title accepted", got, err)
		}
	}
	if !bytes.Equal(before, summaryPresentationBytes(t, store)) {
		t.Fatal("invalid title wrote configuration")
	}
}

func TestSummaryVegetationPresentationSharedKeyCollisionAndStorageRefusal(t *testing.T) {
	ctx := context.Background()
	changes := map[string]any{
		"SVReportTitle": "external", "SVShowEnglishName": -1, "SVShowGuideName": -1,
		"SVShowHierarchyHeader": -1, "SVIncludeEnvFields": -1, "SVReportSummary": 0,
		"SVSubSppAsSpp": -1, "SVConstancyFormat": -1, "SVGroupBy": 4, "SVValue": 6, "SVShowValOpt": 9,
	}
	for key, external := range changes {
		store := summaryPresentationFixture(t, nil)
		expected, err := store.readSummaryVegetationPresentationPreferences(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.update("ReportOptions", map[string]any{key: external}); err != nil {
			t.Fatal(err)
		}
		before := summaryPresentationBytes(t, store)
		proposed := expected
		proposed.Title = "candidate"
		got, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, false)
		if err == nil || got.Committed || err.Error() != "Summary Vegetation presentation preferences changed; reload the saved values before updating" ||
			!bytes.Equal(before, summaryPresentationBytes(t, store)) {
			t.Fatal("all eleven keys require stale CAS including shared options", key, got, err)
		}
	}
	for _, key := range []string{"SVShowEnglishName", "SVShowGuideName", "SVShowHierarchyHeader", "SVIncludeEnvFields",
		"SVReportSummary", "SVSubSppAsSpp", "SVConstancyFormat"} {
		for _, invalid := range []any{nil, "True", "False", 1, 2, 0.5, []any{false}} {
			store := summaryPresentationFixture(t, map[string]any{key: invalid})
			if _, err := store.readSummaryVegetationPresentationPreferences(ctx); err == nil {
				t.Fatal("invented coercion accepted for presentation Boolean", key, invalid)
			}
		}
	}
	for _, key := range []string{"SVReportTitle", "SVGroupBy", "SVValue", "SVShowValOpt"} {
		store := summaryPresentationFixture(t, map[string]any{key: nil})
		if _, err := store.readSummaryVegetationPresentationPreferences(ctx); err == nil {
			t.Fatal("NULL substituted with default", key)
		}
	}
}

func TestSummaryVegetationPresentationReplacementCancellationRetryAndOtherReaders(t *testing.T) {
	ctx := context.Background()
	store := summaryPresentationFixture(t, nil)
	expected, err := store.readSummaryVegetationPresentationPreferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	proposed := expected
	proposed.Title, proposed.Constancy, proposed.ShowCommon, proposed.GroupBy, proposed.Display =
		"new", true, false, 3, 1
	before := summaryPresentationBytes(t, store)
	store.replace = func(string, string) error { return errors.New("physical fixture refuses replacement") }
	got, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, true)
	if err == nil || got.Committed || !bytes.Equal(before, summaryPresentationBytes(t, store)) {
		t.Fatal("failed persistence reported success", got, err)
	}
	staged, err := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if err != nil || len(staged) != 0 {
		t.Fatal("failed persistence staging remains", staged, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.readSummaryVegetationPresentationPreferences(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got, err := store.compareAndSetSummaryVegetationPresentationPreferences(cancelled, expected, proposed, true); !errors.Is(err, context.Canceled) || got.Committed {
		t.Fatal(got, err)
	}
	store.replace = os.Rename
	got, err = store.compareAndSetSummaryVegetationPresentationPreferences(ctx, expected, proposed, true)
	if err != nil || !got.Committed || !got.Changed {
		t.Fatal("retry did not commit complete source plan", got, err)
	}
	options, err := store.readSummaryVegetationPreferences(ctx)
	if err != nil || options.GroupBy != 3 || options.Display != 1 || options.ValueLimit != 0 {
		t.Fatal("shared reader missed actual callback plan", options, err)
	}
	noop, err := store.compareAndSetSummaryVegetationPresentationPreferences(ctx, proposed, proposed, false)
	if err != nil || noop.Committed || noop.Changed {
		t.Fatal("retry repeated committed mutation", noop, err)
	}
	if _, err := (*desktopConfig)(nil).readSummaryVegetationPresentationPreferences(ctx); err == nil {
		t.Fatal("missing config accepted")
	}
	if _, err := store.readSummaryVegetationPresentationPreferences(nil); err == nil {
		t.Fatal("missing context accepted")
	}
}
