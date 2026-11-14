package interchange_test

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/interchange"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestProjectRDSExportVerifiedWithNativeRscript(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	ctx := context.Background()

	// Seed rich UTF-8, NULLs, booleans (-1 and 0), integers, and exact floats in Sample
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		UPDATE Sample_Env
		SET Location='Vallée du Fraser — Parcelle 108050',
		    Elevation=1245,
		    SlopeGradient=32.5,
		    SpeciesListComplete=-1,
		    Flag=0,
		    AirPhotoNum=NULL
		WHERE PlotNumber='108050';
		DELETE FROM Sample_Veg WHERE PlotNumber='108050';
		INSERT INTO Sample_Veg (PlotNumber, ID, Species, Cover1, Height1, Cover6, Height6, Cover10, Collected)
		VALUES
			('108050', 501, 'PSEUMEN', 42.5, 28.4, NULL, NULL, NULL, 'C'),
			('108050', 502, 'LINNBOR', NULL, NULL, 12.25, 0.15, NULL, NULL),
			('108050', 503, 'ALECTOR', NULL, NULL, NULL, NULL, 3.75, 'V');
	`)

	rdsPath := filepath.Join(t.TempDir(), "sample_export.rds")
	outcome, err := interchange.ExportProjectRDS(service, ctx, state.ContextID, interchange.RDSExportRequest{
		DestinationPath: rdsPath,
	})
	if err != nil {
		t.Fatalf("ExportProjectRDS failed: %v", err)
	}
	if outcome.ByteSize == 0 || len(outcome.Tables) != 7 {
		t.Fatalf("unexpected RDS outcome: %+v", outcome)
	}

	rscriptPath, lookErr := exec.LookPath("Rscript")
	if lookErr != nil {
		t.Skip("Rscript not found on PATH")
	}

	rCode := `
args <- commandArgs(trailingOnly = TRUE)
obj <- readRDS(args[[1]])
stopifnot(is.list(obj))
expected_names <- c("env", "admin", "veg_long", "humus", "mineral", "su", "metadata")
stopifnot(identical(names(obj), expected_names))
for (nm in expected_names) {
  stopifnot(is.data.frame(obj[[nm]]))
}
env <- obj$env
stopifnot(nrow(env) >= 1L)
row1 <- env[env$PlotNumber == "108050", , drop = FALSE]
stopifnot(nrow(row1) == 1L)
stopifnot(identical(row1$Location[[1]], "Vallée du Fraser — Parcelle 108050"))
stopifnot(is.integer(env$Elevation), identical(row1$Elevation[[1]], 1245L))
stopifnot(is.double(env$SlopeGradient), abs(row1$SlopeGradient[[1]] - 32.5) < 1e-6)
stopifnot(is.logical(env$SpeciesListComplete), isTRUE(row1$SpeciesListComplete[[1]]))
stopifnot(is.logical(env$Flag), identical(row1$Flag[[1]], FALSE))
stopifnot(is.na(row1$AirPhotoNum[[1]]))

