package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestWorkingUnitHeaderBoundsNULLUTF16AndRawRoundTrip(t *testing.T) {
	service, db := headerFixture(t)
	for i, value := range []*string{nil, becString("001"), becString("Case"), becString("case"),
		becString("O'Brien; preserved"), becString(" "), becString(strings.Repeat("x", 100)), becString(strings.Repeat("\U0001f600", 50))} {
		h := FS882Header{PlotNumber: "WUVALID" + strconv.Itoa(i), UserSiteUnit: value, BECSiteUnit: becString("unchanged source")}
		if err := service.CreatePlot(h); err != nil {
			t.Fatalf("valid source TEXT100 rejected: %v", err)
		}
		got, err := service.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) {
			t.Fatalf("raw nullable text changed: %#v %v", got, err)
		}
		if err := service.CreatePlot(h); err == nil {
			t.Fatal("strict identity collision accepted")
		}
		h.UserSiteUnit = nil
		if err := service.UpdatePlot(h); err != nil {
			t.Fatal(err)
		}
		got, err = service.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) {
			t.Fatalf("NULL clear changed source/partner fields: %#v %v", got, err)
		}
	}
	h := FS882Header{PlotNumber: "WUBOUNDS", UserSiteUnit: becString("old")}
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := auditCount(t, db, h.PlotNumber)
	for _, invalid := range []string{"", strings.Repeat("x", 101), strings.Repeat("\U0001f600", 51), strings.Repeat("x", 99) + "\U0001f600"} {
		next := h
		next.UserSiteUnit = &invalid
		if err := service.UpdatePlot(next); err == nil || !strings.Contains(err.Error(), "UserSiteUnit") {
			t.Fatalf("invalid supplied UTF16 text accepted: %v", err)
		}
		next.PlotNumber = "WUREJECT"
		if err := service.CreatePlot(next); err == nil {
			t.Fatal("invalid Create text accepted")
		}
		got, err := service.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) || auditCount(t, db, h.PlotNumber) != before ||
			auditCount(t, db, "WUREJECT") != 0 {
			t.Fatalf("invalid input mutated data/history: %#v %v", got, err)
		}
	}
}

func TestWorkingUnitHeaderHistoricalRawPreservationOmitsColumn(t *testing.T) {
	for _, historical := range []string{"", strings.Repeat("historical", 15)} {
		t.Run(strconv.Itoa(len(historical)), func(t *testing.T) {
			service, db := headerFixture(t)
			h := FS882Header{PlotNumber: "WUHIST", UserSiteUnit: becString("valid"), BECSiteUnit: becString("source")}
			if err := service.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE Sample_Admin SET UserSiteUnit=? WHERE Plot=?`, historical, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			old, err := service.GetPlot(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TRIGGER unchanged_working_unit BEFORE UPDATE OF UserSiteUnit ON Sample_Admin
 BEGIN SELECT RAISE(ABORT,'historical UserSiteUnit must not be resubmitted'); END`); err != nil {
				t.Fatal(err)
			}
			next := *old
			next.FieldNotes, next.OfficeNotes = becString("Env change"), becString("Admin change")
			if err := service.UpdatePlot(next); err != nil {
				t.Fatalf("unrelated update rewrites historical code: %v", err)
			}
			got, err := service.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(*got, next) {
				t.Fatalf("historical value/partner normalized: %#v %v", got, err)
			}
			next.UserSiteUnit = becString(strings.Repeat("x", 101))
			if err := service.UpdatePlot(next); err == nil {
				t.Fatal("changed invalid historical code accepted")
			}
			if _, err := db.Exec(`DROP TRIGGER unchanged_working_unit`); err != nil {
				t.Fatal(err)
			}
			next.UserSiteUnit = nil
			if err := service.UpdatePlot(next); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkingUnitHeaderActualAdminAuditStrengthsAndNoPhantoms(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, db := headerFixture(t)
			service.SetAuditStrength(strength)
			h := FS882Header{PlotNumber: "WUAUDIT", UserSiteUnit: becString("001")}
			if err := service.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			// Isolate the three source transitions from the independently tested Create audit.
			if _, err := db.Exec(`DELETE FROM Sample_Audit WHERE PlotNumber=?`, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			for _, value := range []*string{becString("Case"), nil, becString("O'Brien")} {
				h.UserSiteUnit = value
				if err := service.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
			}
			want := []int{0, 1, 2, 3}[strength]
			entries, err := service.ListAuditEntries(h.PlotNumber)
			if err != nil || len(entries) != want {
				t.Fatalf("audit strength %d count %d want %d: %v", strength, len(entries), want, err)
			}
			for _, entry := range entries {
				if entry.Table != "_Admin" || entry.EditField != "UserSiteUnit" || entry.ID != nil {
					t.Fatalf("false source-table/unbound audit: %#v", entry)
				}
			}
			if err := service.UpdatePlot(h); err != nil {
				t.Fatal(err)
			}
			if auditCount(t, db, h.PlotNumber) != want {
				t.Fatal("same-value update created phantom history")
			}
		})
	}
}

func TestWorkingUnitHeaderTransactionalEnvAdminAuditFailures(t *testing.T) {
	for _, target := range []string{"Env", "Admin", "Audit"} {
		t.Run(target, func(t *testing.T) {
			service, db := headerFixture(t)
			h := FS882Header{PlotNumber: "WUROLL", UserSiteUnit: becString("old"), BECSiteUnit: becString("source")}
			if err := service.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			old, err := service.GetPlot(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			count := auditCount(t, db, h.PlotNumber)
			event := "UPDATE"
			if target == "Audit" {
				event = "INSERT"
			}
			if _, err := db.Exec(`CREATE TRIGGER working_unit_failure BEFORE ` + event + ` ON Sample_` + target +
				` BEGIN SELECT RAISE(ABORT,'controlled Working Unit rejection'); END`); err != nil {
				t.Fatal(err)
			}
			next := h
			next.UserSiteUnit, next.FieldNotes, next.OfficeNotes = becString("new"), becString("Env new"), becString("Admin new")
			if err := service.UpdatePlot(next); err == nil {
				t.Fatal("rejected Save reported success")
			}
			got, err := service.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(got, old) || auditCount(t, db, h.PlotNumber) != count {
				t.Fatalf("%s rejection failed atomic rollback: %#v %v", target, got, err)
			}
			if _, err := db.Exec(`DROP TRIGGER working_unit_failure`); err != nil {
				t.Fatal(err)
			}
			if err := service.UpdatePlot(next); err != nil {
				t.Fatalf("clean retry failed: %v", err)
			}
		})
	}
}

func TestWorkingUnitHeaderUnsupportedAdminSchema(t *testing.T) {
	service, db := headerFixture(t)
	if _, err := db.Exec(`ALTER TABLE Sample_Admin DROP COLUMN UserSiteUnit`); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"code", strings.Repeat("x", 101)} {
		h := FS882Header{PlotNumber: "WUNOCOLUMN", UserSiteUnit: &value}
		if err := service.CreatePlot(h); err == nil {
			t.Fatal("supplied unsupported UserSiteUnit accepted")
		}
		if auditCount(t, db, h.PlotNumber) != 0 {
			t.Fatal("unsupported schema wrote history")
		}
	}
}
