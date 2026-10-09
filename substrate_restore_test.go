package main

import (
	"database/sql"
	"strconv"
	"strings"
	"testing"
)

func substrateRestoreFixture(t *testing.T, column string, before, after *string) (*PlotService, *sql.DB, string, string) {
	t.Helper()
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "SUBREST", FieldNotes: qualityString("Env after")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET `+quoteHeaderIdentifier(column)+`=? WHERE PlotNumber=?`, after, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env',?,'2026-09-30 12:00:00',?,?,0,0)`,
		h.PlotNumber, column, before, after)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return s, db, h.PlotNumber, strconv.FormatInt(id, 10)
}

func substratePartnerAudit(t *testing.T, db *sql.DB, plot string) string {
	t.Helper()
	result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env','SiteNotes','2026-09-30 12:01:00','Env before','Env after',-1,0)`, plot)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return strconv.FormatInt(id, 10)
}

func TestSubstrateRestoreEveryEntryPointRejectsInvalidPreservesSelection(t *testing.T) {
	for _, field := range substrateHeaderFields(FS882Header{}) {
		for _, invalid := range []string{"3.5e38", "-3.5e38", "NaN", "+Inf", "-Inf", "1e400"} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(field.property+"/"+invalid+"/"+route, func(t *testing.T) {
					s, db, plot, id := substrateRestoreFixture(t, field.column, &invalid, qualityString("1.25"))
					partner := substratePartnerAudit(t, db, plot)
					if strings.HasPrefix(route, "marked") {
						if _, err := db.Exec(`UPDATE Sample_Audit SET Restore=-1 WHERE rowid=?`, id); err != nil {
							t.Fatal(err)
						}
					}
					before := substrateSnapshot(t, db)
					var err error
					switch route {
					case "selection":
						err = s.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := AuditRestoreRetain
						if route == "prune" {
							action = AuditRestorePrune
						}
						_, err = s.RestoreSelectedAuditRecords(plot, []string{partner, id}, action)
					case "marked-retain", "marked-prune":
						err = s.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), id) {
						t.Fatalf("invalid substrate restoration bypassed %s: %v", route, err)
					}
					assertSubstrateSnapshot(t, db, before)
				})
			}
		}
	}
}

func TestSubstrateRestoreValidNullAndHistoricalClear(t *testing.T) {
	for _, field := range substrateHeaderFields(FS882Header{}) {
		for _, target := range []*string{nil, qualityString("0"), qualityString("-3.75"), qualityString(".125"),
			qualityString("99"), qualityString("100"), qualityString("101"), qualityString("1.234567890123"),
			qualityString("3.4028234663852886e38"), qualityString("-3.4028234663852886e38")} {
			name := "NULL"
			if target != nil {
				name = *target
			}
			t.Run(field.property+"/"+name, func(t *testing.T) {
				// Imported overflow can be restored away from, but never into.
				s, db, plot, id := substrateRestoreFixture(t, field.column, target, qualityString("1e40"))
				seedHeight(t, db, plot, 0)
				if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				before := substrateSnapshot(t, db)
				result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain)
				if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 0 {
					t.Fatalf("valid restore from imported overflow: %#v %v", result, err)
				}
				got, err := s.GetPlot(plot)
				if err != nil {
					t.Fatal(err)
				}
				want := FS882Header{PlotNumber: plot, FieldNotes: qualityString("Env after")}
				if target != nil {
					value, err := strconv.ParseFloat(*target, 64)
					if err != nil {
						t.Fatal(err)
					}
					setSubstrate(&want, field.column, &value)
				}
				// Raw precision is compared against the decoded float64 DTO.
				for i, value := range substrateHeaderFields(*got) {
					if !sameHeightValue(value.value, substrateHeaderFields(want)[i].value) {
						t.Fatal("restore normalized substrate precision or changed partner")
					}
				}
				if got.FieldNotes == nil || *got.FieldNotes != "Env after" {
					t.Fatal("restore changed unrelated Env value")
				}
				after := substrateSnapshot(t, db)
				for _, table := range []string{"Admin", "Veg", "Humus", "Mineral", "Other", "Metadata", "Audit"} {
					if before[table] != after[table] {
						t.Fatalf("restore changed unrelated %s/history", table)
					}
				}
			})
		}
	}
}

func TestSubstrateRestoreCancelNoopSchemaAndRollback(t *testing.T) {
	for _, field := range substrateHeaderFields(FS882Header{}) {
		t.Run(field.property, func(t *testing.T) {
			s, db, plot, id := substrateRestoreFixture(t, field.column, qualityString("101"), qualityString("1.25"))
			partner := substratePartnerAudit(t, db, plot)
			if err := s.SetAuditRestoreSelection(plot, []string{partner, id}); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel)
			if err != nil || !result.Cancelled {
				t.Fatalf("cancel: %#v %v", result, err)
			}
			assertSubstrateSnapshot(t, db, before)
			for _, trigger := range []string{
				`CREATE TRIGGER substrate_restore_reject BEFORE UPDATE OF ` + quoteHeaderIdentifier(field.column) +
					` ON Sample_Env BEGIN SELECT RAISE(ABORT,'data restore failure'); END`,
				`CREATE TRIGGER substrate_restore_reject BEFORE DELETE ON Sample_Audit WHEN OLD.EditField='` + field.column +
					`' BEGIN SELECT RAISE(ABORT,'prune failure'); END`,
			} {
				if _, err := db.Exec(trigger); err != nil {
					t.Fatal(err)
				}
				if _, err := s.RestoreSelectedAuditRecords(plot, []string{partner, id}, AuditRestorePrune); err == nil {
					t.Fatal("restore trigger failure silently succeeded")
				}
				assertSubstrateSnapshot(t, db, before)
				if err := s.RestoreAuditRecords(plot, true); err == nil {
					t.Fatal("marked restore trigger failure silently succeeded")
				}
				assertSubstrateSnapshot(t, db, before)
				if _, err := db.Exec(`DROP TRIGGER substrate_restore_reject`); err != nil {
					t.Fatal(err)
				}
			}
			result, err = s.RestoreSelectedAuditRecords(plot, []string{partner, id}, AuditRestorePrune)
			if err != nil || result.RestoredRows != 2 || result.PrunedAuditRows != 2 {
				t.Fatalf("restore retry: %#v %v", result, err)
			}
			// Unsupported schema must refuse restoration, not default to NULL.
			if _, err := db.Exec(`INSERT INTO Sample_Audit
 (Project,User,PlotNumber,"Table",EditField,EditWhen,BeforeEdit,AfterEdit,Restore,Flag)
 VALUES ('Sample','RestoreTester',?,'_Env',?,'2026-09-30 12:02:00','1.25','101',-1,0)`, plot, field.column); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + quoteHeaderIdentifier(field.column)); err != nil {
				t.Fatal(err)
			}
			before = substrateSnapshot(t, db)
			if err := s.RestoreAuditRecords(plot, false); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("missing schema restore accepted: %v", err)
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
	s, db, plot, id := substrateRestoreFixture(t, "SubstrateWater", qualityString("1e40"), qualityString("1e40"))
	before := substrateSnapshot(t, db)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid history was restored")
	}
	assertSubstrateSnapshot(t, db, before)
}
