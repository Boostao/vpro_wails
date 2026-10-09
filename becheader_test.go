package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestBECHeaderBoundsRawCodesAndDependentPreservation(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "BECCODES", Zone: becString("bg"), SubZone: becString("XH1"), SiteSeries: becString("01")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, h) {
		t.Fatalf("raw codes changed: %#v %v", got, err)
	}
	h.Zone = becString("ZZR")
	if err := s.UpdatePlot(h); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, h) || *got.SubZone != "XH1" || *got.SiteSeries != "01" {
		t.Fatalf("parent change silently cleared children: %#v %v", got, err)
	}
	entries, err := s.ListAuditEntries(h.PlotNumber)
	if err != nil || len(entries) != 4 {
		t.Fatalf("bounded code audit: %d %v", len(entries), err)
	}
	before := auditCount(t, db, h.PlotNumber)
	for _, field := range []string{"Zone", "SubZone", "SiteSeries"} {
		for _, invalid := range []string{"", strings.Repeat("x", map[string]int{"Zone": 5, "SubZone": 9, "SiteSeries": 6}[field])} {
			next := h
			reflect.ValueOf(&next).Elem().FieldByName(field).Set(reflect.ValueOf(&invalid))
			if err := s.UpdatePlot(next); err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("invalid %s %q: %v", field, invalid, err)
			}
			got, err := s.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(*got, h) || auditCount(t, db, h.PlotNumber) != before {
				t.Fatalf("invalid input changed project/history: %#v %v", got, err)
			}
		}
	}
	h.Zone, h.SubZone, h.SiteSeries = nil, nil, nil
	if err := s.UpdatePlot(h); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, h) {
		t.Fatalf("NULL clear failed: %#v %v", got, err)
	}
}

func TestBECHeaderHistoricalInvalidCollisionAndRollback(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "BECHIST", Zone: becString("BG"), SiteSeries: becString("01")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET Zone='historical-zone', SubZone='', SiteSeries='historic-series' WHERE PlotNumber=?`, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	old, err := s.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	next := *old
	next.FieldNotes = becString("unrelated edit")
	if _, err := db.Exec(`CREATE TRIGGER bec_unchanged_reject BEFORE UPDATE OF Zone, SubZone, SiteSeries ON Sample_Env
 BEGIN SELECT RAISE(ABORT,'unchanged BEC columns must not be rewritten'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatalf("unchanged historical codes blocked unrelated save: %v", err)
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, next) {
		t.Fatalf("historical invalid code normalized: %#v %v", got, err)
	}
	next.Zone = becString("new-invalid")
	if err := s.UpdatePlot(next); err == nil {
		t.Fatal("changed invalid historical code accepted")
	}
	if _, err := db.Exec(`DROP TRIGGER bec_unchanged_reject`); err != nil {
		t.Fatal(err)
	}
	next = *got
	next.Zone, next.SubZone, next.SiteSeries = becString("BG"), becString("xh1"), becString("Wm05")
	next.OfficeNotes = becString("atomic Admin edit")
	before := auditCount(t, db, next.PlotNumber)
	if _, err := db.Exec(`CREATE TRIGGER bec_audit_reject BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT,'BEC audit rejection'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(next); err == nil {
		t.Fatal("audit trigger failure accepted")
	}
	after, err := s.GetPlot(next.PlotNumber)
	if err != nil || !reflect.DeepEqual(after, got) || auditCount(t, db, next.PlotNumber) != before {
		t.Fatalf("audit failure did not roll back Env/Admin: %#v %v", after, err)
	}
	if _, err := db.Exec(`DROP TRIGGER bec_audit_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePlot(next); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("strict identity collision lost: %v", err)
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	next.PlotNumber = "BECNEW"
	next.SiteSeries = becString("sixsix")
	if err := s.CreatePlot(next); err == nil {
		t.Fatal("invalid new code accepted")
	}
	if auditCount(t, db, "BECNEW") != 0 {
		t.Fatal("invalid create wrote history")
	}
}
