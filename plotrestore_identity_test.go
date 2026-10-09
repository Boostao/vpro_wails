package main

import (
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func vegPreservationFixture(t *testing.T) (*PlotService, *sql.DB, string) {
	t.Helper()
	s, db := restoreFixture(t)
	// Exact native Access cleanup fixture: only the selected Height1 changes.
	if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species,Cover1,Cover5a,Height1,DC) VALUES
		('CHILD1',74005,'PSEUMEN',0,NULL,2,NULL),
		('CHILD1',74006,'PSEUMEN',NULL,NULL,8,NULL),
		('CHILD1',74007,'PSEUMEN',NULL,NULL,NULL,1),
		('CHILD1',74008,'PSEUMEN',NULL,50,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	id := int64(74005)
	row := restoreAuditFixture(t, db, "CHILD1", "_Veg", "Height1", &id, 1.25, 2.0)
	return s, db, row
}

type preservedVegRow struct {
	RowID                    int64
	ID                       sql.NullString
	IDType, Plot, Species    string
	Cover1, Cover5a, Height1 sql.NullFloat64
	DC                       sql.NullInt64
}

func vegPreservationSnapshot(t *testing.T, db *sql.DB) []preservedVegRow {
	t.Helper()
	rows, err := db.Query(`SELECT rowid, CAST(ID AS TEXT), typeof(ID), PlotNumber, Species,
		Cover1, Cover5a, Height1, DC FROM Sample_Veg
		WHERE PlotNumber IN ('CHILD1','CHILD2') ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var snapshot []preservedVegRow
	for rows.Next() {
		var row preservedVegRow
		if err := rows.Scan(&row.RowID, &row.ID, &row.IDType, &row.Plot, &row.Species,
			&row.Cover1, &row.Cover5a, &row.Height1, &row.DC); err != nil {
			t.Fatal(err)
		}
		snapshot = append(snapshot, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func vegPreservationLedger(t *testing.T, db *sql.DB, exists bool) {
	t.Helper()
	var found int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = '__VPRO_ChildIdentity'`).Scan(&found); err != nil {
		t.Fatal(err)
	}
	if !exists {
		if found != 0 {
			t.Fatal("field restoration created an identity ledger")
		}
		return
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity"`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("field restoration changed ledger entries: %d %v", count, err)
	}
	var table string
	var id int64
	if err := db.QueryRow(`SELECT ChildTable, ID FROM "__VPRO_ChildIdentity"`).Scan(&table, &id); err != nil ||
		table != quoteHeaderIdentifier("Sample_Veg") || id != 99 {
		t.Fatalf("field restoration changed existing reservation: %q %d %v", table, id, err)
	}
}

func protectVegPreservationLedger(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable","ID"));
		INSERT INTO "__VPRO_ChildIdentity" VALUES ('"Sample_Veg"',99);
		CREATE TRIGGER no_ledger_insert BEFORE INSERT ON "__VPRO_ChildIdentity"
		BEGIN SELECT RAISE(ABORT,'unexpected ledger insert'); END;
		CREATE TRIGGER no_ledger_update BEFORE UPDATE ON "__VPRO_ChildIdentity"
		BEGIN SELECT RAISE(ABORT,'unexpected ledger update'); END;
		CREATE TRIGGER no_ledger_delete BEFORE DELETE ON "__VPRO_ChildIdentity"
		BEGIN SELECT RAISE(ABORT,'unexpected ledger delete'); END`); err != nil {
		t.Fatal(err)
	}
}

func TestPlotRestore_PreservesExactNativeVegFixture(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		for _, existingLedger := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-ledger-%t", action, existingLedger), func(t *testing.T) {
				s, db, selected := vegPreservationFixture(t)
				if existingLedger {
					protectVegPreservationLedger(t, db)
				}
				if _, err := db.Exec(`CREATE TRIGGER no_veg_delete BEFORE DELETE ON Sample_Veg
					BEGIN SELECT RAISE(ABORT,'unexpected vegetation deletion'); END`); err != nil {
					t.Fatal(err)
				}
				expected := vegPreservationSnapshot(t, db)
				expected[0].Height1.Float64 = 1.25
				if err := s.SetAuditRestoreSelection("CHILD1", []string{selected}); err != nil {
					t.Fatal(err)
				}
				result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{selected}, action)
				if err != nil || result.RestoredRows != 1 || result.CleanedVegRows != 0 {
					t.Fatalf("field-only restore: %+v %v", result, err)
				}
				wantAudit := 1
				if action == AuditRestorePrune {
					wantAudit = 0
				}
				if result.PrunedAuditRows != 1-wantAudit || auditCount(t, db, "CHILD1") != wantAudit {
					t.Fatal("retain/prune altered the wrong history")
				}
				if got := vegPreservationSnapshot(t, db); !reflect.DeepEqual(got, expected) {
					t.Fatalf("field restoration changed unrelated rows/identities: %+v, want %+v", got, expected)
				}
				vegPreservationLedger(t, db, existingLedger)
			})
		}
	}
}

func TestPlotRestore_UnrelatedImportedIdentitiesSurvive(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			s, db, selected := vegPreservationFixture(t)
			for _, id := range []any{int64(1), int64(0), int64(math.MinInt32), int64(math.MaxInt32),
				nil, int64(math.MaxInt32) + 1, int64(math.MinInt32) - 1, int64(9007199254740993), 1.25, "invalid"} {
				if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species,Height1) VALUES ('CHILD1',?,'UNRELATED',8)`, id); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species,Height1)
				VALUES ('CHILD2',2,'FOREIGN',8)`); err != nil {
				t.Fatal(err)
			}
			expected := vegPreservationSnapshot(t, db)
			expected[0].Height1.Float64 = 1.25
			result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{selected}, action)
			if err != nil || result.CleanedVegRows != 0 {
				t.Fatalf("unrelated imported identity prevented field restoration: %+v %v", result, err)
			}
			if got := vegPreservationSnapshot(t, db); !reflect.DeepEqual(got, expected) {
				t.Fatal("restoration modified unrelated imported or foreign identities")
			}
			vegPreservationLedger(t, db, false)
		})
	}
}

func TestPlotRestore_SurvivingIdentitiesCannotBeReallocated(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			s, db, selected := vegPreservationFixture(t)
			if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES
				('CHILD1',1,'IMPORTED'),('CHILD1',0,'ZERO'),('CHILD1',-2147483648,'NEGATIVE'),
				('CHILD1',2147483647,'MAXIMUM'),('CHILD2',2,'FOREIGN')`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{selected}, action); err != nil {
				t.Fatal(err)
			}
			expected := vegPreservationSnapshot(t, db)
			vegPreservationLedger(t, db, false)
			if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD1", Species: "NEW"}); err != nil {
				t.Fatal(err)
			}
			var allocated int64
			if err := db.QueryRow(`SELECT ID FROM Sample_Veg WHERE PlotNumber = 'CHILD1' AND Species = 'NEW'`).Scan(&allocated); err != nil ||
				allocated != 3 {
				t.Fatalf("allocator reused a surviving imported identity: %d %v", allocated, err)
			}
			got := vegPreservationSnapshot(t, db)
			if len(got) != len(expected)+1 || !reflect.DeepEqual(got[:len(expected)], expected) {
				t.Fatal("subsequent explicit creation changed surviving vegetation records")
			}
		})
	}
}

