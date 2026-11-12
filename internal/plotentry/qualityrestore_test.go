package plotentry

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"database/sql"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func qualityRestoreFixture(t *testing.T, member string, before, after *string) (*vprocore.PlotService, *sql.DB, string, string) {
	t.Helper()
	service, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "QUALREST", FieldNotes: vprotest.QualityString("Env after")}
	vprotest.QualitySet(&h, member, vprotest.QualityString("Good"))
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET `+vprocore.QuoteHeaderIdentifier(member)+`=? WHERE Plot=?`, after, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Admin',?,'2026-09-30 12:00:00',?,?,0,0)`,
		h.PlotNumber, member, before, after)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return service, db, h.PlotNumber, strconv.FormatInt(id, 10)
}

func TestQualityRestoreEveryEntryPointRejectsInvalidAndPreservesSelection(t *testing.T) {
	for _, member := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
		for _, invalid := range []string{strings.Repeat("x", 16), "", strings.Repeat("x", 14) + "\U0001F600", string([]byte{0xFF})} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(member+"/"+route+"/"+strconv.Itoa(len(invalid)), func(t *testing.T) {
					service, db, plot, id := qualityRestoreFixture(t, member, &invalid, vprotest.QualityString("Good"))
					// Existing selection is a different, valid Env restoration.
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
					old, err := service.GetPlot(plot)
					if err != nil {
						t.Fatal(err)
					}
					before := vprotest.AuditCount(t, db, plot)
					switch route {
					case "selection":
						err = service.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := vprocore.AuditRestoreRetain
						if route == "prune" {
							action = vprocore.AuditRestorePrune
						}
						_, err = service.RestoreSelectedAuditRecords(plot, []string{strconv.FormatInt(envID, 10), id}, action)
					case "marked-retain", "marked-prune":
						err = service.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), member) || !strings.Contains(err.Error(), id) {
						t.Fatalf("restore bypass accepted invalid quality: %v", err)
					}
					got, err := service.GetPlot(plot)
					if err != nil || !reflect.DeepEqual(got, old) || vprotest.AuditCount(t, db, plot) != before {
						t.Fatal("refused restore changed Env/Admin/history")
					}
					var selected int
					if err := db.QueryRow(`SELECT CAST(Restore AS INTEGER) FROM Sample_Audit WHERE rowid=?`, envID).Scan(&selected); err != nil || selected != -1 {
						t.Fatal("refused quality selection reset existing flags")
					}
				})
			}
		}
	}
}

func TestQualityRestoreValidNullHistoricalClearAndRawCase(t *testing.T) {
	for _, member := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
		for _, target := range []*string{nil, vprotest.QualityString("manual"), vprotest.QualityString("O'Neil"), vprotest.QualityString("LOWER"),
			vprotest.QualityString(strings.Repeat("x", 15)), vprotest.QualityString(strings.Repeat("x", 13) + "\U0001F600"), vprotest.QualityString("\uFFFD")} {
			t.Run(member, func(t *testing.T) {
				historical := vprotest.QualityString(strings.Repeat("h", 16))
				service, db, plot, id := qualityRestoreFixture(t, member, target, historical)
				before := vprotest.AuditCount(t, db, plot)
				if err := service.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				result, err := service.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain)
				if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 0 {
					t.Fatalf("valid restoration from historical invalid failed: %#v %v", result, err)
				}
				got, err := service.GetPlot(plot)
				if err != nil {
					t.Fatal(err)
				}
				actual := reflect.ValueOf(*got).FieldByName(member).Interface().(*string)
				if !reflect.DeepEqual(actual, target) || vprotest.AuditCount(t, db, plot) != before {
					t.Fatal("quality restore normalized raw value or manufactured audits")
				}
				if *got.FieldNotes != "Env after" {
					t.Fatal("quality restore changed partner")
				}
			})
		}
	}
}

func TestQualityRestoreAtomicTriggerRollbackCancelAndNoop(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER quality_restore_reject BEFORE UPDATE OF SitePlotQuality ON Sample_Admin
 BEGIN SELECT RAISE(ABORT,'quality restoration rejected'); END`,
		`CREATE TRIGGER quality_restore_reject BEFORE DELETE ON Sample_Audit
 BEGIN SELECT RAISE(ABORT,'quality audit pruning rejected'); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			service, db, plot, id := qualityRestoreFixture(t, "SitePlotQuality", vprotest.QualityString("Fair"), vprotest.QualityString("Good"))
			env, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env','SiteNotes','2026-09-30 12:01:00','Env before','Env after',0,0)`, plot)
			if err != nil {
				t.Fatal(err)
			}
			envID, err := env.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			old, err := service.GetPlot(plot)
			if err != nil {
				t.Fatal(err)
			}
			before := vprotest.AuditCount(t, db, plot)
			if _, err := service.RestoreSelectedAuditRecords(plot, []string{strconv.FormatInt(envID, 10), id}, vprocore.AuditRestorePrune); err == nil {
				t.Fatal("restore trigger rejection silently succeeded")
			}
			got, err := service.GetPlot(plot)
			if err != nil || !reflect.DeepEqual(got, old) || vprotest.AuditCount(t, db, plot) != before {
				t.Fatal("failed restore left partial Env/Admin/history")
			}
		})
	}
	historical := vprotest.QualityString(strings.Repeat("h", 16))
	service, db, plot, id := qualityRestoreFixture(t, "SitePlotQuality", historical, historical)
	if _, err := db.Exec(`CREATE TRIGGER quality_noop_reject BEFORE UPDATE OF SitePlotQuality ON Sample_Admin
 BEGIN SELECT RAISE(ABORT,'unchanged historical quality rewritten'); END`); err != nil {
		t.Fatal(err)
	}
	old, err := service.GetPlot(plot)
	if err != nil {
		t.Fatal(err)
	}
	next := *old
	next.OfficeNotes = vprotest.QualityString("unrelated edit")
	if err := service.UpdatePlot(next); err != nil {
		t.Fatalf("header unchanged historical quality rewrite: %v", err)
	}
	before := vprotest.AuditCount(t, db, plot)
	result, err := service.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreCancel)
	if err != nil || !result.Cancelled || vprotest.AuditCount(t, db, plot) != before {
		t.Fatal("cancel mutated invalid selected history")
	}
	// Existing restore behavior rejects history that describes no change;
	// it must never issue a quality UPDATE for that historical no-op.
	if _, err := service.RestoreSelectedAuditRecords(plot, []string{id}, vprocore.AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid audit history accepted")
	}
	got, err := service.GetPlot(plot)
	if err != nil || !reflect.DeepEqual(*got, next) || vprotest.AuditCount(t, db, plot) != before {
		t.Fatal("no-op refusal rewrote historical field/history")
	}
}
