package validation_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/validation"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestComplianceStrataCoverBoundsLiteralNonCollisionAnd71kScale(t *testing.T) {
	ctx := context.Background()

	// 1. StrataCover float32 <100 / >=0 bounds + literal '00337' vs '337' vs integer 337 non-collision.
	svc, state := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		DELETE FROM Sample_Env WHERE PlotNumber IN ('00337', '337');
		DELETE FROM Sample_Admin WHERE Plot IN ('00337', '337');
		INSERT INTO Sample_Env (PlotNumber, Zone, StrataCoverTree, StrataCoverShrub, StrataCoverHerb, StrataCoverMoss) VALUES
			('00337', 'CWH', 99.999996, 99.999999, -0.5, 100.0),
			('337',   'IDF', 45.0,      12.5,      99.999996, 0.0);
		INSERT INTO Sample_Admin (Plot, PlotType, StrataCoverTotal) VALUES
			('00337', 'FS882', 99.999999),
			('337',   'FS882', 99.999996);

		-- Put an integer 337 orphan row into Sample_Humus (which has no SwitchContext required-column check on PlotNumber affinity)
		-- and preserve all Sample_Veg columns while inserting text '00337' and '337':
		ALTER TABLE Sample_Humus RENAME TO Sample_Humus_Orig;
		CREATE TABLE Sample_Humus (
			PlotNumber BLOB, Horizon VARCHAR, UpperDepth DOUBLE, LowerDepth DOUBLE,
			HumusStructureDegree VARCHAR, HumusStructureKind VARCHAR, MycelAbundance VARCHAR,
			FecalAbundance VARCHAR, RootsAbundance VARCHAR, RootsSize VARCHAR,
			vonPost INTEGER, HumusFormpH DOUBLE, Consistence VARCHAR, Character VARCHAR,
			Fauna VARCHAR, Comment VARCHAR, Flag BOOLEAN, ID INTEGER
		);
		INSERT INTO Sample_Humus SELECT * FROM Sample_Humus_Orig WHERE PlotNumber NOT IN ('00337', '337');
		DROP TABLE Sample_Humus_Orig;
		INSERT INTO Sample_Humus (PlotNumber, Horizon, UpperDepth, LowerDepth, Flag, ID) VALUES
			('00337', 'Fm', 6.0, 2.0, 0, 9101),
			('337',   'Fm', 5.0, 1.0, 0, 9102),
			(337,     'Fm', 4.0, 1.0, 0, 9103);
		INSERT INTO Sample_Veg (PlotNumber, Species, Cover1, Flag, ID) VALUES
			('00337', 'TSUGHET', 25.0, 0, 9201),
			('337',   'PSEUMEN', 30.0, 0, 9202);
	`)

	extSUPath := filepath.Join(t.TempDir(), "ComplianceLitSU.db")
	extDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(extSUPath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extDB.Exec(`
		CREATE TABLE _table_metadata (table_name TEXT PRIMARY KEY, description TEXT);
		INSERT INTO _table_metadata (table_name, description) VALUES ('LitComp_SU', 'VP04');
		CREATE TABLE LitComp_SU (PlotNumber INT_TEXT, SiteUnit VARCHAR);
		INSERT INTO LitComp_SU (PlotNumber, SiteUnit) VALUES (337, 'BAD_INT_SU');
	`); err != nil {
		extDB.Close()
		t.Fatal(err)
	}
	extDB.Close()

	sel := vprotest.ContextSelection(state)
	sel.SU = "LitComp"
	sel.SUPath = extSUPath
	state, err = svc.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext: %v", err)
	}

	rep, err := validation.RunComplianceValidation(svc, ctx, state.ContextID, validation.ComplianceOptions{UseActiveSU: false})
	if err != nil {
		t.Fatalf("RunComplianceValidation: %v", err)
	}

	flaggedFields := map[string]bool{}
	for _, b := range rep.BoundsFindings {
		flaggedFields[b.PlotNumber+":"+b.Table+":"+b.Field] = true
	}
	for _, mustPass := range []string{
		"00337:Env:StrataCoverTree",
		"337:Env:StrataCoverHerb",
		"337:Admin:StrataCoverTotal",
	} {
		if flaggedFields[mustPass] {
			t.Fatalf("valid float32 <100 cover %s was falsely flagged: %+v", mustPass, rep.BoundsFindings)
		}
	}
	for _, mustFail := range []string{
		"00337:Env:StrataCoverShrub",
		"00337:Env:StrataCoverHerb",
		"00337:Env:StrataCoverMoss",
		"00337:Admin:StrataCoverTotal",
	} {
		if !flaggedFields[mustFail] {
			t.Fatalf("invalid StrataCover %s was NOT flagged: %+v", mustFail, rep.BoundsFindings)
		}
	}

	// Verify integer 337 in Sample_Humus and LitComp_SU is detected as an orphan (and does NOT match text '337' in Sample_Env!)
	humusOrphan337, suOrphan337 := false, false
	for _, o := range rep.OrphanFindings {
		if o.Table == "Humus" && o.PlotNumber == "337" && o.RowCount == 1 {
			humusOrphan337 = true
		}
		if o.Table == "SU" && o.PlotNumber == "337" && o.RowCount == 1 {
			suOrphan337 = true
		}
		if o.PlotNumber == "00337" {
			t.Fatalf("valid leading-zero '00337' was falsely marked orphan: %+v", o)
		}
	}
	if !humusOrphan337 || !suOrphan337 {
		t.Fatalf("expected integer 337 in Humus and SU to be flagged as orphan despite text '337' in Env: %+v", rep.OrphanFindings)
	}

	// QuarantineOrphanRecords must move ONLY integer 337 (ID=9103) and preserve text '337' (ID=9102) and '00337' (ID=9101)!
	qRes, err := validation.QuarantineOrphanRecords(svc, ctx, state.ContextID, validation.QuarantineOrphanRequest{Tables: []string{"Humus"}})
	if err != nil {
		t.Fatalf("QuarantineOrphanRecords: %v", err)
	}
	if qRes.TotalQuarantined != 1 {
		t.Fatalf("expected exactly 1 quarantined row (integer 337), got %+v", qRes)
	}
	roDB, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer roDB.Close()
	var keptCount int
	if err := roDB.QueryRow(`SELECT COUNT(*) FROM Sample_Humus WHERE ID IN (9101, 9102)`).Scan(&keptCount); err != nil || keptCount != 2 {
		t.Fatalf("text '00337' or '337' was wrongly quarantined: keptCount=%d err=%v", keptCount, err)
	}

	// 2. Production-scale 71,567-plot + 250,000-veg-row compliance benchmark (< 2s full, < 500ms SU-scoped).
	scaleSvc, scaleState := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, scaleState.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		DELETE FROM Sample_Veg;
		DELETE FROM Sample_Humus;
		DELETE FROM Sample_Mineral;
		DELETE FROM Sample_Other;
		DELETE FROM Sample_Audit;
		DELETE FROM Sample_Admin;
		DELETE FROM Sample_Env;

		CREATE TABLE Scale_SU (PlotNumber VARCHAR, SiteUnit VARCHAR);
		CREATE UNIQUE INDEX uidx_Scale_SU_PlotNumber ON Scale_SU(PlotNumber);
		INSERT INTO _table_metadata (table_name, description) VALUES ('Scale_SU', 'VP04');

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 71567
		)
		INSERT INTO Sample_Env (PlotNumber, Zone, MoistureRegime, NutrientRegime, Latitude, Longitude, Aspect, SlopeGradient, StrataCoverTree)
		SELECT printf('P%06d', n), 'CWH', '4', 'C', 49.5, -123.5, 180, 15.0, 45.0 FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 71567
		)
		INSERT INTO Sample_Admin (Plot, StartDate, StrataCoverTotal)
		SELECT printf('P%06d', n), 2024, 60.0 FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 250000
		)
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Cover4, Flag)
		SELECT printf('P%06d', ((n - 1) % 71567) + 1),
		       CASE ((n - 1) / 71567) WHEN 0 THEN 'PSEUMEN' WHEN 1 THEN 'TSUGHET' WHEN 2 THEN 'ABIEAMA' ELSE 'THUJPLI' END,
		       n, 25.0, 10.0, 0 FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 438
		)
		INSERT INTO Scale_SU (PlotNumber, SiteUnit)
		SELECT printf('P%06d', n), 'CWHvm1/01' FROM seq;
	`)

	scaleSel := vprotest.ContextSelection(scaleState)
	scaleSel.SU = "Scale"
	scaleSel.SUPath = scaleState.ProjectPath
	scaleState, err = scaleSvc.SwitchContext(scaleState.ContextID, scaleSel)
	if err != nil {
		t.Fatal(err)
	}

	boundedCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tFull := time.Now()
	fullRep, err := validation.RunComplianceValidation(scaleSvc, boundedCtx, scaleState.ContextID, validation.ComplianceOptions{UseActiveSU: false})
	fullDur := time.Since(tFull)
	if err != nil {
		t.Fatalf("RunComplianceValidation(71,567 plots, full) failed or timed out after %v: %v", fullDur, err)
	}
	if !fullRep.Passed {
		t.Fatalf("expected clean 71k dataset to pass compliance, got summary=%+v", fullRep.Summary)
	}
	if fullDur > 2*time.Second {
		t.Fatalf("RunComplianceValidation(71,567 plots, full) took %v, want <= 2s", fullDur)
	}

	tScoped := time.Now()
	scopedRep, err := validation.RunComplianceValidation(scaleSvc, boundedCtx, scaleState.ContextID, validation.ComplianceOptions{UseActiveSU: true})
	scopedDur := time.Since(tScoped)
	if err != nil {
		t.Fatalf("RunComplianceValidation(71,567 plots, UseActiveSU=true) failed or timed out after %v: %v", scopedDur, err)
	}
	if !scopedRep.Passed || scopedDur > 500*time.Millisecond {
		t.Fatalf("RunComplianceValidation(UseActiveSU=true) passed=%v dur=%v (want <= 500ms)", scopedRep.Passed, scopedDur)
	}
}
