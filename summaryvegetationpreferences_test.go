package main

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func summaryPreferenceFixture(t *testing.T, value summaryVegetationPreferenceValues) *desktopConfig {
	t.Helper()
	dataDir, configDir := configFixture(t)
	store, err := openDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.update("ReportOptions", map[string]any{
		"SVGroupBy": value.GroupBy, "SVAvgType": value.Average, "SVValue": value.Display,
		"SVShowClass": value.ShowClass, "SVShowValOpt": value.ValueLimit, "untouched": "retain",
	}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSummaryVegetationPreferenceLeafTransitionsAndOmissions(t *testing.T) {
	for _, group := range []int{1, 2, 3, 4} {
		for _, average := range []int{1, 2} {
			for _, display := range []int{1, 2, 3, 4, 5, 6} {
				for _, show := range []int{1, 10, 20} {
					original := summaryVegetationPreferenceValues{GroupBy: group, Average: average, Display: display, ShowClass: show, ValueLimit: 3.5}
					store := summaryPreferenceFixture(t, original)
					before, err := os.ReadFile(store.path)
					if err != nil {
						t.Fatal(err)
					}
					read, err := store.readSummaryVegetationPreferences(context.Background())
					if err != nil || read != original {
						t.Fatal(original, read, err)
					}
					result, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), original, original)
					after, readErr := os.ReadFile(store.path)
					if err != nil || readErr != nil || result.Committed || result.Changed || !bytes.Equal(before, after) {
						t.Fatal("historical unchanged value/unsupported combination was rewritten", original, result, err, readErr)
					}
					proposed := original
					proposed.GroupBy, proposed.Display, proposed.ValueLimit = 4, 6, 0
					result, err = store.compareAndSetSummaryVegetationPreferences(context.Background(), original, proposed)
					if err != nil || !result.Committed || !result.Changed {
						t.Fatal(result, err)
					}
					reopened := &desktopConfig{path: store.path}
					got, err := reopened.readSummaryVegetationPreferences(context.Background())
					if err != nil || got != proposed {
						t.Fatal(got, proposed, err)
					}
					values, err := reopened.snapshot()
					if err != nil {
						t.Fatal(err)
					}
					section, err := configSection(values, "ReportOptions")
					if err != nil || section["untouched"] != "retain" {
						t.Fatal("other preference changed", err)
					}
					stale, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), original, proposed)
					if err == nil || stale.Committed {
						t.Fatal("stale CAS accepted")
					}
				}
			}
		}
	}
}

func TestSummaryVegetationPreferenceLeafCallbackClearAndRefusals(t *testing.T) {
	base := summaryVegetationPreferenceValues{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1, ValueLimit: 2.25}
	for _, display := range []int{1, 2, 3, 4, 5, 6} {
		store := summaryPreferenceFixture(t, base)
		next := base
		next.Display = display
		if display != 1 && display != 3 {
			next.ValueLimit = 0
		}
		if _, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), base, next); err != nil {
			t.Fatal(display, err)
		}
	}
	store := summaryPreferenceFixture(t, base)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []summaryVegetationPreferenceValues{
		{GroupBy: 2, Average: 1, Display: 2, ShowClass: 1},
		{GroupBy: 1, Average: 1, Display: 4, ShowClass: 1, ValueLimit: 2.25},
		{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1, ValueLimit: 4},
		{GroupBy: 1, Average: 1, Display: 1, ShowClass: 2},
		{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1, ValueLimit: math.NaN()},
	} {
		got, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), base, bad)
		if err == nil || got.Committed {
			t.Fatal("invalid saved proposal accepted", bad, got, err)
		}
	}
	after, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("invalid proposal wrote config", err)
	}
	// An away/back display callback can clear the limit while final display is unchanged.
	next := base
	next.ValueLimit = 0
	if got, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), base, next); err != nil || !got.Committed {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.readSummaryVegetationPreferences(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got, err := store.compareAndSetSummaryVegetationPreferences(ctx, next, base); err == nil || got.Committed {
		t.Fatal(got, err)
	}
}

func TestSummaryVegetationPreferenceLeafReplacementFailureRetry(t *testing.T) {
	base := summaryVegetationPreferenceValues{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1}
	store := summaryPreferenceFixture(t, base)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	next := base
	next.GroupBy = 4
	store.replace = func(string, string) error { return errors.New("owned fixture replacement refusal") }
	got, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), base, next)
	after, readErr := os.ReadFile(store.path)
	if err == nil || got.Committed || readErr != nil || !bytes.Equal(before, after) {
		t.Fatal(got, err, readErr)
	}
	staged, err := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if err != nil || len(staged) != 0 {
		t.Fatal("staging cleanup lost", staged, err)
	}
	store.replace = os.Rename
	if got, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), base, next); err != nil || !got.Committed {
		t.Fatal(got, err)
	}

	for _, field := range []string{"SVGroupBy", "SVAvgType", "SVValue", "SVShowClass", "SVShowValOpt"} {
		invalidStore := summaryPreferenceFixture(t, base)
		if err := invalidStore.update("ReportOptions", map[string]any{field: nil}); err != nil {
			t.Fatal(err)
		}
		got, err := invalidStore.readSummaryVegetationPreferences(context.Background())
		if err == nil || !reflect.DeepEqual(got, summaryVegetationPreferenceValues{}) {
			t.Fatal("NULL implicit default accepted", field, got, err)
		}
	}
}

