package plotentry

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"reflect"
	"strings"
	"testing"
)

func TestRegionRestoreAllAliasesAndRawNullableTargets(t *testing.T) {
	for _, field := range vprocore.RegionHeaderFields(vprocore.FS882Header{}) {
		for _, invalid := range []string{"", strings.Repeat("x", field.Maximum+1), strings.Repeat("x", field.Maximum-1) + "\U0001F600", string([]byte{0xff})} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
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
					t.Fatalf("invalid %s target bypassed %s: %v", field.Name, route, err)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
		for _, target := range []*string{nil, vprotest.QualityString("O'"), vprotest.QualityString("aB"), vprotest.QualityString(" "), vprotest.QualityString(strings.Repeat("x", field.Maximum-2) + "\U0001F600")} {
			s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, field.Name, target, vprotest.QualityString("historical-too-long"))
			count := vprotest.AuditCount(t, db, plot)
			if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain)
			if err != nil || result.RestoredRows != 1 {
				t.Fatalf("valid target restore %v %#v", err, result)
			}
			got, err := s.GetPlot(plot)
			if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.Name).Interface(), target) || vprotest.AuditCount(t, db, plot) != count {
				t.Fatal("raw restore/history mismatch")
			}
		}
		if err := vprocore.ValidateRegionRestoreValue("Env", field.Name, 7); err == nil {
			t.Fatal("non-string restore accepted")
		}
	}
}

func TestRegionRestoreRollbackRetryAndCancel(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER region_restore_reject BEFORE UPDATE OF Ecosection ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER region_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'prune rejected'); END`,
	} {
		s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "Ecosection", vprotest.QualityString("o'"), vprotest.QualityString("AB"))
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
		if _, err := db.Exec(`DROP TRIGGER region_restore_reject`); err != nil {
			t.Fatal(err)
		}
		result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestorePrune)
		if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 1 {
			t.Fatalf("retry failed: %#v %v", result, err)
		}
	}
	invalid := vprotest.QualityString("historical-too-long")
	s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "FSRegionDistrict", invalid, invalid)
	before := vprotest.SubstrateSnapshot(t, db)
	result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreCancel)
	if err != nil || !result.Cancelled {
		t.Fatal("cancel changed/validated invalid target")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid target grandfathered")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}
