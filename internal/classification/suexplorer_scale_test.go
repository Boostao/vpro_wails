package classification

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestSUExplorerLiteralPlotNonCollisionAnd71kScale(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// 1. Literal plot non-collision across '00337' (text), '337' (text), and integer 337.
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		DELETE FROM Sample_Env WHERE PlotNumber IN ('00337', '337');
		DELETE FROM Sample_Admin WHERE Plot IN ('00337', '337');
		INSERT INTO Sample_Env (PlotNumber, Zone, SubZone, SiteSeries) VALUES
			('00337', 'CWH', 'vm1', '01'),
			('337',   'IDF', 'dk1', '03');
		INSERT INTO Sample_Admin (Plot, UserSiteUnit) VALUES
			('00337', 'LZ_UNIT');
		INSERT INTO Sample_Veg (PlotNumber, Species, Cover1, Flag) VALUES
			('00337', 'TSUGHET', 40.0, 0),
			('337',   'PSEUMEN', 25.0, 0);
	`)

	// Create an external SU database with INT_TEXT PlotNumber storing integer 337 (NOT text '337'!),
	// plus another SU row in a VARCHAR SU table for '00337' vs '337'.
	extSUPath := filepath.Join(t.TempDir(), "LiteralNonCollisionSU.db")
	extDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(extSUPath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extDB.Exec(`
		CREATE TABLE _table_metadata (table_name TEXT PRIMARY KEY, description TEXT);
		INSERT INTO _table_metadata (table_name, description) VALUES ('LitInt_SU', 'VP04'), ('LitText_SU', 'VP04');
		CREATE TABLE LitInt_SU (PlotNumber INT_TEXT, SiteUnit VARCHAR);
		INSERT INTO LitInt_SU (PlotNumber, SiteUnit) VALUES (337, 'INT_COLLISION_UNIT');
		CREATE TABLE LitText_SU (PlotNumber VARCHAR, SiteUnit VARCHAR);
		INSERT INTO LitText_SU (PlotNumber, SiteUnit) VALUES ('00337', 'CWHvm1/01');
	`); err != nil {
		extDB.Close()
		t.Fatal(err)
	}
	extDB.Close()

	// Check 1A: integer 337 in LitInt_SU must NOT match text '337' in Sample_Env!
	sel := vprotest.ContextSelection(state)
	sel.SU = "LitInt"
	sel.SUPath = extSUPath
	state, err = service.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext external LitInt_SU: %v", err)
	}
	treeInt, err := GetSUPlotTree(ctx, service, state.ContextID, "")
	if err != nil {
		t.Fatalf("GetSUPlotTree(LitInt): %v", err)
	}
	for _, u := range treeInt.Units {
		for _, p := range u.Plots {
			if p.PlotNumber == "337" {
				t.Fatalf("text plot '337' collided with integer 337 in SU bucket %q", u.SiteUnit)
			}
		}
	}
	if len(treeInt.OrphanPlots) != 1 || treeInt.OrphanPlots[0].SiteUnit != "INT_COLLISION_UNIT" {
		t.Fatalf("expected integer 337 SU row to be classified as orphan, got %+v", treeInt.OrphanPlots)
	}
	sum337Int, err := GetPlotInspectorSummary(ctx, service, state.ContextID, "337")
	if err != nil || sum337Int.CurrentSUUnit != "" {
		t.Fatalf("GetPlotInspectorSummary('337') matched integer 337: %+v (%v)", sum337Int, err)
	}

	// Check 1B: '00337' in LitText_SU matches '00337' ONLY and leaves '337' unassigned!
	sel = vprotest.ContextSelection(state)
	sel.SU = "LitText"
	sel.SUPath = extSUPath
	state, err = service.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext external LitText_SU: %v", err)
	}

	tree, err := GetSUPlotTree(ctx, service, state.ContextID, "")
	if err != nil {
		t.Fatalf("GetSUPlotTree: %v", err)
	}

	foundAssigned00337 := false
	for _, u := range tree.Units {
		for _, p := range u.Plots {
			if p.PlotNumber == "337" {
				t.Fatalf("text plot '337' collided with leading-zero '00337' in SU bucket %q", u.SiteUnit)
			}
			if p.PlotNumber == "00337" && u.SiteUnit == "CWHvm1/01" && p.UserSiteUnit == "LZ_UNIT" {
				foundAssigned00337 = true
			}
		}
	}
	if !foundAssigned00337 {
		t.Fatalf("expected '00337' assigned to CWHvm1/01 with UserSiteUnit=LZ_UNIT, got units=%+v", tree.Units)
	}

	foundUnassigned337 := false
	for _, p := range tree.UnassignedPlots {
		if p.PlotNumber == "337" {
			foundUnassigned337 = true
		}
	}
	if !foundUnassigned337 {
		t.Fatalf("expected text plot '337' to remain unassigned when LitText_SU only has '00337', got unassigned=%+v", tree.UnassignedPlots)
	}

	sum00337, err := GetPlotInspectorSummary(ctx, service, state.ContextID, "00337")
	if err != nil || sum00337.CurrentSUUnit != "CWHvm1/01" || sum00337.UserSiteUnit != "LZ_UNIT" {
		t.Fatalf("GetPlotInspectorSummary('00337') = %+v (%v)", sum00337, err)
	}
	sum337, err := GetPlotInspectorSummary(ctx, service, state.ContextID, "337")
	if err != nil || sum337.CurrentSUUnit != "" || sum337.UserSiteUnit != "" {
		t.Fatalf("GetPlotInspectorSummary('337') collided: %+v (%v)", sum337, err)
	}

	// 2. Production-scale 71,567-plot benchmark for GetSUPlotTree, CreateSUTable(PopulateAllProjectPlots=true), and GetPlotInspectorSummary.
	scaleSvc, scaleState := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, scaleState.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		DELETE FROM Sample_Veg;
		DELETE FROM Sample_Admin;
		DELETE FROM Sample_Env;
		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 71567
		)
		INSERT INTO Sample_Env (PlotNumber, FieldNumber, Zone, SubZone, SiteSeries, MoistureRegime, NutrientRegime, Elevation, Flag)
		SELECT printf('P%06d', n), printf('F%d', n), 'CWH', 'vm1', '01', '4', 'C', 350, 0 FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 71567
		)
		INSERT INTO Sample_Admin (Plot, PlotType, UserSiteUnit)
		SELECT printf('P%06d', n), 'FS882', CASE WHEN n % 2 = 0 THEN 'CWHvm1/01' ELSE 'SBSmk1/05' END FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 150000
		)
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Cover4, Flag)
		SELECT printf('P%06d', ((n - 1) % 71567) + 1), CASE WHEN n % 2 = 0 THEN 'PSEUMEN' ELSE 'TSUGHET' END, n, 25.0, 10.0, 0 FROM seq;
	`)

	boundedCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	t0 := time.Now()
	suCreated, err := CreateSUTable(boundedCtx, scaleSvc, scaleState.ContextID, CreateSUTableRequest{
		Name:                    "BECScale71k",
		Destination:             "project",
		PopulateAllProjectPlots: true,
		SeedFromUserSiteUnit:    true,
		Activate:                true,
	})
	createDur := time.Since(t0)
	if err != nil {
		t.Fatalf("CreateSUTable(71,567 plots) failed or timed out after %v: %v", createDur, err)
	}
	if suCreated.PlotCount != 71567 {
		t.Fatalf("expected 71567 seeded SU rows, got %d", suCreated.PlotCount)
	}
	if createDur > 500*time.Millisecond {
		t.Fatalf("CreateSUTable(71,567 plots) took %v, want <= 500ms", createDur)
	}

	t1 := time.Now()
	scaleTree, err := GetSUPlotTree(boundedCtx, scaleSvc, suCreated.ContextID, "")
	treeDur := time.Since(t1)
	if err != nil {
		t.Fatalf("GetSUPlotTree(71,567 plots) failed or timed out after %v: %v", treeDur, err)
	}
	if scaleTree.TotalProject != 71567 || scaleTree.TotalAssigned != 71567 || len(scaleTree.Units) != 2 {
		t.Fatalf("unexpected 71k SUPlotTree counts: project=%d assigned=%d units=%d", scaleTree.TotalProject, scaleTree.TotalAssigned, len(scaleTree.Units))
	}
	if treeDur > 500*time.Millisecond {
		t.Fatalf("GetSUPlotTree(71,567 plots) took %v, want <= 500ms", treeDur)
	}

	t2 := time.Now()
	insp, err := GetPlotInspectorSummary(boundedCtx, scaleSvc, suCreated.ContextID, "P071567")
	inspDur := time.Since(t2)
	if err != nil || insp.PlotNumber != "P071567" || len(insp.TopSpecies) == 0 {
		t.Fatalf("GetPlotInspectorSummary(P071567) failed: %+v (%v)", insp, err)
	}
	if inspDur > 100*time.Millisecond {
		t.Fatalf("GetPlotInspectorSummary on 71.5k plots / 150k veg rows took %v, want <= 100ms", inspDur)
	}
}
