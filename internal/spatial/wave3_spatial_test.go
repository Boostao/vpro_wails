package spatial

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
	"github.com/boostao/vpro-wails/resources"
)

func TestWave3PMTilesHTTPByteRangeAndManifest(t *testing.T) {
	fallbackHit := false
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("asset-ok"))
	})

	// Fresh-install handler (no dataDir or extraDirs -> falls back to embedded bc-overview.pmtiles)
	freshHandler := NewPMTilesAssetHandler(fallback, "")

	// 1. HEAD request returns exact Content-Length and Accept-Ranges: bytes
	headReq := httptest.NewRequest(http.MethodHead, "/maps/bc-overview.pmtiles", nil)
	headRec := httptest.NewRecorder()
	freshHandler.ServeHTTP(headRec, headReq)
	if headRec.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d, want 200", headRec.Code)
	}
	wantLen := fmt.Sprintf("%d", len(resources.BCOverviewPMTiles))
	if got := headRec.Header().Get("Content-Length"); got != wantLen {
		t.Fatalf("HEAD Content-Length = %q, want %q", got, wantLen)
	}
	if got := headRec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("HEAD Accept-Ranges = %q, want bytes", got)
	}
	if got := headRec.Header().Get("Content-Type"); got != PMTilesContentType {
		t.Fatalf("HEAD Content-Type = %q, want %q", got, PMTilesContentType)
	}
	if headRec.Body.Len() != 0 {
		t.Fatalf("HEAD body len = %d, want 0", headRec.Body.Len())
	}

	// 2. GET with Range: bytes=0-126 returns 206 Partial Content with PMTiles v3 magic
	rangeReq := httptest.NewRequest(http.MethodGet, "/maps/bc-overview.pmtiles", nil)
	rangeReq.Header.Set("Range", "bytes=0-126")
	rangeRec := httptest.NewRecorder()
	freshHandler.ServeHTTP(rangeRec, rangeReq)
	if rangeRec.Code != http.StatusPartialContent {
		t.Fatalf("GET Range status = %d, want 206", rangeRec.Code)
	}
	wantContentRange := fmt.Sprintf("bytes 0-126/%d", len(resources.BCOverviewPMTiles))
	if got := rangeRec.Header().Get("Content-Range"); got != wantContentRange {
		t.Fatalf("Content-Range = %q, want %q", got, wantContentRange)
	}
	body := rangeRec.Body.Bytes()
	if len(body) != 127 || string(body[:7]) != "PMTiles" || body[7] != 3 {
		t.Fatalf("unexpected 127-byte PMTiles header slice: len=%d magic=%q", len(body), body[:8])
	}

	// 3. Fresh install fallback for bc-bgc-v13.pmtiles + /maps/manifest.json
	bgcFallbackReq := httptest.NewRequest(http.MethodGet, "/maps/bc-bgc-v13.pmtiles", nil)
	bgcFallbackReq.Header.Set("Range", "bytes=0-15")
	bgcFallbackRec := httptest.NewRecorder()
	freshHandler.ServeHTTP(bgcFallbackRec, bgcFallbackReq)
	if bgcFallbackRec.Code != http.StatusPartialContent || bgcFallbackRec.Header().Get("X-VPro-PMTiles-Source") != "embedded-overview" {
		t.Fatalf("expected embedded-overview fallback on fresh install, got code=%d src=%q", bgcFallbackRec.Code, bgcFallbackRec.Header().Get("X-VPro-PMTiles-Source"))
	}

	manReq := httptest.NewRequest(http.MethodGet, "/maps/manifest.json", nil)
	manRec := httptest.NewRecorder()
	freshHandler.ServeHTTP(manRec, manReq)
	if manRec.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200", manRec.Code)
	}
	var freshManifest PMTilesManifest
	if err := json.Unmarshal(manRec.Body.Bytes(), &freshManifest); err != nil {
		t.Fatalf("unmarshal fresh manifest: %v", err)
	}
	if !freshManifest.Archives[PMTilesBGCArchiveName].FallbackOverview || freshManifest.Archives[PMTilesOverviewArchiveName].MaxZoom != 6 {
		t.Fatalf("unexpected fresh manifest: %+v", freshManifest)
	}

	// 4. Full-res extraDirs resolution if staged on disk
	if info, err := os.Stat(filepath.Join(DefaultSpatialAssetsDir, PMTilesBGCArchiveName)); err == nil && info.Size() > 1024 {
		fullHandler := NewPMTilesAssetHandler(fallback, "", DefaultSpatialAssetsDir)
		fullReq := httptest.NewRequest(http.MethodGet, "/maps/bc-bgc-v13.pmtiles", nil)
		fullReq.Header.Set("Range", "bytes=0-126")
		fullRec := httptest.NewRecorder()
		fullHandler.ServeHTTP(fullRec, fullReq)
		if fullRec.Code != http.StatusPartialContent || fullRec.Header().Get("X-VPro-PMTiles-Source") != "extraDir" {
			t.Fatalf("expected extraDir full-res PMTiles 206, got code=%d src=%q", fullRec.Code, fullRec.Header().Get("X-VPro-PMTiles-Source"))
		}
		if fullRec.Body.Bytes()[101] != 10 {
			t.Fatalf("expected maxZoom=10 in bc-bgc-v13.pmtiles header, got %d", fullRec.Body.Bytes()[101])
		}
	}

	// 5. Reject path traversal & invalid basenames
	for _, badPath := range []string{
		"/maps/../Sample.db",
		"/maps/%2e%2e/Sample.db",
		"/maps/nested/bc-overview.pmtiles",
		"/maps/evil.txt",
	} {
		req := httptest.NewRequest(http.MethodGet, badPath, nil)
		rec := httptest.NewRecorder()
		freshHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Fatalf("traversal path %q was not rejected: got %d", badPath, rec.Code)
		}
	}

	// 6. Non-map route delegates to fallback asset handler
	appReq := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	appRec := httptest.NewRecorder()
	freshHandler.ServeHTTP(appRec, appReq)
	if !fallbackHit || appRec.Code != http.StatusOK {
		t.Fatalf("fallback handler was not invoked for /index.html")
	}
}

