package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSIVIIdentityOwnedSaveUnauditedAtEveryStrengthAndTypedRestoration(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, state, db, original, heightEdits := siviWriteFixture(t, strength%2 == 0, strength)
			rowID := heightEdits[0].RowID
			var physicalBefore string
			if err := db.QueryRow(`SELECT CAST(rowid AS TEXT) FROM Sample_Veg WHERE ID=10000001`).Scan(&physicalBefore); err != nil {
				t.Fatal(err)
			}
			audit := heightTableSnapshot(t, db, "Sample_Audit")
			veg := heightTableSnapshot(t, db, "Sample_Veg")
			edits := []siviHeightEdit{{rowID, "SubVegC-SIVI", "ID", metadataInteger("10000001"), metadataInteger("-2147483648")}}
			result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, edits)
			if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
				t.Fatal("private identity save failed", result, err)
			}
			var id int64
			var physicalAfter string
			if err := db.QueryRow(`SELECT ID,CAST(rowid AS TEXT) FROM Sample_Veg WHERE rowid=?`, rowID).Scan(&id, &physicalAfter); err != nil ||
				id != -2147483648 || physicalBefore != physicalAfter {
				t.Fatal("application ID assignment changed physical row", id, physicalBefore, physicalAfter, err)
			}
			if got := heightTableSnapshot(t, db, "Sample_Audit"); got != audit {
				t.Fatal("ID inherited phantom source field audits")
			}
			var reservations int
			if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID IN (10000001,-2147483648)`).Scan(&reservations); err != nil || reservations != 2 {
				t.Fatal("old/new identities were not permanently reserved", reservations, err)
			}
			action := AuditRestoreRetain
			if strength%2 == 0 {
				action = AuditRestorePrune
			}
			restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 0 || restored.CleanedVegRows != 0 {
				t.Fatal("identity restoration inherited audit pruning/row deletion", restored, err)
			}
			if got := heightTableSnapshot(t, db, "Sample_Veg"); got != veg {
				t.Fatal("typed identity restoration did not preserve the complete original table")
			}
			if got := heightTableSnapshot(t, db, "Sample_Audit"); got != audit {
				t.Fatal("identity restoration changed source audit bytes")
			}
			beforeRetry := databaseBytes(t, service.projects.sqlite.attachments)
			if again, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, action); err == nil || again != nil {
				t.Fatal("completed identity restoration replayed", again, err)
			}
			assertProfileSUFiles(t, service, beforeRetry)
		})
	}
}

func TestSIVIIdentityOwnedNULLAndHistoricalDuplicateRestoration(t *testing.T) {
	for _, target := range []ProjectMetadataCell{{Storage: "null"}, metadataInteger("2147483647")} {
		service, state, db, _, heightEdits := siviWriteFixture(t, false, 3)
		rowID := heightEdits[0].RowID
		if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover9) VALUES('108050','DUP',10000001,0)`); err != nil {
			t.Fatal(err)
		}
		original, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
		if err != nil {
			t.Fatal(err)
		}
		veg := heightTableSnapshot(t, db, "Sample_Veg")
		result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original,
			[]siviHeightEdit{{rowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), target}})
		if err != nil || result == nil {
			t.Fatal("physical identity editor inherited ambiguous application-ID targeting", result, err)
		}
		var storage string
		if err := db.QueryRow(`SELECT typeof(ID) FROM Sample_Veg WHERE rowid=?`, rowID).Scan(&storage); err != nil || storage != target.Storage {
			t.Fatal("nullable identity storage changed", storage, err)
		}
		restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune)
		if err != nil || restored == nil || restored.RestoredRows != 1 || heightTableSnapshot(t, db, "Sample_Veg") != veg {
			t.Fatal("unchanged historical duplicate could not be safely restored", restored, err)
		}
	}
}