func TestSummaryVegetationPreferenceLeafCommittedCancellationReceipt(t *testing.T) {
	base := summaryVegetationPreferenceValues{GroupBy: 1, Average: 1, Display: 1, ShowClass: 1}
	store := summaryPreferenceFixture(t, base)
	next := base
	next.GroupBy = 4
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(source, destination string) error {
		if err := os.Rename(source, destination); err != nil {
			return err
		}
		cancel()
		return nil
	}
	got, err := store.compareAndSetSummaryVegetationPreferences(ctx, base, next)
	if !got.Committed || !got.Changed || !errors.Is(err, context.Canceled) {
		t.Fatal("committed cancellation receipt lost", got, err)
	}
	reopened := &desktopConfig{path: store.path, replace: os.Rename}
	read, err := reopened.readSummaryVegetationPreferences(context.Background())
	if err != nil || read != next {
		t.Fatal(read, err)
	}
	if got, err := reopened.compareAndSetSummaryVegetationPreferences(context.Background(), base, next); err == nil || got.Committed {
		t.Fatal("committed stale change replayed", got, err)
	}
}

func TestSummaryVegetationPreferenceLeafUnrelatedEditsRetainHistoricalLimit(t *testing.T) {
	for _, display := range []int{4, 5, 6} {
		for _, edit := range []string{"average", "show"} {
			for _, integerStorage := range []bool{false, true} {
				base := summaryVegetationPreferenceValues{GroupBy: 4, Average: 1, Display: display, ShowClass: 1, ValueLimit: 7.25}
				store := summaryPreferenceFixture(t, base)
				if integerStorage {
					base.ValueLimit = 7
					if err := store.update("ReportOptions", map[string]any{"SVShowValOpt": 7}); err != nil {
						t.Fatal(err)
					}
				}
				before, err := store.snapshot()
				if err != nil {
					t.Fatal(err)
				}
				expectedSection, err := configSection(before, "ReportOptions")
				if err != nil {
					t.Fatal(err)
				}
				proposed := base
				if edit == "average" {
					proposed.Average = 2
					expectedSection["SVAvgType"] = 2
				} else {
					proposed.ShowClass = 20
					expectedSection["SVShowClass"] = 20
				}
				got, err := store.compareAndSetSummaryVegetationPreferences(context.Background(), base, proposed)
				if err != nil || !got.Committed || !got.Changed {
					t.Fatal(display, edit, integerStorage, got, err)
				}
				reopened := &desktopConfig{path: store.path, replace: os.Rename}
				read, err := reopened.readSummaryVegetationPreferences(context.Background())
				if err != nil || read != proposed {
					t.Fatal("unrelated edit cleared historical nonzero limit", read, proposed, err)
				}
				after, err := reopened.snapshot()
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("unrelated edit assigned another key or changed integer limit storage", display, edit, integerStorage, err)
				}
			}
		}
	}
}