func TestWave3NAD83GeodesyAndBCGSMapSheets(t *testing.T) {
	cases := []struct {
		name     string
		lat, lon float64
		wantZone int
		wantMap  string
	}{
		{"Vancouver Downtown", 49.26, -123.12, 10, "092G025"},
		{"Bowron 093H015", 53.15, -121.10, 10, "093H015"},
		{"Prince George", 53.9171, -122.7497, 10, "093G097"},
		{"Victoria", 48.4284, -123.3656, 10, "092B044"},
		{"Smithers", 54.7804, -127.1743, 9, "093L075"},
		{"Atlin Zone 8", 59.5775, -133.6953, 8, "104N052"},
		{"Cranbrook Zone 11", 49.5097, -115.7666, 11, "082G052"},
		{"Yukon Border Zone 7", 60.05, -139.50, 7, "115B003"},
	}

	for _, tc := range cases {
		z, e, n, err := ProjectNAD83LatLonToUTM(tc.lat, tc.lon)
		if err != nil {
			t.Fatalf("%s forward UTM error: %v", tc.name, err)
		}
		if z != tc.wantZone {
			t.Fatalf("%s zone = %d, want %d", tc.name, z, tc.wantZone)
		}
		rtLat, rtLon, err := ProjectNAD83UTMToLatLon(z, e, n)
		if err != nil {
			t.Fatalf("%s inverse UTM error: %v", tc.name, err)
		}
		errMeters := EllipsoidalDistanceNAD83(tc.lat, tc.lon, rtLat, rtLon)
		if errMeters >= 0.01 {
			t.Fatalf("%s round-trip error = %.6f m (want < 0.01 m)", tc.name, errMeters)
		}
		gotSheet := ComputeBCGSMapSheet(tc.lat, tc.lon)
		if gotSheet != tc.wantMap {
			t.Fatalf("%s ComputeBCGSMapSheet = %q, want %q", tc.name, gotSheet, tc.wantMap)
		}
	}

	// Positive-West longitude normalization & UTM divergence detection
	lat := 53.9171
	unsignedLon := 122.7497
	_, exactE, exactN, _ := ProjectNAD83LatLonToUTM(lat, -unsignedLon)
	zStr := "10"
	acc := 25
	cleanEval := EvaluatePlotGeodesy(PlotGeodesyInput{
		Latitude:         &lat,
		Longitude:        &unsignedLon,
		UTMZone:          &zStr,
		UTMEasting:       &exactE,
		UTMNorthing:      &exactN,
		LocationAccuracy: &acc,
	})
	if !cleanEval.LongitudeSignFlipped || cleanEval.NormalizedLon == nil || math.Abs(*cleanEval.NormalizedLon - -122.7497) > 1e-6 {
		t.Fatalf("expected positive-West longitude normalization, got %+v", cleanEval)
	}
	if cleanEval.DivergenceFlagged || cleanEval.DivergenceMeters == nil || *cleanEval.DivergenceMeters > 0.1 {
		t.Fatalf("expected <0.1m divergence for matching UTM, got %+v", cleanEval)
	}

	shiftedEast := exactE + 185.0
	divEval := EvaluatePlotGeodesy(PlotGeodesyInput{
		Latitude:         &lat,
		Longitude:        &unsignedLon,
		UTMZone:          &zStr,
		UTMEasting:       &shiftedEast,
		UTMNorthing:      &exactN,
		LocationAccuracy: &acc,
	})
	if !divEval.DivergenceFlagged || divEval.DivergenceMeters == nil || *divEval.DivergenceMeters < 150.0 {
		t.Fatalf("expected flagged >150m UTM divergence, got %+v", divEval)
	}
}

