package main

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
			service, state, db, original, heights := siviWriteFixture(t, strength%2 == 0, strength)
			protected := databaseBytes(t, service.projects.sqlite.attachments)
			for role, path := range service.projects.sqlite.attachments {
				if path == service.projects.sqlite.attachments["project"] {
					delete(protected, role)
				}
			}
			defer assertProfileSUFiles(t, service, protected)
			rowID := heights[0].RowID
			edits := []siviHeightEdit{
				{rowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)},
				{rowID, "SubVegC-SIVI", "Cover6", siviReal(0), ProjectMetadataCell{Storage: "null"}},
			}
			if strength == 3 {
				edits[0].Value = ProjectMetadataCell{Storage: "null"}
			}
			saved, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, edits)
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
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, siviHeightHistoryTable).
				Scan(&heightHistory); err != nil || heightHistory != 0 {
				t.Fatal("cover operation created height history", heightHistory, err)
			}
			if strength == 3 {
				fresh, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
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
			action := AuditRestoreRetain
			if strength%2 == 0 {
				action = AuditRestorePrune
			}
			restored, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != expected || restored.CleanedVegRows != 0 ||
				action == AuditRestorePrune && restored.PrunedAuditRows != expected {
				t.Fatal("typed cover restoration failed", restored, err)
			}
			if err := db.QueryRow(`SELECT Cover1,Cover6 FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &c); err != nil ||
				a == nil || *a != 0 || strength == 3 && (c == nil || *c != 0) || strength != 3 && c != nil {
				t.Fatal("cover restore changed audited/unaudited NULL semantics", a, c, err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if again, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || again != nil {
				t.Fatal("completed cover restoration replayed", again, err)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}

func TestSIVIOwnedCoverRejectsInvalidBatchCollisionTriggerAndRetries(t *testing.T) {
	service, state, db, original, heights := siviWriteFixture(t, false, 3)
	valid := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, invalid := range []siviHeightEdit{
		{valid.RowID, valid.Form, valid.Column, valid.Expected, siviReal(100)},
		{valid.RowID, valid.Form, "Cover5a", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		heights[0],
		valid,
	} {
		if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original,
			[]siviHeightEdit{valid, invalid}); err == nil || result != nil {
			t.Fatal("invalid tail returned a committed cover result", result, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if result, err := service.writeSIVICovers(context.Background(), "foreign", "108050", false, original, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("foreign context wrote covers", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	collision, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, collision, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("duplicate application ID wrote covers", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE ID=10000001 AND Species='DUP';
		CREATE TRIGGER SIVICoverUnexpected AFTER UPDATE OF Cover1 ON Sample_Veg
		BEGIN UPDATE Sample_Veg SET HeightA=9 WHERE ID=10000001; END`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("unexpected trigger side effect committed", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DROP TRIGGER SIVICoverUnexpected`); err != nil {
		t.Fatal(err)
	}
	saved, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{valid})
	if err != nil || saved == nil || saved.ChangedCells != 1 {
		t.Fatal("rejected cover transactions poisoned retry", saved, err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("stale original replayed", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIOwnedCoverRestorationRejectsDriftAuditTamperingAndTrigger(t *testing.T) {
	service, state, db, original, heights := siviWriteFixture(t, false, 3)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	saved, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{edit})
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
		before := databaseBytes(t, service.projects.sqlite.attachments)
		for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
			if result, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, action); err == nil || result != nil {
				t.Fatal("unsafe cover restoration/pruning committed", result, err)
			}
			assertProfileSUFiles(t, service, before)
		}
		if _, err := db.Exec(change.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); err == nil || result != nil {
		t.Fatal("cover history restored through height authority", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if result, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestoreCancel); err != nil || result == nil || !result.Cancelled {
		t.Fatal(result, err)
	}
	assertProfileSUFiles(t, service, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.restoreSIVICovers(ctx, state.ContextID, "108050", saved.HistoryID, AuditRestoreRetain); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled cover restoration returned success/partial result", result, err)
	}
	assertProfileSUFiles(t, service, before)
	result, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune)
	if err != nil || result == nil || result.RestoredRows != 1 || result.PrunedAuditRows != 1 {
		t.Fatal("rejected cover restoration poisoned retry", result, err)
	}
}

func TestSIVIOwnedCoverAndHeightRestorationPoliciesRemainIsolated(t *testing.T) {
	service, state, db, original, heights := siviWriteFixture(t, false, 3)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	saved, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{edit})
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICoverHistory" WHERE ID=?`, saved.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(siviHeightHistorySQL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO "__VPRO_SIVIHeightHistory"(ID,Created,Proposal) VALUES (1,'fixture',?)`, proposal); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", "1", AuditRestorePrune); err == nil ||
		!strings.Contains(err.Error(), "unavailable or repeated cells/audits") || result != nil {
		t.Fatal("height history accepted cover columns", result, err)
	}
	assertProfileSUFiles(t, service, before)
	var event siviHeightHistory
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
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune); err == nil ||
		!strings.Contains(err.Error(), "unavailable or repeated cells/audits") || result != nil {
		t.Fatal("cover history accepted height columns", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIOwnedCoverRejectsChangedFileOwnership(t *testing.T) {
	service, state, _, original, heights := siviWriteFixture(t, true, 3)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
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
	if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{edit}); err == nil || result != nil {
		t.Fatal("changed file ownership authorized cover writes", result, err)
	}
	owner.mu.Lock()
	owner.attachments["VLists"] = reference
	owner.mu.Unlock()
	assertProfileSUFiles(t, service, before)
	if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{edit}); err != nil || result == nil {
		t.Fatal("ownership refusal poisoned cover retry", result, err)
	}
}

func TestSIVIOwnedCoverBlockedCancellationAndRetry(t *testing.T) {
	service, state, db, original, heights := siviWriteFixture(t, true, 3)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
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
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if result, err := service.writeSIVICovers(ctx, state.ContextID, "108050", false, original, []siviHeightEdit{edit}); !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("cancelled cover writer returned a partial/success result", result, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	assertProfileSUFiles(t, service, before)
	if result, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{edit}); err != nil || result == nil {
		t.Fatal("cancelled cover writer lost context ownership", result, err)
	}
}

func TestSIVIOwnedCoverHistoricalNoopAndExtendedRestoration(t *testing.T) {
	service, state, db, _, heights := siviWriteFixture(t, false, 3)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=125,Cover5a=-1 WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", true)
	if err != nil {
		t.Fatal(err)
	}
	noop := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI", "Cover1", siviReal(125), siviReal(125)}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if saved, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", true, original, []siviHeightEdit{noop}); err != nil ||
		saved == nil || saved.ChangedCells != 0 || saved.HistoryID != "" {
		t.Fatal("unchanged historical cover produced writes/history", saved, err)
	}
	assertProfileSUFiles(t, service, before)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI", "Cover5a", siviReal(-1), siviReal(3)}
	saved, err := service.writeSIVICovers(context.Background(), state.ContextID, "108050", true, original, []siviHeightEdit{noop, edit})
	if err != nil || saved == nil || saved.ChangedCells != 1 {
		t.Fatal("extended cover could not save alongside historical no-op", saved, err)
	}
	restored, err := service.restoreSIVICovers(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 {
		t.Fatal("extended cover restoration failed", restored, err)
	}
	var a, b float64
	if err := db.QueryRow(`SELECT Cover1,Cover5a FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &b); err != nil || a != 125 || b != -1 {
		t.Fatal("cover restoration repaired unrelated historical storage", a, b, err)
	}
}
