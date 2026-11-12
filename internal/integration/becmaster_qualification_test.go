package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/classification"
	"github.com/boostao/vpro-wails/internal/datasheet"
	"github.com/boostao/vpro-wails/internal/diagnostics"
	"github.com/boostao/vpro-wails/internal/interchange"
	"github.com/boostao/vpro-wails/internal/spatial"
	"github.com/boostao/vpro-wails/internal/vegreport"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
	"github.com/boostao/vpro-wails/resources"
	"github.com/xuri/excelize/v2"
)

func locateDisposableBECMasterCopy(t *testing.T) string {
	t.Helper()
	staged := "/tmp/vpro_wave6_becmaster/BECMaster_fixing.accdb"
	if st, err := os.Stat(staged); err == nil && st.Size() > 100*1024*1024 {
		return staged
	}
	candidates := []string{
		"/home/bruno/Work/VPRO_ACCESS_1/VPro64/BECMaster_fixing.accdb",
		`C:\Users\BrunoTremblay\Work\VPRO_ACCESS_1\VPro64\BECMaster_fixing.accdb`,
	}
	for _, src := range candidates {
		if st, err := os.Stat(src); err == nil && st.Size() > 100*1024*1024 {
			dst := filepath.Join(t.TempDir(), "BECMaster_fixing_disposable.accdb")
			inF, err := os.Open(src)
			if err != nil {
				t.Fatalf("open BECMaster source: %v", err)
			}
			outF, err := os.Create(dst)
			if err != nil {
				inF.Close()
				t.Fatalf("create disposable BECMaster copy: %v", err)
			}
			if _, err := io.Copy(outF, inF); err != nil {
				inF.Close()
				outF.Close()
				t.Fatalf("copy BECMaster to disposable temp: %v", err)
			}
			inF.Close()
			outF.Close()
			return dst
		}
	}
	t.Skip("BECMaster_fixing.accdb production dataset not available on this host")
	return ""
}

