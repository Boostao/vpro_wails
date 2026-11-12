package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLongVegetationUnitPreferencesEnforcementResetConflictsAndOwnedRetry(t *testing.T) {
	contexts, state := vprotest.ReportServiceFixture(t, true)
	contexts.LongVegetationNoneUnitsEnabled = true
	service := NewLongVegetationUnitPreferencesService(contexts, true)
	store := contexts.Projects.Preferences
	beforeDB := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3)); !result.Committed || !result.Changed || result.ErrorMessage != "" {
		t.Fatal(result)
	}
	enforcement := NewLongVegetationEnforcementPreferencesService(contexts, true)
	enforcement.printers = enforcementFixtureObserver
	review, err := enforcement.GetLongVegetationEnforcementPreferences(context.Background(), state.ContextID)
	if err != nil || review.Values.UnitGroups != 3 {
		t.Fatal("paired enforcement reader lost saved None unit choice", review, err)
	}
	result := enforcement.SaveLongVegetationEnforcementPreferences(context.Background(), state.ContextID,
		enforcementRequest(t, review.Values, LongVegetationEnforcementPreferenceValues{Enforce: !review.Values.Enforce, UnitGroups: 1}))
	if !result.Committed || !result.Changed || result.ErrorMessage != "" {
		t.Fatal("source Form_Load paired reset failed", result)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	stale := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 3, 3))
	if stale.Committed || stale.Changed || !strings.Contains(stale.ErrorMessage, "changed") {
		t.Fatal("stale unavailable no-op hid source enforcement unit1 reset", stale)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("stale unit draft rewrote paired enforcement authority", err)
	}
	current, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values.UnitGroups != 1 {
		t.Fatal("owned observation did not expose reset", current, err)
	}
	if retry := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3)); !retry.Committed || retry.ErrorMessage != "" {
		t.Fatal("explicit owned correction failed", retry)
	}
	vprotest.AssertProfileSUFiles(t, contexts, beforeDB)
}

func TestLongVegetationUnitPreferencesCancellationOwnershipAndStaleContext(t *testing.T) {
	contexts, state := vprotest.ReportServiceFixture(t, true)
	contexts.LongVegetationNoneUnitsEnabled = true
	service := NewLongVegetationUnitPreferencesService(contexts, true)
	owner := contexts.Projects.Sqlite
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	if value, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID); value != nil || err == nil {
		t.Fatal("changed original SU identity returned unit review", value, err)
	}
	if value := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3)); value.Committed || value.Changed || value.ErrorMessage == "" {
		t.Fatal("changed original SU identity allowed preference mutation", value)
	}
	owner.AttachmentInfo["su"] = original
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := service.GetLongVegetationUnitPreferences(ctx, state.ContextID); value != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled unit read returned observations", value, err)
	}
	if value := service.SaveLongVegetationUnitPreferences(ctx, state.ContextID, vegetationUnitRequest(t, 1, 3)); value.Committed || value.Changed || value.ErrorMessage == "" {
		t.Fatal("cancelled unit save committed", value)
	}
	next, err := contexts.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if value := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3)); value.Committed || value.Changed || value.ErrorMessage == "" {
		t.Fatal("stale context wrote new owner's preferences", value)
	}
	if value, err := service.GetLongVegetationUnitPreferences(context.Background(), next.ContextID); value == nil || err != nil || value.Values.UnitGroups != 1 {
		t.Fatal("failed ownership/cancellation/staleness leaked lease or wrote choice", value, err)
	}
}