veg <- obj$veg_long
stopifnot(identical(colnames(veg), c("PlotNumber","SiteUnit","ID","Species","ScientificName","Lifeform","Layer","Cover","Height","Collected")))
v_plot <- veg[veg$PlotNumber == "108050", , drop = FALSE]
stopifnot(nrow(v_plot) == 3L)
stopifnot(any(v_plot$Species == "PSEUMEN" & v_plot$Layer == "A1" & abs(v_plot$Cover - 42.5) < 1e-6))
stopifnot(any(v_plot$Species == "LINNBOR" & v_plot$Layer == "C" & is.na(v_plot$Collected)))
stopifnot(any(v_plot$Species == "ALECTOR" & v_plot$Layer == "Ep" & abs(v_plot$Cover - 3.75) < 1e-6))
cat("RDS_VERIFIED_OK\n")
`
	cmd := exec.CommandContext(ctx, rscriptPath, "--vanilla", "-e", rCode, outcome.Path)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("Rscript readRDS verification failed: %v\nOutput:\n%s", runErr, string(out))
	}
	if !strings.Contains(string(out), "RDS_VERIFIED_OK") {
		t.Fatalf("unexpected Rscript output: %s", string(out))
	}
}

func TestLongAndWideMatrixCSVExports(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	ctx := context.Background()

	vprotest.MutateContextFixture(t, state.ProjectPath, `
		DELETE FROM Sample_Veg;
		INSERT INTO Sample_Veg (PlotNumber, ID, Species, Cover1, Height1, Cover5a, Cover6, Height6, Collected)
		VALUES
			('108050', 601, 'PSEUMEN', 55.5, 31.2, 4.5, NULL, NULL, 'C'),
			('108050', 602, 'TSUGHET', 22.0, 24.0, NULL, 3.25, 0.4, NULL);
	`)

	longPath := filepath.Join(t.TempDir(), "long_tidy.csv")
	longOut, err := interchange.ExportLongTidyCSV(service, ctx, state.ContextID, interchange.LongTidyCSVRequest{
		DestinationPath: longPath,
	})
	if err != nil {
		t.Fatalf("ExportLongTidyCSV: %v", err)
	}
	if longOut.RowCount != 4 || longOut.ColumnCount != 9 {
		t.Fatalf("expected 4 long observations and 9 columns, got %+v", longOut)
	}
	rawLong, err := os.ReadFile(longPath)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(strings.NewReader(string(rawLong))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(recs[0], ",") != "PlotNumber,SiteUnit,Layer,Species,ScientificName,Lifeform,Cover,Height,Collected" {
		t.Fatalf("unexpected Long CSV header: %v", recs[0])
	}

	// Wide matrix in species_layer mode and rollup_all mode
	wideLayerPath := filepath.Join(t.TempDir(), "wide_layer.csv")
	wideLayerOut, err := interchange.ExportWideMatrixCSV(service, ctx, state.ContextID, interchange.WideMatrixCSVRequest{
		DestinationPath: wideLayerPath,
		LayerMode:       "species_layer",
		ZeroFill:        true,
		IncludeSiteUnit: true,
	})
	if err != nil {
		t.Fatalf("ExportWideMatrixCSV(species_layer): %v", err)
	}
	if wideLayerOut.ColumnCount != 2+4 { // PlotNumber, SiteUnit + PSEUMEN_A1, PSEUMEN_B2a, TSUGHET_A1, TSUGHET_C
		t.Fatalf("expected 6 columns in species_layer wide CSV, got %+v", wideLayerOut)
	}

	wideRollupOut, err := interchange.ExportWideMatrixCSV(service, ctx, state.ContextID, interchange.WideMatrixCSVRequest{
		LayerMode:       "rollup_all",
		ZeroFill:        false,
		IncludeSiteUnit: false,
	})
	if err != nil {
		t.Fatalf("ExportWideMatrixCSV(rollup_all): %v", err)
	}
	if !strings.Contains(wideRollupOut.PreviewCSV, "60") { // PSEUMEN 55.5 + 4.5 = 60
		t.Fatalf("expected summed PSEUMEN cover 60 in rollup_all preview, got:\n%s", wideRollupOut.PreviewCSV)
	}
}

func TestUserSpeciesExportAndCollisionImport(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Import initial provisional species
	sciA := "Carex provisoria sp. nov."
	lf6 := 6
	initJSON, _ := json.Marshal([]interchange.UserSpeciesRecord{
		{Code: "CAREPRV", ScientificName: &sciA, LifeForm: &lf6},
	})
	initRes, err := interchange.ImportUserSpeciesList(service, ctx, state.ContextID, interchange.UserSpeciesImportRequest{
		Format:          "json",
		Content:         string(initJSON),
		CollisionPolicy: interchange.CollisionSkip,
	})
	if err != nil || initRes.InsertedCount != 1 {
		t.Fatalf("initial user species import failed: %+v, err=%v", initRes, err)
	}

	// Export as JSON manifest and verify SHA-256 tamper protection
	jsonPath := filepath.Join(t.TempDir(), "crew_species.json")
	expJSON, err := interchange.ExportUserSpeciesList(service, ctx, state.ContextID, interchange.UserSpeciesExportRequest{
		Format:          "json",
		DestinationPath: jsonPath,
	})
	if err != nil || expJSON.RowCount < 1 || expJSON.PayloadSHA256 == "" {
		t.Fatalf("ExportUserSpeciesList(json) failed: %+v, err=%v", expJSON, err)
	}
	tampered := strings.Replace(expJSON.Content, "Carex provisoria", "Carex tampered", 1)
	if _, err := interchange.PreviewUserSpeciesImport(service, ctx, state.ContextID, interchange.UserSpeciesImportRequest{
		Format:  "json",
		Content: tampered,
	}); err == nil {
		t.Fatal("expected tampered manifest SHA-256 to be rejected")
	}

	// Export as CSV as well
	expCSV, err := interchange.ExportUserSpeciesList(service, ctx, state.ContextID, interchange.UserSpeciesExportRequest{
		Format: "csv",
	})
	if err != nil || !strings.Contains(expCSV.Content, "CAREPRV") {
		t.Fatalf("ExportUserSpeciesList(csv) failed: %+v, err=%v", expCSV, err)
	}

	// Test collision review with:
	// 1. CAREPRV (existing user collision)
	// 2. PSEUMEN (master VLists collision -> always skipped!)
	// 3. SALIPRV (brand-new user species)
	csvIncoming := "Code,ScientificName,LifeForm,EnglishName\nCAREPRV,Carex updated,6,Updated sedge\nPSEUMEN,Pseudotsuga hack,1,Douglas-fir\nSALIPRV,Salix provisoria,4,Crew willow\n"

	skipRes, err := interchange.ImportUserSpeciesList(service, ctx, state.ContextID, interchange.UserSpeciesImportRequest{
		Format:          "csv",
		Content:         csvIncoming,
		CollisionPolicy: interchange.CollisionSkip,
	})
	if err != nil {
		t.Fatalf("ImportUserSpeciesList(skip): %v", err)
	}
	if skipRes.InsertedCount != 1 || skipRes.OverwrittenCount != 0 || skipRes.SkippedCount != 2 {
		t.Fatalf("unexpected skip counts: %+v", skipRes)
	}

	overwriteRes, err := interchange.ImportUserSpeciesList(service, ctx, state.ContextID, interchange.UserSpeciesImportRequest{
		Format:          "csv",
		Content:         csvIncoming,
		CollisionPolicy: interchange.CollisionOverwrite,
	})
	if err != nil {
		t.Fatalf("ImportUserSpeciesList(overwrite): %v", err)
	}
	if overwriteRes.InsertedCount != 0 || overwriteRes.OverwrittenCount != 2 || overwriteRes.SkippedCount != 1 {
		t.Fatalf("expected 2 overwrites (CAREPRV + SALIPRV) and 1 master skip (PSEUMEN), got %+v", overwriteRes)
	}
}

func TestTurboVegDualXMLAndSQLiteRoundTrip(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Seed sub-layers including Cover1+Height1, CHARS Cover5a, Cover6, simultaneous Cover7+Cover8+Cover9
	// on the SAME species (HYLOSPL) in the SAME plot (testing TV layer 9=ml disambiguation on observationsspecies_index02),
	// Cover10+Collected, 8-char species code PINUCON1, coordinates, BEC header fields, and an optional *_Lump table.
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		UPDATE Sample_Env
		SET Latitude=54.375, Longitude=-126.875, Elevation=1120, SlopeGradient=21.5, Aspect=195,
		    "Date"='2025-07-14', "Zone"='SBS', SubZone='dk', SiteSeries='01',
		    MoistureRegime='4', NutrientRegime='C', SiteSurveyor='W. MacKenzie',
		    Location='Babine Lake Ridge'
		WHERE PlotNumber='108050';
		UPDATE Sample_Admin SET UserSiteUnit='SBSdk/01', PlotSize=400.0 WHERE Plot='108050';
		DELETE FROM Sample_Veg WHERE PlotNumber='108050';
		INSERT INTO Sample_Veg (PlotNumber, ID, Species, Cover1, Height1, Cover5a, Cover6, Cover7, Cover8, Cover9, Cover10, Collected)
		VALUES
			('108050', 701, 'PSEUMEN', 37.25, 29.5, NULL, NULL, NULL, NULL, NULL, NULL, 'C'),
			('108050', 702, 'VACCMEM', NULL, NULL, 14.75, 0.35, NULL, NULL, NULL, NULL, NULL),
			('108050', 703, 'HYLOSPL', NULL, NULL, NULL, NULL, 68.5, 11.25, 4.5, NULL, NULL),
			('108050', 704, 'ALECTOR', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1.25, 'V'),
			('108050', 705, 'PINUCON1', 12.0, 19.0, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
		CREATE TABLE Tvlump_Lump (LumpCode TEXT, SppCode TEXT, "Use" INTEGER);
		INSERT INTO Tvlump_Lump (LumpCode, SppCode, "Use") VALUES ('PINUCON_L', 'PINUCON1', -1);
	`)

	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "export_tv.xml")
	sqlitePath := filepath.Join(dir, "export_tv3.sqlite")

	xmlExp, err := interchange.ExportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegExportRequest{
		Format:          interchange.TurboVegFormatXML,
		DestinationPath: xmlPath,
	})
	if err != nil {
		t.Fatalf("ExportTurboVeg(xml): %v", err)
	}
	sqliteExp, err := interchange.ExportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegExportRequest{
		Format:          interchange.TurboVegFormatSQLite,
		DestinationPath: sqlitePath,
	})
	if err != nil {
		t.Fatalf("ExportTurboVeg(sqlite): %v", err)
	}
	if xmlExp.PlotCount != sqliteExp.PlotCount || xmlExp.ObservationCount != sqliteExp.ObservationCount {
		t.Fatalf("XML vs SQLite export counts differ: xml=%+v sqlite=%+v", xmlExp, sqliteExp)
	}

	// 1. Verify official Alterra 31-table, 36-index, 37-FK Turboveg 3 SQLite schema & zero foreign_key_check violations
	expected31Tables := []string{
		"associa", "covercodes", "custodians", "datasetgroups", "datasets",
		"datatypes", "fielddomains", "fieldgroups", "fields", "locations",
		"lookupcaptions", "lookupdata", "lookuptables", "metadata", "pictures",
		"plotobservationheaderdata", "plotobservations", "plots", "profiledata",
		"profiles", "projectdatasets", "projects", "speciesobservationdata",
		"speciesobservations", "taxa", "taxongroups", "taxonlists", "taxonranks",
		"traitdata", "units", "vegetationlayers",
	}
	tvDB, err := sqlutil.OpenReadOnly(sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tbl := range expected31Tables {
		var exists bool
		if err := tvDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, tbl).Scan(&exists); err != nil || !exists {
			tvDB.Close()
			t.Fatalf("TurboVeg3 SQLite missing official Alterra table %s (err=%v)", tbl, err)
		}
	}
	var totalTableCount, namedIdxCount int
	if err := tvDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table'`).Scan(&totalTableCount); err != nil || totalTableCount != 31 {
		tvDB.Close()
		t.Fatalf("expected exactly 31 tables in Turboveg 3 SQLite, got %d (err=%v)", totalTableCount, err)
	}
	if err := tvDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_autoindex_%'`).Scan(&namedIdxCount); err != nil || namedIdxCount != 36 {
		tvDB.Close()
		t.Fatalf("expected 36 named Alterra indexes, got %d (err=%v)", namedIdxCount, err)
	}
	totalFKs := 0
	for _, tbl := range expected31Tables {
		fkRows, fkErr := tvDB.Query(`PRAGMA foreign_key_list(` + tbl + `)`)
		if fkErr != nil {
			tvDB.Close()
			t.Fatal(fkErr)
		}
		for fkRows.Next() {
			totalFKs++
		}
		fkRows.Close()
	}
	if totalFKs != 37 {
		tvDB.Close()
		t.Fatalf("expected 37 Alterra foreign keys, got %d", totalFKs)
	}
	fkCheckRows, err := tvDB.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		tvDB.Close()
		t.Fatal(err)
	}
	if fkCheckRows.Next() {
		fkCheckRows.Close()
		tvDB.Close()
		t.Fatal("exported Turboveg 3 SQLite failed PRAGMA foreign_key_check")
	}
	fkCheckRows.Close()

	// Verify 8-char PINUCON1 truncated safely to 7-char lettercode 'PINUCON' while preserving vpro_code in metadata
	var letterCode, taxaMeta string
	if err := tvDB.QueryRow(`SELECT lettercode, metadata FROM taxa WHERE scientificname = 'Pinus contorta var. contorta'`).Scan(&letterCode, &taxaMeta); err != nil {
		tvDB.Close()
		t.Fatalf("expected PINUCON1 row in taxa: %v", err)
	}
	if letterCode != "PINUCON" || !strings.Contains(taxaMeta, "PINUCON1") {
		tvDB.Close()
		t.Fatalf("expected lettercode='PINUCON' and metadata with PINUCON1, got lettercode=%q metadata=%q", letterCode, taxaMeta)
	}
	// Verify all 3 HYLOSPL bryoid sublayers (D, Dw, Dr) exported to canonical TV layer 9 (ml)
	var mlCount int
	if err := tvDB.QueryRow(`
		SELECT COUNT(*) FROM speciesobservations so
		INNER JOIN taxa t ON t.id = so.taxonid
		WHERE t.scientificname LIKE 'Hylocomium splendens%' AND so.vegetationlayerid = 9`).Scan(&mlCount); err != nil || mlCount < 3 {
		tvDB.Close()
		t.Fatalf("expected >=3 HYLOSPL observations in TV layer 9 (ml), got %d (err=%v)", mlCount, err)
	}
	tvDB.Close()

	// 2. Verify exported <Plot_package> XML structure & external R xml2 + RSQLite smoke check (mimicking vegdata::tv.readXML & phylia.io.TvXml)
	rawXMLBytes, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatal(err)
	}
	rawXMLStr := string(rawXMLBytes)
	for _, needle := range []string{
		"<Plot_package", "<Plots>", "<Plot ", "<header_data>", "<standard_record ",
		"<species_data>", "<species>", "<Lookup_tables>", "<Species_list>",
		"<Coverscale_list>", "<Template>", "<Files>", "<Dbasedic>",
	} {
		if !strings.Contains(rawXMLStr, needle) {
			t.Fatalf("exported TurboVeg XML missing official <Plot_package> marker %q", needle)
		}
	}
	if strings.Contains(rawXMLStr, "<Turboveg ") {
		t.Fatal("exported XML must not emit bespoke <Turboveg> root")
	}

	if rscriptPath, lookErr := exec.LookPath("Rscript"); lookErr == nil {
		rSmoke := `
args <- commandArgs(trailingOnly = TRUE)
xml_path <- args[[1]]
sqlite_path <- args[[2]]
if (requireNamespace("xml2", quietly = TRUE)) {
  veg <- xml2::read_xml(xml_path)
  stopifnot(identical(xml2::xml_name(veg), "Plot_package"))
  plots_node <- xml2::xml_find_all(veg, "//Plots")
  plot_children <- xml2::xml_children(plots_node)
  stopifnot(length(plot_children) >= 1L)
  rel_nrs <- as.integer(xml2::xml_attr(plot_children, "releve_nr"))
  stopifnot(!anyNA(rel_nrs), !any(duplicated(rel_nrs)))
  hdr_std <- xml2::xml_find_all(plots_node, "//header_data/standard_record")
  stopifnot(length(hdr_std) == length(plot_children))
  spp_std <- xml2::xml_find_all(plots_node, "//species_data/species/standard_record")
  stopifnot(length(spp_std) >= 7L)
  spp_list <- xml2::xml_find_all(veg, "//Lookup_tables/Species_list/*")
  stopifnot(length(spp_list) >= 5L)
  dbasedic <- xml2::xml_find_all(veg, "//Template/Dbasedic/record")
  stopifnot(length(dbasedic) >= 10L)
}
if (requireNamespace("DBI", quietly = TRUE) && requireNamespace("RSQLite", quietly = TRUE)) {
  con <- DBI::dbConnect(RSQLite::SQLite(), sqlite_path)
  on.exit(DBI::dbDisconnect(con), add = TRUE)
  DBI::dbExecute(con, "PRAGMA foreign_keys = ON")
  fk <- DBI::dbGetQuery(con, "PRAGMA foreign_key_check")
  stopifnot(nrow(fk) == 0L)
  tbls <- DBI::dbListTables(con)
  stopifnot(length(tbls) == 31L)
}
cat("TV3_EXTERNAL_SMOKE_OK\n")
`
		out, runErr := exec.CommandContext(ctx, rscriptPath, "--vanilla", "-e", rSmoke, xmlPath, sqlitePath).CombinedOutput()
		if runErr != nil || !strings.Contains(string(out), "TV3_EXTERNAL_SMOKE_OK") {
			t.Fatalf("Rscript vegdata/RSQLite smoke check failed: %v\n%s", runErr, string(out))
		}
	}

	// 3. Re-import <Plot_package> XML -> VP08 .db and 31-table SQLite -> VP08 .db and verify lossless round-trip
	impXMLDest := filepath.Join(dir, "FromTVXML.db")
	impXML, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      xmlPath,
		Format:          interchange.TurboVegFormatXML,
		TargetProject:   "FromTVXML",
		DestinationPath: impXMLDest,
	})
	if err != nil {
		t.Fatalf("ImportTurboVeg(xml): %v", err)
	}
	impSQLDest := filepath.Join(dir, "FromTVSQL.db")
	impSQL, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      sqlitePath,
		Format:          interchange.TurboVegFormatSQLite,
		TargetProject:   "FromTVSQL",
		DestinationPath: impSQLDest,
	})
	if err != nil {
		t.Fatalf("ImportTurboVeg(sqlite): %v", err)
	}
	if impXML.PlotCount != xmlExp.PlotCount || impSQL.PlotCount != sqliteExp.PlotCount ||
		impXML.SpeciesCount != xmlExp.SpeciesCount || impSQL.SpeciesCount != sqliteExp.SpeciesCount ||
		impXML.ObservationCount != xmlExp.ObservationCount || impSQL.ObservationCount != sqliteExp.ObservationCount {
		t.Fatalf("round-trip counts mismatch: xmlExp=%+v sqliteExp=%+v impXML=%+v impSQL=%+v", xmlExp, sqliteExp, impXML, impSQL)
	}

	for _, check := range []struct {
		path string
		proj string
	}{
		{impXMLDest, "FromTVXML"},
		{impSQLDest, "FromTVSQL"},
	} {
		db, err := sqlutil.OpenReadOnly(check.path)
		if err != nil {
			t.Fatal(err)
		}
		var zeroCov float64
		if err := db.QueryRow(`SELECT Cover6 FROM `+check.proj+`_Veg WHERE typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST('00337' AS BLOB) AND Species='ABIE_RK'`).Scan(&zeroCov); err != nil || zeroCov != 0 {
			db.Close()
			t.Fatalf("verify %s presence-only zero-cover row (00337 / ABIE_RK): cov=%v err=%v", check.proj, zeroCov, err)
		}
		var c1, h1 float64
		var pColl string
		if err := db.QueryRow(`SELECT Cover1, Height1, Collected FROM `+check.proj+`_Veg WHERE PlotNumber='108050' AND Species='PSEUMEN'`).Scan(&c1, &h1, &pColl); err != nil {
			db.Close()
			t.Fatalf("verify %s PSEUMEN: %v", check.proj, err)
		}
		if math.Abs(c1-37.25) > 1e-4 || math.Abs(h1-29.5) > 1e-4 || pColl != "C" {
			db.Close()
			t.Fatalf("%s PSEUMEN mismatch: c1=%v h1=%v coll=%q", check.proj, c1, h1, pColl)
		}
		var c5a, c6 float64
		if err := db.QueryRow(`SELECT Cover5a, Cover6 FROM `+check.proj+`_Veg WHERE PlotNumber='108050' AND Species='VACCMEM'`).Scan(&c5a, &c6); err != nil {
			db.Close()
			t.Fatalf("verify %s VACCMEM covers: %v", check.proj, err)
		}
		var c7, c8, c9 float64
		if err := db.QueryRow(`SELECT Cover7, Cover8, Cover9 FROM `+check.proj+`_Veg WHERE PlotNumber='108050' AND Species='HYLOSPL'`).Scan(&c7, &c8, &c9); err != nil {
			db.Close()
			t.Fatalf("verify %s HYLOSPL simultaneous Cover7/8/9: %v", check.proj, err)
		}
		if math.Abs(c7-68.5) > 1e-4 || math.Abs(c8-11.25) > 1e-4 || math.Abs(c9-4.5) > 1e-4 {
			db.Close()
			t.Fatalf("%s HYLOSPL D/Dw/Dr round-trip collision: c7=%v c8=%v c9=%v", check.proj, c7, c8, c9)
		}
		var c10 float64
		var coll string
		if err := db.QueryRow(`SELECT Cover10, Collected FROM `+check.proj+`_Veg WHERE PlotNumber='108050' AND Species='ALECTOR'`).Scan(&c10, &coll); err != nil {
			db.Close()
			t.Fatalf("verify %s ALECTOR cover10: %v", check.proj, err)
		}
		var pinuCov float64
		if err := db.QueryRow(`SELECT Cover1 FROM `+check.proj+`_Veg WHERE PlotNumber='108050' AND Species='PINUCON1'`).Scan(&pinuCov); err != nil || math.Abs(pinuCov-12.0) > 1e-4 {
			db.Close()
			t.Fatalf("%s 8-char PINUCON1 round-trip failed: cov=%v err=%v", check.proj, pinuCov, err)
		}
		var lat, lon float64
		var zone, subzone, ss, su string
		if err := db.QueryRow(`
			SELECT e.Latitude, e.Longitude, e."Zone", e.SubZone, e.SiteSeries, s.SiteUnit
			FROM `+check.proj+`_Env e
			INNER JOIN `+check.proj+`_SU s ON s.PlotNumber = e.PlotNumber
			WHERE e.PlotNumber='108050'`).Scan(&lat, &lon, &zone, &subzone, &ss, &su); err != nil {
			db.Close()
			t.Fatalf("verify %s Env+SU: %v", check.proj, err)
		}
		db.Close()
		if math.Abs(c5a-14.75) > 1e-4 || math.Abs(c6-0.35) > 1e-4 || math.Abs(c10-1.25) > 1e-4 || coll != "V" {
			t.Fatalf("%s round-trip cover mismatch: c5a=%v c6=%v c10=%v coll=%q", check.proj, c5a, c6, c10, coll)
		}
		if math.Abs(lat-54.375) > 1e-5 || math.Abs(lon - -126.875) > 1e-5 || zone != "SBS" || subzone != "dk" || ss != "01" || su != "SBSdk/01" {
			t.Fatalf("%s Env/SU mismatch: lat=%v lon=%v %s/%s/%s su=%q", check.proj, lat, lon, zone, subzone, ss, su)
		}
	}

	// 4. Verify LumpTable export option maps PINUCON1 -> PINUCON_L
	lumpXMLPath := filepath.Join(dir, "export_lumped.xml")
	if _, err := interchange.ExportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegExportRequest{
		Format:          interchange.TurboVegFormatXML,
		DestinationPath: lumpXMLPath,
		LumpTable:       "Tvlump",
	}); err != nil {
		t.Fatalf("ExportTurboVeg with LumpTable failed: %v", err)
	}
	lumpBytes, _ := os.ReadFile(lumpXMLPath)
	if !strings.Contains(string(lumpBytes), `vpro_code="PINUCON_L"`) {
		t.Fatalf("expected LumpTable export to emit PINUCON_L in <Plot_package>")
	}
}

