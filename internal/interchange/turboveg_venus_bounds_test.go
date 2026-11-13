package interchange_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/interchange"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func utf16LenTest(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func TestTurboVegAndVenusAuditBooleansFloat32CoverUTF16AndSoilDepthSigns(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	dir := t.TempDir()

	// --- Part 1: TurboVeg Import (Audit Restore/Flag booleans, 99.999996 float32 cover, multi-row rollup cap 99.0, astral UTF-16 truncation) ---
	// "🌲🌲🌲🌲" is 4 runes = 8 UTF-16 units -> must truncate to <= 7 UTF-16 units ("🌲🌲🌲", 6 units) without splitting surrogate!
	// "🌲🌲🌲🌲🌲" is 5 runes = 10 UTF-16 units -> Species must truncate to <= 8 UTF-16 units ("🌲🌲🌲🌲", 8 units)!
	tvXMLPath := filepath.Join(dir, "tv_bounds.xml")
	tvXML := `<?xml version="1.0" encoding="UTF-8"?>
<Turboveg version="3.0">
  <Plots>
    <Plot releve_nr="1" plot_number="🌲🌲🌲🌲">
      <Header zone="CWH🌲🌲" subzone="vm1🌲🌲🌲🌲" site_series="01🌲🌲🌲"/>
      <SpeciesData>
        <Species code="PSEUMEN" layer="1" cover="99.999996" collected="🌲V"/>
        <Species code="TSUGHET" layer="4" cover="60.0"/>
        <Species code="TSUGHET" layer="4" cover="55.0"/>
        <Species code="🌲🌲🌲🌲🌲" layer="6" cover="12.5"/>
      </SpeciesData>
    </Plot>
  </Plots>
</Turboveg>`
	if err := os.WriteFile(tvXMLPath, []byte(tvXML), 0o600); err != nil {
		t.Fatal(err)
	}

	tvDest := filepath.Join(dir, "TVBounds.db")
	tvOut, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:        tvXMLPath,
		Format:            interchange.TurboVegFormatXML,
		TargetProject:     "TVBounds",
		DestinationPath:   tvDest,
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatalf("ImportTurboVeg valid payload failed: %v", err)
	}
	activeCtxID := tvOut.State.ContextID

	tvDB, err := sqlutil.OpenReadOnly(tvDest)
	if err != nil {
		t.Fatal(err)
	}
	var tvPlotNum string
	if err := tvDB.QueryRow(`SELECT "PlotNumber" FROM TVBounds_Env LIMIT 1`).Scan(&tvPlotNum); err != nil {
		tvDB.Close()
		t.Fatal(err)
	}
	if !utf8.ValidString(tvPlotNum) || utf16LenTest(tvPlotNum) > 7 {
		tvDB.Close()
		t.Fatalf("TurboVeg PlotNumber %q exceeded 7 UTF-16 units (%d) or invalid UTF-8", tvPlotNum, utf16LenTest(tvPlotNum))
	}

	// Bug A: ListAuditEntries & ListAuditReadEntries must NOT crash with NULL Restore/Flag!
	auditEntries, err := service.ListAuditEntries(ctx, activeCtxID, tvPlotNum)
	if err != nil || len(auditEntries) == 0 {
		tvDB.Close()
		t.Fatalf("ListAuditEntries failed after ImportTurboVeg (NULL Restore/Flag bug): entries=%d err=%v", len(auditEntries), err)
	}
	auditReadEntries, err := service.ListAuditReadEntries(ctx, activeCtxID, tvPlotNum)
	if err != nil || len(auditReadEntries) == 0 {
		tvDB.Close()
		t.Fatalf("ListAuditReadEntries failed after ImportTurboVeg: entries=%d err=%v", len(auditReadEntries), err)
	}

	// Bug B: 99.999996 must remain < 100 in float32 (NOT rounded to 100.0 by math.Round*10000/10000!),
	// and multi-row rollup 60 + 55 must cap at 99.0!
	var pseuCov, tsugCov float64
	if err := tvDB.QueryRow(`SELECT "Cover1" FROM TVBounds_Veg WHERE "Species"='PSEUMEN'`).Scan(&pseuCov); err != nil {
		tvDB.Close()
		t.Fatal(err)
	}
	if float32(pseuCov) >= 100 || math.Abs(pseuCov-float64(float32(99.999996))) > 1e-7 {
		tvDB.Close()
		t.Fatalf("TurboVeg Cover1=99.999996 corrupted to %v (float32=%v, want < 100)", pseuCov, float32(pseuCov))
	}
	if err := tvDB.QueryRow(`SELECT "Cover4" FROM TVBounds_Veg WHERE "Species"='TSUGHET'`).Scan(&tsugCov); err != nil {
		tvDB.Close()
		t.Fatal(err)
	}
	if tsugCov != 99.0 {
		tvDB.Close()
		t.Fatalf("expected multi-row rollup 60+55 capped at 99.0, got %v", tsugCov)
	}

	// Bug C: Check Species & Collected UTF-16 bounds and valid UTF-8 on TVBounds_Veg.
	vRows, err := tvDB.Query(`SELECT "Species", COALESCE("Collected", '') FROM TVBounds_Veg`)
	if err != nil {
		tvDB.Close()
		t.Fatal(err)
	}
	for vRows.Next() {
		var spp, coll string
		if err := vRows.Scan(&spp, &coll); err != nil {
			vRows.Close()
			tvDB.Close()
			t.Fatal(err)
		}
		if !utf8.ValidString(spp) || utf16LenTest(spp) > 8 {
			vRows.Close()
			tvDB.Close()
			t.Fatalf("TurboVeg Species %q exceeded 8 UTF-16 units (%d)", spp, utf16LenTest(spp))
		}
		if !utf8.ValidString(coll) || utf16LenTest(coll) > 1 {
			vRows.Close()
			tvDB.Close()
			t.Fatalf("TurboVeg Collected %q invalid UTF-8 or exceeded 1 UTF-16 unit (%d)", coll, utf16LenTest(coll))
		}
	}
	vRows.Close()
	tvDB.Close()

	// Single invalid observation Cover >= 100 in float32 (99.999999) or < 0 (-5.0) must be rejected!
	for _, badCov := range []string{"99.999999", "-5.0"} {
		badXMLPath := filepath.Join(dir, "tv_bad_"+badCov+".xml")
		badXML := `<?xml version="1.0" encoding="UTF-8"?><Turboveg version="3.0"><Plots><Plot releve_nr="1" plot_number="BAD1"><SpeciesData><Species code="PSEUMEN" layer="1" cover="` + badCov + `"/></SpeciesData></Plot></Plots></Turboveg>`
		if err := os.WriteFile(badXMLPath, []byte(badXML), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := interchange.ImportTurboVeg(service, ctx, activeCtxID, interchange.TurboVegImportRequest{
			SourcePath:      badXMLPath,
			Format:          interchange.TurboVegFormatXML,
			TargetProject:   "BadTV",
			DestinationPath: filepath.Join(dir, "BadTV_"+badCov+".db"),
		}); err == nil {
			t.Fatalf("expected ImportTurboVeg to reject single invalid cover %s", badCov)
		}
	}

	// --- Part 2: VENUS 5.0 Import (Audit booleans, 99.999996 float32 cover, astral UTF-16 bounds, LMH25 soil depth signs) ---
	venusPath := filepath.Join(dir, "venus_bounds.xml")
	venusXML := `<?xml version="1.0" encoding="UTF-8"?>
<VenusArchive version="5.0" projectName="VenusBounds">
  <Plot PlotNumber="🌲🌲🌲🌲" Zone="CWH🌲🌲" NtsMapSheet="092G06🌲🌲🌲">
    <VegLayer code="1">
      <Species code="PSEUMEN" cover="99.999996" collected="🌲"/>
      <Species code="🌲🌲🌲🌲🌲" cover="15.0"/>
    </VegLayer>
    <Humus horizon="Lv🌲🌲🌲🌲" upperdepth="2" lowerdepth="8"/>
    <Humus horizon="Of" upperdepth="25" lowerdepth="5"/>
    <Humus horizon="Fh" upperdepth="4" lowerdepth="4"/>
    <Mineral horizon="Bm🌲🌲🌲🌲" upperdepth="40" lowerdepth="0" texture="🌲🌲🌲" colour="10YR 4/4 🌲🌲🌲🌲🌲"/>
  </Plot>
</VenusArchive>`
	if err := os.WriteFile(venusPath, []byte(venusXML), 0o600); err != nil {
		t.Fatal(err)
	}

	venusDest := filepath.Join(dir, "VenusBoundsImp.db")
	vRes, err := interchange.ImportVenusProject(service, ctx, activeCtxID, interchange.VenusImportRequest{
		SourcePath:        venusPath,
		SourceProject:     "VenusBounds",
		TargetProject:     "VenusBoundsImp",
		DestinationPath:   venusDest,
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatalf("ImportVenusProject failed: %v", err)
	}
	vCtxID := vRes.State.ContextID

	vDB, err := sqlutil.OpenReadOnly(venusDest)
	if err != nil {
		t.Fatal(err)
	}
	defer vDB.Close()

	var vPlot string
	if err := vDB.QueryRow(`SELECT "PlotNumber" FROM VenusBoundsImp_Env LIMIT 1`).Scan(&vPlot); err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(vPlot) || utf16LenTest(vPlot) > 7 {
		t.Fatalf("VENUS PlotNumber %q exceeded 7 UTF-16 units (%d)", vPlot, utf16LenTest(vPlot))
	}

	// Bug A on VENUS: ListAuditEntries & ListAuditReadEntries must succeed!
	if entries, err := service.ListAuditEntries(ctx, vCtxID, vPlot); err != nil || len(entries) == 0 {
		t.Fatalf("VENUS ListAuditEntries failed: entries=%d err=%v", len(entries), err)
	}
	if entries, err := service.ListAuditReadEntries(ctx, vCtxID, vPlot); err != nil || len(entries) == 0 {
		t.Fatalf("VENUS ListAuditReadEntries failed: entries=%d err=%v", len(entries), err)
	}

	// Bug B on VENUS: 99.999996 preserved as < 100 float32
	var vCov float64
	if err := vDB.QueryRow(`SELECT "Cover1" FROM VenusBoundsImp_Veg WHERE "Species"='PSEUMEN'`).Scan(&vCov); err != nil {
		t.Fatal(err)
	}
	if float32(vCov) >= 100 {
		t.Fatalf("VENUS Cover1=99.999996 rounded to >=100: %v", vCov)
	}

	// Bug C & D on VENUS Humus & Mineral: UTF-16 bounds + LMH25 directional depth signs!
	hRows, err := vDB.Query(`SELECT "Horizon", "UpperDepth", "LowerDepth" FROM VenusBoundsImp_Humus ORDER BY "ID"`)
	if err != nil {
		t.Fatal(err)
	}
	humusCount := 0
	for hRows.Next() {
		var hz string
		var u, l float64
		if err := hRows.Scan(&hz, &u, &l); err != nil {
			hRows.Close()
			t.Fatal(err)
		}
		humusCount++
		if utf16LenTest(hz) > 8 {
			hRows.Close()
			t.Fatalf("VENUS Humus Horizon %q exceeded 8 UTF-16 units", hz)
		}
		if hz == "Of" {
			if !(u < l) {
				hRows.Close()
				t.Fatalf("VENUS wetland Of horizon depth sign not normalized: upper=%v lower=%v (want upper < lower)", u, l)
			}
		} else {
			if !(u > l) {
				hRows.Close()
				t.Fatalf("VENUS upland %q horizon depth sign not normalized: upper=%v lower=%v (want upper > lower)", hz, u, l)
			}
		}
	}
	hRows.Close()
	if humusCount != 2 {
		t.Fatalf("expected 2 valid normalized Humus rows (degenerate 4..4 quarantined), got %d", humusCount)
	}

	var mHz, mTex, mCol string
	var mU, mL float64
	if err := vDB.QueryRow(`SELECT "Horizon", "UpperDepth", "LowerDepth", "Texture", "Colour" FROM VenusBoundsImp_Mineral LIMIT 1`).
		Scan(&mHz, &mU, &mL, &mTex, &mCol); err != nil {
		t.Fatal(err)
	}
	if utf16LenTest(mHz) > 8 || utf16LenTest(mTex) > 4 || utf16LenTest(mCol) > 14 {
		t.Fatalf("VENUS Mineral UTF-16 overflow: hz=%q(%d) tex=%q(%d) col=%q(%d)",
			mHz, utf16LenTest(mHz), mTex, utf16LenTest(mTex), mCol, utf16LenTest(mCol))
	}
	if !(mU < mL) {
		t.Fatalf("VENUS Mineral horizon depth sign not normalized: upper=%v lower=%v (want upper < lower)", mU, mL)
	}

	// Verify degenerate Fh (4..4) was quarantined in __VPRO_Quarantine_Humus!
	var qCount int
	if err := vDB.QueryRow(`SELECT COUNT(*) FROM "__VPRO_Quarantine_Humus"`).Scan(&qCount); err != nil || qCount != 1 {
		t.Fatalf("expected 1 degenerate humus row quarantined in __VPRO_Quarantine_Humus, got %d (err=%v)", qCount, err)
	}
}
