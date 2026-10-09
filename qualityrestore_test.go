package main

import (
	"database/sql"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func qualityRestoreFixture(t *testing.T, member string, before, after *string) (*PlotService, *sql.DB, string, string) {
	t.Helper()
	service, db := headerFixture(t)
	h := FS882Header{PlotNumber: "QUALREST", FieldNotes: qualityString("Env after")}
	qualitySet(&h, member, qualityString("Good"))
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET `+quoteHeaderIdentifier(member)+`=? WHERE Plot=?`, after, h.PlotNumber); err != nil {
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
					service, db, plot, id := qualityRestoreFixture(t, member, &invalid, qualityString("Good"))
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
					before := auditCount(t, db, plot)
					switch route {
					case "selection":
						err = service.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := AuditRestoreRetain
						if route == "prune" {
							action = AuditRestorePrune
						}
						_, err = service.RestoreSelectedAuditRecords(plot, []string{strconv.FormatInt(envID, 10), id}, action)
					case "marked-retain", "marked-prune":
						err = service.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), member) || !strings.Contains(err.Error(), id) {
						t.Fatalf("restore bypass accepted invalid quality: %v", err)
					}
					got, err := service.GetPlot(plot)
					if err != nil || !reflect.DeepEqual(got, old) || auditCount(t, db, plot) != before {
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
		for _, target := range []*string{nil, qualityString("manual"), qualityString("O'Neil"), qualityString("LOWER"),
			qualityString(strings.Repeat("x", 15)), qualityString(strings.Repeat("x", 13) + "\U0001F600"), qualityString("\uFFFD")} {
			t.Run(member, func(t *testing.T) {
				historical := qualityString(strings.Repeat("h", 16))
				service, db, plot, id := qualityRestoreFixture(t, member, target, historical)
				before := auditCount(t, db, plot)
				if err := service.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				result, err := service.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain)
				if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != 0 {
					t.Fatalf("valid restoration from historical invalid failed: %#v %v", result, err)
				}
				got, err := service.GetPlot(plot)
				if err != nil {
					t.Fatal(err)
				}
				actual := reflect.ValueOf(*got).FieldByName(member).Interface().(*string)
				if !reflect.DeepEqual(actual, target) || auditCount(t, db, plot) != before {
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
			service, db, plot, id := qualityRestoreFixture(t, "SitePlotQuality", qualityString("Fair"), qualityString("Good"))
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
			before := auditCount(t, db, plot)
			if _, err := service.RestoreSelectedAuditRecords(plot, []string{strconv.FormatInt(envID, 10), id}, AuditRestorePrune); err == nil {
				t.Fatal("restore trigger rejection silently succeeded")
			}
			got, err := service.GetPlot(plot)
			if err != nil || !reflect.DeepEqual(got, old) || auditCount(t, db, plot) != before {
				t.Fatal("failed restore left partial Env/Admin/history")
			}
		})
	}
	historical := qualityString(strings.Repeat("h", 16))
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
	next.OfficeNotes = qualityString("unrelated edit")
	if err := service.UpdatePlot(next); err != nil {
		t.Fatalf("header unchanged historical quality rewrite: %v", err)
	}
	before := auditCount(t, db, plot)
	result, err := service.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel)
	if err != nil || !result.Cancelled || auditCount(t, db, plot) != before {
		t.Fatal("cancel mutated invalid selected history")
	}
	// Existing restore behavior rejects history that describes no change;
	// it must never issue a quality UPDATE for that historical no-op.
	if _, err := service.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil {
		t.Fatal("unchanged invalid audit history accepted")
	}
	got, err := service.GetPlot(plot)
	if err != nil || !reflect.DeepEqual(*got, next) || auditCount(t, db, plot) != before {
		t.Fatal("no-op refusal rewrote historical field/history")
	}
}
