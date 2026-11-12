package validation_test

import (
	"context"
	"testing"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/validation"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestRunComplianceValidationAndQuarantineOrphanRecords(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Seed targeted violations on Sample.db:
	// 1. Plot 'CMP01' in _Env without _Admin (1:1 mismatch) + invalid MoistureRegime ('ZZZ') on 'CMP02'
	// 2. IEEE-754 float32 cover narrowing: Cover1 = 99.999996 (narrows to <100 -> PASSES) vs Cover2 = 99.999999 (narrows to 100 -> FAILS!)
	// 3. LMH25 horizon depth sign violation: upland 'Lv' with UpperDepth=2, LowerDepth=5 (must be UpperDepth > LowerDepth)
	// 4. Orphan _Veg (ID=777) and _Humus (ID=888) rows whose PlotNumber ('ORPH99') is missing from _Env
	// 5. Duplicate (PlotNumber, Species) split on ('CMP02', 'PSEUMEN') + unlisted species 'ZZZZBAD1'
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		INSERT INTO Sample_Env (PlotNumber, ProjectID, Zone, MoistureRegime, Latitude)
			VALUES ('CMP01', 'SMP', 'CWH', '4', 50.0),
			       ('CMP02', 'SMP', 'CWH', 'ZZZ', 95.0);
		INSERT INTO Sample_Admin (Plot, PlotType, StartDate)
			VALUES ('CMP02', 'FS882', 1850);
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Cover2, Flag)
			VALUES ('CMP02', 'PSEUMEN', 701, 99.999996, 99.999999, 0),
			       ('CMP02', 'PSEUMEN', 702, NULL, 15.0, 0),
			       ('CMP02', 'ZZZZBAD1', 703, 5.0, NULL, 0),
			       ('ORPH99', 'TSUGHET', 777, 25.0, NULL, 0);
		INSERT INTO Sample_Humus (PlotNumber, Horizon, UpperDepth, LowerDepth, Flag, ID)
			VALUES ('CMP02', 'Lv', 2.0, 5.0, 0, 801),
			       ('ORPH99', 'Fm', 6.0, 2.0, 0, 888);
	`)

	report, err := validation.RunComplianceValidation(svc, ctx, state.ContextID, validation.ComplianceOptions{
		UseActiveSU: false,
	})
	if err != nil {
		t.Fatalf("RunComplianceValidation failed: %v", err)
	}
	if report.Passed {
		t.Fatalf("expected compliance validation to fail when errors are seeded")
	}

	// Verify 1:1 _Env <-> _Admin finding for CMP01
	foundMissingAdmin := false
	for _, f := range report.EnvAdminFindings {
		if f.PlotNumber == "CMP01" && f.Issue == "missing_admin" {
			foundMissingAdmin = true
		}
	}
	if !foundMissingAdmin {
		t.Fatalf("expected missing_admin finding for CMP01, got %+v", report.EnvAdminFindings)
	}

	// Verify USysTableOfLists code check caught MoistureRegime = 'ZZZ' on CMP02
	foundMoisture := false
	for _, f := range report.EnvCodeFindings {
		if f.PlotNumber == "CMP02" && f.Field == "MoistureRegime" && f.Value == "ZZZ" {
			foundMoisture = true
		}
	}
	if !foundMoisture {
		t.Fatalf("expected MoistureRegime='ZZZ' finding, got %+v", report.EnvCodeFindings)
	}

	// Verify float32 cover narrowing: Cover1=99.999996 MUST NOT be flagged, Cover2=99.999999 MUST be flagged!
	cover1Flagged, cover2Flagged, humusSignFlagged := false, false, false
	for _, b := range report.BoundsFindings {
		if b.PlotNumber == "CMP02" && b.Table == "Veg" && b.Field == "Cover1" {
			cover1Flagged = true
		}
		if b.PlotNumber == "CMP02" && b.Table == "Veg" && b.Field == "Cover2" {
			cover2Flagged = true
		}
		if b.PlotNumber == "CMP02" && b.Table == "Humus" && b.Field == "UpperDepth" {
			humusSignFlagged = true
		}
	}
	if cover1Flagged {
		t.Fatalf("Cover1=99.999996 narrows to 99.99999237 (<100) in float32 and must NOT be flagged")
	}
	if !cover2Flagged {
		t.Fatalf("Cover2=99.999999 narrows to 100 in float32 and MUST be flagged")
	}
	if !humusSignFlagged {
		t.Fatalf("upland Lv with UpperDepth=2 <= LowerDepth=5 MUST be flagged")
	}

	// Verify duplicate species split & unlisted taxonomy finding
	foundDup := false
	for _, d := range report.DuplicateVegFindings {
		if d.PlotNumber == "CMP02" && d.Species == "PSEUMEN" && d.RowCount == 2 {
			foundDup = true
		}
	}
	if !foundDup {
		t.Fatalf("expected duplicate PSEUMEN finding on CMP02, got %+v", report.DuplicateVegFindings)
	}
	if report.Summary.UnlistedSpecies == 0 {
		t.Fatalf("expected unlisted species ZZZZBAD1 to be counted")
	}

	// Verify orphan child rows detected before quarantine
	if report.Summary.OrphanChildRows < 2 {
		t.Fatalf("expected at least 2 orphan child rows (ORPH99 in Veg & Humus), got %d", report.Summary.OrphanChildRows)
	}

	// Execute QuarantineOrphanRecords and verify zero data loss + permanent child ID reservation + clean re-validation!
	qRes, err := validation.QuarantineOrphanRecords(svc, ctx, state.ContextID, validation.QuarantineOrphanRequest{})
	if err != nil {
		t.Fatalf("QuarantineOrphanRecords failed: %v", err)
	}
	if qRes.TotalQuarantined < 2 || qRes.AuditRowID == 0 {
		t.Fatalf("expected >=2 quarantined rows and audit receipt, got %+v", qRes)
	}

	// Verify __VPRO_Quarantine_Veg and __VPRO_Quarantine_Humus contain ORPH99 and IDs 777 & 888 are reserved in __VPRO_ChildIdentity!
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var qVegSpecies, qHumusHorizon string
	if err := db.QueryRow(`SELECT "Species" FROM "__VPRO_Quarantine_Veg" WHERE "PlotNumber" = 'ORPH99' AND "ID" = 777`).Scan(&qVegSpecies); err != nil || qVegSpecies != "TSUGHET" {
		t.Fatalf("expected quarantined Veg row TSUGHET preserved in __VPRO_Quarantine_Veg, got %q (err=%v)", qVegSpecies, err)
	}
	if err := db.QueryRow(`SELECT "Horizon" FROM "__VPRO_Quarantine_Humus" WHERE "PlotNumber" = 'ORPH99' AND "ID" = 888`).Scan(&qHumusHorizon); err != nil || qHumusHorizon != "Fm" {
		t.Fatalf("expected quarantined Humus row Fm preserved in __VPRO_Quarantine_Humus, got %q (err=%v)", qHumusHorizon, err)
	}
	for _, reservedID := range []int64{777, 888} {
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE "ID" = ?`, reservedID).Scan(&cnt); err != nil || cnt == 0 {
			t.Fatalf("expected quarantined child ID %d reserved in __VPRO_ChildIdentity", reservedID)
		}
	}

	// Re-run compliance validation: OrphanChildRows must now be 0!
	afterReport, err := validation.RunComplianceValidation(svc, ctx, state.ContextID, validation.ComplianceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if afterReport.Summary.OrphanChildRows != 0 {
		t.Fatalf("expected 0 orphan child rows after quarantine, got %d (%+v)", afterReport.Summary.OrphanChildRows, afterReport.OrphanFindings)
	}
}