func TestPlotRestore_FieldOnlyVegRollback(t *testing.T) {
	for _, failure := range []string{"data-update", "audit-prune"} {
		for _, existingLedger := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-ledger-%t", failure, existingLedger), func(t *testing.T) {
				s, db, selected := vegPreservationFixture(t)
				expected := vegPreservationSnapshot(t, db)
				if existingLedger {
					protectVegPreservationLedger(t, db)
				}
				trigger := `CREATE TRIGGER fail_restore BEFORE UPDATE ON Sample_Veg
					BEGIN SELECT RAISE(ABORT,'restore failure'); END`
				if failure == "audit-prune" {
					trigger = `CREATE TRIGGER fail_prune BEFORE DELETE ON Sample_Audit
						BEGIN SELECT RAISE(ABORT,'prune failure'); END`
				}
				if _, err := db.Exec(trigger); err != nil {
					t.Fatal(err)
				}
				if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{selected}, AuditRestorePrune); err == nil {
					t.Fatal("expected transactional failure")
				}
				if got := vegPreservationSnapshot(t, db); !reflect.DeepEqual(got, expected) ||
					auditCount(t, db, "CHILD1") != 1 {
					t.Fatal("failure left partial field changes, identity changes or audit pruning")
				}
				vegPreservationLedger(t, db, existingLedger)
			})
		}
	}
}
