package rsidecar_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/rsidecar"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestLayer2ImportPurity(t *testing.T) {
	entries, err := os.ReadDir("internal/rsidecar")
	if err != nil {
		t.Fatalf("read internal/rsidecar: %v", err)
	}
	allowedInternal := map[string]bool{
		"github.com/boostao/vpro-wails/resources":              true,
		"github.com/boostao/vpro-wails/internal/sqlutil":       true,
		"github.com/boostao/vpro-wails/internal/references":    true,
		"github.com/boostao/vpro-wails/internal/vprocore":      true,
		"github.com/boostao/vpro-wails/internal/vprocore/vprotest": true,
		"github.com/boostao/vpro-wails/internal/rsidecar":      true,
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join("internal/rsidecar", entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, imp := range f.Imports {
			pkgPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(pkgPath, "github.com/boostao/vpro-wails/") && !allowedInternal[pkgPath] {
				t.Fatalf("Layer-2 violation in %s: forbidden import %q", entry.Name(), pkgPath)
			}
		}
	}
}

func TestDetectRRuntimeRealAndGracefulMissingFallback(t *testing.T) {
	ctx := context.Background()

	// 1. Real Rscript + Quarto runtime probe on this workstation
	realStatus, err := rsidecar.DetectRRuntime(ctx)
	if err != nil {
		t.Fatalf("DetectRRuntime failed: %v", err)
	}
	if !realStatus.Available || realStatus.RscriptPath == "" || realStatus.RVersion == "" {
		t.Fatalf("expected real Rscript runtime to be available, got %+v", realStatus)
	}
	if realStatus.QuartoAvailable && (realStatus.QuartoPath == "" || realStatus.QuartoVersion == "") {
		t.Fatalf("expected non-empty QuartoPath and QuartoVersion when QuartoAvailable=true, got %+v", realStatus)
	}
	if len(realStatus.Packages) == 0 {
		t.Fatalf("expected non-empty package inventory from probe_runtime.R, got %+v", realStatus)
	}

	installedByName := make(map[string]string, len(realStatus.Packages))
	requiredByName := make(map[string]bool, len(realStatus.Packages))
	for _, pkg := range realStatus.Packages {
		requiredByName[pkg.Name] = pkg.Required
		if pkg.Installed {
			if pkg.Version == "" {
				t.Fatalf("installed R package %q reported empty version: %+v", pkg.Name, pkg)
			}
			installedByName[pkg.Name] = pkg.Version
		}
	}
	for _, miss := range realStatus.MissingRequired {
		if !requiredByName[miss] {
			t.Fatalf("optional package %q must never be reported in MissingRequired: %+v", miss, realStatus)
		}
		if installedByName[miss] != "" {
			t.Fatalf("installed package %q was erroneously reported in MissingRequired: %+v", miss, realStatus)
		}
	}

	wantClimate := installedByName["jsonlite"] != "" && installedByName["data.table"] != "" && installedByName["DBI"] != "" && installedByName["RSQLite"] != ""
	wantCluster := wantClimate && installedByName["vegan"] != "" && installedByName["vegclust"] != ""
	wantQuarto := wantClimate && (realStatus.QuartoAvailable || (installedByName["knitr"] != "" && installedByName["rmarkdown"] != ""))
	if realStatus.ReadyForClimate != wantClimate || realStatus.ReadyForClustering != wantCluster || realStatus.ReadyForQuarto != wantQuarto {
		t.Fatalf("readiness flags mismatch installed packages: got climate=%v cluster=%v quarto=%v, want %v/%v/%v (packages=%+v)",
			realStatus.ReadyForClimate, realStatus.ReadyForClustering, realStatus.ReadyForQuarto,
			wantClimate, wantCluster, wantQuarto, realStatus.Packages)
	}
	if (len(realStatus.MissingRequired) > 0 || !realStatus.QuartoAvailable) && len(realStatus.SetupInstructions) == 0 {
		t.Fatalf("expected SetupInstructions when required packages or Quarto CLI are missing: %+v", realStatus)
	}

	// 2. Missing Rscript binary degrades gracefully with setup instructions (never crashes)
	missingStatus, err := rsidecar.DetectRRuntime(ctx, rsidecar.RuntimeOptions{
		RscriptPath: "/nonexistent/path/to/Rscript",
		QuartoPath:  "none",
	})
	if err != nil {
		t.Fatalf("DetectRRuntime with missing Rscript must not return error, got %v", err)
	}
	if missingStatus.Available || missingStatus.QuartoAvailable || missingStatus.ReadyForClimate || missingStatus.ReadyForClustering {
		t.Fatalf("expected unavailable runtime status for missing binary, got %+v", missingStatus)
	}
	if len(missingStatus.SetupInstructions) == 0 || missingStatus.DiagnosticMessage == "" {
		t.Fatalf("expected setup instructions and diagnostic message when R is absent, got %+v", missingStatus)
	}

	// 3. VPRO_RSCRIPT_PATH env override honored deterministically
	envDisabled, err := rsidecar.DetectRRuntime(ctx, rsidecar.RuntimeOptions{
		LookupEnv: func(key string) (string, bool) {
			if key == "VPRO_RSCRIPT_PATH" {
				return "disabled", true
			}
			return "", false
		},
	})
	if err != nil || envDisabled.Available {
		t.Fatalf("expected VPRO_RSCRIPT_PATH=disabled to report Available=false cleanly, got %+v (%v)", envDisabled, err)
	}
}

