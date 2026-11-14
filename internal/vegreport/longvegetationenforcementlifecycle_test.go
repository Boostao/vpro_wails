package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": -1, "LVUnitGroups": 3,
		"UnrelatedLiteral": " retain \x00 historical "}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	expected, proposed := LongVegetationEnforcementPreferenceValues{true, 3}, LongVegetationEnforcementPreferenceValues{false, 1}
	service := NewLongVegetationEnforcementPreferencesService(contexts)
	service.printers = func(ctx context.Context) (vprocore.PrinterObservation, error) {
		if err := ctx.Err(); err != nil {
			return vprocore.PrinterObservation{}, err
		}
		return vprocore.PrinterObservation{Names: []string{}, Source: vprocore.PrinterObservationSource}, nil
	}
	request := enforcementRequest(t, expected, proposed)
	owner := contexts.Projects.Sqlite
	original := owner.Attachments["project"]
	owner.Attachments["project"] = original + ".unowned"
	result := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, request)
	owner.Attachments["project"] = original
	if result.Changed || result.Committed || result.ErrorMessage == "" {
		t.Fatal("replaced ownership permitted paired save", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	result = service.SaveLongVegetationEnforcementPreferences(ctx, state.ContextID, request)
	if result.Changed || result.Committed || result.ErrorMessage == "" {
		t.Fatal("cancelled snapshot permitted paired save", result)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	replace := store.Replace
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.Replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	result = service.SaveLongVegetationEnforcementPreferences(ctx, state.ContextID, request)
	store.Replace = replace
	if !result.Committed || !result.Changed || !strings.Contains(result.ErrorMessage, "do not replay") {
		t.Fatal("postcommit cancellation erased paired receipt", result)
	}
	after, err := store.Snapshot()
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
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("read commit") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("read rollback") }},
	} {
		service.snapshot = hooks
		if value, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); value != nil || err == nil {
			t.Fatal("failed snapshot returned reviewed values", value, err)
		}
	}
}

func TestLongVegetationEnforcementInvalidPrintersNoopReadAndMalformedGroups(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	store := contexts.Projects.Preferences
	if err := store.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": false, "LVUnitGroups": 2}); err != nil {
		t.Fatal(err)
	}
	service := NewLongVegetationEnforcementPreferencesService(contexts)
	expected := LongVegetationEnforcementPreferenceValues{false, 2}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []vprocore.PrinterObservation{
		{}, {Names: []string{}, Source: "invented"}, {Names: []string{""}, Source: vprocore.PrinterObservationSource},
	} {
		calls := 0
		service.printers = func(context.Context) (vprocore.PrinterObservation, error) { calls++; return bad, nil }
		if value, err := service.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID); value != nil || err == nil {
			t.Fatal("invalid printer DTO permitted read", value, err)
		}
		got := service.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID, enforcementRequest(t, expected, expected))
		after, err := os.ReadFile(store.Path)
		if got.Changed || got.Committed || got.Printers != nil || got.ErrorMessage == "" || calls != 2 ||
			err != nil || !bytes.Equal(before, after) {
			t.Fatal("no-op host failure modified configuration or silently skipped refresh", got, calls, err)
		}
	}
	for _, bad := range []any{nil, 0, 4, "1", false} {
		if err := store.Update("ReportOptions", map[string]any{"LVUnitGroups": bad}); err != nil {
			t.Fatal(err)
		}
		if value, err := ReadLongVegetationEnforcementPreferences(store, context.Background()); err == nil {
			t.Fatal("malformed group gained fallback", bad, value)
		}
		if result, err := CompareAndSetLongVegetationEnforcementPreferences(store, context.Background(), expected, expected); err == nil || result.Committed {
			t.Fatal("malformed historical no-op authorized persistence", bad, result, err)
		}
	}
}
