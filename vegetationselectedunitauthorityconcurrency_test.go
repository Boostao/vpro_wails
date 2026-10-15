package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSelectedUnitAuthorityDeferredDraftCannotRevokeInFlightWorkbookPublication(t *testing.T) {
	workbook, state := vegetationWorkbookServiceFixture(t, false)
	contexts := workbook.contexts
	contexts.longVegetationSelectedUnitsEnabled = true
	su, err := sql.Open("sqlite3", sqliteFileURI(state.SUPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, removeErr := su.Exec(`DELETE FROM ` + quoteHeaderIdentifier(state.ActiveSU+"_SU") + ` WHERE PlotNumber='missing'`)
	if err := errors.Join(removeErr, su.Close()); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	selection := NewLongVegetationSelectedUnitSelectionService(contexts, true)
	preferences := NewLongVegetationUnitPreferencesService(contexts, true)
	draft, err := selection.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	built, err := selection.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
	if err != nil {
		t.Fatal(err)
	}
	saved := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, `{"expected":{"unitGroups":1},"proposed":{"unitGroups":2}}`)
	if !saved.Committed || saved.ErrorMessage != "" {
		t.Fatal("reviewed mode2 setup", saved)
	}
	review, err := workbook.GetReview(ctx, state.ContextID, `{"scope":"unlumped"}`)
	if err != nil {
		t.Fatal(err)
	}
	ready, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	defer resume()
	selection.beforeInstall = func() { close(ready); <-release }
	type result struct {
		selection *LongVegetationSelectedUnitSelection
		err       error
	}
	done := make(chan result, 1)
	go func() {
		changed, err := selection.SetLongVegetationSelectedUnitFields(ctx, state.ContextID,
			selectedUnitDraftJSON(t, built.Revision, []string{"Plot"}))
		done <- result{changed, err}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("draft did not reach completed-snapshot/preinstallation boundary")
	}
	destination := filepath.Join(t.TempDir(), "selection-lease.xlsx")
	request := workbookRequest(t, vegetationWorkbookExportRequest{"unlumped", review.CreatedDate, review.ApprovalHash, destination})
	var early *result
	links := 0
	outcome, err := workbook.exportReviewed(ctx, state.ContextID, request, vegetationWorkbookHooks{
		publication: artifactPublicationHooks{link: func(stage, target string) error {
			links++
			resume()
			select {
			case changed := <-done:
				early = &changed
				return errors.New("draft revoked selected-field authority between final validation and publication")
			case <-time.After(time.Second):
			}
			return os.Link(stage, target)
		}},
	})
	resume()
	var changed result
	if early != nil {
		changed = *early
	} else {
		select {
		case changed = <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("draft installation did not resume after publication lease release")
		}
	}
	if early != nil || err != nil || outcome == nil || outcome.Status != "published" ||
		outcome.ErrorMessage != "" || links != 1 || outcome.SHA256 != review.WorkbookSHA256 {
		t.Fatal("publication lacked stable session field authority", outcome, links, early, err)
	}
	data, err := os.ReadFile(destination)
	digest := sha256.Sum256(data)
	if err != nil || hex.EncodeToString(digest[:]) != review.WorkbookSHA256 {
		t.Fatal("once-only owned publication differs from reviewed exact bytes", err)
	}
	if changed.err != nil || changed.selection.Build != nil || changed.selection.Fields[0] != "Plot" {
		t.Fatal("deferred draft did not invalidate later output", changed)
	}
	if _, err := contexts.PreviewLongVegetation(ctx, state.ContextID); err == nil {
		t.Fatal("later report reused revoked selection after deferred draft completed")
	}
}

func TestSelectedUnitAuthorityCASRefusesApprovalBoundThresholdChangeInsideConfigGap(t *testing.T) {
	workbook, state := vegetationWorkbookServiceFixture(t, false)
	contexts := workbook.contexts
	contexts.longVegetationSelectedUnitsEnabled = true
	ctx := context.Background()
	selection := NewLongVegetationSelectedUnitSelectionService(contexts, true)
	draft, err := selection.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	built, err := selection.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
	if err != nil {
		t.Fatal(err)
	}
	thresholds, err := contexts.projects.preferences.readLongVegetationThresholdPreferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	proposed := thresholds
	proposed.PresenceGreaterThan++
	preferences := NewLongVegetationUnitPreferencesService(contexts, true)
	preferences.beforeSelectedUnitCAS = func() {
		changed, err := contexts.projects.preferences.compareAndSetLongVegetationThresholdPreferences(ctx, thresholds, proposed)
		if err != nil || !changed.Committed {
			t.Fatal("admitted peer threshold mutation did not commit in the approval/CAS gap", changed, err)
		}
	}
	request := `{"expected":{"unitGroups":1},"proposed":{"unitGroups":2}}`
	refused := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request)
	if refused.Changed || refused.Committed || refused.ErrorMessage == "" {
		t.Fatal("new2 committed despite changed approval-bound configuration", refused)
	}
	preferences.beforeSelectedUnitCAS = nil
	saved, err := preferences.GetLongVegetationUnitPreferences(ctx, state.ContextID)
	if err != nil || saved.Values.UnitGroups != 1 || saved.Availability.SelectedFields {
		t.Fatal("rejected new2 changed saved unit mode or retained stale availability", saved, err)
	}
	current, err := contexts.projects.preferences.readLongVegetationThresholdPreferences(ctx)
	if err != nil || current != proposed {
		t.Fatal("new2 refusal rolled back legitimate peer preference commit", current, err)
	}
	if _, err := selection.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, built.Revision); err != nil {
		t.Fatal("explicit Build after configuration correction failed", err)
	}
	retried := preferences.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request)
	if !retried.Committed || retried.ErrorMessage != "" {
		t.Fatal("reviewed new-configuration explicit retry failed", retried)
	}
}
