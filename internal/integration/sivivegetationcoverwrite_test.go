package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSIVIOwnedCoverWritesAuditStrengthsAndRestoresDisappearedRows(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, state, db, original, heights := sftest.SiviWriteFixture(t, strength%2 == 0, strength)
			protected := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			for role, path := range service.Projects.Sqlite.Attachments {
				if path == service.Projects.Sqlite.Attachments["project"] {
					delete(protected, role)
				}
			}
			defer vprotest.AssertProfileSUFiles(t, service, protected)
			rowID := heights[0].RowID
			edits := []sourceform.SiviHeightEdit{
				{rowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
				{rowID, "SubVegC-SIVI", "Cover6", vprotest.SiviReal(0), vprocore.ProjectMetadataCell{Storage: "null"}},
			}
			if strength == 3 {
				edits[0].Value = vprocore.ProjectMetadataCell{Storage: "null"}
			}
			saved, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, edits)
			if err != nil || saved == nil || saved.ChangedCells != 2 {
				t.Fatal(saved, err)
			}
			var a, c *float64
			var heightA float64
			var heightB string
			if err := db.QueryRow(`SELECT Cover1,Cover6,HeightA,HeightB FROM Sample_Veg WHERE ID=10000001`).
				Scan(&a, &c, &heightA, &heightB); err != nil || c != nil || heightA != 2 || heightB != "before" ||
				strength == 3 && a != nil || strength != 3 && (a == nil || *a != 3) {
				t.Fatal("cover save changed NULL or peer heights", a, c, heightA, heightB, err)
			}
			var count, heightHistory int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE ID=10000001`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			expected := []int{0, 1, 1, 2}[strength]
			if count != expected || (saved.HistoryID == "") != (expected == 0) {
				t.Fatal("cover audit strength or history differs", count, saved)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, sourceform.SiviHeightHistoryTable).
				Scan(&heightHistory); err != nil || heightHistory != 0 {
				t.Fatal("cover operation created height history", heightHistory, err)
			}
			if strength == 3 {
				fresh, err := sourceform.ReadSIVIVegetation(service, context.Background(), state.ContextID, "108050", false)
				if err != nil {
					t.Fatal(err)
				}
				for _, group := range fresh {
					for _, row := range group.Rows {
						if row.RowID == rowID {
							t.Fatal("cleared last covers did not remove query membership", group.Form)
						}
					}
				}
			}
			if expected == 0 {
				return
			}
			if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='Sample_Veg' WHERE ID=10000001`); err != nil {
				t.Fatal(err)
			}
			action := vprocore.AuditRestoreRetain
			if strength%2 == 0 {
				action = vprocore.AuditRestorePrune
			}
			restored, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != expected || restored.CleanedVegRows != 0 ||
				action == vprocore.AuditRestorePrune && restored.PrunedAuditRows != expected {
				t.Fatal("typed cover restoration failed", restored, err)
			}
			if err := db.QueryRow(`SELECT Cover1,Cover6 FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &c); err != nil ||
				a == nil || *a != 0 || strength == 3 && (c == nil || *c != 0) || strength != 3 && c != nil {
				t.Fatal("cover restore changed audited/unaudited NULL semantics", a, c, err)
			}
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if again, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || again != nil {
				t.Fatal("completed cover restoration replayed", again, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
		})
	}
}

func TestSIVIOwnedCoverRejectsInvalidBatchCollisionTriggerAndRetries(t *testing.T) {
	service, state, db, original, heights := sftest.SiviWriteFixture(t, false, 3)
	valid := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	for _, invalid := range []sourceform.SiviHeightEdit{
		{valid.RowID, valid.Form, valid.Column, valid.Expected, vprotest.SiviReal(100)},
		{valid.RowID, valid.Form, "Cover5a", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.SiviReal(3)},
		heights[0],
		valid,
	} {
		if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original,
			[]sourceform.SiviHeightEdit{valid, invalid}); err == nil || result != nil {
			t.Fatal("invalid tail returned a committed cover result", result, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), "foreign", "108050", false, original, []sourceform.SiviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("foreign context wrote covers", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	collision, err := sourceform.ReadSIVIVegetation(service, context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, collision, []sourceform.SiviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("duplicate application ID wrote covers", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE ID=10000001 AND Species='DUP';
		CREATE TRIGGER SIVICoverUnexpected AFTER UPDATE OF Cover1 ON Sample_Veg
		BEGIN UPDATE Sample_Veg SET HeightA=9 WHERE ID=10000001; END`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("unexpected trigger side effect committed", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DROP TRIGGER SIVICoverUnexpected`); err != nil {
		t.Fatal(err)
	}
	saved, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{valid})
	if err != nil || saved == nil || saved.ChangedCells != 1 {
		t.Fatal("rejected cover transactions poisoned retry", saved, err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("stale original replayed", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSIVIOwnedCoverRestorationRejectsDriftAuditTamperingAndTrigger(t *testing.T) {
	service, state, db, original, heights := sftest.SiviWriteFixture(t, false, 3)
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	saved, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{edit})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct{ setup, cleanup string }{
		{`UPDATE Sample_Veg SET HeightA=8 WHERE ID=10000001`, `UPDATE Sample_Veg SET HeightA=2 WHERE ID=10000001`},
		{`UPDATE Sample_Audit SET Flag=-1 WHERE ID=10000001`, `UPDATE Sample_Audit SET Flag=0 WHERE ID=10000001`},
		{`CREATE TRIGGER SIVICoverRestoreUnexpected AFTER UPDATE OF Cover1 ON Sample_Veg
		  BEGIN UPDATE Sample_Veg SET HeightA=8 WHERE ID=10000001; END`, `DROP TRIGGER SIVICoverRestoreUnexpected`},
	} {
		if _, err := db.Exec(change.setup); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
			if result, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
				t.Fatal("unsafe cover restoration/pruning committed", result, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
		}
		if _, err := db.Exec(change.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.RestoreSIVIHeights(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); err == nil || result != nil {
		t.Fatal("cover history restored through height authority", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if result, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreCancel); err != nil || result == nil || !result.Cancelled {
		t.Fatal(result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := sourceform.RestoreSIVICovers(service, ctx, state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestoreRetain); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled cover restoration returned success/partial result", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	result, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || result == nil || result.RestoredRows != 1 || result.PrunedAuditRows != 1 {
		t.Fatal("rejected cover restoration poisoned retry", result, err)
	}
}

func TestSIVIOwnedCoverAndHeightRestorationPoliciesRemainIsolated(t *testing.T) {
	service, state, db, original, heights := sftest.SiviWriteFixture(t, false, 3)
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	saved, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{edit})
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICoverHistory" WHERE ID=?`, saved.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(sourceform.SiviHeightHistorySQL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO "__VPRO_SIVIHeightHistory"(ID,Created,Proposal) VALUES (1,'fixture',?)`, proposal); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.RestoreSIVIHeights(service, context.Background(), state.ContextID, "108050", "1", vprocore.AuditRestorePrune); err == nil ||
		!strings.Contains(err.Error(), "unavailable or repeated cells/audits") || result != nil {
		t.Fatal("height history accepted cover columns", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	var event sourceform.SiviHeightHistory
	if err := json.Unmarshal([]byte(proposal), &event); err != nil {
		t.Fatal(err)
	}
	event.Changes[0].Column = "HeightA"
	forged, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICoverHistory" SET Proposal=? WHERE ID=?`, string(forged), saved.HistoryID); err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune); err == nil ||
		!strings.Contains(err.Error(), "unavailable or repeated cells/audits") || result != nil {
		t.Fatal("cover history accepted height columns", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSIVIOwnedCoverRejectsChangedFileOwnership(t *testing.T) {
	service, state, _, original, heights := sftest.SiviWriteFixture(t, true, 3)
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
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
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{edit}); err == nil || result != nil {
		t.Fatal("changed file ownership authorized cover writes", result, err)
	}
	owner.Mu.Lock()
	owner.Attachments["VLists"] = reference
	owner.Mu.Unlock()
	vprotest.AssertProfileSUFiles(t, service, before)
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{edit}); err != nil || result == nil {
		t.Fatal("ownership refusal poisoned cover retry", result, err)
	}
}

func TestSIVIOwnedCoverBlockedCancellationAndRetry(t *testing.T) {
	service, state, db, original, heights := sftest.SiviWriteFixture(t, true, 3)
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if result, err := sourceform.WriteSIVICovers(service, ctx, state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{edit}); !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("cancelled cover writer returned a partial/success result", result, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	vprotest.AssertProfileSUFiles(t, service, before)
	if result, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", false, original, []sourceform.SiviHeightEdit{edit}); err != nil || result == nil {
		t.Fatal("cancelled cover writer lost context ownership", result, err)
	}
}

func TestSIVIOwnedCoverHistoricalNoopAndExtendedRestoration(t *testing.T) {
	service, state, db, _, heights := sftest.SiviWriteFixture(t, false, 3)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=125,Cover5a=-1 WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	original, err := sourceform.ReadSIVIVegetation(service, context.Background(), state.ContextID, "108050", true)
	if err != nil {
		t.Fatal(err)
	}
	noop := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI", "Cover1", vprotest.SiviReal(125), vprotest.SiviReal(125)}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if saved, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", true, original, []sourceform.SiviHeightEdit{noop}); err != nil ||
		saved == nil || saved.ChangedCells != 0 || saved.HistoryID != "" {
		t.Fatal("unchanged historical cover produced writes/history", saved, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI", "Cover5a", vprotest.SiviReal(-1), vprotest.SiviReal(3)}
	saved, err := sourceform.WriteSIVICovers(service, context.Background(), state.ContextID, "108050", true, original, []sourceform.SiviHeightEdit{noop, edit})
	if err != nil || saved == nil || saved.ChangedCells != 1 {
		t.Fatal("extended cover could not save alongside historical no-op", saved, err)
	}
	restored, err := sourceform.RestoreSIVICovers(service, context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 {
		t.Fatal("extended cover restoration failed", restored, err)
	}
	var a, b float64
	if err := db.QueryRow(`SELECT Cover1,Cover5a FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &b); err != nil || a != 125 || b != -1 {
		t.Fatal("cover restoration repaired unrelated historical storage", a, b, err)
	}
}