func TestWave3SuggestSpatialAttributesFullResAndEmbeddedOverview(t *testing.T) {
	// Test both full-resolution (z=10) and embedded-overview-only (z=6) attributors!
	attributors := []struct {
		label string
		attr  *PMTilesAttributor
	}{
		{"embedded-overview-z6", NewPMTilesAttributor("")},
	}
	if _, err := os.Stat(filepath.Join(DefaultSpatialAssetsDir, PMTilesBGCArchiveName)); err == nil {
		attributors = append(attributors, struct {
			label string
			attr  *PMTilesAttributor
		}{"full-res-z10", NewPMTilesAttributor("", DefaultSpatialAssetsDir)})
	}
	defer func() {
		for _, a := range attributors {
			_ = a.attr.Close()
		}
	}()

	benchmarks := []struct {
		city     string
		lat, lon float64
		wantZone string
	}{
		{"Prince George", 53.9171, -122.7497, "SBS"},
		{"Victoria", 48.4284, -123.3656, "CDF"},
		{"Smithers", 54.7804, -127.1743, "SBS"},
	}

	for _, a := range attributors {
		for _, bm := range benchmarks {
			sug, err := a.attr.SuggestSpatialAttributes(bm.lat, bm.lon)
			if err != nil {
				t.Fatalf("[%s] %s SuggestSpatialAttributes error: %v", a.label, bm.city, err)
			}
			if sug.Zone != bm.wantZone {
				t.Fatalf("[%s] %s Zone = %q (GISBGC=%q), want %q", a.label, bm.city, sug.Zone, sug.GISBGC, bm.wantZone)
			}
			if sug.GISBGC == "" || sug.Ecosection == "" || len(sug.FSRegionDistrict) != 7 || !strings.Contains(sug.FSRegionDistrict, ".") {
				t.Fatalf("[%s] %s incomplete admin/BGC suggestion: %+v", a.label, bm.city, sug)
			}
			if sug.ProvinceStateTerritory != "BC" || sug.ExpectedElevationMaxM == nil {
				t.Fatalf("[%s] %s expected Province=BC and elevation bounds, got %+v", a.label, bm.city, sug)
			}
		}
	}
}

