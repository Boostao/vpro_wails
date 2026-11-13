package interchange_test

import (
	"context"
	"database/sql"
	"encoding/json"
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

func TestVenusQuarantinedSoilHorizonsPreserveFullPayloadJSON(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	dir := t.TempDir()

	venusPath := filepath.Join(dir, "venus_quarantine_payload.xml")
	venusXML := `<?xml version="1.0" encoding="UTF-8"?>
<VenusArchive version="5.0" projectName="VenusQ">
  <Plot PlotNumber="VQ001" Zone="CWH">
    <Humus horizon="Fm" upperdepth="5" lowerdepth="5" phorganic="4.2" vonpost="3" comment="fibric zero thickness"/>
    <Mineral horizon="Bm1" upperdepth="15" lowerdepth="15" texture="SiL" colour="10YR 4/3" phmineral="5.6" comments="silt loam zero thickness"/>
  </Plot>
</VenusArchive>`
	if err := os.WriteFile(venusPath, []byte(venusXML), 0o600); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "VenusQImp.db")
	if _, err := interchange.ImportVenusProject(service, ctx, state.ContextID, interchange.VenusImportRequest{
		SourcePath:      venusPath,
		SourceProject:   "VenusQ",
		TargetProject:   "VenusQImp",
		DestinationPath: destPath,
	}); err != nil {
		t.Fatalf("ImportVenusProject failed: %v", err)
	}

	db, err := sqlutil.OpenReadOnly(destPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var humusPayloadRaw string
	if err := db.QueryRow(`SELECT "PayloadJSON" FROM "__VPRO_Quarantine_Humus" WHERE "PlotNumber" = 'VQ001'`).Scan(&humusPayloadRaw); err != nil {
		t.Fatalf("expected PayloadJSON column on __VPRO_Quarantine_Humus: %v", err)
	}
	var humusMap map[string]string
	if err := json.Unmarshal([]byte(humusPayloadRaw), &humusMap); err != nil {
		t.Fatalf("invalid Humus PayloadJSON %q: %v", humusPayloadRaw, err)
	}
	if humusMap["phorganic"] != "4.2" || humusMap["vonpost"] != "3" || humusMap["comment"] != "fibric zero thickness" {
		t.Fatalf("quarantined Humus PayloadJSON lost fields: got %+v", humusMap)
	}

	var mineralPayloadRaw string
	if err := db.QueryRow(`SELECT "PayloadJSON" FROM "__VPRO_Quarantine_Mineral" WHERE "PlotNumber" = 'VQ001'`).Scan(&mineralPayloadRaw); err != nil {
		t.Fatalf("expected PayloadJSON column on __VPRO_Quarantine_Mineral: %v", err)
	}
	var mineralMap map[string]string
	if err := json.Unmarshal([]byte(mineralPayloadRaw), &mineralMap); err != nil {
		t.Fatalf("invalid Mineral PayloadJSON %q: %v", mineralPayloadRaw, err)
	}
	if mineralMap["texture"] != "SiL" || mineralMap["colour"] != "10YR 4/3" || mineralMap["phmineral"] != "5.6" || mineralMap["comments"] != "silt loam zero thickness" {
		t.Fatalf("quarantined Mineral PayloadJSON lost fields: got %+v", mineralMap)
	}
}

func TestTurboVegLongPlotIDCollisionDisambiguationAndExactDuplicateError(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	dir := t.TempDir()

	// 1. TurboVeg3 SQLite with two distinct >7-UTF-16 PlotNumbers ("1234567A" and "1234567B")
	// plus a literal 7-char "1234567" plot to ensure disambiguation never steals a literal short key!
	tvSQLitePath := filepath.Join(dir, "tv_long_collide.db")
	srcDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tvSQLitePath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srcDB.Exec(`
		CREATE TABLE SpeciesList (SpeciesNR INTEGER PRIMARY KEY, SpeciesCode TEXT, ScientificName TEXT, EnglishName TEXT, Lifeform INTEGER, Authority TEXT);
		CREATE TABLE Plots (PlotID INTEGER PRIMARY KEY, PlotNumber TEXT, FieldNumber TEXT, ProjectID TEXT, Date TEXT, Latitude REAL, Longitude REAL, Elevation INTEGER, Slope REAL, Aspect INTEGER, PlotSize REAL);
		CREATE TABLE HeaderData (PlotID INTEGER PRIMARY KEY, Zone TEXT, SubZone TEXT, SiteSeries TEXT, SiteUnit TEXT, MoistureRegime TEXT, NutrientRegime TEXT, MesoSlopePosition TEXT, SurfaceShape TEXT, SoilDrainage TEXT, HumusForm TEXT, Surveyor TEXT, Location TEXT, SiteNotes TEXT, VegNotes TEXT, SoilNotes TEXT);
		CREATE TABLE SpeciesData (ID INTEGER PRIMARY KEY AUTOINCREMENT, PlotID INTEGER, SpeciesNR INTEGER, SpeciesCode TEXT, LayerCode TEXT, TurboVegGroup TEXT, Cover REAL, Height REAL, Collected TEXT);

		INSERT INTO Plots (PlotID, PlotNumber) VALUES
			(1, '1234567A'),
			(2, '1234567B'),
			(3, '1234567');
		INSERT INTO HeaderData (PlotID, Zone, SiteUnit) VALUES
			(1, 'CWH', 'CWHvm1/01'),
			(2, 'IDF', 'IDFdk1/01'),
			(3, 'ESSF', 'ESSFmw/01');
		INSERT INTO SpeciesData (PlotID, SpeciesNR, SpeciesCode, LayerCode, TurboVegGroup, Cover) VALUES
			(1, 1, 'PSEUMEN', '1', 'Tree', 40.0),
			(2, 2, 'PINUPON', '1', 'Tree', 55.0),
			(3, 3, 'ABIEAMA', '1', 'Tree', 30.0);
	`); err != nil {
		srcDB.Close()
		t.Fatal(err)
	}
	srcDB.Close()

	tvDest := filepath.Join(dir, "TVCollideImp.db")
	out, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      tvSQLitePath,
		Format:          interchange.TurboVegFormatSQLite,
		TargetProject:   "TVCollide",
		DestinationPath: tvDest,
	})
	if err != nil {
		t.Fatalf("ImportTurboVeg(1234567A, 1234567B, 1234567) failed: %v", err)
	}
	if out.PlotCount != 3 {
		t.Fatalf("expected PlotCount=3, got %d", out.PlotCount)
	}

	roDB, err := sqlutil.OpenReadOnly(tvDest)
	if err != nil {
		t.Fatal(err)
	}
	defer roDB.Close()

	var envCount, vegCount, suCount int
	if err := roDB.QueryRow(`SELECT COUNT(*) FROM TVCollide_Env`).Scan(&envCount); err != nil || envCount != 3 {
		t.Fatalf("expected all 3 plots preserved in TVCollide_Env (no silent truncation drop), got %d (err=%v)", envCount, err)
	}
	if err := roDB.QueryRow(`SELECT COUNT(*) FROM TVCollide_Veg`).Scan(&vegCount); err != nil || vegCount != 3 {
		t.Fatalf("expected all 3 plots' Veg observations preserved in TVCollide_Veg, got %d (err=%v)", vegCount, err)
	}
	if err := roDB.QueryRow(`SELECT COUNT(*) FROM TVCollide_SU`).Scan(&suCount); err != nil || suCount != 3 {
		t.Fatalf("expected all 3 plots preserved in TVCollide_SU, got %d (err=%v)", suCount, err)
	}

	rows, err := roDB.Query(`SELECT "PlotNumber" FROM TVCollide_Env`)
	if err != nil {
		t.Fatal(err)
	}
	seenImported := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if !utf8.ValidString(p) || utf16LenTest(p) > 7 {
			rows.Close()
			t.Fatalf("disambiguated PlotNumber %q exceeded 7 UTF-16 units (%d)", p, utf16LenTest(p))
		}
		seenImported[p] = true
	}
	rows.Close()
	if len(seenImported) != 3 || !seenImported["1234567"] {
		t.Fatalf("expected 3 unique <=7-UTF-16 PlotNumbers preserving literal '1234567', got %v", seenImported)
	}

	// 2. Also verify XML format disambiguates "1234567A" vs "1234567B" without dropping the second plot!
	xmlCollidePath := filepath.Join(dir, "tv_xml_collide.xml")
	xmlCollide := `<?xml version="1.0" encoding="UTF-8"?>
<Turboveg version="3.0">
  <Plots>
    <Plot releve_nr="1" plot_number="1234567A"><SpeciesData><Species code="PSEUMEN" layer="1" cover="20"/></SpeciesData></Plot>
    <Plot releve_nr="2" plot_number="1234567B"><SpeciesData><Species code="TSUGHET" layer="1" cover="30"/></SpeciesData></Plot>
  </Plots>
</Turboveg>`
	if err := os.WriteFile(xmlCollidePath, []byte(xmlCollide), 0o600); err != nil {
		t.Fatal(err)
	}
	xmlDest := filepath.Join(dir, "TVXMLCollide.db")
	if _, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      xmlCollidePath,
		Format:          interchange.TurboVegFormatXML,
		TargetProject:   "TVXMLCol",
		DestinationPath: xmlDest,
	}); err != nil {
		t.Fatalf("ImportTurboVeg XML collision failed: %v", err)
	}
	xmlDB, err := sqlutil.OpenReadOnly(xmlDest)
	if err != nil {
		t.Fatal(err)
	}
	defer xmlDB.Close()
	var xmlEnvCnt int
	if err := xmlDB.QueryRow(`SELECT COUNT(*) FROM TVXMLCol_Env`).Scan(&xmlEnvCnt); err != nil || xmlEnvCnt != 2 {
		t.Fatalf("expected 2 XML plots preserved after >7-UTF-16 disambiguation, got %d (err=%v)", xmlEnvCnt, err)
	}

	// 3. Exact duplicate source plot keys ("1234567A" and "1234567A") MUST return an explicit error, never drop silently!
	dupXMLPath := filepath.Join(dir, "tv_exact_dup.xml")
	dupXML := `<?xml version="1.0" encoding="UTF-8"?>
<Turboveg version="3.0">
  <Plots>
    <Plot releve_nr="1" plot_number="1234567A"><SpeciesData><Species code="PSEUMEN" layer="1" cover="20"/></SpeciesData></Plot>
    <Plot releve_nr="2" plot_number="1234567A"><SpeciesData><Species code="TSUGHET" layer="1" cover="30"/></SpeciesData></Plot>
  </Plots>
</Turboveg>`
	if err := os.WriteFile(dupXMLPath, []byte(dupXML), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      dupXMLPath,
		Format:          interchange.TurboVegFormatXML,
		TargetProject:   "TVDup",
		DestinationPath: filepath.Join(dir, "TVDup.db"),
	}); err == nil {
		t.Fatalf("expected ImportTurboVeg to return an explicit error on exact duplicate source plot number '1234567A'")
	}
}

