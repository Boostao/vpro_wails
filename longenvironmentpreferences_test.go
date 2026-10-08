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
	"time"

	"gopkg.in/yaml.v3"
)

func TestLongEnvironmentPreferencesStrictLiteralReadAndNoWrites(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	title, err := store.readLongEnvironmentPreference(context.Background())
	if err != nil || title != "Long Evironment Report" {
		t.Fatal("literal source default was repaired", title, err)
	}
	after, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read wrote configuration", err)
	}
	for _, value := range []any{nil, true, 123, []any{"title"}} {
		values, err := store.snapshot()
		if err != nil {
			t.Fatal(err)
		}
		values["ReportOptions"].(map[string]any)["LEReportTitle"] = value
		raw, err := yaml.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.commit(raw); err != nil {
			t.Fatal(err)
		}
		if title, err := store.readLongEnvironmentPreference(context.Background()); err == nil || title != "" {
			t.Fatal("wrong-type/NULL setting became a default", title, err)
		}
	}
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	delete(values["ReportOptions"].(map[string]any), "LEReportTitle")
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	before, err = os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if title, err := store.readLongEnvironmentPreference(context.Background()); err == nil || title != "" {
		t.Fatal("missing setting became a default", title, err)
	}
	if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), "", "new"); err == nil || result.Committed {
		t.Fatal("missing setting authorized creation", result, err)
	}
	after, err = os.ReadFile(store.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("missing-setting refusal wrote config", err)
	}
}

func TestLongEnvironmentPreferencesCASNoopHistoricalAndRetry(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	original, err := store.readLongEnvironmentPreference(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	proposed := " Literal <&\U0001f332\r\n title \x01 "
	result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), original, proposed)
	if err != nil || !result.Committed || !result.Changed {
		t.Fatal(result, err)
	}
	after, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	before["ReportOptions"].(map[string]any)["LEReportTitle"] = proposed
	if !reflect.DeepEqual(before, after) {
		t.Fatal("title changed unrelated preferences or repaired literal text")
	}
	bytesBefore, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), proposed, proposed); err != nil || result.Changed || result.Committed {
		t.Fatal("no-op rewrote YAML", result, err)
	}
	if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), original, "stale"); err == nil || result.Committed {
		t.Fatal("stale expected title accepted", result, err)
	}
	for _, invalid := range []string{"bad\x00title", "\xff"} {
		if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), proposed, invalid); err == nil || result.Committed {
			t.Fatal("new malformed title accepted", result, err)
		}
	}
	bytesAfter, err := os.ReadFile(store.path)
	if err != nil || !bytes.Equal(bytesBefore, bytesAfter) {
		t.Fatal("no-op/stale/invalid refusal changed configuration", err)
	}
	historical := " historical\x00title "
	after["ReportOptions"].(map[string]any)["LEReportTitle"] = historical
	raw, err := yaml.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	if title, err := store.readLongEnvironmentPreference(context.Background()); err != nil || title != historical {
		t.Fatal("historical literal could not be explicitly loaded", title, err)
	}
	if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), historical, historical); err != nil || result.Committed {
		t.Fatal("unchanged historical literal was repaired/rejected", result, err)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("owned replacement failure") }
	if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), historical, "corrected"); err == nil || result.Committed {
		t.Fatal("failed replacement looked successful", result, err)
	}
	if title, err := store.readLongEnvironmentPreference(context.Background()); err != nil || title != historical {
		t.Fatal("failed replacement changed saved title", title, err)
	}
	store.replace = replace
	if result, err := store.compareAndSetLongEnvironmentPreference(context.Background(), historical, ""); err != nil || !result.Committed {
		t.Fatal("explicit empty correction/retry rejected", result, err)
	}
	if files, err := filepath.Glob(filepath.Join(config, ".config-*.yml")); err != nil || len(files) != 0 {
		t.Fatal("title staging leaked", files, err)
	}
}

func TestLongEnvironmentPreferencesCancellationAndCommittedReceipt(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	original, err := store.readLongEnvironmentPreference(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, cancelled} {
		if _, err := store.readLongEnvironmentPreference(ctx); err == nil {
			t.Fatal("invalid read context accepted")
		}
		if result, err := store.compareAndSetLongEnvironmentPreference(ctx, original, "new"); err == nil || result.Committed {
			t.Fatal("invalid mutation context accepted", result, err)
		}
	}
	store.mu.Lock()
	queued, cancelQueued := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, err := store.readLongEnvironmentPreference(queued)
		finished <- err
	}()
	cancelQueued()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			store.mu.Unlock()
			t.Fatal("queued cancellation lost identity", err)
		}
	case <-time.After(2 * time.Second):
		store.mu.Unlock()
		t.Fatal("queued preference read did not cancel")
	}
	store.mu.Unlock()
	ctx, cancelCommit := context.WithCancel(context.Background())
	defer cancelCommit()
	replace := store.replace
	store.replace = func(stage, destination string) error {
		if err := replace(stage, destination); err != nil {
			return err
		}
		cancelCommit()
		return nil
	}
	result, err := store.compareAndSetLongEnvironmentPreference(ctx, original, "committed")
	if !result.Committed || !result.Changed || !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "do not replay") {
		t.Fatal("postcommit cancellation lost receipt", result, err)
	}
	if title, err := store.readLongEnvironmentPreference(context.Background()); err != nil || title != "committed" {
		t.Fatal("committed title absent", title, err)
	}
	var missing *desktopConfig
	if _, err := missing.readLongEnvironmentPreference(context.Background()); err == nil {
		t.Fatal("nil config read accepted")
	}
	if result, err := missing.compareAndSetLongEnvironmentPreference(context.Background(), "", "title"); err == nil || result.Committed {
		t.Fatal("nil config change accepted", result, err)
	}
}

func TestReportStringPreferencesRejectsAmbiguousKeysAndReadsAllTypesBeforeCAS(t *testing.T) {
	data, config := configFixture(t)
	store, err := openDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	for _, changes := range [][]reportStringPreferenceChange{
		nil, {{Key: ""}}, {{Key: "LEReportTitle"}, {Key: "LEReportTitle"}},
	} {
		if result, err := store.compareAndSetReportStrings(context.Background(), "preferences", changes); err == nil || result.Committed {
			t.Fatal("invalid helper key contract accepted", result, err)
		}
	}
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	values["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = nil
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.commit(raw); err != nil {
		t.Fatal(err)
	}
	result, err := store.compareAndSetGoogleEarthPreferences(context.Background(),
		googleEarthPreferences{"stale", "expected"}, googleEarthPreferences{"new", "Zone"})
	if err == nil || !strings.Contains(err.Error(), "GoogleEarthDescField") || result.Committed {
		t.Fatal("shared extraction changed strict decode/CAS error precedence", result, err)
	}
}
