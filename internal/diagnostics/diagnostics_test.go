package diagnostics

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func workspaceScratchDir(tb testing.TB) string {
	tb.Helper()
	return tb.TempDir()
}

func codePtr(s string) *string { return &s }

func TestDiagnosticClassificationParityWithVProR(t *testing.T) {
	// Exact parity vectors from sibling vpro/tests/testthat/test-diagnostic-classification.R
	cases := []struct {
		name     string
		inputs   []OrderedUnitCode
		wantUnit string
		wantDiag string
	}{
		{
			name:     "d_and_cd_overlap",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("5 - 5")}, {Unit: "B", Code: codePtr("1 - 1")}},
			wantUnit: "A",
			wantDiag: "d, cd",
		},
		{
			name:     "constant_only",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("5 - 4")}, {Unit: "B", Code: codePtr("4 - 4")}},
			wantUnit: "A",
			wantDiag: "c",
		},
		{
			name:     "dominant_differential_equal_presence",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("2 - 7")}, {Unit: "B", Code: codePtr("2 - 1")}},
			wantUnit: "A",
			wantDiag: "dd",
		},
		{
			name: "ic_unreachable_when_multiple_present_units_within_two_classes",
			inputs: []OrderedUnitCode{
				{Unit: "A", Code: codePtr("2 - +")},
				{Unit: "B", Code: codePtr("1 - +")},
				{Unit: "C", Code: codePtr("1 - +")},
			},
			wantUnit: "",
			wantDiag: "",
		},
		{
			name:     "single_unit_presence_2_reaches_ic",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("2 - 3")}, {Unit: "B", Code: nil}},
			wantUnit: "A",
			wantDiag: "ic",
		},
		{
			name:     "differential_presence_3_vs_1",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("3 - 4")}, {Unit: "B", Code: codePtr("1 - 1")}},
			wantUnit: "A",
			wantDiag: "d",
		},
		{
			name:     "single_unit_high_cover_dd",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("1 - 9")}},
			wantUnit: "A",
			wantDiag: "dd",
		},
		{
			name:     "d_cd_high_cover_in_second_unit",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("5 - 9")}, {Unit: "B", Code: codePtr("3 - 8")}},
			wantUnit: "A",
			wantDiag: "d, cd",
		},
		{
			name: "skips_leading_null_and_picks_first_max_presence",
			inputs: []OrderedUnitCode{
				{Unit: "Z", Code: nil},
				{Unit: "B", Code: codePtr("5 - 8")},
				{Unit: "A", Code: codePtr("5 - 1")},
			},
			wantUnit: "B",
			wantDiag: "dd",
		},
		{
			name:     "first_max_field_wins_even_when_second_has_higher_significance",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("5 - 1")}, {Unit: "B", Code: codePtr("5 - 8")}},
			wantUnit: "",
			wantDiag: "",
		},
		{
			name:     "all_null_cells",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: nil}, {Unit: "B", Code: nil}},
			wantUnit: "",
			wantDiag: "",
		},
		{
			name:     "low_presence_no_diagnosis",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("1 - +")}, {Unit: "B", Code: codePtr("1 - 1")}},
			wantUnit: "",
			wantDiag: "",
		},
		{
			name:     "d_and_c_with_plus_significance",
			inputs:   []OrderedUnitCode{{Unit: "A", Code: codePtr("5 - +")}, {Unit: "B", Code: codePtr("1 - +")}},
			wantUnit: "A",
			wantDiag: "d, c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClassifySpeciesDiagnostic(tc.inputs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Unit != tc.wantUnit || got.Diagnosis != tc.wantDiag {
				t.Fatalf("got (unit=%q, diagnosis=%q), want (unit=%q, diagnosis=%q)",
					got.Unit, got.Diagnosis, tc.wantUnit, tc.wantDiag)
			}
		})
	}

	// Validation error cases matching test-diagnostic-classification.R
	invalidCases := [][]OrderedUnitCode{
		{},
		{{Unit: "", Code: codePtr("3 - 5")}},
		{{Unit: "A", Code: codePtr("3 - 5")}, {Unit: "A", Code: codePtr("2 - 3")}},
		{{Unit: "A", Code: codePtr("3-5")}},
		{{Unit: "A", Code: codePtr("0 - 5")}},
		{{Unit: "A", Code: codePtr("3 - 0")}},
		{{Unit: "A", Code: codePtr("3 - X")}},
	}
	for idx, inv := range invalidCases {
		if _, err := ClassifySpeciesDiagnostic(inv); err == nil {
			t.Fatalf("invalid case #%d expected an error, got nil", idx+1)
		}
	}
}

