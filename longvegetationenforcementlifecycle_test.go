package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLongVegetationEnforcementCancellationAttachmentAndUnrelatedPreservation(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": -1, "LVUnitGroups": 3,
		"UnrelatedLiteral": " retain \x00 historical "}); err != nil {
		t.Fatal(err)
	}
	before, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	expected, proposed := LongVegetationEnforcementPreferenceValues{true, 3}, LongVegetationEnforcementPreferenceValues{false, 1}
	service := NewLongVegetationEnforcementPreferencesService(contexts, true)
	service.printers = func(ctx context.Context) (PrinterObservation, error) {
		if err := ctx.Err(); err != nil {
			return PrinterObservation{}, err
		}
		return PrinterObservation{Names: []string{}, Source: printerObservationSource}, nil
	}
	request := enforcementRequest(t, expected, proposed)
	owner := contexts.projects.sqlite
	original := owner.attachments["project"]
	owner.attachments["project"] = original + ".unowned"
	result := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request)
	owner.attachments["project"] = original
	if result.Changed || result.Committed || result.ErrorMessage == "" {
		t.Fatal("replaced ownership permitted paired save", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	result = service.SaveLongVegetationEnforcementPreferences(ctx, state.ContextID, request)
	if result.Changed || result.Committed || result.ErrorMessage == "" {
		t.Fatal("cancelled snapshot permitted paired save", result)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	replace := store.replace
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	result = service.SaveLongVegetationEnforcementPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !result.Committed || !result.Changed || !strings.Contains(result.ErrorMessage, "do not replay") {
		t.Fatal("postcommit cancellation erased paired receipt", result)
	}
	after, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	expectedMap := before["ReportOptions"].(map[string]any)
	expectedMap["DataQualityFilterEnforceLV"] = false
	expectedMap["LVUnitGroups"] = 1
	if !reflect.DeepEqual(before, after) {
		t.Fatal("paired save modified unrelated configuration", before, after)
	}
	review, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID)
	if err != nil || review.Values != proposed {
		t.Fatal("cancellation poisoned owned coordinator/retry", review, err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("read commit") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("read rollback") }},
	} {
		service.snapshot = hooks
		if value, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); value != nil || err == nil {
			t.Fatal("failed snapshot returned reviewed values", value, err)
		}
	}
}

func TestLongVegetationEnforcementInvalidPrintersNoopReadAndMalformedGroups(t *testing.T) {
	contexts, state := prepareKMLContextFixture(t)
	store := contexts.projects.preferences
	if err := store.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": false, "LVUnitGroups": 2}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationEnforcementPreferencesService(contexts, true)
	expected := LongVegetationEnforcementPreferenceValues{false, 2}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []PrinterObservation{
		{}, {Names: []string{}, Source: "invented"}, {Names: []string{""}, Source: printerObservationSource},
	} {
		calls := 0
		service.printers = func(context.Context) (PrinterObservation, error) { calls++; return bad, nil }
		if value, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); value != nil || err == nil {
			t.Fatal("invalid printer DTO permitted read", value, err)
		}
		got := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, enforcementRequest(t, expected, expected))
		after, err := os.ReadFile(store.path)
		if got.Changed || got.Committed || got.Printers != nil || got.ErrorMessage == "" || calls != 2 ||
			err != nil || !bytes.Equal(before, after) {
			t.Fatal("no-op host failure modified configuration or silently skipped refresh", got, calls, err)
		}
	}
	for _, bad := range []any{nil, 0, 4, "1", false} {
		if err := store.update("ReportOptions", map[string]any{"LVUnitGroups": bad}); err != nil {
			t.Fatal(err)
		}
		if value, err := store.readLongVegetationEnforcementPreferences(context.Background()); err == nil {
			t.Fatal("malformed group gained fallback", bad, value)
		}
		if result, err := store.compareAndSetLongVegetationEnforcementPreferences(context.Background(), expected, expected); err == nil || result.Committed {
			t.Fatal("malformed historical no-op authorized persistence", bad, result, err)
		}
	}
}