func TestTurboVegExternalFixturesPlotPackageCP1252AndMinimalTV3SQLite(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()
	dir := t.TempDir()

	// 1. External <Plot_package> XML fixture (windows-1252 encoding, trailing <Lookup_tables>, Braun-Blanquet scale "01", TV2 "null" literals, scientific name + synonym crosswalk)
	extXMLFixture := filepath.Join("internal", "interchange", "testdata", "turboveg3", "external_plot_package.xml")
	extXMLDest := filepath.Join(dir, "ExtXML.db")
	xmlOut, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      extXMLFixture,
		Format:          interchange.TurboVegFormatXML,
		TargetProject:   "ExtXML",
		DestinationPath: extXMLDest,
	})
	if err != nil {
		t.Fatalf("ImportTurboVeg(external_plot_package.xml) failed: %v", err)
	}
	if xmlOut.PlotCount != 2 {
		t.Fatalf("expected 2 plots from external_plot_package.xml, got %+v", xmlOut)
	}

	xdb, err := sqlutil.OpenReadOnly(extXMLDest)
	if err != nil {
		t.Fatal(err)
	}
	var loc101, date101, surv101 string
	var notes101 *string
	if err := xdb.QueryRow(`SELECT Location, CAST("Date" AS TEXT), SiteSurveyor, SiteNotes FROM ExtXML_Env WHERE PlotNumber='101'`).Scan(&loc101, &date101, &surv101, &notes101); err != nil {
		xdb.Close()
		t.Fatal(err)
	}
	// Verify CP1252 characters (é, –, ö), 8-digit TV2 date normalization, Author_list lookup, and TV2 "null" remarks stripped to NULL!
	if !strings.Contains(loc101, "Vallée") || !strings.Contains(loc101, "–") || !strings.Contains(loc101, "Görlitz") || date101 != "2024-07-19" || surv101 != "J. Pojar" || notes101 != nil {
		xdb.Close()
		t.Fatalf("CP1252 / TV2 header decode mismatch: loc=%q date=%q surv=%q notes=%v", loc101, date101, surv101, notes101)
	}
	// Plot 101: Pseudotsuga menziesii (external code PSEDMEN -> resolved via scientific name to PSEUMEN! layer 1=42.5, layer 8=sl -> Cover6=0.5),
	// Vaccinium membranaceum (layer 4=15 -> Cover4, layer 7=jl woody -> Cover5=3.5), Hylocomium splendens (layer 9=ml -> Cover7=64)
	var pseuC1, pseuC6, vaccC4, vaccC5, hyloC7 float64
	if err := xdb.QueryRow(`SELECT Cover1, Cover6 FROM ExtXML_Veg WHERE PlotNumber='101' AND Species='PSEUMEN'`).Scan(&pseuC1, &pseuC6); err != nil {
		xdb.Close()
		t.Fatalf("expected external scientific name Pseudotsuga menziesii -> PSEUMEN: %v", err)
	}
	if err := xdb.QueryRow(`SELECT Cover4, Cover5 FROM ExtXML_Veg WHERE PlotNumber='101' AND Species='VACCMEM'`).Scan(&vaccC4, &vaccC5); err != nil {
		xdb.Close()
		t.Fatal(err)
	}
	if err := xdb.QueryRow(`SELECT Cover7 FROM ExtXML_Veg WHERE PlotNumber='101' AND Species='HYLOSPL'`).Scan(&hyloC7); err != nil {
		xdb.Close()
		t.Fatal(err)
	}
	if math.Abs(pseuC1-42.5) > 1e-4 || math.Abs(pseuC6-0.5) > 1e-4 || math.Abs(vaccC4-15.0) > 1e-4 || math.Abs(vaccC5-3.5) > 1e-4 || math.Abs(hyloC7-64.0) > 1e-4 {
		xdb.Close()
		t.Fatalf("Plot 101 layer crosswalk mismatch: pseu=(%v,%v) vacc=(%v,%v) hylo=%v", pseuC1, pseuC6, vaccC4, vaccC5, hyloC7)
	}
	// Plot 102: Braun-Blanquet scale "01": Pinus contorta var. contorta ('2a' -> 8.75, 8-char PINUCON1),
	// Chamaecyparis nootkatensis synonym -> CALLNOO ('3' -> 37.5), Cornus canadensis layer 7 (jl, herb -> Cover6, '+' -> 0.5)
	var pinuC1, callC2, cornC6, hylo102C7 float64
	if err := xdb.QueryRow(`SELECT Cover1 FROM ExtXML_Veg WHERE PlotNumber='102' AND Species='PINUCON1'`).Scan(&pinuC1); err != nil {
		xdb.Close()
		t.Fatalf("expected Pinus contorta var. contorta -> PINUCON1: %v", err)
	}
	if err := xdb.QueryRow(`SELECT Cover2 FROM ExtXML_Veg WHERE PlotNumber='102' AND Species='CALLNOO'`).Scan(&callC2); err != nil {
		xdb.Close()
		t.Fatalf("expected synonym Chamaecyparis nootkatensis -> CALLNOO: %v", err)
	}
	if err := xdb.QueryRow(`SELECT Cover6 FROM ExtXML_Veg WHERE PlotNumber='102' AND Species='CORNCAN'`).Scan(&cornC6); err != nil {
		xdb.Close()
		t.Fatal(err)
	}
	if err := xdb.QueryRow(`SELECT Cover7 FROM ExtXML_Veg WHERE PlotNumber='102' AND Species='HYLOSPL'`).Scan(&hylo102C7); err != nil {
		xdb.Close()
		t.Fatal(err)
	}
	xdb.Close()
	if math.Abs(pinuC1-8.75) > 1e-4 || math.Abs(callC2-37.5) > 1e-4 || math.Abs(cornC6-0.5) > 1e-4 || math.Abs(hylo102C7-0.1) > 1e-4 {
		t.Fatalf("Plot 102 Braun-Blanquet / synonym mismatch: pinu=%v call=%v corn=%v hylo=%v", pinuC1, callC2, cornC6, hylo102C7)
	}

	// 2. External 31-table Turboveg 3 SQLite fixture seeded from external_tv3_minimal.sql (WKT POINT, Delphi TDateTime 45492.0 = 2024-07-19, layers 7=jl & 8=sl & 9=ml & 10=ep, deleted plot/species skipped)
	extSQLScript, err := os.ReadFile(filepath.Join("internal", "interchange", "testdata", "turboveg3", "external_tv3_minimal.sql"))
	if err != nil {
		t.Fatal(err)
	}
	extSQLFixture := filepath.Join(dir, "external_tv3_minimal.sqlite")
	seedTV3DB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(extSQLFixture, "rwc")+"&_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seedTV3DB.Exec(string(extSQLScript)); err != nil {
		seedTV3DB.Close()
		t.Fatalf("seed external_tv3_minimal.sql: %v", err)
	}
	if err := seedTV3DB.Close(); err != nil {
		t.Fatal(err)
	}
	extSQLDest := filepath.Join(dir, "ExtTV3.db")
	sqlOut, err := interchange.ImportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      extSQLFixture,
		Format:          interchange.TurboVegFormatSQLite,
		TargetProject:   "ExtTV3",
		DestinationPath: extSQLDest,
	})
	if err != nil {
		t.Fatalf("ImportTurboVeg(external_tv3_minimal.sqlite) failed: %v", err)
	}
	if sqlOut.PlotCount != 1 { // Plot 2 has deleted=1 and must be ignored
		t.Fatalf("expected 1 active plot from external_tv3_minimal.sqlite, got %+v", sqlOut)
	}

	sdb, err := sqlutil.OpenReadOnly(extSQLDest)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()

	var sLat, sLon float64
	var sDate, sSurv string
	if err := sdb.QueryRow(`SELECT Latitude, Longitude, CAST("Date" AS TEXT), SiteSurveyor FROM ExtTV3_Env WHERE PlotNumber='501'`).Scan(&sLat, &sLon, &sDate, &sSurv); err != nil {
		t.Fatal(err)
	}
	if math.Abs(sLat-54.75) > 1e-6 || math.Abs(sLon - -127.125) > 1e-6 || sDate != "2024-07-19" || sSurv != "S.M. Hennekens" {
		t.Fatalf("TV3 WKT / Delphi TDateTime / lookup mismatch: lat=%v lon=%v date=%q surv=%q", sLat, sLon, sDate, sSurv)
	}
	// Vaccinium membranaceum: layer 4 (18.75 -> Cover4) + layer 7=jl woody (3.0 -> Cover5, height 0.8 -> Height5)
	var vC4, vC5, vH5 float64
	if err := sdb.QueryRow(`SELECT Cover4, Cover5, Height5 FROM ExtTV3_Veg WHERE PlotNumber='501' AND Species='VACCMEM'`).Scan(&vC4, &vC5, &vH5); err != nil {
		t.Fatal(err)
	}
	if math.Abs(vC4-18.75) > 1e-4 || math.Abs(vC5-3.0) > 1e-4 || math.Abs(vH5-0.8) > 1e-4 {
		t.Fatalf("TV3 woody jl layer 7 mismatch: c4=%v c5=%v h5=%v", vC4, vC5, vH5)
	}
	// Cornus canadensis: layer 6 (8.75) + layer 7=jl non-woody (0.5) -> both roll into Cover6 = 9.25!
	var cC6 float64
	if err := sdb.QueryRow(`SELECT Cover6 FROM ExtTV3_Veg WHERE PlotNumber='501' AND Species='CORNCAN'`).Scan(&cC6); err != nil || math.Abs(cC6-9.25) > 1e-4 {
		t.Fatalf("TV3 non-woody jl layer 7 rollup into Cover6 mismatch: c6=%v err=%v", cC6, err)
	}
	// Pseudotsuga menziesii: layer 1 (37.5, height 31.5) + layer 8=sl seedling (0.5 -> Cover6)
	var pC1, pH1, pC6 float64
	if err := sdb.QueryRow(`SELECT Cover1, Height1, Cover6 FROM ExtTV3_Veg WHERE PlotNumber='501' AND Species='PSEUMEN'`).Scan(&pC1, &pH1, &pC6); err != nil {
		t.Fatal(err)
	}
	if math.Abs(pC1-37.5) > 1e-4 || math.Abs(pH1-31.5) > 1e-4 || math.Abs(pC6-0.5) > 1e-4 {
		t.Fatalf("TV3 seedling sl layer 8 mismatch: c1=%v h1=%v c6=%v", pC1, pH1, pC6)
	}
	// Alectoria sarmentosa: layer 10=ep (0.1 -> Cover10), while deleted layer 6 row is ignored (Cover6 IS NULL)
	var aC10 float64
	var aC6 *float64
	if err := sdb.QueryRow(`SELECT Cover10, Cover6 FROM ExtTV3_Veg WHERE PlotNumber='501' AND Species='ALECSAR'`).Scan(&aC10, &aC6); err != nil {
		t.Fatal(err)
	}
	if math.Abs(aC10-0.1) > 1e-4 || aC6 != nil {
		t.Fatalf("TV3 epiphyte / deleted speciesobservation mismatch: c10=%v c6=%v", aC10, aC6)
	}

	// 3. Cancelled context must abort single-plot <Plot_package> XML import immediately
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := interchange.ImportTurboVeg(service, cancelCtx, state.ContextID, interchange.TurboVegImportRequest{
		SourcePath:      extXMLFixture,
		Format:          interchange.TurboVegFormatXML,
		TargetProject:   "ExtCancel",
		DestinationPath: filepath.Join(dir, "ExtCancel.db"),
	}); err == nil {
		t.Fatal("expected cancelled context to abort ImportTurboVeg(xml)")
	}

	// 4. Verify <Syntaxon_list> and <Author_list> disambiguate colliding >7-char BEC SiteUnits and >4-char Surveyors on Sample.db export
	sampleXMLPath := filepath.Join(dir, "sample_disambig.xml")
	if _, err := interchange.ExportTurboVeg(service, ctx, state.ContextID, interchange.TurboVegExportRequest{
		Format:          interchange.TurboVegFormatXML,
		DestinationPath: sampleXMLPath,
	}); err != nil {
		t.Fatalf("ExportTurboVeg(Sample.db): %v", err)
	}
	sampleXMLStrBytes, err := os.ReadFile(sampleXMLPath)
	if err != nil {
		t.Fatal(err)
	}
	sampleXMLStr := string(sampleXMLStrBytes)
	if !strings.Contains(sampleXMLStr, `description="BWBSdk 1 /08"`) || !strings.Contains(sampleXMLStr, `description="BWBSdk 1 /09"`) {
		t.Fatal("expected distinct <syntaxon_record> descriptions preserved for colliding 7-char prefixes BWBSdk 1 /08 and BWBSdk 1 /09")
	}
}