func TestPresenceDominKrajinaFloat32AndACERounding(t *testing.T) {
	// PresenceClass 1..5 / I..V boundaries
	presCases := []struct {
		ratio float64
		num   int
		rom   string
	}{
		{0.05, 1, "I"},
		{0.20, 1, "I"},
		{0.2001, 2, "II"},
		{0.40, 2, "II"},
		{0.4001, 3, "III"},
		{0.60, 3, "III"},
		{0.6001, 4, "IV"},
		{0.80, 4, "IV"},
		{0.8001, 5, "V"},
		{1.00, 5, "V"},
	}
	for _, pc := range presCases {
		gotNum, gotRom := DiagnosticPresenceClass(pc.ratio)
		if gotNum != pc.num || gotRom != pc.rom {
			t.Fatalf("DiagnosticPresenceClass(%v) = (%d, %q), want (%d, %q)", pc.ratio, gotNum, gotRom, pc.num, pc.rom)
		}
	}

	// Domin-Krajina SignifClass boundaries matching V7mdlValueConversions.SignifClass
	sigCases := []struct {
		cover float64
		cls   string
		num   int
	}{
		{0.1, "+", 0},
		{0.3, "+", 0},
		{0.31, "1", 1},
		{1.0, "1", 1},
		{1.01, "2", 2},
		{2.2, "2", 2},
		{2.21, "3", 3},
		{5.0, "3", 3},
		{5.01, "4", 4},
		{10.0, "4", 4},
		{10.01, "5", 5},
		{20.0, "5", 5},
		{20.01, "6", 6},
		{33.0, "6", 6},
		{33.01, "7", 7},
		{50.0, "7", 7},
		{50.01, "8", 8},
		{75.0, "8", 8},
		{75.01, "9", 9},
		{99.0, "9", 9},
	}
	for _, sc := range sigCases {
		gotCls, gotNum := DominKrajinaSignifClass(sc.cover)
		if gotCls != sc.cls || gotNum != sc.num {
			t.Fatalf("DominKrajinaSignifClass(%v) = (%q, %d), want (%q, %d)", sc.cover, gotCls, gotNum, sc.cls, sc.num)
		}
	}

	// IEEE-754 SINGLE float32 narrowing (<100 Or Is Null invariant)
	if s, ok := NarrowSingleCover(99.999996); !ok || s >= 100 {
		t.Fatalf("expected 99.999996 to narrow below 100 and pass, got (%v, %v)", s, ok)
	}
	if s, ok := NarrowSingleCover(99.999999); ok || s != 100 {
		t.Fatalf("expected 99.999999 to narrow to 100 and reject, got (%v, %v)", s, ok)
	}

	// ACE 15-sig-digit rounding invariant (1.005 -> 1.01 at 2 decimals)
	if got := FormatACE15(1.005, 2); got != "1.01" {
		t.Fatalf("FormatACE15(1.005, 2) = %q, want %q", got, "1.01")
	}
	if got := RoundACE15(1.005, 2); got != 1.01 {
		t.Fatalf("RoundACE15(1.005, 2) = %v, want 1.01", got)
	}
}

type synthesisTestEnv struct {
	ctx     context.Context
	service *vprocore.ContextService
	state   vprocore.ProjectState
	rootDir string
}

