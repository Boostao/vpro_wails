package main

import (
	"context"
	"errors"
	"testing"
)

func TestSIVIOwnedHeightRestorationRejectsDriftTamperedAuditAndTrigger(t *testing.T) {
	service, state, db, original, edits := siviWriteFixture(t, false, 3)
	saved, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits)
	if err != nil || saved == nil {
		t.Fatal(saved, err)
	}
	for _, change := range []struct{ setup, cleanup string }{
		{`UPDATE Sample_Veg SET Species='OTHER' WHERE ID=10000001`, `UPDATE Sample_Veg SET Species='RAW' WHERE ID=10000001`},
		{`UPDATE Sample_Audit SET Flag=-1 WHERE ID=10000001 AND EditField='HeightB'`, `UPDATE Sample_Audit SET Flag=0 WHERE ID=10000001 AND EditField='HeightB'`},
		{`UPDATE Sample_Audit SET Flag=0.5 WHERE ID=10000001 AND EditField='HeightB'`, `UPDATE Sample_Audit SET Flag=0 WHERE ID=10000001 AND EditField='HeightB'`},
		{`CREATE TRIGGER SIVIRestoreSideEffect AFTER UPDATE OF HeightB ON Sample_Veg
		  BEGIN UPDATE Sample_Veg SET Cover1=1 WHERE ID=10000001; END`, `DROP TRIGGER SIVIRestoreSideEffect`},
	} {
		if _, err := db.Exec(change.setup); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
			if result, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
				t.Fatal("unsafe restoration/pruning committed", action, result, err)
			}
			assertProfileSUFiles(t, service, before)
		}
		if _, err := db.Exec(change.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestoreCancel); err != nil || result == nil || !result.Cancelled {
		t.Fatal(result, err)
	}
	assertProfileSUFiles(t, service, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.restoreSIVIHeights(ctx, state.ContextID, "108050", saved.HistoryID, AuditRestoreRetain); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled restoration returned success", result, err)
	}
	assertProfileSUFiles(t, service, before)
	result, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune)
	if err != nil || result == nil || result.RestoredRows != 3 || result.PrunedAuditRows != 3 || result.CleanedVegRows != 0 {
		t.Fatal("rejected restoration poisoned retry", result, err)
	}
}

func TestSIVIOwnedHeightRejectsChangedFileOwnershipWithoutWrites(t *testing.T) {
	service, state, _, original, edits := siviWriteFixture(t, true, 3)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	reference := owner.attachments["VLists"]
	owner.mu.Lock()
	owner.attachments["VLists"] = owner.attachments["project"]
	owner.mu.Unlock()
	defer func() {
		owner.mu.Lock()
		owner.attachments["VLists"] = reference
		owner.mu.Unlock()
	}()
	if result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits); err == nil || result != nil {
		t.Fatal("changed ownership was accepted", result, err)
	}
	owner.mu.Lock()
	owner.attachments["VLists"] = reference
	owner.mu.Unlock()
	assertProfileSUFiles(t, service, before)
	if result, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits); err != nil || result == nil {
		t.Fatal("ownership rejection poisoned retry", result, err)
	}
}