func TestVenus50XMLArchivesImportToVP08(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()
	dir := t.TempDir()

	// Create relational VENUS 5.0 XML archive (matching Access/VENUS table structure)
	venusXMLPath := filepath.Join(dir, "venus5_archive.xml")
	venusXML := `<?xml version="1.0" encoding="UTF-8"?>
<dataroot version="5.0">
  <USysVersion><Version>5</Version><Beta>3</Beta></USysVersion>
  <tblProjectManager>
    <ProjectID>P100</ProjectID>
    <ProjectName>SkeenaVenus</ProjectName>
    <PlotQuality>Good</PlotQuality>
  </tblProjectManager>
  <tblPlotData>
    <PlotID>1</PlotID>
    <ProjectID>P100</ProjectID>
    <PlotNumber>SV001</PlotNumber>
    <Date>2024-07-19</Date>
    <SiteSurveyor>J. Pojar</SiteSurveyor>
    <LatitudeDegree>54</LatitudeDegree>
    <LatitudeMinute>30</LatitudeMinute>
    <LatitudeSecond>0</LatitudeSecond>
    <LongitudeDegree>126</LongitudeDegree>
    <LongitudeMinute>30</LongitudeMinute>
    <LongitudeSecond>0</LongitudeSecond>
    <Zone>SBS</Zone>
    <SubZone>dk</SubZone>
    <SiteSeries>01</SiteSeries>
    <MoistureRegime>4</MoistureRegime>
    <NutrientRegime>C</NutrientRegime>
    <OrganicSoilTexture>2</OrganicSoilTexture>
    <OrganicHorizonThickness>1</OrganicHorizonThickness>
    <CoarseFragContent>2</CoarseFragContent>
    <TerrainSMaterial1>M</TerrainSMaterial1>
    <TerrainSMaterial2>b</TerrainSMaterial2>
    <Graviti>true</Graviti>
  </tblPlotData>
  <tblVegLayer><VegLayerID>10</VegLayerID><PlotID>1</PlotID><Layer>2</Layer><Height>26.5</Height></tblVegLayer>
  <tblVegLayer><VegLayerID>11</VegLayerID><PlotID>1</PlotID><Layer>6</Layer></tblVegLayer>
  <tblVegLayer><VegLayerID>12</VegLayerID><PlotID>1</PlotID><Layer>10</Layer></tblVegLayer>
  <tblVegSpecies><VegLayerID>10</VegLayerID><Species>PICEMAR</Species><Cover>45.5</Cover><Collected>C</Collected></tblVegSpecies>
  <tblVegSpecies><VegLayerID>11</VegLayerID><Species>CORNCAN</Species><Cover>18.25</Cover><Height>0.12</Height></tblVegSpecies>
  <tblVegSpecies><VegLayerID>12</VegLayerID><Species>BRYORIA</Species><Cover>4.0</Cover></tblVegSpecies>
  <tblOrganicHorizons><PlotID>1</PlotID><Horizon>Fm</Horizon><DepthUpper>6</DepthUpper><DepthLower>0</DepthLower><pHOrganic>4.3</pHOrganic></tblOrganicHorizons>
  <tblMineralHorizons><PlotID>1</PlotID><Horizon>Bm1</Horizon><DepthUpper>0</DepthUpper><DepthLower>35</DepthLower><Colour>10YR 4/3</Colour><Texture>SL</Texture><PercentCoarseFragsTotal>25</PercentCoarseFragsTotal></tblMineralHorizons>
</dataroot>`
	if err := os.WriteFile(venusXMLPath, []byte(venusXML), 0o600); err != nil {
		t.Fatal(err)
	}

	insp, err := interchange.InspectVenusSource(service, ctx, state.ContextID, venusXMLPath)
	if err != nil {
		t.Fatalf("InspectVenusSource: %v", err)
	}
	if len(insp.Projects) != 1 || insp.Projects[0].ProjectName != "SkeenaVenus" || insp.Projects[0].PlotCount != 1 {
		t.Fatalf("unexpected VENUS inspection: %+v", insp)
	}

	destDB := filepath.Join(dir, "SkeenaVenusImp.db")
	res, err := interchange.ImportVenusProject(service, ctx, state.ContextID, interchange.VenusImportRequest{
		SourcePath:      venusXMLPath,
		SourceProject:   "SkeenaVenus",
		TargetProject:   "SkeenaVenusImp",
		DestinationPath: destDB,
	})
	if err != nil {
		t.Fatalf("ImportVenusProject: %v", err)
	}
	if res.PlotCount != 1 || res.VegRowCount != 3 || res.HumusRowCount != 1 || res.MineralRowCount != 1 {
		t.Fatalf("unexpected VENUS import result: %+v", res)
	}

	db, err := sqlutil.OpenReadOnly(destDB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var lat, lon, svCF float64
	var rzps, surfMat string
	if err := db.QueryRow(`SELECT Latitude, Longitude, RootZoneParticleSize, SurficialMaterialSurf, SV_PercentCoarseFrags FROM SkeenaVenusImp_Env WHERE PlotNumber='SV001'`).
		Scan(&lat, &lon, &rzps, &surfMat, &svCF); err != nil {
		t.Fatal(err)
	}
	if math.Abs(lat-54.5) > 1e-6 || math.Abs(lon - -126.5) > 1e-6 || rzps != "Me" || surfMat != "Mb" || math.Abs(svCF-27.0) > 1e-6 {
		t.Fatalf("VENUS domain field mapping mismatch: lat=%v lon=%v rzps=%q surfMat=%q svCF=%v", lat, lon, rzps, surfMat, svCF)
	}
	var plotType string
	var humThick float64
	if err := db.QueryRow(`SELECT PlotType, HumusThickness FROM SkeenaVenusImp_Admin WHERE Plot='SV001'`).Scan(&plotType, &humThick); err != nil {
		t.Fatal(err)
	}
	if plotType != "1" || math.Abs(humThick-10.0) > 1e-6 {
		t.Fatalf("VENUS Admin mapping mismatch: plotType=%q humThick=%v", plotType, humThick)
	}

	// Also test hierarchical <VenusArchive> XML format
	hierXMLPath := filepath.Join(dir, "venus5_hier.xml")
	hierXML := `<?xml version="1.0" encoding="UTF-8"?>
<VenusArchive version="5.0" projectName="HaidaVenus">
  <Plot PlotNumber="HV009" Date="2025-06-11" Zone="CWH" SubZone="wh1" MoistureRegime="5" NutrientRegime="D" Latitude="53.25" Longitude="-132.15">
    <VegLayer code="1" height="35.0">
      <Species code="PICESIT" cover="62.5" collected="C"/>
    </VegLayer>
    <VegLayer code="7">
      <Species code="RHYTLOR" cover="48.0"/>
    </VegLayer>
    <Humus horizon="Lv" upperdepth="8" lowerdepth="5"/>
    <Mineral horizon="Bf" upperdepth="0" lowerdepth="40" texture="SL" colour="7.5YR 3/4"/>
  </Plot>
</VenusArchive>`
	if err := os.WriteFile(hierXMLPath, []byte(hierXML), 0o600); err != nil {
		t.Fatal(err)
	}
	hierRes, err := interchange.ImportVenusProject(service, ctx, state.ContextID, interchange.VenusImportRequest{
		SourcePath:      hierXMLPath,
		TargetProject:   "HaidaVenusImp",
		DestinationPath: filepath.Join(dir, "HaidaVenusImp.db"),
	})
	if err != nil || hierRes.PlotCount != 1 || hierRes.VegRowCount != 2 {
		t.Fatalf("hierarchical VENUS XML import failed: %+v, err=%v", hierRes, err)
	}

	// Test real Jet .mdb VENUS 5.0 template via internal/accessreader when sibling asset is present
	const realVenusMDB = "/home/bruno/Work/VPRO_ACCESS/VPro64/Templates/TemplateVENUS5.mdb"
	if _, statErr := os.Stat(realVenusMDB); statErr == nil {
		mdbInsp, mdbErr := interchange.InspectVenusSource(service, ctx, state.ContextID, realVenusMDB)
		if mdbErr != nil {
			t.Fatalf("InspectVenusSource on real TemplateVENUS5.mdb failed: %v", mdbErr)
		}
		if mdbInsp.SourceFormat != "mdb" || !strings.HasPrefix(mdbInsp.VenusVersion, "5") {
			t.Fatalf("unexpected TemplateVENUS5.mdb inspection metadata: %+v", mdbInsp)
		}
	}
}

func TestGeoPackageExportEPSG3005AndEPSG4326(t *testing.T) {
	// 1. Verify Snyder Albers Equal-Area Conic (EPSG:3005) projection accuracy and inverse round-trip
	eOrigin, nOrigin := interchange.ProjectNAD83ToBCAlbers(45.0, -126.0)
	if math.Abs(eOrigin-1000000.0) > 1e-6 || math.Abs(nOrigin-0.0) > 1e-6 {
		t.Fatalf("expected BC Albers origin (45N, -126W) -> (1000000, 0), got (%f, %f)", eOrigin, nOrigin)
	}
	// Victoria BC (48.4284 N, -123.3656 W): PROJ/GDAL EPSG:3005 = (1195327.902934, 382812.069317)
	eVic, nVic := interchange.ProjectNAD83ToBCAlbers(48.4284, -123.3656)
	if math.Abs(eVic-1195327.902934) > 1e-4 || math.Abs(nVic-382812.069317) > 1e-4 {
		t.Fatalf("unexpected BC Albers coordinates for Victoria: E=%f N=%f", eVic, nVic)
	}
	invLat, invLon := interchange.ProjectBCAlbersToNAD83(eVic, nVic)
	if math.Abs(invLat-48.4284) > 1e-7 || math.Abs(invLon - -123.3656) > 1e-7 {
		t.Fatalf("BC Albers inverse round-trip error: got (%f, %f)", invLat, invLon)
	}

	// 2. Test GeoPackage export with positive West longitude normalization (125.4 -> -125.4) and attribute tables
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		UPDATE Sample_Env SET Latitude=54.25, Longitude=125.4 WHERE PlotNumber='108050';
	`)

	dir := t.TempDir()
	for _, crs := range []string{interchange.CRSEPSG3005, interchange.CRSEPSG4326} {
		gpkgPath := filepath.Join(dir, "plots_"+strings.ReplaceAll(crs, ":", "_")+".gpkg")
		out, err := interchange.ExportGeoPackage(service, ctx, state.ContextID, interchange.GeoPackageExportRequest{
			DestinationPath:        gpkgPath,
			CRS:                    crs,
			IncludeAttributeTables: true,
		})
		if err != nil {
			t.Fatalf("ExportGeoPackage(%s): %v", crs, err)
		}
		if out.FeatureCount < 1 || out.NormalizedWestLongitudes < 1 || len(out.AttributeTables) != 3 {
			t.Fatalf("unexpected GeoPackage outcome for %s: %+v", crs, out)
		}

		db, err := sqlutil.OpenReadOnly(gpkgPath)
		if err != nil {
			t.Fatal(err)
		}
		var appID int
		if err := db.QueryRow(`PRAGMA application_id`).Scan(&appID); err != nil || appID != 0x47504B47 {
			db.Close()
			t.Fatalf("expected GPKG magic application_id 0x47504B47, got 0x%x (err=%v)", appID, err)
		}
		var geomBlob []byte
		var storedLon float64
		if err := db.QueryRow(`SELECT geom, Longitude FROM plots WHERE PlotNumber='108050'`).Scan(&geomBlob, &storedLon); err != nil {
			db.Close()
			t.Fatalf("read plot 108050 from GPKG (%s): %v", crs, err)
		}
		if math.Abs(storedLon - -125.4) > 1e-6 {
			db.Close()
			t.Fatalf("expected normalized Longitude=-125.4 in GPKG attributes, got %v", storedLon)
		}
		srsID, gx, gy, err := interchange.DecodeGeoPackagePoint(geomBlob)
		if err != nil {
			db.Close()
			t.Fatalf("DecodeGeoPackagePoint(%s): %v", crs, err)
		}
		if crs == interchange.CRSEPSG3005 {
			expE, expN := interchange.ProjectNAD83ToBCAlbers(54.25, -125.4)
			if srsID != 3005 || math.Abs(gx-expE) > 1e-4 || math.Abs(gy-expN) > 1e-4 {
				db.Close()
				t.Fatalf("EPSG:3005 point mismatch: srs=%d (%f,%f) vs expected (%f,%f)", srsID, gx, gy, expE, expN)
			}
		} else {
			if srsID != 4326 || math.Abs(gx - -125.4) > 1e-6 || math.Abs(gy-54.25) > 1e-6 {
				db.Close()
				t.Fatalf("EPSG:4326 point mismatch: srs=%d (%f,%f)", srsID, gx, gy)
			}
		}
		var attrCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM gpkg_contents WHERE data_type='attributes'`).Scan(&attrCount); err != nil || attrCount != 3 {
			db.Close()
			t.Fatalf("expected 3 attribute tables registered in gpkg_contents, got %d", attrCount)
		}
		db.Close()

		// Verify GDAL/sf in Rscript reads our .gpkg feature & attribute layers cleanly when sf is installed
		if rscriptPath, lookErr := exec.LookPath("Rscript"); lookErr == nil {
			rCheck := `
args <- commandArgs(trailingOnly = TRUE)
if (requireNamespace("sf", quietly = TRUE)) {
  pts <- sf::st_read(args[[1]], layer = "plots", quiet = TRUE)
  stopifnot(nrow(pts) >= 1L)
  stopifnot(!is.na(sf::st_crs(pts)$epsg))
  veg <- sf::st_read(args[[1]], layer = "veg", quiet = TRUE)
  stopifnot(is.data.frame(veg), "Species" %in% colnames(veg))
}
cat("GPKG_SF_OK\n")
`
			if outBytes, runErr := exec.CommandContext(ctx, rscriptPath, "--vanilla", "-e", rCheck, gpkgPath).CombinedOutput(); runErr != nil {
				t.Fatalf("sf::st_read failed on %s: %v\n%s", crs, runErr, string(outBytes))
			}
		}
	}
}
