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

	// --- Part 1: TurboVeg <Plot_package> Import (Audit Restore/Flag booleans, 99.999996 float32 cover, multi-row rollup cap 99.0, astral UTF-16 truncation) ---
	// "🌲🌲🌲🌲" is 4 runes = 8 UTF-16 units -> must truncate to <= 7 UTF-16 units ("🌲🌲🌲", 6 units) without splitting surrogate!
	// "🌲🌲🌲🌲🌲" is 5 runes = 10 UTF-16 units -> Species must truncate to <= 8 UTF-16 units ("🌲🌲🌲🌲", 8 units)!
	tvXMLPath := filepath.Join(dir, "tv_bounds.xml")
	tvXML := `<?xml version="1.0" encoding="UTF-8"?>
<Plot_package UserName="VPro" TurbovegVersion="3.0" Dictionary="vpro_bec_dict" SpeciesList="BCPlantTaxonomy">
  <Plots>
    <Plot database="BoundsTest" releve_nr="1" guid="{11111111-2222-4333-8444-555555555555}">
      <header_data>
        <standard_record releve_nr="1" country="CA" coverscale="00"/>
        <udf_record ispredefined="false" name="vpro_plot_number" type="C" len="10" dec="0" value="🌲🌲🌲🌲"/>
        <udf_record ispredefined="false" name="zone" type="C" len="10" dec="0" value="CWH🌲🌲"/>
        <udf_record ispredefined="false" name="subzone" type="C" len="15" dec="0" value="vm1🌲🌲🌲🌲"/>
        <udf_record ispredefined="false" name="site_series" type="C" len="10" dec="0" value="01🌲🌲🌲"/>
      </header_data>
      <species_data>
        <species>
          <standard_record nr="1" cover="99.999996" layer="1"/>
          <udf_record ispredefined="false" name="vpro_code" type="C" len="8" dec="0" value="PSEUMEN"/>
          <udf_record ispredefined="false" name="collected" type="C" len="4" dec="0" value="🌲V"/>
        </species>
        <species>
          <standard_record nr="2" cover="60.0" layer="4"/>
        </species>
        <species>
          <standard_record nr="2" cover="55.0" layer="4"/>
        </species>
        <species>
          <standard_record nr="3" cover="12.5" layer="6"/>
          <udf_record ispredefined="false" name="vpro_code" type="C" len="16" dec="0" value="🌲🌲🌲🌲🌲"/>
        </species>
      </species_data>
    </Plot>
  </Plots>
  <Lookup_tables>
    <Coverscale_list>
      <coverscale_record><code>00</code><description>Percentage (%)</description></coverscale_record>
    </Coverscale_list>
    <Species_list>
      <species_record nr="1" code="PSEUMEN" name="Pseudotsuga menziesii" author="" valid_nr="1" valid_name="" valid_author="" synonym="no" nativename=""/>
      <species_record nr="2" code="TSUGHET" name="Tsuga heterophylla" author="" valid_nr="2" valid_name="" valid_author="" synonym="no" nativename=""/>
      <species_record nr="3" code="🌲🌲🌲" name="🌲🌲🌲🌲🌲" author="" valid_nr="3" valid_name="" valid_author="" synonym="no" nativename=""/>
    </Species_list>
  </Lookup_tables>
</Plot_package>`
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
		badXML := `<?xml version="1.0" encoding="UTF-8"?><Plot_package><Plots><Plot releve_nr="1" guid="{22222222-3333-4444-8555-666666666666}"><header_data><standard_record releve_nr="1" coverscale="00"/><udf_record ispredefined="false" name="vpro_plot_number" type="C" len="7" dec="0" value="BAD1"/></header_data><species_data><species><standard_record nr="1" cover="` + badCov + `" layer="1"/></species></species_data></Plot></Plots><Lookup_tables><Species_list><species_record nr="1" code="PSEUMEN" name="Pseudotsuga menziesii" author="" valid_nr="1" valid_name="" valid_author="" synonym="no" nativename=""/></Species_list></Lookup_tables></Plot_package>`
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

	// Non-standard <Turboveg> XML root must also be rejected cleanly
	legacyXMLPath := filepath.Join(dir, "legacy_bespoke.xml")
	if err := os.WriteFile(legacyXMLPath, []byte(`<?xml version="1.0"?><Turboveg version="3.0"><Plots><Plot releve_nr="1" plot_number="001"/></Plots></Turboveg>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := interchange.ImportTurboVeg(service, ctx, activeCtxID, interchange.TurboVegImportRequest{
		SourcePath:      legacyXMLPath,
		Format:          interchange.TurboVegFormatXML,
		TargetProject:   "BadLegacy",
		DestinationPath: filepath.Join(dir, "BadLegacy.db"),
	}); err == nil {
		t.Fatal("expected non-standard <Turboveg> root XML to be rejected")
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

	// Seed a real 31-table Turboveg 3 SQLite baseline via ExportTurboVeg, then replace its plots with
	// two distinct >7-UTF-16 PlotNumbers ("1234567A" and "1234567B") plus a literal 7-char "1234567" plot
	// to ensure disambiguation never steals a literal short key!
	tvSQLitePath := filepath.Join(dir, "tv_long_collide.sqlite")
	if _, err := interchange.ExportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegExportRequest{
		Format:          interchange.TurboVegFormatSQLite,
		DestinationPath: tvSQLitePath,
	}); err != nil {
		t.Fatalf("seed ExportTurboVeg(sqlite): %v", err)
	}
	srcDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tvSQLitePath, "rw")+"&_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srcDB.Exec(`
		DELETE FROM speciesobservationdata;
		DELETE FROM speciesobservations;
		DELETE FROM plotobservationheaderdata;
		DELETE FROM plotobservations;
		DELETE FROM plots;
		DELETE FROM locations;
		DELETE FROM covercodes;
		DELETE FROM taxa;

		INSERT INTO covercodes (id, lookupdataid, code, percentage_mean) VALUES
			(1, 1, '40', 40.0),
			(2, 1, '55', 55.0),
			(3, 1, '30', 30.0);
		INSERT INTO taxa (id, taxonlistid, scientificname, lettercode, tv2speciesid) VALUES
			(1, 1, 'Pseudotsuga menziesii', 'PSEUMEN', 1),
			(2, 1, 'Pinus ponderosa', 'PINUPON', 2),
			(3, 1, 'Abies amabilis', 'ABIEAMA', 3);

		INSERT INTO plots (id, datasetid) VALUES (1, 1), (2, 1), (3, 1);
		INSERT INTO plotobservations (id, plotid, guid, deleted) VALUES
			(1, 1, '10000000-0000-4000-8000-000000000001', 0),
			(2, 2, '10000000-0000-4000-8000-000000000002', 0),
			(3, 3, '10000000-0000-4000-8000-000000000003', 0);

		INSERT INTO plotobservationheaderdata (id, plotobservationid, fieldid, valuestring)
		SELECT 1, 1, id, '1234567A' FROM fields WHERE fieldname='VPRO_PLOT'
		UNION ALL SELECT 2, 1, id, 'CWH' FROM fields WHERE fieldname='ZONE'
		UNION ALL SELECT 3, 1, id, 'CWHvm1/01' FROM fields WHERE fieldname='SITE_UNIT'
		UNION ALL SELECT 4, 2, id, '1234567B' FROM fields WHERE fieldname='VPRO_PLOT'
		UNION ALL SELECT 5, 2, id, 'IDF' FROM fields WHERE fieldname='ZONE'
		UNION ALL SELECT 6, 2, id, 'IDFdk1/01' FROM fields WHERE fieldname='SITE_UNIT'
		UNION ALL SELECT 7, 3, id, '1234567' FROM fields WHERE fieldname='VPRO_PLOT'
		UNION ALL SELECT 8, 3, id, 'ESSF' FROM fields WHERE fieldname='ZONE'
		UNION ALL SELECT 9, 3, id, 'ESSFmw/01' FROM fields WHERE fieldname='SITE_UNIT';

		INSERT INTO speciesobservations (id, plotobservationid, taxonid, vegetationlayerid, covercodeid, originaltaxonconcept, deleted) VALUES
			(1, 1, 1, 1, 1, 'Pseudotsuga menziesii', 0),
			(2, 2, 2, 1, 2, 'Pinus ponderosa', 0),
			(3, 3, 3, 1, 3, 'Abies amabilis', 0);
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

	// 2. Also verify <Plot_package> XML format disambiguates "1234567A" vs "1234567B" without dropping the second plot!
	xmlCollidePath := filepath.Join(dir, "tv_xml_collide.xml")
	xmlCollide := `<?xml version="1.0" encoding="UTF-8"?>
<Plot_package>
  <Plots>
    <Plot releve_nr="1" guid="{10000000-0000-4000-8000-000000000011}">
      <header_data><standard_record releve_nr="1" coverscale="00"/><udf_record ispredefined="false" name="vpro_plot_number" type="C" len="8" dec="0" value="1234567A"/></header_data>
      <species_data><species><standard_record nr="1" cover="20" layer="1"/></species></species_data>
    </Plot>
    <Plot releve_nr="2" guid="{10000000-0000-4000-8000-000000000012}">
      <header_data><standard_record releve_nr="2" coverscale="00"/><udf_record ispredefined="false" name="vpro_plot_number" type="C" len="8" dec="0" value="1234567B"/></header_data>
      <species_data><species><standard_record nr="2" cover="30" layer="1"/></species></species_data>
    </Plot>
  </Plots>
  <Lookup_tables>
    <Species_list>
      <species_record nr="1" code="PSEUMEN" name="Pseudotsuga menziesii" author="" valid_nr="1" valid_name="" valid_author="" synonym="no" nativename=""/>
      <species_record nr="2" code="TSUGHET" name="Tsuga heterophylla" author="" valid_nr="2" valid_name="" valid_author="" synonym="no" nativename=""/>
    </Species_list>
  </Lookup_tables>
</Plot_package>`
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
<Plot_package>
  <Plots>
    <Plot releve_nr="1" guid="{10000000-0000-4000-8000-000000000021}">
      <header_data><standard_record releve_nr="1" coverscale="00"/><udf_record ispredefined="false" name="vpro_plot_number" type="C" len="8" dec="0" value="1234567A"/></header_data>
      <species_data><species><standard_record nr="1" cover="20" layer="1"/></species></species_data>
    </Plot>
    <Plot releve_nr="2" guid="{10000000-0000-4000-8000-000000000022}">
      <header_data><standard_record releve_nr="2" coverscale="00"/><udf_record ispredefined="false" name="vpro_plot_number" type="C" len="8" dec="0" value="1234567A"/></header_data>
      <species_data><species><standard_record nr="2" cover="30" layer="1"/></species></species_data>
    </Plot>
  </Plots>
  <Lookup_tables>
    <Species_list>
      <species_record nr="1" code="PSEUMEN" name="Pseudotsuga menziesii" author="" valid_nr="1" valid_name="" valid_author="" synonym="no" nativename=""/>
      <species_record nr="2" code="TSUGHET" name="Tsuga heterophylla" author="" valid_nr="2" valid_name="" valid_author="" synonym="no" nativename=""/>
    </Species_list>
  </Lookup_tables>
</Plot_package>`
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

	// 4. Legacy invented 5-table SQLite file must also be rejected cleanly
	legacySQLPath := filepath.Join(dir, "legacy_5table.sqlite")
	legDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(legacySQLPath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = legDB.Exec(`CREATE TABLE Plots (PlotID INTEGER PRIMARY KEY, PlotNumber TEXT); INSERT INTO Plots VALUES (1, '001');`)
	legDB.Close()
	if _, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      legacySQLPath,
		Format:          interchange.TurboVegFormatSQLite,
		TargetProject:   "TVBadSQL",
		DestinationPath: filepath.Join(dir, "TVBadSQL.db"),
	}); err == nil {
		t.Fatal("expected legacy 5-table SQLite file to be rejected")
	}
}

func TestTurboVegSynonymMultiplicityAndDelphiDateTimeEdgeCases(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	dir := t.TempDir()

	// 1. Export timestamped Date ("2024-07-19 14:30:00") and verify numeric Delphi TDateTime valuefloat is written!
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		UPDATE Sample_Env SET "Date"='2024-07-19 14:30:00' WHERE PlotNumber='108050';
	`)
	tvSQLPath := filepath.Join(dir, "tv_syn_date.sqlite")
	if _, err := interchange.ExportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegExportRequest{
		Format:          interchange.TurboVegFormatSQLite,
		DestinationPath: tvSQLPath,
	}); err != nil {
		t.Fatal(err)
	}

	mutDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tvSQLPath, "rw")+"&_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	var dateFloat float64
	if err := mutDB.QueryRow(`
		SELECT poh.valuefloat
		FROM plotobservationheaderdata poh
		INNER JOIN fields f ON f.id = poh.fieldid
		INNER JOIN plotobservationheaderdata pplot ON pplot.plotobservationid = poh.plotobservationid
		INNER JOIN fields fp ON fp.id = pplot.fieldid AND fp.fieldname = 'VPRO_PLOT' AND pplot.valuestring = '108050'
		WHERE f.fieldname = 'DATE'`).Scan(&dateFloat); err != nil || dateFloat < 45492.0 {
		mutDB.Close()
		t.Fatalf("expected populated Delphi TDateTime valuefloat >= 45492.0 for timestamped Date, got %v (err=%v)", dateFloat, err)
	}

	// 2. Replace with a single plot having pre-1899 negative fractional Delphi TDateTime (-1.5 -> 1899-12-29)
	// and a synonym taxon where tv2validid=200 matches MULTIPLE rows in taxa (pro-parte + orthographic) — cover must NOT double!
	if _, err := mutDB.Exec(`
		DELETE FROM speciesobservationdata;
		DELETE FROM speciesobservations;
		DELETE FROM plotobservationheaderdata;
		DELETE FROM plotobservations;
		DELETE FROM plots;
		DELETE FROM locations;
		DELETE FROM covercodes;
		DELETE FROM taxa;

		INSERT INTO covercodes (id, lookupdataid, code, percentage_mean) VALUES (1, 1, '20', 20.0);
		INSERT INTO taxa (id, taxonlistid, scientificname, lettercode, tv2speciesid, tv2validid, issynonym) VALUES
			(1, 1, 'Obsolete local name', 'OBSOL01', 200, 200, -1),
			(2, 1, 'Unmapped synonym -1', 'UNMAP01', 200, 200, -1),
			(3, 1, 'Callitropsis nootkatensis', 'CALLNOO', 200, 200, 0);

		ALTER TABLE locations ADD COLUMN wkt TEXT;
		INSERT INTO locations (id, the_geom, wkt) VALUES (1, '   ', 'POINT(-125.5 50.25)');
		INSERT INTO plots (id, datasetid, locationid) VALUES (1, 1, 1), (2, 1, NULL);
		INSERT INTO plotobservations (id, plotid, guid, deleted) VALUES
			(1, 1, '99999999-0000-4000-8000-000000000001', 0),
			(2, 2, '99999999-0000-4000-8000-000000000002', 0);
		INSERT INTO plotobservationheaderdata (id, plotobservationid, fieldid, valuestring, valueinteger, valuefloat)
		SELECT 1, 1, id, 'HIST01', NULL, NULL FROM fields WHERE fieldname='VPRO_PLOT'
		UNION ALL SELECT 2, 1, id, NULL, NULL, -1.5 FROM fields WHERE fieldname='DATE'
		UNION ALL SELECT 3, 2, id, 'HIST02', NULL, NULL FROM fields WHERE fieldname='VPRO_PLOT'
		UNION ALL SELECT 4, 2, id, NULL, 45492, NULL FROM fields WHERE fieldname='DATE';

		INSERT INTO speciesobservations (id, plotobservationid, taxonid, vegetationlayerid, covercodeid, originaltaxonconcept, deleted)
		VALUES (1, 1, 1, 1, 1, 'Obsolete local name', 0);
	`); err != nil {
		mutDB.Close()
		t.Fatal(err)
	}
	mutDB.Close()

	impDest := filepath.Join(dir, "TVHistSyn.db")
	if _, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      tvSQLPath,
		Format:          interchange.TurboVegFormatSQLite,
		TargetProject:   "TVHistSyn",
		DestinationPath: impDest,
	}); err != nil {
		t.Fatalf("ImportTurboVeg(TVHistSyn): %v", err)
	}

	roDB, err := sqlutil.OpenReadOnly(impDest)
	if err != nil {
		t.Fatal(err)
	}
	defer roDB.Close()

	var histDate string
	var histLat, histLon float64
	if err := roDB.QueryRow(`SELECT CAST("Date" AS TEXT), Latitude, Longitude FROM TVHistSyn_Env WHERE PlotNumber='HIST01'`).Scan(&histDate, &histLat, &histLon); err != nil {
		t.Fatal(err)
	}
	if histDate != "1899-12-29" || math.Abs(histLat-50.25) > 1e-6 || math.Abs(histLon - -125.5) > 1e-6 {
		t.Fatalf("expected -1.5 -> 1899-12-29 and empty the_geom fallback to wkt (-125.5, 50.25), got date=%q (%v,%v)", histDate, histLon, histLat)
	}
	var intDate string
	if err := roDB.QueryRow(`SELECT CAST("Date" AS TEXT) FROM TVHistSyn_Env WHERE PlotNumber='HIST02'`).Scan(&intDate); err != nil || intDate != "2024-07-19" {
		t.Fatalf("expected integer Delphi DATE 45492 -> 2024-07-19, got %q (err=%v)", intDate, err)
	}
	var callCov float64
	if err := roDB.QueryRow(`SELECT Cover1 FROM TVHistSyn_Veg WHERE PlotNumber='HIST01' AND Species='CALLNOO'`).Scan(&callCov); err != nil {
		t.Fatal(err)
	}
	if math.Abs(callCov-20.0) > 1e-4 {
		t.Fatalf("expected non-duplicated synonym Cover1=20.0, got %v", callCov)
	}

	// 3. Reject malformed UTF-8 (CAST(X'FF' AS TEXT)) in SQLite header text
	badUTFDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tvSQLPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = badUTFDB.Exec(`UPDATE plotobservationheaderdata SET valuestring = CAST(X'FF' AS TEXT) WHERE id = 1`)
	badUTFDB.Close()
	if _, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      tvSQLPath,
		Format:          interchange.TurboVegFormatSQLite,
		TargetProject:   "TVBadUTF8",
		DestinationPath: filepath.Join(dir, "TVBadUTF8.db"),
	}); err == nil {
		t.Fatal("expected malformed UTF-8 in SQLite source to be rejected")
	}
}

