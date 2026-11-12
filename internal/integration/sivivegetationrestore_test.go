package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"testing"
)

func TestSIVIOwnedHeightRestorationRejectsDriftTamperedAuditAndTrigger(t *testing.T) {
	service, state, db, original, edits := sftest.SiviWriteFixture(t, false, 3)
	saved, err := sourceform.WriteSIVIHeights(service, context.Background(), state.ContextID, "108050", false, original, edits)
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
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
			if result, err := sourceform.RestoreSIVIHeights(service, context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
				t.Fatal("unsafe restoration/pruning committed", action, result, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
		}
		if _, err := db.Exec(change.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.RestoreSIVIHeights(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreCancel); err != nil || result == nil || !result.Cancelled {
		t.Fatal(result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := sourceform.RestoreSIVIHeights(service, ctx, state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreRetain); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled restoration returned success", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	result, err := sourceform.RestoreSIVIHeights(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || result == nil || result.RestoredRows != 3 || result.PrunedAuditRows != 3 || result.CleanedVegRows != 0 {
		t.Fatal("rejected restoration poisoned retry", result, err)
	}
}

func TestSIVIOwnedHeightRejectsChangedFileOwnershipWithoutWrites(t *testing.T) {
	service, state, _, original, edits := sftest.SiviWriteFixture(t, true, 3)
	owner := service.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	reference := owner.Attachments["VLists"]
	owner.Mu.Lock()
	owner.Attachments["VLists"] = owner.Attachments["project"]
	owner.Mu.Unlock()
	defer func() {
		owner.Mu.Lock()
		owner.Attachments["VLists"] = reference
		owner.Mu.Unlock()
	}()
	if result, err := sourceform.WriteSIVIHeights(service, context.Background(), state.ContextID, "108050", false, original, edits); err == nil || result != nil {
		t.Fatal("changed ownership was accepted", result, err)
	}
	owner.Mu.Lock()
	owner.Attachments["VLists"] = reference
	owner.Mu.Unlock()
	vprotest.AssertProfileSUFiles(t, service, before)
	if result, err := sourceform.WriteSIVIHeights(service, context.Background(), state.ContextID, "108050", false, original, edits); err != nil || result == nil {
		t.Fatal("ownership rejection poisoned retry", result, err)
	}
}
