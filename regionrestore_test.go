package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestRegionRestoreAllAliasesAndRawNullableTargets(t *testing.T) {
	for _, field := range regionHeaderFields(FS882Header{}) {
		for _, invalid := range []string{"", strings.Repeat("x", field.maximum+1), strings.Repeat("x", field.maximum-1) + "\U0001F600", string([]byte{0xff})} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				s, db, plot, id := siteCodeRestoreFixture(t, field.name, &invalid, qualityString("ok"))
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
					t.Fatalf("invalid %s target bypassed %s: %v", field.name, route, err)
				}
				assertSubstrateSnapshot(t, db, before)
			}
		}
		for _, target := range []*string{nil, qualityString("O'"), qualityString("aB"), qualityString(" "), qualityString(strings.Repeat("x", field.maximum-2) + "\U0001F600")} {
			s, db, plot, id := siteCodeRestoreFixture(t, field.name, target, qualityString("historical-too-long"))
			count := auditCount(t, db, plot)
			if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain)
			if err != nil || result.RestoredRows != 1 {
				t.Fatalf("valid target restore %v %#v", err, result)
			}
			got, err := s.GetPlot(plot)
			if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.name).Interface(), target) || auditCount(t, db, plot) != count {
				t.Fatal("raw restore/history mismatch")
			}
		}
		if err := validateRegionRestoreValue("Env", field.name, 7); err == nil {
			t.Fatal("non-string restore accepted")
		}
	}
}

func TestRegionRestoreRollbackRetryAndCancel(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER region_restore_reject BEFORE UPDATE OF Ecosection ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER region_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'prune rejected'); END`,
	} {
		s, db, plot, id := siteCodeRestoreFixture(t, "Ecosection", qualityString("o'"), qualityString("AB"))
		if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := substrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestorePrune); err == nil {
			t.Fatal("restore failure accepted")
		}
		assertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER region_restore_reject`); err != nil {
			t.Fatal(err)
		}
		result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestorePrune)
		if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 1 {
			t.Fatalf("retry failed: %#v %v", result, err)
		}
	}
	invalid := qualityString("historical-too-long")
	s, db, plot, id := siteCodeRestoreFixture(t, "FSRegionDistrict", invalid, invalid)
	before := substrateSnapshot(t, db)
	result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel)
	if err != nil || !result.Cancelled {
		t.Fatal("cancel changed/validated invalid target")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid target grandfathered")
	}
	assertSubstrateSnapshot(t, db, before)
}
