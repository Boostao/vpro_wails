package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMasterBECSourcePolicyAndScopedOwnership(t *testing.T) {
	service, state := contextServiceFixture(t)
	for _, test := range []struct {
		user    string
		allowed bool
	}{{"Admin", false}, {"Will MacKenzie ", false}, {" Will MacKenzie", false}, {"Will MacKenzie", true}, {"will mackenzie", true}} {
		user := test.user
		if err := service.plots.SetCurrentUser(user); err != nil {
			t.Fatal(err)
		}
		allowed, err := service.CanEditMasterBEC(context.Background(), state.ContextID)
		if err != nil || allowed != test.allowed {
			t.Fatal("scoped policy differs from its audit identity", user, allowed, err)
		}
	}
	if _, err := service.CanEditMasterBEC(context.Background(), "stale"); err == nil {
		t.Fatal("stale policy identity accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.CanEditMasterBEC(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("policy cancellation was not propagated", err)
	}
	if err := service.plots.SetCurrentUser("Admin"); err != nil {
		t.Fatal(err)
	}
	if err := service.CreatePlot(state.ContextID, FS882Header{PlotNumber: "MASTER-DENIED", BECSiteUnit: qualityString("new")}); err == nil || !strings.Contains(err.Error(), "restricted") {
		t.Fatal("scoped write bypassed authorization", err)
	}
}

func TestMasterBECAuthorizationHistoricalOmissionAllSaveRoutes(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "MASTER", BECSiteUnit: qualityString("initial")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET BECSiteUnit=? WHERE Plot='MASTER';
		CREATE TRIGGER master_unchanged BEFORE UPDATE OF BECSiteUnit ON Sample_Admin
		BEGIN SELECT RAISE(ABORT,'unchanged Master assigned'); END`, strings.Repeat("x", 101)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCurrentUser("Admin"); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	stored.OfficeNotes = qualityString("unrelated")
	if err := s.UpdatePlot(*stored); err != nil {
		t.Fatal("unprivileged historical preservation failed", err)
	}
	if _, err := db.Exec(`DROP TRIGGER master_unchanged`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	for _, value := range []*string{nil, qualityString("changed")} {
		next := *stored
		next.BECSiteUnit = value
		for _, save := range []func(FS882Header) error{s.SavePlot, s.UpdatePlot} {
			if err := save(next); err == nil || !strings.Contains(err.Error(), "restricted") {
				t.Fatal("unauthorized change or clear accepted", err)
			}
			assertSubstrateSnapshot(t, db, before)
		}
	}
}

func TestMasterBECPhysicalUnicodeAndAtomicAuditRetry(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "MASTER-VALID"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", strings.Repeat("x", 101), strings.Repeat("\U0001f332", 51), string([]byte{0xff})} {
		h.BECSiteUnit = &value
		before := substrateSnapshot(t, db)
		if err := s.UpdatePlot(h); err == nil {
			t.Fatal("invalid Master text accepted")
		}
		assertSubstrateSnapshot(t, db, before)
	}
	for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`} {
		header := FS882Header{PlotNumber: "untouched"}
		if err := json.Unmarshal([]byte(`{"BECSITEUNIT":`+raw+`,"becSiteUnit":"ok"}`), &header); err == nil || header.PlotNumber != "untouched" {
			t.Fatal("malformed duplicate-key raw Unicode repaired")
		}
	}
	value := " 00'O " + strings.Repeat("\U0001f332", 47)
	h.BECSiteUnit = &value
	if _, err := db.Exec(`CREATE TRIGGER master_audit_reject BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='BECSiteUnit' BEGIN SELECT RAISE(ABORT,'Master audit rollback'); END`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	if err := s.UpdatePlot(h); err == nil || !strings.Contains(err.Error(), "Master audit rollback") {
		t.Fatal("audit failure did not reject Master update", err)
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER master_audit_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(h); err != nil {
		t.Fatal("exact100 retry rejected", err)
	}
	stored, err := s.GetPlot(h.PlotNumber)
	if err != nil || !sameSiteCode(stored.BECSiteUnit, &value) {
		t.Fatal("raw Master code changed", err)
	}
}

func TestMasterBECRestorationAliasesRequireAuthorizationAndPhysicalTargets(t *testing.T) {
	for _, alias := range []string{"_Admin", "Sample_Admin"} {
		t.Run(alias, func(t *testing.T) {
			s, db := restoreFixture(t)
			if _, err := db.Exec(`UPDATE Sample_Admin SET BECSiteUnit='current' WHERE Plot='CHILD1'`); err != nil {
				t.Fatal(err)
			}
			id := restoreAuditFixture(t, db, "CHILD1", alias, "BECSiteUnit", nil, nil, "current")
			before := substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{id}, AuditRestorePrune); err == nil || !strings.Contains(err.Error(), "restricted") {
				t.Fatal("unauthorized NULL restoration accepted", err)
			}
			assertSubstrateSnapshot(t, db, before)
			if err := s.SetCurrentUser("Will MacKenzie"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE Sample_Audit SET BeforeEdit=? WHERE rowid=?`, strings.Repeat("x", 101), id); err != nil {
				t.Fatal(err)
			}
			before = substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{id}, AuditRestorePrune); err == nil || !strings.Contains(err.Error(), "at most 100") {
				t.Fatal("overlength restoration accepted", err)
			}
			assertSubstrateSnapshot(t, db, before)
			if _, err := db.Exec(`UPDATE Sample_Audit SET BeforeEdit=NULL WHERE rowid=?`, id); err != nil {
				t.Fatal(err)
			}
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{id}, AuditRestorePrune); err != nil {
				t.Fatal("authorized NULL restoration failed", err)
			}
			assertRestoreField(t, db, "Admin", "BECSiteUnit", "CHILD1", nil, nil)
		})
	}
}