func TestWindowsQuartoPandocDiscoveryAndChildEnvConfiguration(t *testing.T) {
	fakeQuartoRoot := t.TempDir()
	binDir := filepath.Join(fakeQuartoRoot, "bin")
	toolsDir := filepath.Join(binDir, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	fakeQuartoExe := filepath.Join(binDir, "quarto.exe")
	fakePandocExe := filepath.Join(toolsDir, "pandoc.exe")
	for _, f := range []string{fakeQuartoExe, fakePandocExe} {
		if err := os.WriteFile(f, []byte("stub"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. When RSTUDIO_PANDOC is unset and pandoc is absent from PATH, ResolveBundledPandocDir finds <quarto_bin>/tools
	gotDir := rsidecar.ResolveBundledPandocDir(rsidecar.RuntimeOptions{
		QuartoPath: fakeQuartoExe,
		LookupEnv: func(key string) (string, bool) {
			if key == "PATH" {
				return filepath.Join(fakeQuartoRoot, "empty-path"), true
			}
			return "", false
		},
	})
	if gotDir != toolsDir {
		t.Fatalf("ResolveBundledPandocDir = %q, want %q", gotDir, toolsDir)
	}

	// 2. Even when QuartoPath="none" (forcing rmarkdown_fallback), system VPRO_QUARTO_PATH still supplies bundled pandoc!
	gotFallbackDir := rsidecar.ResolveBundledPandocDir(rsidecar.RuntimeOptions{
		QuartoPath: "none",
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "VPRO_QUARTO_PATH":
				return fakeQuartoExe, true
			case "PATH":
				return filepath.Join(fakeQuartoRoot, "empty-path"), true
			default:
				return "", false
			}
		},
	})
	if gotFallbackDir != toolsDir {
		t.Fatalf("ResolveBundledPandocDir with QuartoPath=none = %q, want %q", gotFallbackDir, toolsDir)
	}

	// 3. ConfigureChildREnv handles Windows mixed-case Path=, omits QUARTO_R on Windows, and injects RSTUDIO_PANDOC
	winEnv := rsidecar.ConfigureChildREnv(
		[]string{`SystemRoot=C:\Windows`, `Path=C:\Windows\System32`, `QUARTO_R=C:\stale\Rscript.exe`},
		"windows",
		`C:\Program Files\R\R-4.5.3\bin\x64\Rscript.exe`,
		toolsDir,
	)
	foundPath := ""
	foundPandoc := ""
	foundQuartoR := false
	for _, kv := range winEnv {
		if strings.HasPrefix(kv, "Path=") {
			foundPath = strings.TrimPrefix(kv, "Path=")
		} else if strings.HasPrefix(kv, "RSTUDIO_PANDOC=") {
			foundPandoc = strings.TrimPrefix(kv, "RSTUDIO_PANDOC=")
		} else if strings.HasPrefix(strings.ToUpper(kv), "QUARTO_R=") {
			foundQuartoR = true
		}
	}
	if !strings.Contains(foundPath, `R-4.5.3`) || !strings.HasSuffix(foundPath, `;C:\Windows\System32`) {
		t.Fatalf("expected R bin dir prepended to mixed-case Windows Path, got %q", foundPath)
	}
	if foundPandoc != toolsDir {
		t.Fatalf("expected RSTUDIO_PANDOC=%q, got %q", toolsDir, foundPandoc)
	}
	if foundQuartoR {
		t.Fatalf("expected QUARTO_R to be omitted on Windows so Quarto resolves Rscript.exe via Path, got %v", winEnv)
	}
}

func TestContextCancellationAndSpacesInWorkPath(t *testing.T) {
	// Pre-cancelled context rejects immediately
	preCancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if _, err := rsidecar.DetectRRuntime(preCancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on pre-cancelled DetectRRuntime, got %v", err)
	}

	// WorkRootDir containing spaces proves strict argv passing (fixing legacy frmR.txt:1632 unquoted path bug)
	svc, state := vprotest.ContextServiceFixture(t)
	spacedWorkDir := filepath.Join(filepath.Dir(state.ProjectPath), "Work Dir With Spaces")
	if err := os.MkdirAll(spacedWorkDir, 0700); err != nil {
		t.Fatal(err)
	}
	status, err := rsidecar.DetectRRuntime(context.Background(), rsidecar.RuntimeOptions{
		WorkRootDir: spacedWorkDir,
	})
	if err != nil || !status.Available {
		t.Fatalf("DetectRRuntime failed with spaces in WorkRootDir: %+v (%v)", status, err)
	}
	// Verify temporary run directory inside spacedWorkDir was cleaned up
	if _, statErr := os.Stat(filepath.Join(spacedWorkDir, ".vpro_rsidecar_work")); !os.IsNotExist(statErr) {
		entries, _ := os.ReadDir(filepath.Join(spacedWorkDir, ".vpro_rsidecar_work"))
		if len(entries) > 0 {
			t.Fatalf("expected .vpro_rsidecar_work run subdirectories to be cleaned up, found %d", len(entries))
		}
	}

	// Mid-flight timeout during R clustering terminates subprocess and returns context.DeadlineExceeded
	shortCtx, cancelShort := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelShort()
	_, err = rsidecar.RunMultivariateClustering(shortCtx, svc, state.ContextID, rsidecar.ClusteringRequest{
		Method: "NC",
		K:      3,
	}, rsidecar.RuntimeOptions{WorkRootDir: spacedWorkDir})
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context deadline/cancellation error on short timeout, got %v", err)
	}
}

func TestPlotClimateCachingLeadingZeroPlotAndMinistryASMRParity(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Enrich leading-zero plot '00337' with steep warm SW aspect coordinates & rSMR/rSNR so literal text key & aspect rules are tested
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		UPDATE "Sample_Env"
		   SET "Zone" = 'SBS', "SubZone" = 'dk', "Latitude" = 54.7804, "Longitude" = 127.1743,
		       "Elevation" = 520, "SlopeGradient" = 42, "Aspect" = 225,
		       "MoistureRegime" = '2', "NutrientRegime" = 'B'
		 WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST('00337' AS BLOB)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("update plot 00337: %v", err)
	}
	db.Close()

	// Run 1: Compute & cache __VPRO_PlotClimate for 1961_1990
	res1, err := rsidecar.ComputePlotClimateAndDualEdatopic(ctx, svc, state.ContextID, rsidecar.ClimateEdatopicRequest{
		RefPeriod: "1961_1990",
		GroupBy:   "bgc",
	})
	if err != nil {
		t.Fatalf("ComputePlotClimateAndDualEdatopic Run 1 failed: %v", err)
	}
	if res1.CoordinatizedPlots < 35 || res1.CacheMisses != res1.CoordinatizedPlots || res1.CacheHits != 0 {
		t.Fatalf("unexpected Run 1 counts: %+v", res1)
	}
	if len(res1.OverallRelative.Cells) == 0 || len(res1.OverallActual.Cells) == 0 || len(res1.UnitSummaries) == 0 {
		t.Fatalf("expected populated relative & actual edatopic grids and unit summaries, got rel=%d act=%d units=%d",
			len(res1.OverallRelative.Cells), len(res1.OverallActual.Cells), len(res1.UnitSummaries))
	}

	// Verify leading-zero plot '00337' preserved literally as text in __VPRO_PlotClimate and has steep warm SW aspect factor 1.25
	var found00337 *rsidecar.PlotClimateRow
	for i := range res1.Plots {
		if res1.Plots[i].PlotNumber == "00337" {
			found00337 = &res1.Plots[i]
			break
		}
	}
	if found00337 == nil {
		t.Fatalf("leading-zero plot '00337' missing from computed climate rows")
	}
	if found00337.AspectFactor != 1.25 || found00337.ASMR == nil || found00337.CMDSite == nil {
		t.Fatalf("expected AspectFactor=1.25 and non-nil aSMR/CMDSite on steep SW plot 00337, got %+v", found00337)
	}
	if found00337.Source != "climr_downscale" && found00337.Source != "bgc_lapse_model" {
		t.Fatalf("unexpected climate source tag %q", found00337.Source)
	}

	roDB, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var literalType string
	var cachedHash string
	if err := roDB.QueryRow(`SELECT typeof("PlotNumber"), "CoordElevHash" FROM "__VPRO_PlotClimate" WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST('00337' AS BLOB)`).
		Scan(&literalType, &cachedHash); err != nil {
		roDB.Close()
		t.Fatalf("query literal '00337' in __VPRO_PlotClimate: %v", err)
	}
	roDB.Close()
	if literalType != "text" || len(cachedHash) != 32 {
		t.Fatalf("expected literal text PlotNumber and 32-char CoordElevHash, got type=%q hash=%q", literalType, cachedHash)
	}

	// Run 2: Identical coordinates & RefPeriod MUST hit cache 100% WITHOUT invoking Rscript!
	// Passing RscriptPath="disabled" proves zero Rscript subprocess was spawned on cache hit!
	res2, err := rsidecar.ComputePlotClimateAndDualEdatopic(ctx, svc, state.ContextID, rsidecar.ClimateEdatopicRequest{
		RefPeriod: "1961_1990",
		GroupBy:   "site_unit",
	}, rsidecar.RuntimeOptions{RscriptPath: "disabled"})
	if err != nil {
		t.Fatalf("expected 100%% cache hit to succeed even with Rscript disabled, got: %v", err)
	}
	if res2.CacheHits != res2.CoordinatizedPlots || res2.CacheMisses != 0 {
		t.Fatalf("expected 100%% cache hits on Run 2, got hits=%d misses=%d", res2.CacheHits, res2.CacheMisses)
	}

	// Verify GetCachedPlotClimate returns the cached 1961_1990 rows
	cachedRows, err := rsidecar.GetCachedPlotClimate(ctx, svc, state.ContextID, "1961_1990")
	if err != nil || len(cachedRows) != res2.CoordinatizedPlots {
		t.Fatalf("GetCachedPlotClimate mismatch: len=%d want=%d err=%v", len(cachedRows), res2.CoordinatizedPlots, err)
	}

	// Run 3: Changing RefPeriod to 1991_2020 invalidates hash and computes warmer 1991_2020 normals via Rscript
	res3, err := rsidecar.ComputePlotClimateAndDualEdatopic(ctx, svc, state.ContextID, rsidecar.ClimateEdatopicRequest{
		RefPeriod: "1991_2020",
	})
	if err != nil {
		t.Fatalf("ComputePlotClimateAndDualEdatopic 1991_2020 failed: %v", err)
	}
	if res3.CacheMisses != res3.CoordinatizedPlots {
		t.Fatalf("expected cache miss on new RefPeriod 1991_2020, got hits=%d misses=%d", res3.CacheHits, res3.CacheMisses)
	}

	// Direct Ministry CMD -> aSMR transfer rule checks
	warmSlope := 45.0
	swAspect := 225.0
	coolAspect := 15.0
	// Wet coastal site with all 5 summer months >= 90mm and high winter precip (dormant deficit = 0)
	wetOut, err := rsidecar.EvaluateMinistryASMR(180, 110, 105, 95, 100, 140, 400, 500, &warmSlope, &coolAspect, "4")
	if err != nil {
		t.Fatal(err)
	}
	// MoistMonths == 5 -> multiplier 1.2 -> 180/1.2 = 150; Def = 0; cool steep aspect * 0.85 -> 127.5 -> SMRLevel 0, <= 150 -> aSMR = 4.5
	if wetOut.MoistMonths != 5 || wetOut.DormantDeficit != 0 || wetOut.AspectFactor != 0.85 || wetOut.ASMR == nil || math.Abs(*wetOut.ASMR-4.5) > 1e-6 {
		t.Fatalf("unexpected wet cool-slope Ministry aSMR output: %+v (aSMR=%v)", wetOut, wetOut.ASMR)
	}

	// Dry interior crest (rSMR=1) on warm SW steep slope with low winter precip (PPT_at+PPT_wt = 150 -> Def = 200)
	dryOut, err := rsidecar.EvaluateMinistryASMR(300, 30, 35, 25, 28, 30, 70, 80, &warmSlope, &swAspect, "1")
	if err != nil {
		t.Fatal(err)
	}
	// CMD_gs=300 + Def=200 = 500; *1.25 = 625; rSMR=1 adds 3*125=375 -> cmdSite=1000 -> SMRLevel 0, <= 9999 -> aSMR = 0
	if dryOut.DormantDeficit != 200 || dryOut.AspectFactor != 1.25 || dryOut.ASMR == nil || *dryOut.ASMR != 0.0 {
		t.Fatalf("unexpected dry SW-slope Ministry aSMR output: %+v (aSMR=%v)", dryOut, dryOut.ASMR)
	}

	// Hydric site (rSMR=8) always maps to aSMR=8.0
	hydricOut, err := rsidecar.EvaluateMinistryASMR(400, 20, 20, 20, 20, 20, 50, 50, nil, nil, "8")
	if err != nil || hydricOut.ASMR == nil || *hydricOut.ASMR != 8.0 {
		t.Fatalf("expected rSMR=8 -> aSMR=8.0, got %+v (%v)", hydricOut, err)
	}
}

