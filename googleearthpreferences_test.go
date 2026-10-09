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

	"gopkg.in/yaml.v3"
)

func TestGoogleEarthPreferencesStrictReadAndNoWrites(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.readGoogleEarthPreferences(context.Background())
	if err != nil || got != (googleEarthPreferences{"VPro Plot Locations", "PlotRepresenting"}) {
		t.Fatalf("source defaults: %+v %v", got, err)
	}
	for _, values := range []configValues{
		{}, {"ReportOptions": nil}, {"ReportOptions": map[string]any{}},
		{"ReportOptions": map[string]any{"GoogleEarthPlaceName": nil, "GoogleEarthDescField": "Zone"}},
		{"ReportOptions": map[string]any{"GoogleEarthPlaceName": "Title", "GoogleEarthDescField": true}},
	} {
		if value, err := decodeGoogleEarthPreferences(values); err == nil || value != (googleEarthPreferences{}) {
			t.Fatalf("invalid configuration silently defaulted: %+v %v", value, err)
		}
	}
	after, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("preference reads changed configuration", err)
	}
}

func TestGoogleEarthPreferencesCASPreservationRetryAndHistoricalValues(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	original, err := store.readGoogleEarthPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	proposed := googleEarthPreferences{" Literal & <name> \U0001f332\r\n ", " Zone "}
	outcome, err := store.compareAndSetGoogleEarthPreferences(context.Background(), original, proposed)
	if err != nil || !outcome.Committed || !outcome.Changed {
		t.Fatalf("change: %+v %v", outcome, err)
	}
	after, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	before["ReportOptions"].(map[string]any)["GoogleEarthPlaceName"] = proposed.PlaceName
	before["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = proposed.DescriptionField
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated configuration changed or literal values repaired")
	}
	if result, err := store.compareAndSetGoogleEarthPreferences(context.Background(), original, proposed); err == nil || result.Committed {
		t.Fatal("stale original/replay accepted", result, err)
	}
	bytesBefore, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := store.compareAndSetGoogleEarthPreferences(context.Background(), proposed, proposed); err != nil || result.Changed || result.Committed {
		t.Fatal("unchanged preferences committed", result, err)
	}
	bytesAfter, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(bytesBefore, bytesAfter) {
		t.Fatal("no-op reformatted historical YAML", err)
	}
	for _, invalid := range []googleEarthPreferences{
		{"new\x00name", proposed.DescriptionField}, {"new\x01name", proposed.DescriptionField},
		{string([]byte{0xff}), proposed.DescriptionField}, {proposed.PlaceName, ""},
		{proposed.PlaceName, "Zone\x00"}, {proposed.PlaceName, string([]byte{0xff})},
	} {
		if result, err := store.compareAndSetGoogleEarthPreferences(context.Background(), proposed, invalid); err == nil || result.Committed {
			t.Fatal("new invalid preference accepted", result, err)
		}
	}
	historical := googleEarthPreferences{"historical\x01title", "historical\x00field"}
	after["ReportOptions"].(map[string]any)["GoogleEarthPlaceName"] = historical.PlaceName
	after["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = historical.DescriptionField
	raw, err := yaml.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	changed := historical
	changed.DescriptionField = "Zone"
	if result, err := store.compareAndSetGoogleEarthPreferences(context.Background(), historical, changed); err != nil || !result.Committed {
		t.Fatal("unchanged historical invalid title was repaired/rejected", result, err)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("owned replacement failure") }
	retry := changed
	retry.PlaceName = "Corrected title"
	if result, err := store.compareAndSetGoogleEarthPreferences(context.Background(), changed, retry); err == nil || result.Committed {
		t.Fatal("failed replacement looked committed", result, err)
	}
	store.replace = replace
	if result, err := store.compareAndSetGoogleEarthPreferences(context.Background(), changed, retry); err != nil || !result.Committed {
		t.Fatal("failure did not permit explicit retry", result, err)
	}
	if files, err := filepath.Glob(filepath.Join(config, ".config-*.yml")); err != nil || len(files) != 0 {
		t.Fatal("configuration staging leaked", files, err)
	}
}

func TestGoogleEarthPreferencesCancellationAndCommittedOutcome(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	original, err := store.readGoogleEarthPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	proposed := original
	proposed.PlaceName = "Changed"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, requestCtx := range []context.Context{nil, ctx} {
		if _, err := store.readGoogleEarthPreferences(requestCtx); err == nil {
			t.Fatal("invalid read context accepted")
		}
		if result, err := store.compareAndSetGoogleEarthPreferences(requestCtx, original, proposed); err == nil || result.Committed {
			t.Fatal("invalid write context accepted", result, err)
		}
	}
	store.mu.Lock()
	queued, cancelQueued := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := store.compareAndSetGoogleEarthPreferences(queued, original, proposed)
		done <- err
	}()
	cancelQueued()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("queued cancellation lost", err)
	}
	store.mu.Unlock()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	replace := store.replace
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	result, err := store.compareAndSetGoogleEarthPreferences(ctx, original, proposed)
	if !result.Committed || !result.Changed || !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "do not replay") {
		t.Fatal("postcommit cancellation erased irreversible preference result", result, err)
	}
	got, err := store.readGoogleEarthPreferences(context.Background())
	if err != nil || got != proposed {
		t.Fatal("committed preferences absent", got, err)
	}
}