func buildSynthesisFixture(tb testing.TB, extraPlotsPerUnit int) synthesisTestEnv {
	tb.Helper()
	ctx := context.Background()
	root := workspaceScratchDir(tb)
	dataDir := filepath.Join(root, "data")
	cfgDir := filepath.Join(root, "config")
	for _, d := range []string{dataDir, cfgDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			tb.Fatal(err)
		}
	}

	preferences, err := vprocore.OpenDesktopConfig(dataDir, cfgDir)
	if err != nil {
		tb.Fatal(err)
	}
	projects, err := vprocore.NewSQLiteProjectService(dataDir, cfgDir, preferences)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		_ = projects.CloseSQLiteContext()
	})
	plots, err := vprocore.NewPlotServiceWithPreferences(projects)
	if err != nil {
		tb.Fatal(err)
	}
	contexts, err := vprocore.NewContextService(projects, plots)
	if err != nil {
		tb.Fatal(err)
	}

	initState, err := projects.GetState(ctx)
	if err != nil {
		tb.Fatal(err)
	}

	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(initState.ProjectPath, "rw")+"&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		tb.Fatal(err)
	}

	// Clear default Sample plots so we have an exact, self-contained ecological synthesis dataset
	for _, tbl := range []string{"Sample_Veg", "Sample_Humus", "Sample_Mineral", "Sample_Other", "Sample_Audit", "Sample_Admin", "Sample_Env"} {
		if _, err := db.Exec(`DELETE FROM ` + vprocore.QuoteHeaderIdentifier(tbl)); err != nil {
			_ = db.Close()
			tb.Fatal(err)
		}
	}

	if _, err := db.Exec(`
		CREATE TABLE Working_SU ("PlotNumber" VARCHAR UNIQUE, "SiteUnit" VARCHAR);
		CREATE TABLE Master_SU ("PlotNumber" VARCHAR UNIQUE, "SiteUnit" VARCHAR);
		CREATE TABLE CombineTest_Lump ("LumpCode" VARCHAR(50), "SppCode" VARCHAR(8) UNIQUE, "Use" BOOLEAN);
		INSERT OR REPLACE INTO _table_metadata (table_name, description) VALUES
			('Working_SU', 'VP04'),
			('Master_SU', 'VP04'),
			('CombineTest_Lump', 'VP05');
		INSERT INTO CombineTest_Lump ("LumpCode", "SppCode", "Use") VALUES
			('TSUGHET', 'TSUGHET1', -1),
			('TSUGHET', 'TSUGHET2', -1),
			('IGNORED', 'THUJPLI', 0);
	`); err != nil {
		_ = db.Close()
		tb.Fatal(err)
	}

	insertPlot := func(plotNum, zone, subZone, series, smr, snr, suUnit string, veg map[string]float64) {
		if _, err := db.Exec(
			`INSERT INTO Sample_Env ("PlotNumber", "Zone", "SubZone", "SiteSeries", "MoistureRegime", "NutrientRegime")
			 VALUES (?, ?, ?, ?, ?, ?)`,
			plotNum, zone, subZone, series, smr, snr,
		); err != nil {
			_ = db.Close()
			tb.Fatal(err)
		}
		if _, err := db.Exec(
			`INSERT INTO Sample_Admin ("Plot", "UserSiteUnit") VALUES (?, ?)`,
			plotNum, suUnit,
		); err != nil {
			_ = db.Close()
			tb.Fatal(err)
		}
		if suUnit != "" {
			if _, err := db.Exec(`INSERT INTO Working_SU ("PlotNumber", "SiteUnit") VALUES (?, ?)`, plotNum, suUnit); err != nil {
				_ = db.Close()
				tb.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO Master_SU ("PlotNumber", "SiteUnit") VALUES (?, ?)`, plotNum, suUnit); err != nil {
				_ = db.Close()
				tb.Fatal(err)
			}
		}
		idSeq := 1
		for sp, cov := range veg {
			if _, err := db.Exec(
				`INSERT INTO Sample_Veg ("PlotNumber", "ID", "Species", "Cover1", "Cover6") VALUES (?, ?, ?, ?, NULL)`,
				plotNum, idSeq, sp, cov,
			); err != nil {
				_ = db.Close()
				tb.Fatal(err)
			}
			idSeq++
		}
	}

	// Unit 1: CWHvm1/01 (5 normal mesic HwBa-Blueberry plots + 1 misplaced outlier '00337' with CWHvm1/05 rich flora)
	nUnit := 5 + extraPlotsPerUnit
	for i := 1; i <= nUnit; i++ {
		pNum := fmt.Sprintf("U01_%02d", i)
		veg := map[string]float64{
			"TSUGHET1": 65.0, // lumps with TSUGHET2 -> 120% capped to 99%
			"TSUGHET2": 55.0,
			"ABIEAMA":  28.0,
			"VACCMEM":  18.0,
			"HYLOCOM":  40.0,
		}
		insertPlot(pNum, "CWH", "vm1", "01", "4", "C", "CWHvm1/01", veg)
	}

	// Leading-zero plot '00337' deliberately misassigned to CWHvm1/01 while carrying pure CWHvm1/05 rich Swordfern/Foamflower veg!
	insertPlot("00337", "CWH", "vm1", "05", "5", "D", "CWHvm1/01", map[string]float64{
		"THUJPLI": 52.0,
		"POLYMUN": 38.0,
		"TIARTRI": 22.0,
		"OPANHOR": 16.0,
	})

	// Unit 2: CWHvm1/05 (5 rich CwHw-Foamflower plots)
	for i := 1; i <= nUnit; i++ {
		pNum := fmt.Sprintf("U05_%02d", i)
		insertPlot(pNum, "CWH", "vm1", "05", "5", "D", "CWHvm1/05", map[string]float64{
			"THUJPLI": 50.0,
			"POLYMUN": 35.0,
			"TIARTRI": 24.0,
			"OPANHOR": 15.0,
		})
	}

	// Unit 3: CWHvm1/09 (singleton unit with 1 plot to verify Leave-One-Out 0% self-bias)
	insertPlot("U09_01", "CWH", "vm1", "09", "7", "E", "CWHvm1/09", map[string]float64{
		"PICESIT": 45.0,
		"LYSIAME": 30.0,
	})

	// Standard excluded modifier units (.support, .poor, .nudum) + unassigned plot
	insertPlot("SUP_01", "CWH", "vm1", "01", "4", "B", "CWHvm1/01.support", map[string]float64{"TSUGHET": 30.0})
	insertPlot("POR_01", "CWH", "vm1", "05", "3", "A", "CWHvm1/05.poor", map[string]float64{"PINUCON": 25.0})
	insertPlot("UNAS_1", "CWH", "vm1", "", "2", "B", "", map[string]float64{"PSEUMEN": 40.0})

	if err := db.Close(); err != nil {
		tb.Fatal(err)
	}

	nextState, err := contexts.SwitchContext(initState.ContextID, vprocore.ContextSelection{
		Project:       initState.ActiveProject,
		ProjectPath:   initState.ProjectPath,
		SU:            "Working",
		SUPath:        initState.ProjectPath,
		Hierarchy:     initState.ActiveHierarchy,
		HierarchyPath: initState.HierarchyPath,
	})
	if err != nil {
		tb.Fatal(err)
	}

	return synthesisTestEnv{
		ctx:     ctx,
		service: contexts,
		state:   nextState,
		rootDir: root,
	}
}

func TestComputeSynthesisSnapshotLumpingExclusionsOutliersAndEdatopic(t *testing.T) {
	env := buildSynthesisFixture(t, 0)

	lumps, err := ListSynthesisLumpTables(env.ctx, env.service, env.state.ContextID)
	if err != nil {
		t.Fatalf("ListSynthesisLumpTables failed: %v", err)
	}
	var combineOpt *SynthesisLumpTableOption
	for i := range lumps {
		if lumps[i].TableName == "CombineTest_Lump" {
			combineOpt = &lumps[i]
			break
		}
	}
	if combineOpt == nil || combineOpt.RowCount != 3 || combineOpt.ActiveCount != 2 {
		t.Fatalf("unexpected CombineTest_Lump option in %+v", lumps)
	}

	snap, err := ComputeSynthesisSnapshot(env.ctx, env.service, env.state.ContextID, SynthesisOptions{
		UnitSource:               "su",
		LumpTable:                "CombineTest",
		ExcludeStandardModifiers: true,
	})
	if err != nil {
		t.Fatalf("ComputeSynthesisSnapshot failed: %v", err)
	}

	if snap.ExcludedPlotCount != 2 || len(snap.ExcludedUnits) != 2 {
		t.Fatalf("expected 2 excluded modifier plots (.support & .poor), got count=%d units=%+v",
			snap.ExcludedPlotCount, snap.ExcludedUnits)
	}
	if snap.UnassignedPlotCount != 1 {
		t.Fatalf("expected 1 unassigned plot, got %d", snap.UnassignedPlotCount)
	}
	if len(snap.Units) != 3 {
		t.Fatalf("expected 3 retained units, got %d", len(snap.Units))
	}

	// Verify 99% cap on lumped TSUGHET (65 + 55 = 120 -> capped to 99 across 5 present plots in 6-plot unit = 82.5% mean cover)
	if snap.LumpFoldedTaxa != 2 || snap.LumpCappedCells == 0 {
		t.Fatalf("expected 2 folded taxa and >0 capped cells, got folded=%d capped=%d", snap.LumpFoldedTaxa, snap.LumpCappedCells)
	}
	var tsuRow *SynthesisSpeciesRow
	for i := range snap.Species {
		if snap.Species[i].Species == "TSUGHET" {
			tsuRow = &snap.Species[i]
			break
		}
	}
	if tsuRow == nil {
		t.Fatalf("expected folded TSUGHET row in species synthesis")
	}
	// 5 plots with 99.0% cover out of 6 plots in CWHvm1/01 => MeanCoverPresent == 99.0, MeanCoverAll == 82.5
	if tsuRow.Cells[0].MeanCoverPresent != 99.0 || tsuRow.Cells[0].MeanCoverAll != 82.5 {
		t.Fatalf("expected TSUGHET 99%% cap (present=99.0, all=82.5), got present=%v all=%v",
			tsuRow.Cells[0].MeanCoverPresent, tsuRow.Cells[0].MeanCoverAll)
	}
	// Presence 5/6 = 83.3% -> V, Significance 82.5% -> 9, absent in other units -> "d, cd"
	if tsuRow.DiagnosticUnit != "CWHvm1/01" || tsuRow.Diagnosis != "d, cd" {
		t.Fatalf("expected TSUGHET diagnosis 'd, cd' on CWHvm1/01, got unit=%q diag=%q",
			tsuRow.DiagnosticUnit, tsuRow.Diagnosis)
	}

	// Verify Pairwise Similarity Matrix symmetry & diagonal 100%
	if len(snap.SimilarityMatrix) != 3 {
		t.Fatalf("expected 3x3 similarity matrix, got %d", len(snap.SimilarityMatrix))
	}
	for i := range 3 {
		if snap.SimilarityMatrix[i].Cells[i].SimilarityPct != 100.0 {
			t.Fatalf("expected diagonal similarity 100%%, got %v", snap.SimilarityMatrix[i].Cells[i].SimilarityPct)
		}
		for j := range 3 {
			a := snap.SimilarityMatrix[i].Cells[j].Similarity
			b := snap.SimilarityMatrix[j].Cells[i].Similarity
			if math.Abs(a-b) > 1e-9 {
				t.Fatalf("similarity matrix not symmetric at (%d,%d): %v != %v", i, j, a, b)
			}
		}
	}
	if len(snap.PairwiseComparisons) != 3 {
		t.Fatalf("expected 3 pairwise comparisons, got %d", len(snap.PairwiseComparisons))
	}

	// Verify Outlier Queue: leading-zero plot '00337' must rank #1 Misplaced toward CWHvm1/05!
	if len(snap.OutlierQueue) != snap.ClassifiedPlotCount {
		t.Fatalf("expected %d outlier queue rows, got %d", snap.ClassifiedPlotCount, len(snap.OutlierQueue))
	}
	topOutlier := snap.OutlierQueue[0]
	if topOutlier.PlotNumber != "00337" || !topOutlier.Misplaced || topOutlier.NearestRivalUnit != "CWHvm1/05" || !topOutlier.Top5PercentAnomaly {
		t.Fatalf("expected plot '00337' ranked #1 misplaced toward CWHvm1/05, got %+v", topOutlier)
	}
	if topOutlier.OwnSimilarity != 0 || topOutlier.RivalSimilarity <= 0.9 {
		t.Fatalf("expected LOO OwnSimilarity=0 and RivalSimilarity>0.9 for '00337', got own=%v rival=%v",
			topOutlier.OwnSimilarity, topOutlier.RivalSimilarity)
	}

	// Verify singleton unit U09_01 has Leave-One-Out SingletonUnit=true and OwnSimilarity=0 (no 100% self-bias)
	var singletonFound bool
	for _, cand := range snap.OutlierQueue {
		if cand.PlotNumber == "U09_01" {
			singletonFound = true
			if !cand.SingletonUnit || cand.OwnSimilarity != 0 {
				t.Fatalf("expected singleton unit plot U09_01 to have SingletonUnit=true and OwnSimilarity=0, got %+v", cand)
			}
		}
	}
	if !singletonFound {
		t.Fatalf("expected U09_01 in outlier queue")
	}

	// Verify 9x6 Relative Edatopic Grids (rSMR 0..8 x rSNR A..F = 54 cells)
	if len(snap.OverallEdatopicGrid.Cells) != 54 {
		t.Fatalf("expected 54 edatopic cells (9x6), got %d", len(snap.OverallEdatopicGrid.Cells))
	}
	// SMR 4 (row 4), SNR C (col 2) => index 4*6 + 2 = 26 should have 5 U01 plots
	cell4C := snap.OverallEdatopicGrid.Cells[4*6+2]
	if cell4C.SMR != "4" || cell4C.SNR != "C" || cell4C.PlotCount != 5 {
		t.Fatalf("expected edatopic cell (4, C) count=5, got %+v", cell4C)
	}
}

func TestReassignSynthesisPlotUnitLatencyUndoAndProtectionGuards(t *testing.T) {
	// Seed 25 plots per main unit (~53 total plots, ~250 veg rows) to test realistic <50ms target
	env := buildSynthesisFixture(t, 20)

	// Warm SQLite page cache once, then measure live ReassignSynthesisPlotUnit + full snapshot recompute
	_, err := ComputeSynthesisSnapshot(env.ctx, env.service, env.state.ContextID, SynthesisOptions{
		UnitSource:               "su",
		ExcludeStandardModifiers: true,
	})
	if err != nil {
		t.Fatalf("initial snapshot failed: %v", err)
	}

	wallStart := time.Now()
	res, err := ReassignSynthesisPlotUnit(env.ctx, env.service, env.state.ContextID, ReassignSynthesisPlotRequest{
		PlotNumber:     "00337",
		TargetSiteUnit: "CWHvm1/05",
		Options: SynthesisOptions{
			UnitSource:               "su",
			ExcludeStandardModifiers: true,
		},
	})
	wallElapsedMs := float64(time.Since(wallStart).Microseconds()) / 1000.0
	if err != nil {
		t.Fatalf("ReassignSynthesisPlotUnit failed: %v", err)
	}

	if !res.Changed || res.BeforeSiteUnit != "CWHvm1/01" || res.AfterSiteUnit != "CWHvm1/05" || res.HistoryID <= 0 {
		t.Fatalf("unexpected reassignment result: %+v", res)
	}
	if res.LatencyMs >= 50.0 || wallElapsedMs >= 50.0 {
		t.Fatalf("expected reassignment + recompute well under < 50 ms, got reported=%.2fms wall=%.2fms",
			res.LatencyMs, wallElapsedMs)
	}
	if !res.Snapshot.CanUndo || res.Snapshot.UndoDepth != 1 || res.Snapshot.LatestUndo == nil || res.Snapshot.LatestUndo.PlotNumber != "00337" {
		t.Fatalf("expected active undo entry for '00337', got canUndo=%v depth=%d latest=%+v",
			res.Snapshot.CanUndo, res.Snapshot.UndoDepth, res.Snapshot.LatestUndo)
	}

	// Verify '00337' is now in CWHvm1/05 and no longer Misplaced
	for _, cand := range res.Snapshot.OutlierQueue {
		if cand.PlotNumber == "00337" {
			if cand.AssignedUnit != "CWHvm1/05" || cand.Misplaced {
				t.Fatalf("expected '00337' cleanly assigned to CWHvm1/05 after move, got %+v", cand)
			}
		}
	}

	// Undo reassignment and verify restoration + < 50 ms
	undoRes, err := UndoSynthesisReassignment(env.ctx, env.service, env.state.ContextID, UndoSynthesisReassignmentRequest{
		Options: SynthesisOptions{
			UnitSource:               "su",
			ExcludeStandardModifiers: true,
		},
	})
	if err != nil {
		t.Fatalf("UndoSynthesisReassignment failed: %v", err)
	}
	if undoRes.PlotNumber != "00337" || undoRes.AfterSiteUnit != "CWHvm1/01" || undoRes.Snapshot.CanUndo {
		t.Fatalf("expected undo to restore '00337' -> CWHvm1/01 and clear undo stack, got %+v", undoRes)
	}
	if undoRes.LatencyMs >= 50.0 {
		t.Fatalf("expected undo + recompute < 50 ms, got %.2fms", undoRes.LatencyMs)
	}

	// Protection Guard 1: Master_SU must reject writes
	masterState, err := env.service.SwitchContext(env.state.ContextID, vprocore.ContextSelection{
		Project:       env.state.ActiveProject,
		ProjectPath:   env.state.ProjectPath,
		SU:            "Master",
		SUPath:        env.state.ProjectPath,
		Hierarchy:     env.state.ActiveHierarchy,
		HierarchyPath: env.state.HierarchyPath,
	})
	if err != nil {
		t.Fatalf("switch to Master_SU failed: %v", err)
	}
	masterSnap, err := ComputeSynthesisSnapshot(env.ctx, env.service, masterState.ContextID, SynthesisOptions{UnitSource: "su"})
	if err != nil {
		t.Fatalf("snapshot on Master_SU failed: %v", err)
	}
	if masterSnap.SUWritable {
		t.Fatalf("expected Master_SU to be marked SUWritable=false")
	}
	if _, err := ReassignSynthesisPlotUnit(env.ctx, env.service, masterState.ContextID, ReassignSynthesisPlotRequest{
		PlotNumber:     "00337",
		TargetSiteUnit: "CWHvm1/05",
	}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "protected master") {
		t.Fatalf("expected Master_SU write rejection, got %v", err)
	}

	// Protection Guard 2: SU == None must reject writes
	noneState, err := env.service.SwitchContext(masterState.ContextID, vprocore.ContextSelection{
		Project:       masterState.ActiveProject,
		ProjectPath:   masterState.ProjectPath,
		SU:            "None",
		SUPath:        "",
		Hierarchy:     masterState.ActiveHierarchy,
		HierarchyPath: masterState.HierarchyPath,
	})
	if err != nil {
		t.Fatalf("switch to SU=None failed: %v", err)
	}
	if _, err := ReassignSynthesisPlotUnit(env.ctx, env.service, noneState.ContextID, ReassignSynthesisPlotRequest{
		PlotNumber:     "00337",
		TargetSiteUnit: "CWHvm1/05",
	}); err == nil {
		t.Fatalf("expected SU=None write rejection")
	}
}

func TestExportDiagnosticWorkbookStructureAndEqualFoldCollisions(t *testing.T) {
	env := buildSynthesisFixture(t, 0)

	// 1. strings.EqualFold collision detection on sheet names
	if _, err := ResolveDiagnosticWorkbookSheets([]string{"Diagnostic Summary", "diagnostic summary"}); err == nil {
		t.Fatalf("expected case-insensitive sheet collision error")
	}

	collidePath := filepath.Join(env.rootDir, "collision.xlsx")
	if _, err := ExportDiagnosticWorkbook(env.ctx, env.service, env.state.ContextID, ExportDiagnosticWorkbookRequest{
		OutputPath:          collidePath,
		DiagnosticSheetName: "Outlier Queue",
		OutlierSheetName:    "outlier queue",
	}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "collision") {
		t.Fatalf("expected ExportDiagnosticWorkbook sheet collision error, got %v", err)
	}

	// 2. Publish 4-sheet Diagnostic10.xla-compatible .xlsx workbook
	outPath := filepath.Join(env.rootDir, "BEC_Diagnostic_Synthesis.xlsx")
	expRes, err := ExportDiagnosticWorkbook(env.ctx, env.service, env.state.ContextID, ExportDiagnosticWorkbookRequest{
		OutputPath:         outPath,
		IncludeCommonNames: true,
		Options: SynthesisOptions{
			UnitSource:               "su",
			LumpTable:                "CombineTest",
			ExcludeStandardModifiers: true,
		},
	})
	if err != nil {
		t.Fatalf("ExportDiagnosticWorkbook failed: %v", err)
	}
	wantSheets := []string{"Diagnostic Summary", "Pairwise Similarity", "Outlier Queue", "Edatopic Grid"}
	if !slices.Equal(expRes.SheetNames, wantSheets) {
		t.Fatalf("got sheets %v, want %v", expRes.SheetNames, wantSheets)
	}

	// 3. Atomic no-overwrite check: re-exporting to existing path must fail cleanly without replacing
	if _, err := ExportDiagnosticWorkbook(env.ctx, env.service, env.state.ContextID, ExportDiagnosticWorkbookRequest{
		OutputPath: outPath,
	}); err == nil {
		t.Fatalf("expected atomic no-overwrite error when destination already exists")
	}

	// 4. Inspect published workbook structure via excelize
	wb, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("open published workbook failed: %v", err)
	}
	defer func() { _ = wb.Close() }()

	if !slices.Equal(wb.GetSheetList(), wantSheets) {
		t.Fatalf("published sheet list = %v, want %v", wb.GetSheetList(), wantSheets)
	}

	diagRows, err := wb.GetRows("Diagnostic Summary")
	if err != nil || len(diagRows) < 10 {
		t.Fatalf("unexpected Diagnostic Summary rows: len=%d err=%v", len(diagRows), err)
	}
	if !strings.Contains(diagRows[2][0], "CombineTest_Lump") {
		t.Fatalf("expected Row 3 lumping note mentioning CombineTest_Lump, got %q", diagRows[2][0])
	}
	if diagRows[4][1] != "Species" || diagRows[4][5] != "Diagnostic" {
		t.Fatalf("unexpected Row 5 headers: %v", diagRows[4])
	}
	var sawNonDiagBanner bool
	for _, r := range diagRows {
		if len(r) > 1 && r[1] == "Non-Diagnostic Species" {
			sawNonDiagBanner = true
			break
		}
	}
	if !sawNonDiagBanner {
		t.Fatalf("expected 'Non-Diagnostic Species' banner row in Diagnostic Summary sheet")
	}

	outRows, err := wb.GetRows("Outlier Queue")
	if err != nil || len(outRows) < 6 {
		t.Fatalf("unexpected Outlier Queue rows: len=%d err=%v", len(outRows), err)
	}
	// Row 5 (index 4) is Rank #1 outlier -> '00337'
	if outRows[4][1] != "00337" || outRows[4][5] != "CWHvm1/05" {
		t.Fatalf("expected top outlier row in Excel to be plot '00337' -> CWHvm1/05, got %v", outRows[4])
	}

	edaRows, err := wb.GetRows("Edatopic Grid")
	if err != nil || len(edaRows) < 12 {
		t.Fatalf("unexpected Edatopic Grid rows: len=%d err=%v", len(edaRows), err)
	}
}

func BenchmarkReassignSynthesisPlotUnit(b *testing.B) {
	env := buildSynthesisFixture(b, 20)
	opts := SynthesisOptions{
		UnitSource:               "su",
		ExcludeStandardModifiers: true,
	}
	units := []string{"CWHvm1/05", "CWHvm1/01"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := units[i%2]
		res, err := ReassignSynthesisPlotUnit(env.ctx, env.service, env.state.ContextID, ReassignSynthesisPlotRequest{
			PlotNumber:     "00337",
			TargetSiteUnit: target,
			Options:        opts,
		})
		if err != nil {
			b.Fatal(err)
		}
		if res.LatencyMs >= 50.0 {
			b.Fatalf("reassignment exceeded 50ms budget: %.2fms", res.LatencyMs)
		}
	}
}