func TestSIVIIdentityOwnedRejectsStaleCollisionReservationAndCancelledRequests(t *testing.T) {
	service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
	rowID := heightEdits[0].RowID
	valid := siviHeightEdit{rowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("2147483647")}
	// Both aliases are reserved, including an ID belonging only to a deleted row.
	if _, err := db.Exec(`INSERT INTO Sample_Audit("Table",ID) VALUES('_vEg',2147483647),('SAMPLE_VEG',-2147483648)`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"2147483647", "-2147483648"} {
		edit := valid
		edit.Value = metadataInteger(id)
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{edit}); err == nil || result != nil {
			t.Fatal("deleted/audited ID was reassigned", result, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Audit WHERE ID IN (2147483647,-2147483648)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID) VALUES('outside','COLLIDE',2147483647)`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("outside-plot/hidden collision was accepted", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE Species='COLLIDE'; UPDATE Sample_Veg SET HeightA=9 WHERE rowid=?`, rowID); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("complete source CAS overlooked a non-ID drift", result, err)
	}
	assertProfileSUFiles(t, service, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.writeSIVIIdentities(ctx, state.ContextID, "108050", false, original, []siviHeightEdit{valid}); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled identity save returned success", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if result, err := service.writeSIVIIdentities(context.Background(), "stale", "108050", false, original, []siviHeightEdit{valid}); err == nil || result != nil {
		t.Fatal("stale owner was granted identity authority", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIIdentityOwnedNoopHasNoLedgerHistoryOrAudit(t *testing.T) {
	service, state, _, original, heightEdits := siviWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original,
		[]siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("10000001")}})
	if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("unchanged identity created a mutation-shaped result", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIIdentityOwnedRollsBackTriggerDriftAndRetries(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER fail_id AFTER UPDATE OF ID ON Sample_Veg BEGIN UPDATE Sample_Veg SET Species='SIDE' WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_id AFTER UPDATE OF ID ON Sample_Veg BEGIN INSERT INTO Sample_Audit("Table",ID) VALUES('_Veg',NEW.ID); END`,
		`CREATE TRIGGER fail_id AFTER UPDATE OF ID ON Sample_Veg BEGIN SELECT RAISE(ABORT,'blocked identity'); END`,
	} {
		service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		edits := []siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")}}
		if result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, edits); err == nil || result != nil {
			t.Fatal("trigger escaped the complete unaudited plan", result, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(`DROP TRIGGER fail_id`); err != nil {
			t.Fatal(err)
		}
		if result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, edits); err != nil || result == nil || result.ChangedCells != 1 {
			t.Fatal("rolled-back identity intent could not retry", result, err)
		}
	}
}

func TestSIVIIdentityOwnedRestorationRefusesPeerOccupancyAndFullRowDrift(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE Sample_Veg SET HeightA=99 WHERE ID=-1`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID) VALUES('outside','NEWOWNER',10000001)`,
		`UPDATE "__VPRO_SIVIIdentityHistory" SET Proposal=replace(Proposal,'SubVegA-SIVI_BC','SubVegD-SIVI')`,
	} {
		service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
		result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original,
			[]siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err == nil || restored != nil {
			t.Fatal("drifted identity history/row/return occupancy was restored", restored, err)
		}
		assertProfileSUFiles(t, service, before)
	}
}

func TestSIVIIdentityOwnedHistoryStrictTransportAndCancellation(t *testing.T) {
	service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
	result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original,
		[]siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")}})
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIIdentityHistory" WHERE ID=?`, result.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var event siviIdentityHistory
	if err := json.Unmarshal([]byte(proposal), &event); err != nil || len(event.Edits) != 1 {
		t.Fatal("complete typed source plan was not recorded", event, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if restored, err := service.restoreSIVIIdentities(ctx, state.ContextID, "108050", result.HistoryID, AuditRestoreRetain); restored != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled restoration returned success", restored, err)
	}
	assertProfileSUFiles(t, service, before)
	if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestoreCancel); err != nil || restored == nil || !restored.Cancelled {
		t.Fatal("explicit restoration cancel failed", restored, err)
	}
	assertProfileSUFiles(t, service, before)
	for _, broken := range []string{
		strings.Replace(proposal, `"Extended":false`, `"Extended":false,"Unknown":true`, 1),
		strings.Replace(proposal, `"rowId":`, `"rowId":null,"missing":`, 1),
		strings.Replace(proposal, `"Plot":"108050"`, `"Plot":"\ud800"`, 1),
	} {
		if broken == proposal {
			t.Fatal("strict history test failed to mutate its actual wire shape")
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIIdentityHistory" SET Proposal=? WHERE ID=?`, broken, result.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err == nil || restored != nil {
			t.Fatal("malformed/unknown history transport was repaired", restored, err)
		}
		assertProfileSUFiles(t, service, before)
	}
}