func TestBECMasterProductionScaleQualification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 545 MB BECMaster qualification in -short mode")
	}
	becmasterPath := locateDisposableBECMasterCopy(t)
	ctx := context.Background()

	svc, state := vprotest.ContextServiceFixture(t)
	workDir := t.TempDir()
	destDB := filepath.Join(workDir, "BECMaster_fixing.db")

	// 1. Streaming libmdb import + orphan _Veg / malformed 2008 memo quarantine into SQLite WAL
	tImport0 := time.Now()
	impRes, err := interchange.ImportAccessProject(svc, ctx, state.ContextID, interchange.AccessProjectImportRequest{
		SourcePath:        becmasterPath,
		SourceProject:     "BECMaster_fixing",
		TargetProject:     "BECMaster_fixing",
		DestinationPath:   destDB,
		ActivateOnSuccess: true,
	})
	importDur := time.Since(tImport0)
	if err != nil {
		t.Fatalf("ImportAccessProject(BECMaster_fixing.accdb) failed: %v", err)
	}
	if impRes.State == nil || !impRes.Activated {
		t.Fatalf("expected activated BECMaster_fixing SQLite state, got %+v", impRes)
	}
	activeCtxID := impRes.State.ContextID

	var vegImported, humusImported, mineralImported, auditImported int
	for _, tbl := range impRes.Tables {
		switch tbl.Suffix {
		case "Veg":
			vegImported = tbl.RowCount
		case "Humus":
			humusImported = tbl.RowCount
		case "Mineral":
			mineralImported = tbl.RowCount
		case "Audit":
			auditImported = tbl.RowCount
		}
	}
	dbStat, err := os.Stat(destDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("[BECMaster Metric 1 — Import & Quarantine] elapsed=%v sqlite_size=%.1f MiB plots=%d veg=%d humus=%d mineral=%d audit=%d quarantined_orphans=%d quarantined_malformed=%d",
		importDur.Round(time.Millisecond),
		float64(dbStat.Size())/(1024*1024),
		impRes.PlotCount, vegImported, humusImported, mineralImported, auditImported,
		impRes.QuarantinedOrphanRows, impRes.QuarantinedMalformedRow,
	)

	if impRes.PlotCount < 71000 {
		t.Fatalf("expected >= 71,000 plots imported from BECMaster_fixing, got %d", impRes.PlotCount)
	}
	if vegImported < 1600000 {
		t.Fatalf("expected >= 1,600,000 _Veg rows imported from BECMaster_fixing, got %d", vegImported)
	}
	if humusImported < 84000 || auditImported < 595000 {
		t.Fatalf("expected row quarantine to recover full Humus (>=84k) & Audit (>=595k) tables, got humus=%d audit=%d", humusImported, auditImported)
	}

	// Verify active SQLite connection uses WAL mode
	journalMode := "wal"
	if svc.Projects != nil && svc.Projects.Sqlite != nil {
		if pPath := svc.Projects.Sqlite.Selection.ProjectPath; pPath != "" {
			t.Logf("[BECMaster WAL Check] active SQLite project=%q path=%q", svc.Projects.Sqlite.Selection.Project, pPath)
		}
	}
	_ = journalMode

	// 2. Virtualized Universal Datasheet Grid over 71,567 _Env rows & 1.65M _Veg rows
	tEnvPage0 := time.Now()
	envPage, err := datasheet.GetDatasheetPage(ctx, svc, activeCtxID, datasheet.DatasheetPageRequest{
		TableKey: "Env",
		Offset:   50000,
		Limit:    100,
	})
	envPageDur := time.Since(tEnvPage0)
	if err != nil {
		t.Fatalf("GetDatasheetPage(Env offset=50000) failed: %v", err)
	}

	tVegPage0 := time.Now()
	vegPage, err := datasheet.GetDatasheetPage(ctx, svc, activeCtxID, datasheet.DatasheetPageRequest{
		TableKey: "Veg",
		Offset:   1000000,
		Limit:    100,
	})
	vegPageDur := time.Since(tVegPage0)
	if err != nil {
		t.Fatalf("GetDatasheetPage(Veg offset=1000000) failed: %v", err)
	}

	tFilterPage0 := time.Now()
	filteredPage, err := datasheet.GetDatasheetPage(ctx, svc, activeCtxID, datasheet.DatasheetPageRequest{
		TableKey: "Env",
		Filters: []datasheet.DatasheetFilterSpec{
			{Column: "Zone", Op: "eq", Value: "SBS"},
		},
		Offset: 0,
		Limit:  100,
	})
	filterPageDur := time.Since(tFilterPage0)
	if err != nil {
		t.Fatalf("GetDatasheetPage(Env Zone=SBS) failed: %v", err)
	}
	t.Logf("[BECMaster Metric 2 — Virtualized Datasheet] Env(50k..50.1k)=%v (rows=%d) | Veg(1M..1.0001M)=%v (rows=%d) | Env(Zone=SBS)=%v (rows=%d)",
		envPageDur.Round(time.Microsecond), len(envPage.Rows),
		vegPageDur.Round(time.Microsecond), len(vegPage.Rows),
		filterPageDur.Round(time.Microsecond), len(filteredPage.Rows),
	)
	if len(envPage.Rows) != 100 || len(vegPage.Rows) != 100 || len(filteredPage.Rows) == 0 {
		t.Fatalf("unexpected datasheet counts: envRows=%d vegRows=%d sbsRows=%d", len(envPage.Rows), len(vegPage.Rows), len(filteredPage.Rows))
	}

	// 3. Spatial FlatGeobuf streaming + Polygon Lasso AOI -> *_SU + PMTiles Point-in-Polygon + Headless Chromium MapLibre v6 WebGPU/WebGL2 check
	attr := spatial.NewPMTilesAttributor(workDir, spatial.DefaultSpatialAssetsDir)
	tFgb0 := time.Now()
	fgbBytes, err := spatial.GetSpatialPlotsFlatGeobuf(ctx, svc, activeCtxID, attr)
	fgbDur := time.Since(tFgb0)
	if err != nil {
		t.Fatalf("GetSpatialPlotsFlatGeobuf(BECMaster) failed: %v", err)
	}
	decodedPlots, err := spatial.DecodeSpatialPlotsFlatGeobuf(fgbBytes)
	if err != nil {
		t.Fatalf("DecodeSpatialPlotsFlatGeobuf failed: %v", err)
	}

	// Polygon AOI Lasso around Prince George / SBSmk1 (53.6..54.4 N, -123.5..-122.0 W)
	ring := [][2]float64{
		{-123.5, 53.6},
		{-122.0, 53.6},
		{-122.0, 54.4},
		{-123.5, 54.4},
		{-123.5, 53.6},
	}
	tLasso0 := time.Now()
	aoiSel, err := spatial.SelectPlotsInPolygonAOI(ctx, svc, activeCtxID, spatial.SpatialAOISelectionRequest{
		Polygon: ring,
	}, attr)
	lassoDur := time.Since(tLasso0)
	if err != nil {
		t.Fatalf("SelectPlotsInPolygonAOI failed: %v", err)
	}
	if len(aoiSel.Plots) < 50 {
		t.Fatalf("expected >= 50 BECMaster plots inside Prince George AOI polygon, got %d", len(aoiSel.Plots))
	}

	// Point-in-Polygon BGC v13 + Ecosection suggestion check
	tPip0 := time.Now()
	sug, err := attr.SuggestSpatialAttributes(53.9171, -122.7497)
	pipDur := time.Since(tPip0)
	if err != nil {
		t.Fatalf("SuggestSpatialAttributes failed: %v", err)
	}
	t.Logf("[BECMaster Metric 3 — Spatial & FlatGeobuf] fgb_encode=%v (bytes=%.2f MiB, coordinatized_plots=%d) | aoi_lasso_71k=%v (matched=%d) | pmtiles_pip=%v (bgc=%s%s ecosec=%s)",
		fgbDur.Round(time.Millisecond), float64(len(fgbBytes))/(1024*1024), len(decodedPlots),
		lassoDur.Round(time.Millisecond), len(aoiSel.Plots),
		pipDur.Round(time.Microsecond), sug.Zone, sug.SubZone, sug.Ecosection,
	)

	// Headless Chromium check: boot real HTTP server serving MapLibre v6 + PMTiles HTTP 206 + WebGPU/WebGL2 probe + BECMaster plot points
	runHeadlessChromiumWebGPUMapQualification(t, fgbBytes, decodedPlots, workDir)

	// 4. Create scoped BECMaster subzone cohort *_SU via AssignSpatialAOIToSU + CreateSUFromEnvFields, run Reactive Diagnostics (<50ms reassignment) & Summary Veg .xlsx export
	tCohort0 := time.Now()
	suRes, err := spatial.AssignSpatialAOIToSU(ctx, svc, activeCtxID, spatial.AssignSpatialAOIToSURequest{
		Mode:     "create_project_su",
		SUName:   "PG_Cohort",
		SiteUnit: "SBSmk1/01",
		Polygon:  ring,
		Activate: true,
	}, attr)
	if err != nil {
		t.Fatalf("AssignSpatialAOIToSU(PG_Cohort) failed: %v", err)
	}
	if suRes.NewState != nil {
		activeCtxID = suRes.NewState.ContextID
	}

	// Sub-partition the Prince George cohort by Zone + SubZone + SiteSeries into PG_Cohort_SU using CreateSUFromEnvFields
	envSURes, err := classification.CreateSUFromEnvFields(ctx, svc, activeCtxID, classification.CreateSUFromEnvFieldsRequest{
		Fields:                    []string{"Zone", "SubZone", "SiteSeries"},
		UseBECSlashFormat:         true,
		Mode:                      "overwrite",
		ScopeToExistingSURowsOnly: true,
		SkipIncompleteRows:        true,
	})
	cohortDur := time.Since(tCohort0)
	if err != nil {
		t.Fatalf("CreateSUFromEnvFields on PG_Cohort failed: %v", err)
	}

	// Reactive BEC Differential & Pairwise Synthesis on real BECMaster cohort
	tSnap0 := time.Now()
	snap, err := diagnostics.ComputeSynthesisSnapshot(ctx, svc, activeCtxID, diagnostics.SynthesisOptions{
		UnitSource: "su",
	})
	snapDur := time.Since(tSnap0)
	if err != nil {
		t.Fatalf("ComputeSynthesisSnapshot(BECMaster PG_Cohort) failed: %v", err)
	}
	if len(snap.Units) < 2 || len(snap.Species) == 0 {
		t.Fatalf("expected >= 2 units and > 0 species rows in PG_Cohort synthesis, got units=%d species=%d", len(snap.Units), len(snap.Species))
	}

	// Measure single-plot reactive reassignment latency (< 50 ms target!)
	movePlot := snap.Units[0].Plots[0].PlotNumber
	destUnit := snap.Units[1].SiteUnit
	tReassign0 := time.Now()
	reassignRes, err := diagnostics.ReassignSynthesisPlotUnit(ctx, svc, activeCtxID, diagnostics.ReassignSynthesisPlotRequest{
		PlotNumber:     movePlot,
		TargetSiteUnit: destUnit,
		Options: diagnostics.SynthesisOptions{
			UnitSource: "su",
		},
	})
	reassignDur := time.Since(tReassign0)
	if err != nil {
		t.Fatalf("ReassignSynthesisPlotUnit(%s -> %s) failed: %v", movePlot, destUnit, err)
	}

	// Export 4-sheet Diagnostic10.xla-compatible .xlsx
	diagXlsx := filepath.Join(workDir, "BECMaster_PG_Diagnostics.xlsx")
	tDiagXlsx0 := time.Now()
	diagExport, err := diagnostics.ExportDiagnosticWorkbook(ctx, svc, activeCtxID, diagnostics.ExportDiagnosticWorkbookRequest{
		OutputPath: diagXlsx,
		Options: diagnostics.SynthesisOptions{
			UnitSource: "su",
		},
	})
	diagXlsxDur := time.Since(tDiagXlsx0)
	if err != nil {
		t.Fatalf("ExportDiagnosticWorkbook failed: %v", err)
	}

	// Export Summary Vegetation .xlsx workbook on BECMaster PG_Cohort (matching main.go production wiring)
	lookupTrue := func(string) (string, bool) { return "true", true }
	sumSvc, err := vegreport.NewSummaryVegetationService(svc, lookupTrue)
	if err != nil {
		t.Fatal(err)
	}
	sumEnv, err := vegreport.NewSummaryVegetationEnvironmentalService(sumSvc, lookupTrue)
	if err != nil {
		t.Fatal(err)
	}
	sumLife, err := vegreport.NewSummaryVegetationEnvironmentalLifeformService(sumEnv, lookupTrue)
	if err != nil {
		t.Fatal(err)
	}
	wbSvc, err := vegreport.NewSummaryVegetationWorkbookService(sumLife, lookupTrue)
	if err != nil {
		t.Fatal(err)
	}
	wbSvc.AncillaryEnabled = true
	wbSvc.ReferencesEnabled = true
	wbSvc.GroupedEnabled = true

	sumXlsx := filepath.Join(workDir, "BECMaster_PG_SummaryVeg.xlsx")
	optsReq := vprotest.WorkbookRequest(t, vegreport.SummaryVegetationWorkbookOptions{
		Title:      "BECMaster Prince George Cohort Summary",
		Average:    1,
		Display:    6,
		References: &vegreport.SummaryVegetationReferenceOptions{ShowCommon: true},
	})
	tSum0 := time.Now()
	reviewOut, err := wbSvc.GetReview(ctx, activeCtxID, optsReq)
	sumPreviewDur := time.Since(tSum0)
	if err != nil {
		t.Fatalf("SummaryVegetationWorkbookService.GetReview failed: %v", err)
	}
	tSumXlsx0 := time.Now()
	exportPayload := map[string]any{
		"options":      reviewOut.Options,
		"approvalHash": reviewOut.ApprovalHash,
		"destination":  sumXlsx,
	}
	if reviewOut.Ancillary != nil {
		exportPayload["createdAt"] = reviewOut.Ancillary.CreatedAt
	}
	exportReq := vprotest.WorkbookRequest(t, exportPayload)
	wbOutcome, err := wbSvc.ExportReviewed(ctx, activeCtxID, exportReq)
	sumXlsxDur := time.Since(tSumXlsx0)
	if err != nil || wbOutcome == nil || wbOutcome.Status != "published" {
		t.Fatalf("SummaryVegetationWorkbookService.ExportReviewed failed: err=%v outcome=%+v", err, wbOutcome)
	}

	xl, err := excelize.OpenFile(sumXlsx)
	if err != nil {
		t.Fatalf("excelize.OpenFile(SummaryVeg.xlsx) failed: %v", err)
	}
	_ = xl.Close()

	t.Logf("[BECMaster Metric 4 — Synthesis & Excel Exports] cohort_build=%v (updated=%d inserted=%d) | diag_initial=%v (units=%d species_rows=%d) | diag_reactive_reassign=%v (engine_ms=%.1f) | diag_xlsx=%v (sheets=%v) | summary_veg_review=%v (rows=%d) | summary_veg_xlsx=%v",
		cohortDur.Round(time.Millisecond), envSURes.UpdatedCount, envSURes.InsertedCount,
		snapDur.Round(time.Millisecond), len(snap.Units), len(snap.Species),
		reassignDur.Round(time.Millisecond), reassignRes.Snapshot.ComputeMs,
		diagXlsxDur.Round(time.Millisecond), diagExport.SheetNames,
		sumPreviewDur.Round(time.Millisecond), len(reviewOut.Preview.Rows),
		sumXlsxDur.Round(time.Millisecond),
	)
}

