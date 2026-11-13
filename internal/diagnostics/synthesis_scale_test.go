package diagnostics

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func TestSynthesisSnapshotAndReassign71kBECMasterCohortUnder50ms(t *testing.T) {
	ctx := context.Background()
	root := workspaceScratchDir(t)
	dataDir := filepath.Join(root, "data")
	cfgDir := filepath.Join(root, "config")
	for _, d := range []string{dataDir, cfgDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	preferences, err := vprocore.OpenDesktopConfig(dataDir, cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	projects, err := vprocore.NewSQLiteProjectService(dataDir, cfgDir, preferences)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projects.CloseSQLiteContext() })
	plots, err := vprocore.NewPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	contexts, err := vprocore.NewContextService(projects, plots)
	if err != nil {
		t.Fatal(err)
	}
	initState, err := projects.GetState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Seed 71,567 plots in Sample_Env & Sample_Admin + 500,000 rows in Sample_Veg,
	// plus a 438-plot BECMaster-scale SU cohort (including leading-zero '00337')!
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(initState.ProjectPath, "rw")+"&_foreign_keys=off&_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		DELETE FROM Sample_Veg;
		DELETE FROM Sample_Admin;
		DELETE FROM Sample_Env;

		CREATE TABLE Cohort438_SU ("PlotNumber" VARCHAR UNIQUE, "SiteUnit" VARCHAR);
		CREATE INDEX idx_Cohort438_SU_SiteUnit ON Cohort438_SU("SiteUnit");
		INSERT OR REPLACE INTO _table_metadata (table_name, description) VALUES ('Cohort438_SU', 'VP04');

		INSERT INTO Sample_Env ("PlotNumber", "Zone", "SubZone", "SiteSeries", "MoistureRegime", "NutrientRegime")
		VALUES ('00337', 'CWH', 'vm1', '01', '4', 'C');
		INSERT INTO Sample_Admin ("Plot", "UserSiteUnit") VALUES ('00337', 'CWHvm1/01');
		INSERT INTO Cohort438_SU ("PlotNumber", "SiteUnit") VALUES ('00337', 'CWHvm1/01');
		INSERT INTO Sample_Veg ("PlotNumber", "ID", "Species", "Cover1", "Cover6") VALUES
			('00337', 1, 'PSEUMEN', 40.0, NULL),
			('00337', 2, 'POLYMUN', NULL, 25.0);

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 71566
		)
		INSERT INTO Sample_Env ("PlotNumber", "Zone", "SubZone", "SiteSeries", "MoistureRegime", "NutrientRegime")
		SELECT printf('P%06d', n), 'CWH', 'vm1', '01', '4', 'C' FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 71566
		)
		INSERT INTO Sample_Admin ("Plot", "UserSiteUnit")
		SELECT printf('P%06d', n), 'CWHvm1/01' FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 500000
		)
		INSERT INTO Sample_Veg ("PlotNumber", "ID", "Species", "Cover1", "Cover4", "Cover6")
		SELECT printf('P%06d', ((n - 1) % 71566) + 1),
		       n + 10,
		       CASE (n % 5)
		           WHEN 0 THEN 'TSUGHET'
		           WHEN 1 THEN 'THUJPLI'
		           WHEN 2 THEN 'ABIEAMA'
		           WHEN 3 THEN 'VACCMEM'
		           ELSE 'HYLOCOM'
		       END,
		       30.0, 15.0, 10.0
		FROM seq;

		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 437
		)
		INSERT INTO Cohort438_SU ("PlotNumber", "SiteUnit")
		SELECT printf('P%06d', n),
		       CASE (n % 3) WHEN 0 THEN 'CWHvm1/01' WHEN 1 THEN 'CWHvm1/05' ELSE 'CWHvm1/07' END
		FROM seq;
	`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	state, err := contexts.SwitchContext(initState.ContextID, vprocore.ContextSelection{
		Project:       initState.ActiveProject,
		ProjectPath:   initState.ProjectPath,
		SU:            "Cohort438",
		SUPath:        initState.ProjectPath,
		Hierarchy:     initState.ActiveHierarchy,
		HierarchyPath: initState.HierarchyPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	opts := SynthesisOptions{
		UnitSource:               "su",
		ExcludeStandardModifiers: true,
	}
	// Warm SQLite page cache once
	if _, err := ComputeSynthesisSnapshot(ctx, contexts, state.ContextID, opts); err != nil {
		t.Fatalf("warmup ComputeSynthesisSnapshot: %v", err)
	}

	tSnap := time.Now()
	snap, err := ComputeSynthesisSnapshot(ctx, contexts, state.ContextID, opts)
	snapDur := time.Since(tSnap)
	if err != nil {
		t.Fatalf("ComputeSynthesisSnapshot(71.5k plots, 438 SU): %v", err)
	}
	if snap.ClassifiedPlotCount != 438 || snap.TotalProjectPlots != 71567 || len(snap.Units) != 3 {
		t.Fatalf("unexpected 438-cohort snapshot counts: classified=%d total=%d units=%d",
			snap.ClassifiedPlotCount, snap.TotalProjectPlots, len(snap.Units))
	}
	if snapDur >= synthesisLatencyBudget {
		t.Fatalf("ComputeSynthesisSnapshot on 71.5k plots / 500k veg rows took %v, want < %v", snapDur, synthesisLatencyBudget)
	}

	tReassign := time.Now()
	reassigned, err := ReassignSynthesisPlotUnit(ctx, contexts, state.ContextID, ReassignSynthesisPlotRequest{
		PlotNumber:     "00337",
		TargetSiteUnit: "CWHvm1/05",
		Options:        opts,
	})
	reassignDur := time.Since(tReassign)
	if err != nil {
		t.Fatalf("ReassignSynthesisPlotUnit(00337): %v", err)
	}
	if !reassigned.Changed || reassignDur >= synthesisLatencyBudget {
		t.Fatalf("ReassignSynthesisPlotUnit on 71.5k plots / 500k veg rows took %v (reported %.2fms), want < %v",
			reassignDur, reassigned.LatencyMs, synthesisLatencyBudget)
	}
}