func TestSIVIIdentityOwnedReservationsCannotBeReassignedAfterRestore(t *testing.T) {
	service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
	edits := []siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("1")}}
	result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, edits)
	if err != nil {
		t.Fatal(err)
	}

	if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestoreRetain); err != nil || restored == nil {
		t.Fatal(restored, err)
	}
	reloaded, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(reloaded, original) {
		t.Fatal("restored source differs from the original", reloaded, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if again, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, reloaded, edits); err == nil || again != nil {
		t.Fatal("restored/released ID was granted to a fresh intent", again, err)
	}
	assertProfileSUFiles(t, service, before)
	var ledger int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=1`).Scan(&ledger); err != nil || ledger != 1 {
		t.Fatal("permanent identity reservation was pruned", ledger, err)
	}
}

func TestSIVIIdentityOwnedBlockedWriteAndRestorationCancellationRetry(t *testing.T) {
	service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
	edits := []siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")}}
	var historyID string
	for _, restoring := range []bool{false, true} {
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		if restoring {
			result, err := service.restoreSIVIIdentities(ctx, state.ContextID, "108050", historyID, AuditRestorePrune)
			if result != nil || !errors.Is(err, context.DeadlineExceeded) {
				t.Error("blocked restoration returned partial/success-shaped result", result, err)
			}
		} else {
			result, err := service.writeSIVIIdentities(ctx, state.ContextID, "108050", false, original, edits)
			if result != nil || !errors.Is(err, context.DeadlineExceeded) {
				t.Error("blocked identity save returned partial/success-shaped result", result, err)
			}
		}
		cancel()
		if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
			t.Error(err)
		}
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
		assertProfileSUFiles(t, service, before)
		if restoring {
			if result, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", historyID, AuditRestorePrune); err != nil || result == nil {
				t.Fatal("cancelled restoration discarded owner/retry", result, err)
			}
		} else {
			result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, edits)
			if err != nil || result == nil {
				t.Fatal("cancelled save discarded owner/retry", result, err)
			}
			historyID = result.HistoryID
		}
	}
}

func TestSIVIIdentityOwnedOriginalNULLAndExtendedSourceRoundTrip(t *testing.T) {
	service, state, db, _, heightEdits := siviWriteFixture(t, false, 0)
	rowID := heightEdits[0].RowID
	if _, err := db.Exec(`UPDATE Sample_Veg SET ID=NULL WHERE rowid=?`, rowID); err != nil {
		t.Fatal(err)
	}
	original, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", true)
	if err != nil {
		t.Fatal(err)
	}
	veg := heightTableSnapshot(t, db, "Sample_Veg")
	result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", true, original,
		[]siviHeightEdit{{rowID, "SubVegA-SIVI", "ID", ProjectMetadataCell{Storage: "null"}, metadataInteger("0")}})
	if err != nil || result == nil || result.HistoryID == "" {
		t.Fatal("nullable source ID required an invented old identity", result, err)
	}
	if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestoreRetain); err != nil || restored == nil {
		t.Fatal("original NULL identity did not restore", restored, err)
	}
	if heightTableSnapshot(t, db, "Sample_Veg") != veg {
		t.Fatal("original NULL was replaced by zero or a new physical row")
	}
}

func TestSIVIIdentityOwnedLateHistoryFailureAndLedgerTriggerRollBack(t *testing.T) {
	for _, setup := range []string{
		`CREATE TABLE "__VPRO_SIVIIdentityHistory"(ID INTEGER PRIMARY KEY,Created TEXT,Proposal TEXT,Restored TEXT)`,
		siviIdentityHistorySQL + `; CREATE TRIGGER fail_history BEFORE INSERT ON "__VPRO_SIVIIdentityHistory" BEGIN SELECT RAISE(ABORT,'stop history'); END`,
		`CREATE TABLE "__VPRO_ChildIdentity"(ChildTable TEXT NOT NULL,ID INTEGER NOT NULL,PRIMARY KEY(ChildTable,ID));
			 CREATE TRIGGER fail_reservation AFTER INSERT ON "__VPRO_ChildIdentity" BEGIN UPDATE Sample_Veg SET HeightA=99 WHERE ID=10000001; END`,
	} {
		service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
		if _, err := db.Exec(setup); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original,
			[]siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")}}); err == nil || result != nil {
			t.Fatal("late history/ledger failure escaped rollback", result, err)
		}
		assertProfileSUFiles(t, service, before)
	}
}

func TestSIVIIdentityOwnedMixedBatchAndUnaffectedAuditedHistory(t *testing.T) {
	service, state, db, original, heightEdits := siviWriteFixture(t, false, 3)
	rowID := heightEdits[0].RowID
	// A completed height audit remains owned by that editor, not ID history.
	height, err := service.writeSIVIHeights(context.Background(), state.ContextID, "108050", false, original, heightEdits[:1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover9) VALUES('108050','SECOND',10000002,0)`); err != nil {
		t.Fatal(err)
	}
	original, err = service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	var second string
	if err := db.QueryRow(`SELECT CAST(rowid AS TEXT) FROM Sample_Veg WHERE ID=10000002`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	audit := heightTableSnapshot(t, db, "Sample_Audit")
	veg := heightTableSnapshot(t, db, "Sample_Veg")
	heightHistory := heightTableSnapshot(t, db, siviHeightHistoryTable)
	result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original, []siviHeightEdit{
		{rowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")},
		{second, "SubVegD-SIVI", "ID", metadataInteger("10000002"), ProjectMetadataCell{Storage: "null"}},
	})
	if err != nil || result == nil || result.ChangedCells != 2 {
		t.Fatal("mixed nullable/integer identities were not atomic", result, err)
	}
	blocked := databaseBytes(t, service.projects.sqlite.attachments)
	if restored, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", height.HistoryID, AuditRestorePrune); err == nil || restored != nil {
		t.Fatal("old editor restored across a changed identity", restored, err)
	}
	assertProfileSUFiles(t, service, blocked)
	if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil || restored == nil || restored.RestoredRows != 2 {
		t.Fatal("mixed identities did not restore atomically", restored, err)
	}
	if heightTableSnapshot(t, db, "Sample_Audit") != audit || heightTableSnapshot(t, db, "Sample_Veg") != veg ||
		heightTableSnapshot(t, db, siviHeightHistoryTable) != heightHistory {
		t.Fatal("identity restoration inherited peer audit/history mutations")
	}
	if restored, err := service.restoreSIVIHeights(context.Background(), state.ContextID, "108050", height.HistoryID, AuditRestorePrune); err != nil || restored == nil {
		t.Fatal("restored identity prevented separately owned height recovery", restored, err)
	}
}

