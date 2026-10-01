package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParentCodeRestoreAllFieldsBoundsAndAliases(t *testing.T) {
	for _, field := range parentCodeFields(FS882Header{}) {
		for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
			t.Run(field.name+"/"+route, func(t *testing.T) {
				invalid := strings.Repeat("x", field.maximum+1)
				s, db, plot, id := siteCodeRestoreFixture(t, field.name, &invalid, qualityString("X"))
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
					_, err = s.RestoreSelectedAuditRecords(plot, []string{id}, action)
				default:
					err = s.RestoreAuditRecords(plot, route == "marked-prune")
				}
				if err == nil || !strings.Contains(err.Error(), field.name) {
					t.Fatal("invalid restore accepted", err)
				}
				assertSubstrateSnapshot(t, db, before)
			})
		}
		valid := "z"
		if field.maximum >= 2 {
			valid = strings.Repeat("z", field.maximum-2) + "\U0001F600"
		}
		for i, target := range []*string{nil, &valid} {
			t.Run(field.name+"/valid/"+strconv.Itoa(i), func(t *testing.T) {
				s, db, plot, id := siteCodeRestoreFixture(t, field.name, target, qualityString("historical-too-long-"+strings.Repeat("x", field.maximum)))
				count := auditCount(t, db, plot)
				if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				action, pruned := AuditRestoreRetain, 0
				if i == 0 {
					action, pruned = AuditRestorePrune, 1
				}
				result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, action)
				if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != pruned {
					t.Fatal("valid restore failed", err)
				}
				got, err := s.GetPlot(plot)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.name).Interface(), target) ||
					auditCount(t, db, plot) != count-pruned {
					t.Fatal("raw restore/audit retention differs")
				}
			})
		}
		for _, invalid := range []string{"", strings.Repeat("x", field.maximum-1) + "\U0001F600", string([]byte{0xff})} {
			if err := validateParentCodeRestoreValue("Env", field.name, invalid); err == nil {
				t.Fatal("restore physical boundary bypassed", field.name)
			}
		}
		if err := validateParentCodeRestoreValue("Env", field.name, 7); err == nil {
			t.Fatal("restore type bypassed")
		}
	}
}

func TestParentCodeRestoreOwnershipSchemaAndCancel(t *testing.T) {
	for _, mutate := range []string{
		`UPDATE Sample_Audit SET PlotNumber='foreign' WHERE rowid=?`,
		`UPDATE Sample_Audit SET Project='foreign' WHERE rowid=?`,
		`UPDATE Sample_Audit SET ID=0 WHERE rowid=?`,
		`UPDATE Sample_Audit SET AfterEdit='stale' WHERE rowid=?`,
		`ALTER TABLE Sample_Env DROP COLUMN HumusFormPhase`,
	} {
		t.Run(mutate, func(t *testing.T) {
			s, db, plot, id := siteCodeRestoreFixture(t, "HumusFormPhase", qualityString("manual FULL phase"), qualityString("AB"))
			if err := s.CreatePlot(FS882Header{PlotNumber: "foreign"}); err != nil {
				t.Fatal(err)
			}
			args := []any{id}
			if strings.HasPrefix(mutate, "ALTER") {
				args = nil
			}
			if _, err := db.Exec(mutate, args...); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestorePrune); err == nil {
				t.Fatal("foreign/child/stale/schema restore accepted")
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
	s, db, plot, id := siteCodeRestoreFixture(t, "FloodingRegimeDur", qualityString("invalid"), qualityString("X"))
	before := substrateSnapshot(t, db)
	if result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel); err != nil || !result.Cancelled {
		t.Fatal("cancel changed/validated history")
	}
	if result, err := s.RestoreSelectedAuditRecords(plot, nil, AuditRestorePrune); err != nil || *result != (AuditRestoreResult{}) {
		t.Fatal("empty selection changed history")
	}
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id, id}, AuditRestorePrune); err == nil {
		t.Fatal("duplicate selection accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestParentCodeRestoreRollbackAndIndependentPartners(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER parent_restore_reject BEFORE UPDATE OF CoarseFragLith1 ON Sample_Env BEGIN SELECT RAISE(ABORT,'later restore rejected'); END`,
		`CREATE TRIGGER parent_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'prune rejected'); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			s, db, plot, first := siteCodeRestoreFixture(t, "CoarseFragLith1", qualityString("rawFULLitem1"), qualityString("AB"))
			if _, err := db.Exec(`UPDATE Sample_Env SET CoarseFragLith2='CD',BedrockGeology1='EF',HumusForm='GH' WHERE PlotNumber=?`, plot); err != nil {
				t.Fatal(err)
			}
			inserted, err := db.Exec(`INSERT INTO Sample_Audit
			 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
			 VALUES ('Sample','RestoreTester',?,'_Env','HumusFormPhase','2026-09-30 12:00:01','phase q''X4',NULL,0,0)`, plot)
			if err != nil {
				t.Fatal(err)
			}
			row, err := inserted.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{first, strconv.FormatInt(row, 10)}
			if err := s.SetAuditRestoreSelection(plot, ids); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune); err == nil {
				t.Fatal("coupled restore failure accepted")
			}
			assertSubstrateSnapshot(t, db, before)
			if _, err := db.Exec(`DROP TRIGGER parent_restore_reject`); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune)
			if err != nil || result.RestoredRows != 2 || result.PrunedAuditRows != 2 {
				t.Fatal("coupled retry failed", err)
			}
			got, err := s.GetPlot(plot)
			if err != nil || !sameSiteCode(got.CoarseFragLith1, qualityString("rawFULLitem1")) ||
				!sameSiteCode(got.HumusFormPhase, qualityString("phase q'X4")) ||
				!sameSiteCode(got.CoarseFragLith2, qualityString("CD")) || !sameSiteCode(got.BedrockGeology1, qualityString("EF")) ||
				!sameSiteCode(got.HumusForm, qualityString("GH")) {
				t.Fatal("raw independent restore targets/partners changed")
			}
		})
	}
}
