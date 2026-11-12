package plotentry

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSoilPhysicalRestoreAllRoutes(t *testing.T) {
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		for i, invalid := range []string{"", "12345", "abc\U0001F600", string([]byte{0xff})} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(field.Name+"/invalid/"+strconv.Itoa(i)+"/"+route, func(t *testing.T) {
					s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, field.Name, &invalid, vprotest.QualityString("ok"))
					if strings.HasPrefix(route, "marked") {
						if _, err := db.Exec(`UPDATE Sample_Audit SET Restore=-1 WHERE rowid=?`, id); err != nil {
							t.Fatal(err)
						}
					}
					before := vprotest.SubstrateSnapshot(t, db)
					var err error
					switch route {
					case "selection":
						err = s.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := vprocore.AuditRestoreRetain
						if route == "prune" {
							action = vprocore.AuditRestorePrune
						}
						_, err = s.RestoreSelectedAuditRecords(plot, []string{id}, action)
					default:
						err = s.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), field.Name) {
						t.Fatalf("invalid %s restore bypassed %s: %v", field.Name, route, err)
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
				})
			}
		}
		for i, target := range []*string{nil, vprotest.QualityString("q'X4"), vprotest.QualityString("aB"), vprotest.QualityString(" "),
			vprotest.QualityString("\nX"), vprotest.QualityString("e\u0301"), vprotest.QualityString("a\U0001F600b")} {
			t.Run(field.Name+"/valid/"+strconv.Itoa(i), func(t *testing.T) {
				s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, field.Name, target, vprotest.QualityString("historical-too-long"))
				count := vprotest.AuditCount(t, db, plot)
				if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain)
				if err != nil || result.RestoredRows != 1 {
					t.Fatalf("valid restore failed: %#v %v", result, err)
				}
				got, err := s.GetPlot(plot)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.Name).Interface(), target) || vprotest.AuditCount(t, db, plot) != count {
					t.Fatal("restore repaired raw text or changed audit history")
				}
			})
		}
		if err := vprocore.ValidateSoilRestoreValue("Env", field.Name, 7); err == nil {
			t.Fatal("non-string restore target accepted")
		}
	}
}

func TestSoilPhysicalRestoreRollbackCancelAndOwnership(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER soil_restore_reject BEFORE UPDATE OF SoilClassGroup ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER soil_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'prune rejected'); END`,
	} {
		s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "SoilClassGroup", vprotest.QualityString("q'X4"), vprotest.QualityString("AB"))
		if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestorePrune); err == nil {
			t.Fatal("restore failure accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER soil_restore_reject`); err != nil {
			t.Fatal(err)
		}
		result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestorePrune)
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
		s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "SoilClassSubGroup", vprotest.QualityString("Z9"), vprotest.QualityString("CA"))
		if err := s.CreatePlot(vprocore.FS882Header{PlotNumber: "foreign"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(mutate, id); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestorePrune); err == nil {
			t.Fatal("foreign/child/stale soil restore accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}

	invalid := vprotest.QualityString("historical-too-long")
	s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "SoilClassGroup", invalid, invalid)
	before := vprotest.SubstrateSnapshot(t, db)
	result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreCancel)
	if err != nil || !result.Cancelled {
		t.Fatal("cancel validated or changed invalid soil history")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain); err == nil {
		t.Fatal("invalid no-change restore grandfathered")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}

func TestSoilPhysicalRestoreCoupledAndSchemaPending(t *testing.T) {
	s, db, plot, groupID := vprotest.SiteCodeRestoreFixture(t, "SoilClassGroup", vprotest.QualityString("1234"), vprotest.QualityString("DBC"))
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
	before := vprotest.SubstrateSnapshot(t, db)
	if _, err := s.RestoreSelectedAuditRecords(plot, ids, vprocore.AuditRestorePrune); err == nil {
		t.Fatal("coupled restore failure accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER soil_coupled_reject`); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreSelectedAuditRecords(plot, ids, vprocore.AuditRestorePrune)
	if err != nil || result.RestoredRows != 2 || result.PrunedAuditRows != 2 {
		t.Fatalf("coupled retry failed %#v %v", result, err)
	}
	got, err := s.GetPlot(plot)
	if err != nil || !vprocore.SameSiteCode(got.SoilClassGroup, vprotest.QualityString("1234")) || !vprocore.SameSiteCode(got.SoilClassSubGroup, vprotest.QualityString("Z9")) {
		t.Fatal("coupled restore changed raw targets")
	}
	s, db, plot, groupID = vprotest.SiteCodeRestoreFixture(t, "SoilClassGroup", vprotest.QualityString("1234"), vprotest.QualityString("DBC"))
	if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN SoilClassGroup`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.SubstrateSnapshot(t, db)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{groupID}, vprocore.AuditRestorePrune); err == nil {
		t.Fatal("schema-pending restore accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}
