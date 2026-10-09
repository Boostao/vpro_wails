package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func siteCodeRestoreFixture(t *testing.T, member string, before, after *string) (*PlotService, *sql.DB, string, string) {
	t.Helper()
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "CODERESTORE", FieldNotes: qualityString("Env after")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET `+quoteHeaderIdentifier(member)+`=? WHERE PlotNumber=?`, after, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env',?,'2026-09-30 12:00:00',?,?,0,0)`, h.PlotNumber, member, before, after)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return s, db, h.PlotNumber, strconv.FormatInt(id, 10)
}

func TestSiteCodeRestoreAllAliasesRejectInvalidPreserveSelection(t *testing.T) {
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		invalids := []string{"", strings.Repeat("x", field.maximum+1),
			strings.Repeat("x", field.maximum-1) + "\U0001F600", string([]byte{0xFF})}
		if field.maximum == 2 {
			invalids = append(invalids, "zz", "at", "A")
		}
		for _, invalid := range invalids {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(field.name+"/"+route+"/"+strconv.Itoa(len(invalid)), func(t *testing.T) {
					s, db, plot, id := siteCodeRestoreFixture(t, field.name, &invalid, qualityString("AT"))
					result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env','SiteNotes','2026-09-30 12:01:00','Env before','Env after',-1,0)`, plot)
					if err != nil {
						t.Fatal(err)
					}
					envID, err := result.LastInsertId()
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(route, "marked") {
						if _, err := db.Exec(`UPDATE Sample_Audit SET Restore=-1 WHERE rowid=?`, id); err != nil {
							t.Fatal(err)
						}
					}
					before := substrateSnapshot(t, db)
					switch route {
					case "selection":
						err = s.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := AuditRestoreRetain
						if route == "prune" {
							action = AuditRestorePrune
						}
						_, err = s.RestoreSelectedAuditRecords(plot, []string{strconv.FormatInt(envID, 10), id}, action)
					default:
						err = s.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), field.name) || !strings.Contains(err.Error(), id) {
						t.Fatalf("restore alias bypassed %s: %v", field.name, err)
					}
					assertSubstrateSnapshot(t, db, before)
				})
			}
		}
	}
}

func TestSiteCodeRestoreValidAndNullWithoutCatalogue(t *testing.T) {
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		values := []*string{nil, qualityString("AT"), qualityString("NA")}
		if field.maximum == 8 {
			values = append(values, qualityString("O'"), qualityString("unknown"),
				qualityString("rawCase"), qualityString(strings.Repeat("x", 6)+"\U0001F600"))
		}
		for _, target := range values {
			s, db, plot, id := siteCodeRestoreFixture(t, field.name, target, qualityString("historical-overlength"))
			before := auditCount(t, db, plot)
			if target == nil || field.maximum == 8 {
				if err := os.WriteFile(filepath.Join(s.projects.root, "site-codes.db"), []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain)
			if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 0 {
				t.Fatalf("valid restore failed: %#v %v", result, err)
			}
			got, err := s.GetPlot(plot)
			if err != nil {
				t.Fatal(err)
			}
			actual := reflect.ValueOf(*got).FieldByName(field.name).Interface().(*string)
			if !reflect.DeepEqual(actual, target) || auditCount(t, db, plot) != before || *got.FieldNotes != "Env after" {
				t.Fatal("restore normalized value or changed history/partner")
			}
		}
	}
	for _, failure := range []string{"corrupt", "closed"} {
		s, db, plot, id := siteCodeRestoreFixture(t, "Exposure1", qualityString("AT"), qualityString("NA"))
		if failure == "closed" {
			var err error
			s.siteCodes, err = NewSiteCodeService(s.projects.root)
			if err != nil || s.siteCodes.Close() != nil {
				t.Fatal("closed catalogue fixture failed")
			}
		} else if err := os.WriteFile(filepath.Join(s.projects.root, "site-codes.db"), []byte("corrupt"), 0600); err != nil {
			t.Fatal(err)
		}
		before := substrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil ||
			!strings.Contains(err.Error(), "catalogue") {
			t.Fatalf("restore new Exposure bypassed unavailable catalogue: %v", err)
		}
		assertSubstrateSnapshot(t, db, before)
	}
}

func TestSiteCodeRestoreRollbackRetryCancelAndNoop(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER codes_restore_reject BEFORE UPDATE OF Exposure1 ON Sample_Env BEGIN SELECT RAISE(ABORT,'data restore rejected'); END`,
		`CREATE TRIGGER codes_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'history pruning rejected'); END`,
	} {
		s, db, plot, id := siteCodeRestoreFixture(t, "Exposure1", qualityString("AT"), qualityString("NA"))
		result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env','SiteNotes','2026-09-30 12:01:00','Env before','Env after',0,0)`, plot)
		if err != nil {
			t.Fatal(err)
		}
		envID, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{strconv.FormatInt(envID, 10), id}
		if err := s.SetAuditRestoreSelection(plot, ids); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := substrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune); err == nil {
			t.Fatal("restore data/audit failure silently succeeded")
		}
		assertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER codes_restore_reject`); err != nil {
			t.Fatal(err)
		}
		restored, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune)
		if err != nil || restored.RestoredRows != 2 || restored.PrunedAuditRows != 2 {
			t.Fatalf("retry failed: %#v %v", restored, err)
		}
	}
	historical := qualityString("historical-overlength")
	s, db, plot, id := siteCodeRestoreFixture(t, "Exposure1", historical, historical)
	before := substrateSnapshot(t, db)
	cancelled, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel)
	if err != nil || !cancelled.Cancelled {
		t.Fatal("cancel validated/changed invalid selection")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid audit restored")
	}
	assertSubstrateSnapshot(t, db, before)
}