func TestSIVIIdentityOwnedRestorationRejectsChangedOrReplacedDuplicatePeer(t *testing.T) {
	for _, replace := range []bool{false, true} {
		service, state, db, _, heightEdits := siviWriteFixture(t, false, 3)
		inserted, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover9) VALUES('108050','DUP',10000001,0)`)
		if err != nil {
			t.Fatal(err)
		}
		peer, err := inserted.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		original, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.writeSIVIIdentities(context.Background(), state.ContextID, "108050", false, original,
			[]siviHeightEdit{{heightEdits[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")}})
		if err != nil {
			t.Fatal(err)
		}
		if replace {
			if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE rowid=?`, peer); err != nil {
				t.Fatal(err)
			}
			replacement, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover9) VALUES('108050','REPLACEMENT',10000001,0)`)
			if err != nil {
				t.Fatal(err)
			}
			reused, err := replacement.LastInsertId()
			if err != nil || reused != peer {
				t.Fatal("acceptance fixture did not exercise automatic physical-rowid reuse", reused, peer, err)
			}
		} else {
			if _, err := db.Exec(`UPDATE Sample_Veg SET Species='CHANGED' WHERE rowid=?`, peer); err != nil {
				t.Fatal(err)
			}
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err == nil || restored != nil {
			t.Fatal("changed or replacement duplicate peer authorized restoration", restored, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(`UPDATE Sample_Veg SET Species='DUP' WHERE rowid=?`, peer); err != nil {
			t.Fatal(err)
		}
		if restored, err := service.restoreSIVIIdentities(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil || restored == nil {
			t.Fatal("restored exact peer source did not allow explicit retry", restored, err)
		}
	}
}