func TestWave3FlatGeobufWorkbenchAuditStampAndAOIToSU(t *testing.T) {
	contexts, state := vprotest.ContextServiceFixture(t)

	// Seed clean BC coordinates on literal leading-zero plot '00337' (Prince George) and '108050' (Victoria)
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		UPDATE Sample_Env SET Latitude = 53.9171, Longitude = 122.7497, Zone = 'ICH', SubZone = 'vk2', Ecosection = NULL, FSRegionDistrict = NULL, NtsMapSheet = NULL, UTMZone = NULL, UTMEasting = NULL, UTMNorthing = NULL WHERE PlotNumber = '00337';
		UPDATE Sample_Admin SET GIS_BGC = NULL, GIS_BGC_VER = NULL WHERE Plot = '00337';
		UPDATE Sample_Env SET Latitude = 48.4284, Longitude = -123.3656, Zone = 'CDF', SubZone = 'mm', Elevation = 45 WHERE PlotNumber = '108050';
		UPDATE Sample_Admin SET GIS_BGC = NULL, GIS_BGC_VER = NULL WHERE Plot = '108050';
	`)

	ctx := context.Background()
	attr := NewPMTilesAttributor("", DefaultSpatialAssetsDir)
	defer attr.Close()

	// 1. Workbench GeoArrow Payload
	payload, err := GetSpatialWorkbenchPayload(ctx, contexts, state.ContextID, "", []string{DefaultSpatialAssetsDir}, attr)
	if err != nil {
		t.Fatalf("GetSpatialWorkbenchPayload: %v", err)
	}
	if payload.PlottableCount < 2 || payload.GeoArrow.Count != payload.PlottableCount {
		t.Fatalf("unexpected GeoArrow count: %+v", payload)
	}

	// 2. Binary OGC FlatGeobuf v3 round-trip
	fgbBytes, err := GetSpatialPlotsFlatGeobuf(ctx, contexts, state.ContextID, attr)
	if err != nil {
		t.Fatalf("GetSpatialPlotsFlatGeobuf: %v", err)
	}
	if len(fgbBytes) < 32 || !bytes.Equal(fgbBytes[:8], FlatGeobufMagicV3[:]) {
		t.Fatalf("invalid FlatGeobuf v3 header bytes: %x", fgbBytes[:min(8, len(fgbBytes))])
	}
	decoded, err := DecodeSpatialPlotsFlatGeobuf(fgbBytes)
	if err != nil {
		t.Fatalf("DecodeSpatialPlotsFlatGeobuf: %v", err)
	}
	if len(decoded) != payload.PlottableCount {
		t.Fatalf("decoded FlatGeobuf count = %d, want %d", len(decoded), payload.PlottableCount)
	}
	var found00337 *SpatialPlotRecord
	for i := range decoded {
		if decoded[i].PlotNumber == "00337" {
			found00337 = &decoded[i]
			break
		}
	}
	if found00337 == nil || math.Abs(found00337.Lon - -122.7497) > 1e-4 || !found00337.BGCMismatch {
		t.Fatalf("plot '00337' missing or inaccurate in FlatGeobuf round-trip: %+v", found00337)
	}

	// 3. BatchAuditSpatialQA + transactional ApplySpatialAttributes with <P>_Audit verification
	qa, err := BatchAuditSpatialQA(ctx, contexts, state.ContextID, attr)
	if err != nil {
		t.Fatalf("BatchAuditSpatialQA: %v", err)
	}
	if qa.UnsignedLonCount < 1 || qa.BGCMismatchCount < 1 || qa.MissingGISBGCCount < 2 {
		t.Fatalf("expected unsigned lon, BGC mismatch, and missing GIS_BGC in QA report: %+v", qa)
	}

	stampRes, err := ApplySpatialAttributes(ctx, contexts, state.ContextID, ApplySpatialAttributesRequest{
		PlotNumbers:            []string{"00337", "108050"},
		StampGISBGC:            true,
		StampAdminFields:       true,
		OnlyWhenBlank:          true,
		NormalizeLongitudeSign: true,
		SyncUTMFromLatLon:      true,
	}, attr)
	if err != nil {
		t.Fatalf("ApplySpatialAttributes: %v", err)
	}
	if stampRes.UpdatedPlots != 2 || stampRes.AuditRows < 6 {
		t.Fatalf("expected 2 updated plots and >=6 audit rows, got %+v", stampRes)
	}

	// Verify stored SQLite values and <P>_Audit rows for '00337'
	db, err := sql.Open("sqlite3", state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var lon float64
	var utmZone, eco, dist, mapSheet, gisBGC string
	var gisVer int
	if err := db.QueryRow(`SELECT e.Longitude, e.UTMZone, e.Ecosection, e.FSRegionDistrict, e.NtsMapSheet, a.GIS_BGC, a.GIS_BGC_VER
		FROM Sample_Env e JOIN Sample_Admin a ON CAST(e.PlotNumber AS BLOB)=CAST(a.Plot AS BLOB)
		WHERE CAST(e.PlotNumber AS BLOB)=CAST('00337' AS BLOB)`).Scan(&lon, &utmZone, &eco, &dist, &mapSheet, &gisBGC, &gisVer); err != nil {
		t.Fatalf("query stamped 00337: %v", err)
	}
	if lon >= 0 || utmZone != "10" || eco == "" || len(dist) != 7 || !strings.HasPrefix(gisBGC, "SBS") || gisVer != 13 || !strings.HasPrefix(mapSheet, "093G") {
		t.Fatalf("unexpected stamped values on 00337: lon=%v utm=%q eco=%q dist=%q sheet=%q gis=%q ver=%d", lon, utmZone, eco, dist, mapSheet, gisBGC, gisVer)
	}
	var auditCnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE CAST(PlotNumber AS BLOB)=CAST('00337' AS BLOB) AND EditField IN ('GIS_BGC','GIS_BGC_VER','Ecosection','FSRegionDistrict','Longitude','UTMZone')`).Scan(&auditCnt); err != nil {
		t.Fatal(err)
	}
	if auditCnt < 6 {
		t.Fatalf("expected >=6 audit rows on '00337', got %d", auditCnt)
	}

	// 4. Polygon AOI selection around Prince George (-123.5.. -122.0, 53.5.. 54.5) -> AssignSpatialAOIToSU
	pgPoly := [][2]float64{
		{-123.5, 53.5},
		{-122.0, 53.5},
		{-122.0, 54.5},
		{-123.5, 54.5},
	}
	aoiSel, err := SelectPlotsInPolygonAOI(ctx, contexts, state.ContextID, SpatialAOISelectionRequest{Polygon: pgPoly}, attr)
	if err != nil {
		t.Fatalf("SelectPlotsInPolygonAOI: %v", err)
	}
	foundInAOI := false
	for _, p := range aoiSel.PlotNumbers {
		if p == "00337" {
			foundInAOI = true
		}
		if p == "108050" {
			t.Fatalf("Victoria plot 108050 should not match Prince George AOI")
		}
	}
	if !foundInAOI {
		t.Fatalf("expected plot '00337' inside Prince George AOI, got %v", aoiSel.PlotNumbers)
	}

	suAssign, err := AssignSpatialAOIToSU(ctx, contexts, state.ContextID, AssignSpatialAOIToSURequest{
		Mode:     "create_project_su",
		SUName:   "PG_AOI_Wave3",
		SiteUnit: "SBSmk1/01",
		Polygon:  pgPoly,
		Activate: true,
	}, attr)
	if err != nil {
		t.Fatalf("AssignSpatialAOIToSU create_project_su: %v", err)
	}
	if !suAssign.Activated || suAssign.NewState == nil || suAssign.TableName != "PG_AOI_Wave3_SU" {
		t.Fatalf("unexpected SU create result: %+v", suAssign)
	}

	// Also test update_active on the newly activated SU table!
	vicPoly := [][2]float64{
		{-124.0, 48.0},
		{-123.0, 48.0},
		{-123.0, 49.0},
		{-124.0, 49.0},
	}
	// Note: once PG_AOI_Wave3_SU is active, scoped plots are restricted to PG_AOI_Wave3_SU, so update '00337' inside active SU!
	updRes, err := AssignSpatialAOIToSU(ctx, contexts, suAssign.ContextID, AssignSpatialAOIToSURequest{
		Mode:        "update_active",
		SiteUnit:    "SBSmk1/05",
		PlotNumbers: []string{"00337"},
	}, attr)
	if err != nil {
		t.Fatalf("AssignSpatialAOIToSU update_active: %v", err)
	}
	if updRes.ChangedRows != 1 {
		t.Fatalf("expected 1 updated row in active SU, got %+v", updRes)
	}
	_ = vicPoly
	_ = io.EOF
}

