package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSoilPhysicalRestoreAllRoutes(t *testing.T) {
	for _, field := range soilHeaderFields(FS882Header{}) {
		for i, invalid := range []string{"", "12345", "abc\U0001F600", string([]byte{0xff})} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(field.name+"/invalid/"+strconv.Itoa(i)+"/"+route, func(t *testing.T) {
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
						t.Fatalf("invalid %s restore bypassed %s: %v", field.name, route, err)
					}
					assertSubstrateSnapshot(t, db, before)
				})
			}
		}
		for i, target := range []*string{nil, qualityString("q'X4"), qualityString("aB"), qualityString(" "),
			qualityString("\nX"), qualityString("e\u0301"), qualityString("a\U0001F600b")} {
			t.Run(field.name+"/valid/"+strconv.Itoa(i), func(t *testing.T) {
				s, db, plot, id := siteCodeRestoreFixture(t, field.name, target, qualityString("historical-too-long"))
				count := auditCount(t, db, plot)
				if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain)
				if err != nil || result.RestoredRows != 1 {
					t.Fatalf("valid restore failed: %#v %v", result, err)
				}
				got, err := s.GetPlot(plot)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.name).Interface(), target) || auditCount(t, db, plot) != count {
					t.Fatal("restore repaired raw text or changed audit history")
				}
			})
		}
		if err := validateSoilRestoreValue("Env", field.name, 7); err == nil {
			t.Fatal("non-string restore target accepted")
		}
	}
}

func TestSoilPhysicalRestoreRollbackCancelAndOwnership(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER soil_restore_reject BEFORE UPDATE OF SoilClassGroup ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER soil_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'prune rejected'); END`,
	} {
		s, db, plot, id := siteCodeRestoreFixture(t, "SoilClassGroup", qualityString("q'X4"), qualityString("AB"))
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
		if _, err := db.Exec(`DROP TRIGGER soil_restore_reject`); err != nil {
			t.Fatal(err)
		}
		result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestorePrune)
		if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 1 {
			t.Fatalf("restore retry failed %#v %v", result, err)
		}
	}
	for _, mutate := range []string{
		`UPDATE Sample_Audit SET PlotNumber='foreign' WHERE rowid=?`,
		`UPDATE Sample_Audit SET Project='foreign' WHERE rowid=?`,
		`UPDATE Sample_Audit SET ID=0 WHERE rowid=?`,
		`UPDATE Sample_Audit SET AfterEdit='stale' WHERE rowid=?`,
	} {
		s, db, plot, id := siteCodeRestoreFixture(t, "SoilClassSubGroup", qualityString("Z9"), qualityString("CA"))
		if err := s.CreatePlot(FS882Header{PlotNumber: "foreign"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(mutate, id); err != nil {
			t.Fatal(err)
		}
		before := substrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestorePrune); err == nil {
			t.Fatal("foreign/child/stale soil restore accepted")
		}
		assertSubstrateSnapshot(t, db, before)
	}

	invalid := qualityString("historical-too-long")
	s, db, plot, id := siteCodeRestoreFixture(t, "SoilClassGroup", invalid, invalid)
	before := substrateSnapshot(t, db)
	result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel)
	if err != nil || !result.Cancelled {
		t.Fatal("cancel validated or changed invalid soil history")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil {
		t.Fatal("invalid no-change restore grandfathered")
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestSoilPhysicalRestoreCoupledAndSchemaPending(t *testing.T) {
	s, db, plot, groupID := siteCodeRestoreFixture(t, "SoilClassGroup", qualityString("1234"), qualityString("DBC"))
	if _, err := db.Exec(`UPDATE Sample_Env SET SoilClassSubGroup='CA' WHERE PlotNumber=?`, plot); err != nil {
		t.Fatal(err)
	}
	inserted, err := db.Exec(`INSERT INTO Sample_Audit
	 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
	 VALUES ('Sample','RestoreTester',?,'_Env','SoilClassSubGroup','2026-09-30 12:00:01','Z9','CA',0,0)`, plot)
	if err != nil {
		t.Fatal(err)
	}
	rowID, err := inserted.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	subgroupID := strconv.FormatInt(rowID, 10)
	ids := []string{groupID, subgroupID}
	if err := s.SetAuditRestoreSelection(plot, ids); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER soil_coupled_reject BEFORE UPDATE OF SoilClassGroup ON Sample_Env BEGIN SELECT RAISE(ABORT,'later group restore rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	if _, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune); err == nil {
		t.Fatal("coupled restore failure accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER soil_coupled_reject`); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune)
	if err != nil || result.RestoredRows != 2 || result.PrunedAuditRows != 2 {
		t.Fatalf("coupled retry failed %#v %v", result, err)
	}
	got, err := s.GetPlot(plot)
	if err != nil || !sameSiteCode(got.SoilClassGroup, qualityString("1234")) || !sameSiteCode(got.SoilClassSubGroup, qualityString("Z9")) {
		t.Fatal("coupled restore changed raw targets")
	}
	s, db, plot, groupID = siteCodeRestoreFixture(t, "SoilClassGroup", qualityString("1234"), qualityString("DBC"))
	if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN SoilClassGroup`); err != nil {
		t.Fatal(err)
	}
	before = substrateSnapshot(t, db)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{groupID}, AuditRestorePrune); err == nil {
		t.Fatal("schema-pending restore accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}
