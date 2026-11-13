package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSoilDrainageScopedEditsRetainOwnedCatalogueFailures(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	if err := service.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: "DRNSCOP"}); err != nil {
		t.Fatal(err)
	}
	header, err := service.GetPlot(context.Background(), state.ContextID, "DRNSCOP")
	if err != nil {
		t.Fatal(err)
	}
	code := "w"
	if header.SoilDrainage != nil && *header.SoilDrainage == code {
		code = "r"
	}
	header.SoilDrainage = &code
	service.Plots.ParentCodesError = errors.New("owned drainage reference failure")
	if err := service.UpdatePlot(state.ContextID, *header); err == nil || !strings.Contains(err.Error(), "owned drainage reference failure") {
		t.Fatal("scoped operation bypassed the owned catalogue error", err)
	}
}

func TestSoilDrainageCanonicalMembershipAllRoutesAndRawUnicode(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "DRAIN", SoilDrainage: vprotest.QualityString("w")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	rows, err := s.DrainageChoices()
	if err != nil || len(rows) != 14 {
		t.Fatal("frozen drainage definitions changed", len(rows), err)
	}
	for _, row := range rows {
		if !row.Selectable {
			continue
		}
		h.SoilDrainage = row.Code
		if err := s.UpdatePlot(h); err != nil {
			t.Fatal("canonical Item rejected", *row.Code, err)
		}
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, value := range []string{"", "W", "PA", " w", "w ", "pax", "?????", "abcdef", string([]byte{0xff})} {
		next := h
		next.SoilDrainage = &value
		for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := save(next); err == nil {
				t.Fatal("unlisted or physically invalid Item accepted", value)
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
		next.PlotNumber = "DRAIN-NEW"
		if err := s.CreatePlot(next); err == nil {
			t.Fatal("unlisted Item created")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
	for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`} {
		header := vprocore.FS882Header{PlotNumber: "unchanged"}
		if err := json.Unmarshal([]byte(`{"SOILDRAINAGE":`+raw+`,"soilDrainage":"w"}`), &header); err == nil || header.PlotNumber != "unchanged" {
			t.Fatal("duplicate-key malformed Unicode was repaired")
		}
	}
}

func TestSoilDrainageHistoricalOmissionAvailabilityAndAuditRollback(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "DRAIN-HISTORY"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET SoilDrainage='historical-long' WHERE PlotNumber='DRAIN-HISTORY';
      CREATE TRIGGER drainage_unchanged BEFORE UPDATE OF SoilDrainage ON Sample_Env BEGIN SELECT RAISE(ABORT,'unchanged drainage assigned'); END`); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	s.ParentCodesError = errors.New("drainage reference unavailable")
	stored.OfficeNotes = vprotest.QualityString("unrelated")
	if err := s.UpdatePlot(*stored); err != nil {
		t.Fatal("historical value was reassigned or needed reference availability", err)
	}
	if _, err := db.Exec(`DROP TRIGGER drainage_unchanged`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	stored.SoilDrainage = vprotest.QualityString("w")
	if err := s.UpdatePlot(*stored); err == nil || !strings.Contains(err.Error(), "drainage reference unavailable") {
		t.Fatal("new member bypassed reference failure", err)
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	stored.SoilDrainage = nil
	if err := s.UpdatePlot(*stored); err != nil {
		t.Fatal("NULL incorrectly requires membership", err)
	}
	s.ParentCodesError = nil
	stored.SoilDrainage = vprotest.QualityString("w")
	if _, err := db.Exec(`CREATE TRIGGER drainage_audit_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='SoilDrainage' BEGIN SELECT RAISE(ABORT,'drainage audit rollback'); END`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.SubstrateSnapshot(t, db)
	if err := s.UpdatePlot(*stored); err == nil || !strings.Contains(err.Error(), "drainage audit rollback") {
		t.Fatal("audit failure did not reject save", err)
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER drainage_audit_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(*stored); err != nil {
		t.Fatal("retry rejected", err)
	}
}

func TestSoilDrainageRestoreAliasesEnforceMembershipAndNull(t *testing.T) {
	for _, alias := range []string{"_Env", "Sample_Env"} {
		t.Run(alias, func(t *testing.T) {
			s, db := restoreFixture(t)
			if _, err := db.Exec(`UPDATE Sample_Env SET SoilDrainage='r' WHERE PlotNumber='CHILD1'`); err != nil {
				t.Fatal(err)
			}
			bad := restoreAuditFixture(t, db, "CHILD1", alias, "SoilDrainage", nil, "W", "r")
			before := vprotest.SubstrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{bad}, vprocore.AuditRestorePrune); err == nil || !strings.Contains(err.Error(), "exact canonical") {
				t.Fatal("restore alias bypassed strict membership", err)
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
			if _, err := db.Exec(`DELETE FROM Sample_Audit WHERE rowid=?`, bad); err != nil {
				t.Fatal(err)
			}
			good := restoreAuditFixture(t, db, "CHILD1", alias, "SoilDrainage", nil, "w", "r")
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{good}, vprocore.AuditRestorePrune); err != nil {
				t.Fatal("canonical restoration rejected", err)
			}
			assertRestoreField(t, db, "Env", "SoilDrainage", "CHILD1", nil, "w")
			clear := restoreAuditFixture(t, db, "CHILD1", alias, "SoilDrainage", nil, nil, "w")
			s.ParentCodesError = errors.New("unavailable")
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{clear}, vprocore.AuditRestorePrune); err != nil {
				t.Fatal("NULL restoration requires reference", err)
			}
		})
	}
}