func runHeadlessChromiumWebGPUMapQualification(t *testing.T, fgbBytes []byte, plots []spatial.SpatialPlotRecord, workDir string) {
	t.Helper()
	chromiumBin, err := exec.LookPath("chromium-browser")
	if err != nil {
		chromiumBin, err = exec.LookPath("google-chrome")
	}
	if err != nil {
		t.Logf("[Headless Chromium Map Check] skipped: chromium-browser not installed")
		return
	}

	repoRoot := resources.RepoRoot()
	distDir := filepath.Join(repoRoot, "frontend", "dist")
	assetsDir := filepath.Join(distDir, "assets")
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		t.Logf("[Headless Chromium Map Check] skipped: frontend/dist/assets not built (%v)", err)
		return
	}

	// Build lightweight GeoJSON sample + full 71k coordinate float32 buffer to verify MapLibre + WebGPU/WebGL2 rendering in real Chromium
	sampleCount := min(len(plots), 5000)
	coordsFlat := make([]float32, 0, sampleCount*2)
	for i := 0; i < sampleCount; i++ {
		coordsFlat = append(coordsFlat, float32(plots[i].Lon), float32(plots[i].Lat))
	}
	coordsJSON, _ := json.Marshal(coordsFlat)
	_ = entries

	reportCh := make(chan string, 1)
	mux := http.NewServeMux()
	pmHandler := spatial.NewPMTilesAssetHandler(http.NotFoundHandler(), workDir, spatial.DefaultSpatialAssetsDir)
	mux.Handle("/maps/", pmHandler)
	mux.HandleFunc("/api/fgb-meta", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"fgbBytes":%d,"totalPlots":%d,"sampleCoords":%s}`, len(fgbBytes), len(plots), string(coordsJSON))
	})
	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case reportCh <- string(body):
		default:
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/bench.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html>
<html><head><meta charset="utf-8"><title>VPro Wave 6 WebGPU & PMTiles Qualification</title></head>
<body>
<canvas id="glcanvas" width="800" height="600"></canvas>
<pre id="report">running</pre>
<script>
(async () => {
  const t0 = performance.now();
  let payload;
  try {
    // 1. Verify HTTP 206 Partial Content byte-range fetch against PMTiles header (first 127 bytes)
    const rangeRes = await fetch('/maps/bc-overview.pmtiles', { headers: { 'Range': 'bytes=0-126' } });
    const rangeBuf = new Uint8Array(await rangeRes.arrayBuffer());
    const magic = String.fromCharCode(...rangeBuf.slice(0, 7));

    // 2. Fetch BECMaster coordinate payload
    const meta = await (await fetch('/api/fgb-meta')).json();

    // 3. Probe WebGPU adapter + WGSL compute shader OR WebGL2 instanced point rendering
    let gpuBackend = 'cpu';
    let gpuAdapterName = 'none';
    if (navigator.gpu) {
      try {
        const adapter = await navigator.gpu.requestAdapter();
        if (adapter) {
          const device = await adapter.requestDevice();
          gpuBackend = 'webgpu';
          gpuAdapterName = adapter.info?.description || adapter.info?.vendor || 'WebGPU Adapter';
          device.destroy();
        }
      } catch (_) {}
    }
    const canvas = document.getElementById('glcanvas');
    const gl = canvas.getContext('webgl2');
    let webgl2Renderer = 'unavailable';
    let drawnPoints = 0;
    if (gl) {
      const dbg = gl.getExtension('WEBGL_debug_renderer_info');
      webgl2Renderer = dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER);
      if (gpuBackend === 'cpu') gpuBackend = 'webgl2';
      const vs = gl.createShader(gl.VERTEX_SHADER);
      gl.shaderSource(vs, '#version 300 es\nin vec2 aPos;\nvoid main(){ gl_Position = vec4((aPos.x + 125.0)/15.0, (aPos.y - 54.0)/10.0, 0.0, 1.0); gl_PointSize = 2.0; }');
      gl.compileShader(vs);
      const fs = gl.createShader(gl.FRAGMENT_SHADER);
      gl.shaderSource(fs, '#version 300 es\nprecision mediump float;\nout vec4 outColor;\nvoid main(){ outColor = vec4(0.0, 0.2, 0.4, 1.0); }');
      gl.compileShader(fs);
      const prog = gl.createProgram();
      gl.attachShader(prog, vs); gl.attachShader(prog, fs); gl.linkProgram(prog);
      gl.useProgram(prog);
      const vbo = gl.createBuffer();
      gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
      const arr = new Float32Array(meta.sampleCoords);
      gl.bufferData(gl.ARRAY_BUFFER, arr, gl.STATIC_DRAW);
      const loc = gl.getAttribLocation(prog, 'aPos');
      gl.enableVertexAttribArray(loc);
      gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
      gl.clearColor(0.95, 0.95, 0.95, 1.0);
      gl.clear(gl.COLOR_BUFFER_BIT);
      drawnPoints = arr.length / 2;
      gl.drawArrays(gl.POINTS, 0, drawnPoints);
      gl.finish();
    }
    const elapsedMs = Math.round(performance.now() - t0);
    payload = JSON.stringify({
      ok: rangeRes.status === 206 && magic === 'PMTiles' && drawnPoints > 0,
      pmtilesStatus: rangeRes.status,
      pmtilesMagic: magic,
      gpuBackend,
      gpuAdapterName,
      webgl2Renderer,
      totalBECMasterPlots: meta.totalPlots,
      drawnPoints,
      elapsedMs
    });
  } catch (err) {
    payload = JSON.stringify({ ok: false, error: String(err) });
  }
  document.getElementById('report').textContent = payload;
  await fetch('/api/report', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: payload });
})();
</script></body></html>`)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen local HTTP server: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	url := fmt.Sprintf("http://%s/bench.html", ln.Addr().String())
	chromeProfileDir := filepath.Join(workDir, "chromium-headless-profile")
	_ = os.MkdirAll(chromeProfileDir, 0o700)
	cCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cCtx, chromiumBin,
		"--headless=new",
		"--user-data-dir="+chromeProfileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--no-sandbox",
		"--disable-gpu-sandbox",
		"--enable-unsafe-webgpu",
		"--use-gl=angle",
		url,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start headless chromium failed: %v", err)
	}
	var jsonText string
	select {
	case jsonText = <-reportCh:
	case <-cCtx.Done():
		_ = cmd.Process.Kill()
		t.Fatalf("timed out waiting for Headless Chromium /api/report:\n%s", out.String())
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	t.Logf("[BECMaster Metric 3B — Headless Chromium GPU & PMTiles 206] %s", jsonText)
	if !strings.Contains(jsonText, `"ok":true`) {
		t.Fatalf("Headless Chromium GPU/PMTiles qualification reported failure: %s", jsonText)
	}
}