func TestWave3SharedFlatGeobufWireFixtureMatchesGoEncoder(t *testing.T) {
	uLat, uLon, div, elev, slope, aspect := 53.9182, -122.7512, 146.4, 615.0, 12.5, 185.0
	input := []SpatialPlotRecord{
		{
			PlotNumber:       "00337",
			Lat:              53.9171,
			Lon:              -122.7497,
			UTMDerivedLat:    &uLat,
			UTMDerivedLon:    &uLon,
			DivergenceMeters: &div,
			Elevation:        &elev,
			Slope:            &slope,
			Aspect:           &aspect,
			FieldBGC:         "ICHvk2",
			GISBGC:           "SBSmk1",
			BGCMismatch:      true,
			SiteUnit:         "SBSmk1/01",
		},
		{
			PlotNumber:  "108050",
			Lat:         48.4284,
			Lon:         -123.3656,
			FieldBGC:    "CDFmm",
			GISBGC:      "CDFmm",
			BGCMismatch: false,
			SiteUnit:    "CDFmm/01",
		},
	}
	encoded, err := EncodeSpatialPlotsFlatGeobuf("Sample_Plots", input)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(encoded)
	fixturePath := resources.Path("testdata", "wave3-plots-flatgeobuf.b64")
	rawOnDisk, err := os.ReadFile(fixturePath)
	if os.IsNotExist(err) {
		if wErr := os.WriteFile(fixturePath, []byte(b64+"\n"), 0644); wErr != nil {
			t.Fatal(wErr)
		}
		rawOnDisk = []byte(b64)
	} else if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(rawOnDisk)) != b64 {
		t.Fatalf("shared FlatGeobuf wire fixture drifted from Go encoder")
	}
	roundTrip, err := DecodeSpatialPlotsFlatGeobuf(encoded)
	if err != nil || len(roundTrip) != 2 || roundTrip[0].PlotNumber != "00337" || !roundTrip[0].BGCMismatch || roundTrip[1].SiteUnit != "CDFmm/01" {
		t.Fatalf("Go round-trip mismatch: %+v (%v)", roundTrip, err)
	}
}