func TestVeganAndVegclustClusteringCandidateSUAndPromotionGuard(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Snapshot active Sample_SU before clustering to prove it is never overwritten
	readSURows := func(table string) map[string]string {
		t.Helper()
		ro, err := sqlutil.OpenReadOnly(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		defer ro.Close()
		rows, err := ro.Query(`SELECT CAST("PlotNumber" AS TEXT), COALESCE(CAST("SiteUnit" AS TEXT), '') FROM "` + table + `" ORDER BY "PlotNumber"`)
		if err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		defer rows.Close()
		mp := make(map[string]string)
		for rows.Next() {
			var p, u string
			if err := rows.Scan(&p, &u); err != nil {
				t.Fatal(err)
			}
			mp[p] = u
		}
		return mp
	}
	originalSampleSU := readSURows("Sample_SU")
	if len(originalSampleSU) == 0 {
		t.Fatal("expected non-empty Sample_SU baseline")
	}

	// 1. Run real R vegclust Noise Clustering (method="NC") and materialize Kootenay_ClusterCandidate_SU
	ncRes, err := rsidecar.RunMultivariateClustering(ctx, svc, state.ContextID, rsidecar.ClusteringRequest{
		Method:               "NC",
		Distance:             "bray",
		Linkage:              "ward.D2",
		Transform:            "sqrt",
		K:                    4,
		Fuzziness:            1.25,
		DNoise:               0.75,
		ApplyLump:            true,
		UnitPrefix:           "BEC",
		CandidatePrefix:      "Kootenay",
		MaterializeCandidate: true,
	})
	if err != nil {
		t.Fatalf("RunMultivariateClustering (NC) failed: %v", err)
	}
	if ncRes.PlotCount < 30 || ncRes.SpeciesCount < 50 {
		t.Fatalf("unexpected matrix dimensions plots=%d species=%d", ncRes.PlotCount, ncRes.SpeciesCount)
	}
	if ncRes.CandidateTableName != "Kootenay_ClusterCandidate_SU" || ncRes.MaterializedRows != ncRes.PlotCount {
		t.Fatalf("unexpected candidate materialization: %+v", ncRes)
	}
	if len(ncRes.Dendrogram.Leaves) != ncRes.PlotCount || len(ncRes.Dendrogram.Segments) != 3*(ncRes.PlotCount-1) {
		t.Fatalf("unexpected dendrogram geometry: leaves=%d segments=%d wantSegments=%d",
			len(ncRes.Dendrogram.Leaves), len(ncRes.Dendrogram.Segments), 3*(ncRes.PlotCount-1))
	}
	if !strings.HasSuffix(ncRes.Dendrogram.Newick, ";") || ncRes.Dendrogram.MaxHeight <= 0 {
		t.Fatalf("invalid dendrogram Newick/height: maxH=%f newick=%q", ncRes.Dendrogram.MaxHeight, ncRes.Dendrogram.Newick)
	}
	if len(ncRes.Clusters) == 0 || len(ncRes.Clusters[0].Indicators) == 0 {
		t.Fatalf("expected non-empty clusters with indicator species, got %+v", ncRes.Clusters)
	}

	// Prove active Sample_SU was completely untouched
	afterSampleSU := readSURows("Sample_SU")
	if len(afterSampleSU) != len(originalSampleSU) {
		t.Fatalf("Sample_SU row count mutated from %d to %d!", len(originalSampleSU), len(afterSampleSU))
	}
	for k, v := range originalSampleSU {
		if afterSampleSU[k] != v {
			t.Fatalf("Sample_SU plot %q mutated from %q to %q!", k, v, afterSampleSU[k])
		}
	}

	// Verify Kootenay_ClusterCandidate_SU exists on disk with VP04 metadata
	candRows := readSURows("Kootenay_ClusterCandidate_SU")
	if len(candRows) != ncRes.PlotCount {
		t.Fatalf("Kootenay_ClusterCandidate_SU count=%d, want %d", len(candRows), ncRes.PlotCount)
	}

	// 2. Run Hierarchical UPGMA + Chord distance mode (without candidate materialization)
	hcRes, err := rsidecar.RunMultivariateClustering(ctx, svc, state.ContextID, rsidecar.ClusteringRequest{
		Method:               "HCLUST",
		Distance:             "chord",
		Linkage:              "average",
		Transform:            "hellinger",
		K:                    3,
		MaterializeCandidate: false,
	})
	if err != nil {
		t.Fatalf("RunMultivariateClustering (HCLUST/chord) failed: %v", err)
	}
	if len(hcRes.Clusters) != 3 || hcRes.CopheneticCorrelation <= 0 {
		t.Fatalf("expected 3 hierarchical clusters and positive cophenetic correlation, got k=%d coph=%f",
			len(hcRes.Clusters), hcRes.CopheneticCorrelation)
	}

	// 3. Promote Kootenay_ClusterCandidate_SU -> KootenayFinal_SU and test overwrite confirmation guard
	promoted1, err := rsidecar.PromoteClusterCandidateToSU(ctx, svc, state.ContextID, rsidecar.PromoteCandidateSURequest{
		CandidateSUName:  "Kootenay_ClusterCandidate",
		TargetSUName:     "KootenayFinal",
		ExcludeNoise:     false,
		ConfirmOverwrite: false,
		Activate:         false,
	})
	if err != nil {
		t.Fatalf("initial PromoteClusterCandidateToSU failed: %v", err)
	}
	if promoted1.OverwroteExisting || promoted1.PromotedRows != ncRes.PlotCount || promoted1.TargetTableName != "KootenayFinal_SU" {
		t.Fatalf("unexpected initial promotion result: %+v", promoted1)
	}

	// Attempting to overwrite existing KootenayFinal_SU with ConfirmOverwrite=false MUST fail!
	if _, err := rsidecar.PromoteClusterCandidateToSU(ctx, svc, state.ContextID, rsidecar.PromoteCandidateSURequest{
		CandidateSUName:  "Kootenay_ClusterCandidate",
		TargetSUName:     "KootenayFinal",
		ConfirmOverwrite: false,
	}); err == nil || !strings.Contains(err.Error(), "confirmOverwrite=true") {
		t.Fatalf("expected overwrite guard rejection without confirmOverwrite=true, got %v", err)
	}

	//Protected master target MUST always be rejected even with ConfirmOverwrite=true
	if _, err := rsidecar.PromoteClusterCandidateToSU(ctx, svc, state.ContextID, rsidecar.PromoteCandidateSURequest{
		CandidateSUName:  "Kootenay_ClusterCandidate",
		TargetSUName:     "BECMaster",
		ConfirmOverwrite: true,
	}); err == nil {
		t.Fatal("expected protected master target SU name to be rejected")
	}

	// Confirmed overwrite + Activate=true switches context cleanly
	promoted2, err := rsidecar.PromoteClusterCandidateToSU(ctx, svc, state.ContextID, rsidecar.PromoteCandidateSURequest{
		CandidateSUName:  "Kootenay_ClusterCandidate",
		TargetSUName:     "KootenayFinal",
		ExcludeNoise:     true,
		ConfirmOverwrite: true,
		Activate:         true,
	})
	if err != nil {
		t.Fatalf("confirmed PromoteClusterCandidateToSU failed: %v", err)
	}
	if !promoted2.OverwroteExisting || !promoted2.Activated || promoted2.NewState == nil || promoted2.NewState.ActiveSU != "KootenayFinal" {
		t.Fatalf("expected activated KootenayFinal SU state after confirmed overwrite, got %+v", promoted2)
	}
	if promoted2.ContextID == state.ContextID {
		t.Fatal("expected context switch on Activate=true to mint a fresh 128-bit ContextID")
	}
	_ = vprocore.ProjectState(*promoted2.NewState)
}

func TestRealQuartoSingleUnitAndBGCGuidePublicationAndRMarkdownFallback(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()
	outDir := filepath.Join(filepath.Dir(state.ProjectPath), "guide_outputs")

	// 1. Real /usr/local/bin/quarto single-unit guide render (--to html -M embed-resources:true)
	singleDest := filepath.Join(outDir, "bwbs_unit_05_guide.html")
	singleRes, err := rsidecar.RenderQuartoGuide(ctx, svc, state.ContextID, rsidecar.QuartoGuideRequest{
		GuideType:       "single_unit",
		Title:           "BWBSmw2 / 05 — Black Spruce Feathermoss Guide",
		BGCFilter:       "BWBS",
		ConceptNotes:    "Mesic to subhygric boreal white and black spruce ecosystems on medium-textured morainal blankets.",
		RefPeriod:       "1961_1990",
		MinConstancyPct: 20.0,
		ApplyLump:       true,
		IncludePDF:      true,
		DestinationPath: singleDest,
	})
	if err != nil {
		t.Fatalf("RenderQuartoGuide (single_unit via real quarto) failed: %v", err)
	}
	if singleRes.RendererUsed != "quarto" {
		t.Fatalf("expected RendererUsed='quarto', got %q", singleRes.RendererUsed)
	}
	if singleRes.HTMLSizeBytes < 4000 || len(singleRes.HTMLSHA256) != 64 {
		t.Fatalf("unexpected HTML size/SHA256: size=%d sha=%q", singleRes.HTMLSizeBytes, singleRes.HTMLSHA256)
	}
	rawHTMLBytes, err := os.ReadFile(singleRes.HTMLPath)
	if err != nil {
		t.Fatalf("read published Quarto HTML: %v", err)
	}
	rawHTML := string(rawHTMLBytes)
	for _, tokenStr := range []string{
		"#003366",
		"#FCBA19",
		"BWBSmw2 / 05",
		"Relative Edatopic Grid",
		"Actual Edatopic Grid",
		"Aspect Rose",
		"Elevation Distribution",
		"Characteristic Vegetation Summary",
		"<svg",
	} {
		if !strings.Contains(rawHTML, tokenStr) {
			t.Fatalf("published Quarto single-unit HTML missing expected content %q", tokenStr)
		}
	}
	if singleRes.PDFStatus != "published" && singleRes.PDFStatus != "tex_unavailable_print_html_ready" {
		t.Fatalf("unexpected PDFStatus %q", singleRes.PDFStatus)
	}

	// 2. Real /usr/local/bin/quarto full BGC subzone synthesis guide render
	bgcDest := filepath.Join(outDir, "mh_bgc_synthesis.html")
	bgcRes, err := rsidecar.RenderQuartoGuide(ctx, svc, state.ContextID, rsidecar.QuartoGuideRequest{
		GuideType:       "bgc_unit",
		BGCFilter:       "MH",
		ConceptNotes:    "Maritime high-elevation Mountain Hemlock subzone synthesis.",
		MinConstancyPct: 25.0,
		DestinationPath: bgcDest,
	})
	if err != nil {
		t.Fatalf("RenderQuartoGuide (bgc_unit via real quarto) failed: %v", err)
	}
	if bgcRes.RendererUsed != "quarto" || !strings.Contains(bgcRes.HTMLPreview, "Component Site Units Summary") {
		t.Fatalf("expected quarto BGC guide with Component Site Units Summary, got renderer=%q", bgcRes.RendererUsed)
	}

	// 3. Standalone R rmarkdown/knitr fallback when Quarto CLI is absent
	fbDest := filepath.Join(outDir, "fallback_guide.html")
	fbRes, err := rsidecar.RenderQuartoGuide(ctx, svc, state.ContextID, rsidecar.QuartoGuideRequest{
		GuideType:       "single_unit",
		BGCFilter:       "BWBS",
		DestinationPath: fbDest,
	}, rsidecar.RuntimeOptions{QuartoPath: "none"})
	if err != nil {
		t.Fatalf("RenderQuartoGuide rmarkdown fallback failed: %v", err)
	}
	if fbRes.RendererUsed != "rmarkdown_fallback" || fbRes.HTMLSizeBytes < 4000 {
		t.Fatalf("expected rmarkdown_fallback standalone HTML, got renderer=%q size=%d", fbRes.RendererUsed, fbRes.HTMLSizeBytes)
	}

	// Ensure JSON wire serialization of result DTOs is clean and round-trippable
	wireBytes, err := json.Marshal(singleRes)
	if err != nil || len(wireBytes) == 0 {
		t.Fatalf("marshal QuartoGuideResult: %v", err)
	}
}
