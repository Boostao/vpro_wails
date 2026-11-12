package integration

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSiteCodeRestoreAllAliasesRejectInvalidPreserveSelection(t *testing.T) {
	for _, field := range vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}) {
		invalids := []string{"", strings.Repeat("x", field.Maximum+1),
			strings.Repeat("x", field.Maximum-1) + "\U0001F600", string([]byte{0xFF})}
		if field.Maximum == 2 {
			invalids = append(invalids, "zz", "at", "A")
		}
		for _, invalid := range invalids {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(field.Name+"/"+route+"/"+strconv.Itoa(len(invalid)), func(t *testing.T) {
					s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, field.Name, &invalid, vprotest.QualityString("AT"))
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
					before := vprotest.SubstrateSnapshot(t, db)
					switch route {
					case "selection":
						err = s.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := vprocore.AuditRestoreRetain
						if route == "prune" {
							action = vprocore.AuditRestorePrune
						}
						_, err = s.RestoreSelectedAuditRecords(plot, []string{strconv.FormatInt(envID, 10), id}, action)
					default:
						err = s.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), field.Name) || !strings.Contains(err.Error(), id) {
						t.Fatalf("restore alias bypassed %s: %v", field.Name, err)
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
				})
			}
		}
	}
}

func TestSiteCodeRestoreValidAndNullWithoutCatalogue(t *testing.T) {
	for _, field := range vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}) {
		values := []*string{nil, vprotest.QualityString("AT"), vprotest.QualityString("NA")}
		if field.Maximum == 8 {
			values = append(values, vprotest.QualityString("O'"), vprotest.QualityString("unknown"),
				vprotest.QualityString("rawCase"), vprotest.QualityString(strings.Repeat("x", 6)+"\U0001F600"))
		}
		for _, target := range values {
			s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, field.Name, target, vprotest.QualityString("historical-overlength"))
			before := vprotest.AuditCount(t, db, plot)
			if target == nil || field.Maximum == 8 {
				if err := os.WriteFile(filepath.Join(s.Projects.Root, "site-codes.db"), []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain)
			if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 0 {
				t.Fatalf("valid restore failed: %#v %v", result, err)
			}
			got, err := s.GetPlot(plot)
			if err != nil {
				t.Fatal(err)
			}
			actual := reflect.ValueOf(*got).FieldByName(field.Name).Interface().(*string)
			if !reflect.DeepEqual(actual, target) || vprotest.AuditCount(t, db, plot) != before || *got.FieldNotes != "Env after" {
				t.Fatal("restore normalized value or changed history/partner")
			}
		}
	}
	for _, failure := range []string{"corrupt", "closed"} {
		s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "Exposure1", vprotest.QualityString("AT"), vprotest.QualityString("NA"))
		if failure == "closed" {
			var err error
			s.SiteCodes, err = references.NewSiteCodeService(s.Projects.Root)
			if err != nil || s.SiteCodes.Close() != nil {
				t.Fatal("closed catalogue fixture failed")
			}
		} else if err := os.WriteFile(filepath.Join(s.Projects.Root, "site-codes.db"), []byte("corrupt"), 0600); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain); err == nil ||
			!strings.Contains(err.Error(), "catalogue") {
			t.Fatalf("restore new Exposure bypassed unavailable catalogue: %v", err)
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
}

func TestSiteCodeRestoreRollbackRetryCancelAndNoop(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER codes_restore_reject BEFORE UPDATE OF Exposure1 ON Sample_Env BEGIN SELECT RAISE(ABORT,'data restore rejected'); END`,
		`CREATE TRIGGER codes_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'history pruning rejected'); END`,
	} {
		s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "Exposure1", vprotest.QualityString("AT"), vprotest.QualityString("NA"))
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
		before := vprotest.SubstrateSnapshot(t, db)
		if _, err := s.RestoreSelectedAuditRecords(plot, ids, vprocore.AuditRestorePrune); err == nil {
			t.Fatal("restore data/audit failure silently succeeded")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER codes_restore_reject`); err != nil {
			t.Fatal(err)
		}
		restored, err := s.RestoreSelectedAuditRecords(plot, ids, vprocore.AuditRestorePrune)
		if err != nil || restored.RestoredRows != 2 || restored.PrunedAuditRows != 2 {
			t.Fatalf("retry failed: %#v %v", restored, err)
		}
	}
	historical := vprotest.QualityString("historical-overlength")
	s, db, plot, id := vprotest.SiteCodeRestoreFixture(t, "Exposure1", historical, historical)
	before := vprotest.SubstrateSnapshot(t, db)
	cancelled, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreCancel)
	if err != nil || !cancelled.Cancelled {
		t.Fatal("cancel validated/changed invalid selection")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid audit restored")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}
