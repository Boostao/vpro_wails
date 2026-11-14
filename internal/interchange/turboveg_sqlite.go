package interchange

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

var delphiEpochUTC = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

func timeToDelphiDateTime(t time.Time) float64 {
	u := t.UTC()
	y, m, d := u.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	days := math.Round(float64(dayStart.Unix()-delphiEpochUTC.Unix()) / 86400.0)
	dayFrac := float64(u.Hour()*3600+u.Minute()*60+u.Second()) / 86400.0
	if days < 0 && dayFrac > 0 {
		return days - dayFrac
	}
	return days + dayFrac
}

func parseISODateToDelphi(raw string) (delphiDays float64, exactISODate bool, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false, false
	}
	for _, layout := range []string{"2006-01-02", "2006/01/02", "20060102", "2006-01-02 15:04:05", "2006/01/02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			d := timeToDelphiDateTime(t)
			isExact := s == t.UTC().Format("2006-01-02") && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
			return d, isExact, true
		}
	}
	return 0, false, false
}

func delphiDateTimeToISODate(days float64) string {
	if math.IsNaN(days) || math.IsInf(days, 0) {
		return ""
	}
	// Delphi TDateTime encodes pre-1899-12-30 dates as -(absDays + timeFraction),
	// so integer day offset is Trunc toward zero (with tiny epsilon for float rounding).
	eps := 1e-9
	if days < 0 {
		eps = -1e-9
	}
	wholeDays := int(math.Trunc(days + eps))
	return delphiEpochUTC.AddDate(0, 0, wholeDays).Format("2006-01-02")
}

func formatWKTPoint(lon, lat float64) string {
	return fmt.Sprintf("POINT(%s %s)",
		strconv.FormatFloat(lon, 'f', -1, 64),
		strconv.FormatFloat(lat, 'f', -1, 64),
	)
}

func parseWKTPointLonLat(raw string) (lon, lat float64, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, 0, false
	}
	if semi := strings.IndexByte(s, ';'); semi >= 0 {
		s = strings.TrimSpace(s[semi+1:])
	}
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "POINT") {
		return 0, 0, false
	}
	openIdx := strings.IndexByte(s, '(')
	closeIdx := strings.LastIndexByte(s, ')')
	if openIdx < 0 || closeIdx <= openIdx+1 {
		return 0, 0, false
	}
	inner := strings.TrimSpace(s[openIdx+1 : closeIdx])
	if strings.EqualFold(inner, "EMPTY") {
		return 0, 0, false
	}
	parts := strings.Fields(strings.ReplaceAll(inner, ",", " "))
	if len(parts) < 2 {
		return 0, 0, false
	}
	x, errX := strconv.ParseFloat(parts[0], 64)
	y, errY := strconv.ParseFloat(parts[1], 64)
	if errX != nil || errY != nil || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return 0, 0, false
	}
	return x, y, true
}

func parseCompassAspectDegrees(raw string) (int, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		return int(math.Round(f)), true
	}
	compass := map[string]int{
		"N": 0, "NNE": 22, "NE": 45, "ENE": 68,
		"E": 90, "ESE": 112, "SE": 135, "SSE": 158,
		"S": 180, "SSW": 202, "SW": 225, "WSW": 248,
		"W": 270, "WNW": 292, "NW": 315, "NNW": 338,
		"O": 90, "NO": 45, "ZO": 135, "Z": 180, "ZW": 225,
	}
	if deg, ok := compass[s]; ok {
		return deg, true
	}
	return 0, false
}

// Official 31-table, 37-FK, 36-index Alterra Turboveg 3 SQLite schema.
const turboVeg3OfficialDDL = `
CREATE TABLE datatypes (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR NOT NULL
);

CREATE TABLE units (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	desciption VARCHAR
);

CREATE TABLE fielddomains (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50)
);

CREATE TABLE fieldgroups (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	"order" INTEGER,
	metadata VARCHAR
);

CREATE TABLE lookuptables (
	id INTEGER NOT NULL PRIMARY KEY,
	filename VARCHAR(50) NOT NULL,
	description VARCHAR
);

CREATE TABLE lookupcaptions (
	id INTEGER NOT NULL PRIMARY KEY,
	lookuptableid INTEGER NOT NULL,
	columnno INTEGER NOT NULL,
	caption VARCHAR(50) NOT NULL,
	CONSTRAINT lookupcaptions_fkey FOREIGN KEY (lookuptableid) REFERENCES lookuptables(id)
);

CREATE TABLE lookupdata (
	id INTEGER NOT NULL PRIMARY KEY,
	lookuptableid INTEGER NOT NULL,
	code VARCHAR(100) NOT NULL,
	uri VARCHAR,
	description1 VARCHAR(255) NOT NULL,
	description2 VARCHAR(255) NOT NULL,
	description3 VARCHAR(255) NOT NULL,
	description4 VARCHAR(255) NOT NULL,
	CONSTRAINT lookuptablesdata_fkey01 FOREIGN KEY (lookuptableid) REFERENCES lookuptables(id)
);

CREATE TABLE covercodes (
	id INTEGER NOT NULL PRIMARY KEY,
	lookupdataid INTEGER NOT NULL,
	code VARCHAR NOT NULL,
	uri VARCHAR,
	percentage_mean REAL NOT NULL,
	percentage_min REAL,
	percentage_max REAL,
	CONSTRAINT covercodes_fkey01 FOREIGN KEY (lookupdataid) REFERENCES lookupdata(id)
);

CREATE TABLE custodians (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR NOT NULL,
	institution VARCHAR,
	email VARCHAR,
	metadata VARCHAR
);

CREATE TABLE datasetgroups (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	metadata VARCHAR
);

CREATE TABLE profiles (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL
);

CREATE TABLE taxonlists (
	id INTEGER NOT NULL PRIMARY KEY,
	sbstaxonlistid INTEGER,
	name VARCHAR(50) NOT NULL,
	metadata VARCHAR
);

CREATE TABLE taxongroups (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	metadata VARCHAR
);

CREATE TABLE taxonranks (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	metadata VARCHAR
);

CREATE TABLE vegetationlayers (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	abbreviation VARCHAR(50),
	description VARCHAR
);

CREATE TABLE metadata (
	"key" VARCHAR NOT NULL PRIMARY KEY,
	"value" VARCHAR NOT NULL
);

CREATE TABLE projects (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	metadata VARCHAR
);

CREATE TABLE datasets (
	id INTEGER NOT NULL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	datasetgroupid INTEGER NOT NULL,
	profileid INTEGER NOT NULL,
	taxonlistid INTEGER NOT NULL,
	custodianid INTEGER,
	deputycustodianid INTEGER,
	givdid VARCHAR,
	guid VARCHAR,
	metadata VARCHAR,
	CONSTRAINT datasets_fkey01 FOREIGN KEY (datasetgroupid) REFERENCES datasetgroups(id),
	CONSTRAINT datasets_fkey02 FOREIGN KEY (taxonlistid) REFERENCES taxonlists(id),
	CONSTRAINT datasets_fkey03 FOREIGN KEY (profileid) REFERENCES profiles(id),
	CONSTRAINT datasets_fkey04 FOREIGN KEY (custodianid) REFERENCES custodians(id),
	CONSTRAINT datasets_fkey05 FOREIGN KEY (deputycustodianid) REFERENCES custodians(id)
);

CREATE TABLE projectdatasets (
	projectid INTEGER NOT NULL,
	datasetid INTEGER NOT NULL,
	CONSTRAINT projectdatasets_fkey01 FOREIGN KEY (projectid) REFERENCES projects(id),
	CONSTRAINT projectdatasets_fkey02 FOREIGN KEY (datasetid) REFERENCES datasets(id)
);

CREATE TABLE fields (
	id INTEGER NOT NULL PRIMARY KEY,
	fielddomainid INTEGER NOT NULL,
	fieldgroupid INTEGER,
	lookuptableid INTEGER,
	unitid INTEGER,
	fieldname VARCHAR(20) NOT NULL,
	fieldtype VARCHAR(1) NOT NULL,
	description VARCHAR,
	datatypeid INTEGER NOT NULL,
	width SMALLINT NOT NULL,
	decimals SMALLINT NOT NULL,
	mandatory BOOLEAN,
	readonly BOOLEAN,
	valmin REAL,
	valmax REAL,
	valin VARCHAR,
	hint VARCHAR,
	CONSTRAINT fields_fkey01 FOREIGN KEY (unitid) REFERENCES units(id),
	CONSTRAINT fields_fkey02 FOREIGN KEY (lookuptableid) REFERENCES lookuptables(id),
	CONSTRAINT fields_fkey03 FOREIGN KEY (fieldgroupid) REFERENCES fieldgroups(id),
	CONSTRAINT fields_fkey04 FOREIGN KEY (datatypeid) REFERENCES datatypes(id),
	CONSTRAINT fields_fkey05 FOREIGN KEY (fielddomainid) REFERENCES fielddomains(id)
);

CREATE TABLE profiledata (
	id INTEGER NOT NULL PRIMARY KEY,
	profileid INTEGER NOT NULL,
	fieldid INTEGER NOT NULL,
	"order" INTEGER,
	"show" BOOLEAN,
	mandatory BOOLEAN,
	readonly BOOLEAN,
	CONSTRAINT profiledata_fkey01 FOREIGN KEY (profileid) REFERENCES profiles(id),
	CONSTRAINT profiledata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id)
);

CREATE TABLE locations (
	id INTEGER NOT NULL PRIMARY KEY,
	the_geom GEOMETRY NOT NULL
);

CREATE TABLE plots (
	id INTEGER NOT NULL PRIMARY KEY,
	datasetid INTEGER NOT NULL,
	locationid INTEGER,
	locationuncertainty INTEGER,
	metadata VARCHAR,
	CONSTRAINT plots_fkey01 FOREIGN KEY (datasetid) REFERENCES datasets(id),
	CONSTRAINT plots_fkey02 FOREIGN KEY (locationid) REFERENCES locations(id)
);

CREATE TABLE plotobservations (
	id INTEGER NOT NULL PRIMARY KEY,
	plotid INTEGER NOT NULL,
	guid VARCHAR NOT NULL,
	changedby VARCHAR(50),
	changedwhen DOUBLE,
	quality SMALLINT,
	deleted BOOLEAN,
	synoptic BOOLEAN,
	nrofplotobservations INTEGER,
	accessregime SMALLINT,
	CONSTRAINT plotobservations_fkey01 FOREIGN KEY (plotid) REFERENCES plots(id)
);

CREATE TABLE plotobservationheaderdata (
	id INTEGER NOT NULL PRIMARY KEY,
	plotobservationid INTEGER NOT NULL,
	fieldid INTEGER NOT NULL,
	lookupdataid INTEGER,
	valuestring VARCHAR,
	valueinteger INTEGER,
	valuefloat DOUBLE,
	deleted BOOLEAN,
	CONSTRAINT plotobservationheaderdata_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id),
	CONSTRAINT plotobservationheaderdata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id),
	CONSTRAINT plotobservationheaderdata_fkey03 FOREIGN KEY (lookupdataid) REFERENCES lookupdata(id)
);

CREATE TABLE pictures (
	id INTEGER NOT NULL PRIMARY KEY,
	plotobservationid INTEGER NOT NULL,
	url VARCHAR,
	author VARCHAR,
	date DOUBLE,
	longitude DOUBLE,
	latitude DOUBLE,
	description VARCHAR,
	CONSTRAINT pictures_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id)
);

CREATE TABLE associa (
	id INTEGER NOT NULL PRIMARY KEY,
	plotobservationid INTEGER NOT NULL,
	lookupdataid INTEGER NOT NULL,
	combinedindex REAL NOT NULL,
	normalised_likelyhood REAL NOT NULL,
	incompleteness REAL NOT NULL,
	weirdness REAL NOT NULL,
	CONSTRAINT associa_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id),
	CONSTRAINT associa_fkey02 FOREIGN KEY (lookupdataid) REFERENCES lookupdata(id)
);

CREATE TABLE taxa (
	id INTEGER NOT NULL PRIMARY KEY,
	taxonlistid INTEGER NOT NULL,
	taxongroupid INTEGER,
	taxonrankid INTEGER,
	parent INTEGER,
	scientificname VARCHAR NOT NULL,
	vernacularname VARCHAR,
	author VARCHAR,
	tv2speciesid INTEGER,
	tv2validid INTEGER,
	sbsspeciesid INTEGER,
	lettercode VARCHAR(7),
	issynonym INTEGER,
	uri VARCHAR,
	metadata VARCHAR,
	CONSTRAINT species_fkey01 FOREIGN KEY (taxonlistid) REFERENCES taxonlists(id),
	CONSTRAINT species_fkey02 FOREIGN KEY (taxongroupid) REFERENCES taxongroups(id),
	CONSTRAINT species_fkey03 FOREIGN KEY (taxonrankid) REFERENCES taxonranks(id)
);

CREATE TABLE traitdata (
	id INTEGER NOT NULL PRIMARY KEY,
	taxonid INTEGER NOT NULL,
	fieldid INTEGER NOT NULL,
	value VARCHAR NOT NULL,
	CONSTRAINT traitdata_fkey01 FOREIGN KEY (taxonid) REFERENCES taxa(id),
	CONSTRAINT traitdata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id)
);

CREATE TABLE speciesobservations (
	id INTEGER NOT NULL PRIMARY KEY,
	plotobservationid INTEGER NOT NULL,
	taxonid INTEGER NOT NULL,
	vegetationlayerid INTEGER NOT NULL,
	covercodeid INTEGER NOT NULL,
	originaltaxonconcept VARCHAR,
	metadata VARCHAR,
	deleted BOOLEAN,
	CONSTRAINT speciesdata_fkey01 FOREIGN KEY (plotobservationid) REFERENCES plotobservations(id),
	CONSTRAINT speciesdata_fkey02 FOREIGN KEY (taxonid) REFERENCES taxa(id),
	CONSTRAINT speciesdata_fkey03 FOREIGN KEY (vegetationlayerid) REFERENCES vegetationlayers(id),
	CONSTRAINT speciesdata_fkey04 FOREIGN KEY (covercodeid) REFERENCES covercodes(id)
);

CREATE TABLE speciesobservationdata (
	id INTEGER NOT NULL PRIMARY KEY,
	speciesobservationid INTEGER NOT NULL,
	fieldid INTEGER NOT NULL,
	lookupdataid INTEGER,
	valuestring VARCHAR,
	valueinteger INTEGER,
	valuefloat REAL,
	deleted BOOLEAN,
	CONSTRAINT observationsspeciesdata_fkey01 FOREIGN KEY (speciesobservationid) REFERENCES speciesobservations(id),
	CONSTRAINT observationsspeciesdata_fkey02 FOREIGN KEY (fieldid) REFERENCES fields(id)
);

CREATE INDEX associa_index01 ON associa (plotobservationid);
CREATE UNIQUE INDEX associa_index02 ON associa (plotobservationid, lookupdataid);
CREATE UNIQUE INDEX covercodes_index01 ON covercodes (lookupdataid, code);
CREATE UNIQUE INDEX custodians_index01 ON custodians (name);
CREATE UNIQUE INDEX datasetgroups_index01 ON datasetgroups (name);
CREATE INDEX datasets_index01 ON datasets (datasetgroupid);
CREATE UNIQUE INDEX datasets_index02 ON datasets (name);
CREATE UNIQUE INDEX datasetgroups_index02 ON fieldgroups (name);
CREATE UNIQUE INDEX fields_index01 ON fields (fielddomainid, fieldname, fieldtype);
CREATE INDEX locations_index01 ON locations (the_geom);
CREATE UNIQUE INDEX lookupcaptions_index01 ON lookupcaptions (lookuptableid, columnno);
CREATE UNIQUE INDEX lookuptabledata_index01 ON lookupdata (lookuptableid, code, description1, description2, description3, description4);
CREATE UNIQUE INDEX lookuptables_index01 ON lookuptables (filename, description);
CREATE INDEX pictures_index01 ON pictures (plotobservationid);
CREATE INDEX plotobservationheaderdata_index01 ON plotobservationheaderdata (plotobservationid);
CREATE INDEX plotobservationheaderdata_index02 ON plotobservationheaderdata (fieldid);
CREATE INDEX plotobservations_index01 ON plotobservations (plotid);
CREATE UNIQUE INDEX plotobservations_index02 ON plotobservations (guid);
CREATE INDEX plots_index01 ON plots (datasetid);
CREATE INDEX profiledata_index01 ON profiledata (profileid, fieldid);
CREATE UNIQUE INDEX profiles_index01 ON profiles (name);
CREATE UNIQUE INDEX projects_index01 ON projects (name);
CREATE INDEX speciesobservationdata_index01 ON speciesobservationdata (speciesobservationid);
CREATE INDEX speciesobservationdata_index02 ON speciesobservationdata (fieldid);
CREATE INDEX observationsspecies_index01 ON speciesobservations (plotobservationid);
CREATE UNIQUE INDEX observationsspecies_index02 ON speciesobservations (plotobservationid, taxonid, vegetationlayerid, originaltaxonconcept);
CREATE INDEX observationsspecies_index03 ON speciesobservations (taxonid);
CREATE UNIQUE INDEX taxa_index01 ON taxa (taxonlistid, scientificname, tv2speciesid);
CREATE INDEX taxa_index02 ON taxa (tv2speciesid);
CREATE INDEX taxa_index03 ON taxa (taxonlistid);
CREATE INDEX taxa_index04 ON taxa (sbsspeciesid);
CREATE UNIQUE INDEX speciesgroups_index01 ON taxongroups (name);
CREATE UNIQUE INDEX floras_index01 ON taxonlists (name);
CREATE INDEX traitdata_index01 ON traitdata (taxonid);
CREATE UNIQUE INDEX traitdata_index02 ON traitdata (taxonid, fieldid);
CREATE UNIQUE INDEX units_index01 ON units (name);
`

type tv3FieldSeed struct {
	ID            int
	DomainID      int
	GroupID       any
	LookupTableID any
	UnitID        any
	Name          string
	FieldType     string
	Description   string
	DataTypeID    int
	Width         int
	Decimals      int
	Mandatory     int
}

const (
	tv3FieldReleveNR    = 1
	tv3FieldCountry     = 2
	tv3FieldCoverscale  = 3
	tv3FieldDate        = 4
	tv3FieldSurfArea    = 5
	tv3FieldAltitude    = 6
	tv3FieldExposition  = 7
	tv3FieldInclinatio  = 8
	tv3FieldAuthor      = 9
	tv3FieldRemarks     = 10
	tv3FieldLatitude    = 11
	tv3FieldLongitude   = 12
	tv3FieldVProPlot    = 13
	tv3FieldFieldNR     = 14
	tv3FieldProjectID   = 15
	tv3FieldZone        = 16
	tv3FieldSubzone     = 17
	tv3FieldSiteSeries  = 18
	tv3FieldSiteUnit    = 19
	tv3FieldMoisture    = 20
	tv3FieldNutrient    = 21
	tv3FieldMesoSlope   = 22
	tv3FieldSurfShape   = 23
	tv3FieldDrainage    = 24
	tv3FieldHumusForm   = 25
	tv3FieldLocation    = 26
	tv3FieldSiteNotes   = 27
	tv3FieldVegNotes    = 28
	tv3FieldSoilNotes   = 29
	tv3FieldSppHeight   = 30
	tv3FieldSppCollect  = 31
	tv3FieldSppVProLay  = 32
	tv3FieldSppVProCode = 33
)

var canonicalTV3Fields = []tv3FieldSeed{
	{tv3FieldReleveNR, 1, 1, nil, nil, "RELEVE_NR", "N", "Releve number", 2, 7, 0, 1},
	{tv3FieldCountry, 1, 1, 2, nil, "COUNTRY", "C", "Country code", 1, 2, 0, 0},
	{tv3FieldCoverscale, 1, 1, 1, nil, "COVERSCALE", "C", "Cover abundance scale", 1, 2, 0, 0},
	{tv3FieldDate, 1, 1, nil, nil, "DATE", "D", "Survey date", 4, 10, 0, 0},
	{tv3FieldSurfArea, 1, 2, nil, 4, "SURF_AREA", "N", "Plot surface area (m2)", 3, 8, 2, 0},
	{tv3FieldAltitude, 1, 2, nil, 2, "ALTITUDE", "N", "Elevation (m)", 2, 5, 0, 0},
	{tv3FieldExposition, 1, 2, nil, 3, "EXPOSITION", "N", "Aspect (degrees)", 2, 3, 0, 0},
	{tv3FieldInclinatio, 1, 2, nil, 3, "INCLINATIO", "N", "Slope gradient", 3, 5, 1, 0},
	{tv3FieldAuthor, 1, 1, nil, nil, "AUTHOR", "C", "Site surveyor", 1, 30, 0, 0},
	{tv3FieldRemarks, 1, 4, nil, nil, "REMARKS", "C", "General remarks", 1, 255, 0, 0},
	{tv3FieldLatitude, 1, 2, nil, 3, "LATITUDE", "N", "Latitude (decimal degrees)", 3, 13, 8, 0},
	{tv3FieldLongitude, 1, 2, nil, 3, "LONGITUDE", "N", "Longitude (decimal degrees)", 3, 13, 8, 0},
	{tv3FieldVProPlot, 1, 1, nil, nil, "VPRO_PLOT", "C", "VPro literal PlotNumber", 1, 7, 0, 0},
	{tv3FieldFieldNR, 1, 1, nil, nil, "FIELD_NR", "C", "VPro FieldNumber", 1, 50, 0, 0},
	{tv3FieldProjectID, 1, 1, nil, nil, "PROJECT_ID", "C", "VPro ProjectID", 1, 20, 0, 0},
	{tv3FieldZone, 1, 3, nil, nil, "ZONE", "C", "BEC Zone", 1, 4, 0, 0},
	{tv3FieldSubzone, 1, 3, nil, nil, "SUBZONE", "C", "BEC SubZone/Variant", 1, 8, 0, 0},
	{tv3FieldSiteSeries, 1, 3, nil, nil, "SITE_SERIES", "C", "BEC SiteSeries", 1, 5, 0, 0},
	{tv3FieldSiteUnit, 1, 3, nil, nil, "SITE_UNIT", "C", "Assigned SiteUnit", 1, 100, 0, 0},
	{tv3FieldMoisture, 1, 3, nil, nil, "MOISTURE", "C", "Relative soil moisture regime", 1, 3, 0, 0},
	{tv3FieldNutrient, 1, 3, nil, nil, "NUTRIENT", "C", "Relative soil nutrient regime", 1, 2, 0, 0},
	{tv3FieldMesoSlope, 1, 2, nil, nil, "MESO_SLOPE", "C", "Meso slope position", 1, 3, 0, 0},
	{tv3FieldSurfShape, 1, 2, nil, nil, "SURF_SHAPE", "C", "Surface shape", 1, 3, 0, 0},
	{tv3FieldDrainage, 1, 3, nil, nil, "DRAINAGE", "C", "Soil drainage", 1, 5, 0, 0},
	{tv3FieldHumusForm, 1, 3, nil, nil, "HUMUS_FORM", "C", "Humus form", 1, 4, 0, 0},
	{tv3FieldLocation, 1, 2, nil, nil, "LOCATION", "C", "Location description", 1, 255, 0, 0},
	{tv3FieldSiteNotes, 1, 4, nil, nil, "SITE_NOTES", "C", "VPro SiteNotes", 1, 255, 0, 0},
	{tv3FieldVegNotes, 1, 4, nil, nil, "VEG_NOTES", "C", "VPro VegNotes", 1, 255, 0, 0},
	{tv3FieldSoilNotes, 1, 4, nil, nil, "SOIL_NOTES", "C", "VPro SoilNotes", 1, 255, 0, 0},
	{tv3FieldSppHeight, 2, nil, nil, 2, "HEIGHT", "N", "Species stratum height (m)", 3, 6, 2, 0},
	{tv3FieldSppCollect, 2, nil, nil, nil, "COLLECTED", "C", "Voucher collection status", 1, 1, 0, 0},
	{tv3FieldSppVProLay, 2, nil, nil, nil, "VPRO_LAYER", "C", "VPro sublayer code", 1, 8, 0, 0},
	{tv3FieldSppVProCode, 2, nil, nil, nil, "VPRO_CODE", "C", "VPro 8-char species code", 1, 8, 0, 0},
}

func formatCoverCodeKey(cov float64) string {
	return strconv.FormatFloat(cov, 'f', -1, 64)
}

func writeTurboVeg3SQLite(ctx context.Context, destinationPath string, ds tvDataset) (string, string, error) {
	absDest, err := filepath.Abs(filepath.Clean(destinationPath))
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(absDest), 0o755); err != nil {
		return "", "", err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(absDest), ".vpro-tv3-*.sqlite")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	_ = tmpFile.Close()

	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL&_foreign_keys=on")
	if err != nil {
		return "", "", err
	}
	db.SetMaxOpenConns(1)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.Close()
		return "", "", err
	}

	build := func() error {
		if _, err := tx.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, turboVeg3OfficialDDL); err != nil {
			return fmt.Errorf("create official Turboveg 3 schema: %w", err)
		}

		projName := truncateUTF16Units(strings.TrimSpace(ds.ProjectName), 50)
		if projName == "" {
			projName = "VProExport"
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO datatypes (id, name) VALUES
				(1, 'TEXT'), (2, 'INTEGER'), (3, 'REAL'), (4, 'DATE'), (5, 'BOOLEAN');
			INSERT INTO units (id, name, desciption) VALUES
				(1, '%', 'Percentage'),
				(2, 'm', 'Meters'),
				(3, 'deg', 'Degrees'),
				(4, 'm2', 'Square meters');
			INSERT INTO fielddomains (id, name) VALUES
				(1, 'Administrative and header data'),
				(2, 'Species attributes'),
				(3, 'Species trait data');
			INSERT INTO fieldgroups (id, name, "order") VALUES
				(1, 'General', 1),
				(2, 'Location & Topography', 2),
				(3, 'BEC Classification & Edaphics', 3),
				(4, 'Notes', 4);
			INSERT INTO lookuptables (id, filename, description) VALUES
				(1, 'tvscale', 'Cover abundance scales'),
				(2, 'country', 'Countries');
			INSERT INTO lookupcaptions (id, lookuptableid, columnno, caption) VALUES
				(1, 1, 1, 'Scale code'),
				(2, 1, 2, 'Description'),
				(3, 2, 1, 'Country code'),
				(4, 2, 2, 'Country');
			INSERT INTO lookupdata (id, lookuptableid, code, description1, description2, description3, description4) VALUES
				(1, 1, '00', 'Percentage (%)', '', '', ''),
				(2, 2, 'CA', 'Canada', '', '', '');
			INSERT INTO custodians (id, name, institution) VALUES
				(1, 'B.C. Ministry of Forests BEC Program', 'Province of British Columbia, Research Branch');
			INSERT INTO datasetgroups (id, name) VALUES (1, 'Core');
			INSERT INTO profiles (id, name) VALUES (1, 'VPro_BEC_Standard');
			INSERT INTO taxonlists (id, name) VALUES (1, 'BCPlantTaxonomy');
			INSERT INTO taxongroups (id, name) VALUES
				(1, 'Vascular plants'),
				(2, 'Stoneworts'),
				(3, 'Mosses'),
				(4, 'Lichens'),
				(5, 'Mushrooms'),
				(6, 'Algae');
			INSERT INTO taxonranks (id, name) VALUES
				(1, 'Genus'),
				(2, 'Species'),
				(3, 'Subspecies'),
				(4, 'Variety');
			INSERT INTO metadata ("key", "value") VALUES
				('dbversion', '3.9'),
				('generator', 'VPro (B.C. Ministry of Forests BEC Program)');
		`); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO projects (id, name) VALUES (1, ?)`, projName); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO datasets (id, name, datasetgroupid, profileid, taxonlistid, custodianid, guid)
			VALUES (1, ?, 1, 1, 1, 1, ?)`, projName, newTurboVegUUID()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO projectdatasets (projectid, datasetid) VALUES (1, 1)`); err != nil {
			return err
		}

		vlStmt, err := tx.PrepareContext(ctx, `INSERT INTO vegetationlayers (id, name, abbreviation, description) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		for _, vl := range canonicalTV3VegetationLayers {
			if _, err := vlStmt.ExecContext(ctx, vl.ID, vl.Name, vl.Abbreviation, vl.Description); err != nil {
				vlStmt.Close()
				return err
			}
		}
		vlStmt.Close()

		fStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO fields (
				id, fielddomainid, fieldgroupid, lookuptableid, unitid,
				fieldname, fieldtype, description, datatypeid, width, decimals, mandatory, readonly
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`)
		if err != nil {
			return err
		}
		pdStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO profiledata (id, profileid, fieldid, "order", "show", mandatory, readonly)
			VALUES (?, 1, ?, ?, 1, ?, 0)`)
		if err != nil {
			fStmt.Close()
			return err
		}
		pdID := 1
		for _, f := range canonicalTV3Fields {
			if _, err := fStmt.ExecContext(ctx,
				f.ID, f.DomainID, f.GroupID, f.LookupTableID, f.UnitID,
				f.Name, f.FieldType, f.Description, f.DataTypeID, f.Width, f.Decimals, f.Mandatory,
			); err != nil {
				pdStmt.Close()
				fStmt.Close()
				return err
			}
			if f.DomainID == 1 {
				if _, err := pdStmt.ExecContext(ctx, pdID, f.ID, pdID, f.Mandatory); err != nil {
					pdStmt.Close()
					fStmt.Close()
					return err
				}
				pdID++
			}
		}
		pdStmt.Close()
		fStmt.Close()

		// Seed taxa
		taxaStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO taxa (
				id, taxonlistid, taxongroupid, taxonrankid, scientificname,
				vernacularname, author, tv2speciesid, tv2validid, lettercode, issynonym, metadata
			) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`)
		if err != nil {
			return err
		}
		taxonIDByNR := make(map[int]int, len(ds.Species))
		for _, sp := range ds.Species {
			sci := strings.TrimSpace(sp.ScientificName)
			if sci == "" {
				sci = sp.SpeciesCode
			}
			letter := truncateUTF16Units(strings.TrimSpace(sp.LetterCode), 7)
			if letter == "" {
				letter = truncateUTF16Units(strings.TrimSpace(sp.SpeciesCode), 7)
			}
			metaBytes, _ := json.Marshal(map[string]any{
				"vpro_code": sp.SpeciesCode,
				"lifeform":  sp.Lifeform,
			})
			if _, err := taxaStmt.ExecContext(ctx,
				sp.SpeciesNR,
				taxonGroupForLifeform(sp.Lifeform),
				taxonRankForSciName(sci),
				sci,
				nullEmpty(sp.EnglishName),
				nullEmpty(sp.Authority),
				sp.SpeciesNR,
				sp.SpeciesNR,
				nullEmpty(letter),
				string(metaBytes),
			); err != nil {
				taxaStmt.Close()
				return err
			}
			taxonIDByNR[sp.SpeciesNR] = sp.SpeciesNR
		}
		taxaStmt.Close()

		// Collect and seed distinct covercodes under lookupdataid = 1 ('00' Percentage (%))
		coverMeanByKey := map[string]float64{"0": 0}
		for _, pb := range ds.Plots {
			for _, o := range pb.Observations {
				key := formatCoverCodeKey(o.Cover)
				if _, exists := coverMeanByKey[key]; !exists {
					coverMeanByKey[key] = o.Cover
				}
			}
		}
		coverKeys := make([]string, 0, len(coverMeanByKey))
		for k := range coverMeanByKey {
			coverKeys = append(coverKeys, k)
		}
		slices.SortFunc(coverKeys, func(a, b string) int {
			va, vb := coverMeanByKey[a], coverMeanByKey[b]
			if va < vb {
				return -1
			}
			if va > vb {
				return 1
			}
			return strings.Compare(a, b)
		})

		ccStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO covercodes (id, lookupdataid, code, percentage_mean, percentage_min, percentage_max)
			VALUES (?, 1, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		coverCodeIDByKey := make(map[string]int, len(coverKeys))
		for idx, key := range coverKeys {
			ccID := idx + 1
			mean := coverMeanByKey[key]
			if _, err := ccStmt.ExecContext(ctx, ccID, key, mean, mean, mean); err != nil {
				ccStmt.Close()
				return err
			}
			coverCodeIDByKey[key] = ccID
		}
		ccStmt.Close()

		locStmt, err := tx.PrepareContext(ctx, `INSERT INTO locations (id, the_geom) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer locStmt.Close()

		plotStmt, err := tx.PrepareContext(ctx, `INSERT INTO plots (id, datasetid, locationid, metadata) VALUES (?, 1, ?, ?)`)
		if err != nil {
			return err
		}
		defer plotStmt.Close()

		poStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO plotobservations (
				id, plotid, guid, changedby, changedwhen, quality, deleted, synoptic, nrofplotobservations, accessregime
			) VALUES (?, ?, ?, 'VPro', ?, 3, 0, 0, NULL, 0)`)
		if err != nil {
			return err
		}
		defer poStmt.Close()

		hdrStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO plotobservationheaderdata (
				id, plotobservationid, fieldid, lookupdataid, valuestring, valueinteger, valuefloat, deleted
			) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`)
		if err != nil {
			return err
		}
		defer hdrStmt.Close()

		soStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO speciesobservations (
				id, plotobservationid, taxonid, vegetationlayerid, covercodeid, originaltaxonconcept, metadata, deleted
			) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`)
		if err != nil {
			return err
		}
		defer soStmt.Close()

		sodStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO speciesobservationdata (
				id, speciesobservationid, fieldid, lookupdataid, valuestring, valueinteger, valuefloat, deleted
			) VALUES (?, ?, ?, NULL, ?, NULL, ?, 0)`)
		if err != nil {
			return err
		}
		defer sodStmt.Close()

		nowDelphi := timeToDelphiDateTime(time.Now())
		nextLocID := 1
		nextHdrID := 1
		nextSOID := 1
		nextSODID := 1

		addStrHdr := func(poID, fieldID int, val string) error {
			if strings.TrimSpace(val) == "" {
				return nil
			}
			_, err := hdrStmt.ExecContext(ctx, nextHdrID, poID, fieldID, nil, val, nil, nil)
			nextHdrID++
			return err
		}
		addIntHdr := func(poID, fieldID int, val *int) error {
			if val == nil {
				return nil
			}
			_, err := hdrStmt.ExecContext(ctx, nextHdrID, poID, fieldID, nil, nil, *val, nil)
			nextHdrID++
			return err
		}
		addFloatHdr := func(poID, fieldID int, val *float64) error {
			if val == nil {
				return nil
			}
			_, err := hdrStmt.ExecContext(ctx, nextHdrID, poID, fieldID, nil, nil, nil, *val)
			nextHdrID++
			return err
		}

		for idx, pb := range ds.Plots {
			if idx%128 == 0 {
				runtime.Gosched()
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			h := pb.Header
			plotID := h.PlotID
			if plotID <= 0 {
				plotID = idx + 1
			}
			guid := strings.TrimSpace(h.GUID)
			if guid == "" {
				guid = newTurboVegUUID()
			}

			var locIDAny any
			if h.Latitude != nil && h.Longitude != nil {
				wkt := formatWKTPoint(*h.Longitude, *h.Latitude)
				if _, err := locStmt.ExecContext(ctx, nextLocID, wkt); err != nil {
					return err
				}
				locIDAny = nextLocID
				nextLocID++
			}
			plotMeta, _ := json.Marshal(map[string]string{"vpro_plot": h.PlotNumber})
			if _, err := plotStmt.ExecContext(ctx, plotID, locIDAny, string(plotMeta)); err != nil {
				return err
			}
			if _, err := poStmt.ExecContext(ctx, plotID, plotID, guid, nowDelphi); err != nil {
				return err
			}

			releveNR := plotID
			if err := addIntHdr(plotID, tv3FieldReleveNR, &releveNR); err != nil {
				return err
			}
			// Country CA (lookupdataid=2) and Coverscale 00 (lookupdataid=1)
			if _, err := hdrStmt.ExecContext(ctx, nextHdrID, plotID, tv3FieldCountry, 2, "CA", nil, nil); err != nil {
				return err
			}
			nextHdrID++
			if _, err := hdrStmt.ExecContext(ctx, nextHdrID, plotID, tv3FieldCoverscale, 1, "00", nil, nil); err != nil {
				return err
			}
			nextHdrID++

			if strings.TrimSpace(h.Date) != "" {
				if dFloat, exactISO, ok := parseISODateToDelphi(h.Date); ok {
					var rawStrAny any
					if !exactISO {
						rawStrAny = h.Date
					}
					if _, err := hdrStmt.ExecContext(ctx, nextHdrID, plotID, tv3FieldDate, nil, rawStrAny, nil, dFloat); err != nil {
						return err
					}
				} else {
					if _, err := hdrStmt.ExecContext(ctx, nextHdrID, plotID, tv3FieldDate, nil, h.Date, nil, nil); err != nil {
						return err
					}
				}
				nextHdrID++
			}

			for _, fOp := range []error{
				addFloatHdr(plotID, tv3FieldSurfArea, h.PlotSize),
				addIntHdr(plotID, tv3FieldAltitude, h.Elevation),
				addIntHdr(plotID, tv3FieldExposition, h.Aspect),
				addFloatHdr(plotID, tv3FieldInclinatio, h.Slope),
				addStrHdr(plotID, tv3FieldAuthor, h.Surveyor),
				addFloatHdr(plotID, tv3FieldLatitude, h.Latitude),
				addFloatHdr(plotID, tv3FieldLongitude, h.Longitude),
				addStrHdr(plotID, tv3FieldVProPlot, h.PlotNumber),
				addStrHdr(plotID, tv3FieldFieldNR, h.FieldNumber),
				addStrHdr(plotID, tv3FieldProjectID, h.ProjectID),
				addStrHdr(plotID, tv3FieldZone, h.Zone),
				addStrHdr(plotID, tv3FieldSubzone, h.SubZone),
				addStrHdr(plotID, tv3FieldSiteSeries, h.SiteSeries),
				addStrHdr(plotID, tv3FieldSiteUnit, h.SiteUnit),
				addStrHdr(plotID, tv3FieldMoisture, h.MoistureRegime),
				addStrHdr(plotID, tv3FieldNutrient, h.NutrientRegime),
				addStrHdr(plotID, tv3FieldMesoSlope, h.MesoSlopePosition),
				addStrHdr(plotID, tv3FieldSurfShape, h.SurfaceShape),
				addStrHdr(plotID, tv3FieldDrainage, h.SoilDrainage),
				addStrHdr(plotID, tv3FieldHumusForm, h.HumusForm),
				addStrHdr(plotID, tv3FieldLocation, h.Location),
				addStrHdr(plotID, tv3FieldSiteNotes, h.SiteNotes),
				addStrHdr(plotID, tv3FieldVegNotes, h.VegNotes),
				addStrHdr(plotID, tv3FieldSoilNotes, h.SoilNotes),
			} {
				if fOp != nil {
					return fOp
				}
			}

			// Count (taxonid, vegetationlayerid) multiplicity in this plot to guarantee observationsspecies_index02 uniqueness.
			type taxLayerKey struct {
				taxonID int
				layerID int
			}
			multiplicity := map[taxLayerKey]int{}
			for _, o := range pb.Observations {
				multiplicity[taxLayerKey{taxonIDByNR[o.SpeciesNR], o.TVLayerID}]++
			}
			usedConceptKey := map[string]int{}

			for _, o := range pb.Observations {
				taxonID := taxonIDByNR[o.SpeciesNR]
				tvLayerID := o.TVLayerID
				if tvLayerID < 0 || tvLayerID > 10 {
					tvLayerID = 6
				}
				ccID := coverCodeIDByKey[formatCoverCodeKey(o.Cover)]
				baseConcept := strings.TrimSpace(o.SciName)
				if baseConcept == "" {
					baseConcept = o.SpeciesCode
				}
				cw, hasCW := vproSublayerCodeToTV[o.VProLayer]
				concept := baseConcept
				if multiplicity[taxLayerKey{taxonID, tvLayerID}] > 1 || (hasCW && !cw.IsPrimary) {
					if strings.TrimSpace(o.VProLayer) != "" {
						concept = baseConcept + " [" + o.VProLayer + "]"
					}
				}
				uniqProbe := fmt.Sprintf("%d|%d|%s", taxonID, tvLayerID, concept)
				usedConceptKey[uniqProbe]++
				if n := usedConceptKey[uniqProbe]; n > 1 {
					concept = fmt.Sprintf("%s #%d", concept, n)
				}

				soMeta, _ := json.Marshal(map[string]string{
					"vpro_code":  o.SpeciesCode,
					"vpro_layer": o.VProLayer,
				})
				soID := nextSOID
				nextSOID++
				if _, err := soStmt.ExecContext(ctx, soID, plotID, taxonID, tvLayerID, ccID, concept, string(soMeta)); err != nil {
					return err
				}

				if strings.TrimSpace(o.SpeciesCode) != "" {
					if _, err := sodStmt.ExecContext(ctx, nextSODID, soID, tv3FieldSppVProCode, o.SpeciesCode, nil); err != nil {
						return err
					}
					nextSODID++
				}
				if strings.TrimSpace(o.VProLayer) != "" {
					if _, err := sodStmt.ExecContext(ctx, nextSODID, soID, tv3FieldSppVProLay, o.VProLayer, nil); err != nil {
						return err
					}
					nextSODID++
				}
				if o.Height != nil {
					if _, err := sodStmt.ExecContext(ctx, nextSODID, soID, tv3FieldSppHeight, nil, *o.Height); err != nil {
						return err
					}
					nextSODID++
				}
				if strings.TrimSpace(o.Collected) != "" {
					if _, err := sodStmt.ExecContext(ctx, nextSODID, soID, tv3FieldSppCollect, o.Collected, nil); err != nil {
						return err
					}
					nextSODID++
				}
			}
		}

		// Self-verify zero foreign key violations before committing.
		fkRows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
		if err != nil {
			return err
		}
		hasViolation := fkRows.Next()
		_ = fkRows.Close()
		if hasViolation {
			return errors.New("exported Turboveg 3 SQLite database failed PRAGMA foreign_key_check")
		}
		return nil
	}

	if err := build(); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		_ = db.Close()
		return "", "", err
	}
	if err := db.Close(); err != nil {
		return "", "", err
	}
	if err := os.Rename(tmpPath, absDest); err != nil {
		return "", "", err
	}
	published = true

	rawBytes, err := os.ReadFile(absDest)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(rawBytes)
	return absDest, hex.EncodeToString(sum[:]), nil
}

func readSQLiteTableSet(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT LOWER(name) FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		set[n] = true
	}
	return set, rows.Err()
}

func readSQLiteTableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+vprocore.QuoteHeaderIdentifier(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[strings.ToLower(name)] = true
	}
	return cols, rows.Err()
}

func readTurboVeg3SQLiteFile(ctx context.Context, path string, resolver *speciesImportResolver) ([]tvImportPlot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resolved, err := vprocore.ExistingDatabasePath(path)
	if err != nil {
		return nil, err
	}
	db, err := sqlutil.OpenReadOnlyContext(ctx, resolved)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tbls, err := readSQLiteTableSet(ctx, db)
	if err != nil {
		return nil, err
	}
	for _, req := range []string{"plotobservations", "plots", "speciesobservations", "taxa", "covercodes", "vegetationlayers"} {
		if !tbls[req] {
			return nil, fmt.Errorf("source SQLite file %s is missing official Turboveg 3 table %q", filepath.Base(resolved), req)
		}
	}

	geomExpr := "''"
	locJoin := ""
	if tbls["locations"] {
		locCols, err := readSQLiteTableColumns(ctx, db, "locations")
		if err != nil {
			return nil, err
		}
		locJoin = ` LEFT JOIN locations AS l ON l.id = p.locationid`
		switch {
		case locCols["the_geom"] && locCols["wkt"]:
			geomExpr = `COALESCE(NULLIF(TRIM(CAST(l.the_geom AS TEXT)), ''), NULLIF(TRIM(CAST(l.wkt AS TEXT)), ''), '')`
		case locCols["the_geom"]:
			geomExpr = `COALESCE(NULLIF(TRIM(CAST(l.the_geom AS TEXT)), ''), '')`
		case locCols["wkt"]:
			geomExpr = `COALESCE(NULLIF(TRIM(CAST(l.wkt AS TEXT)), ''), '')`
		}
	}

	var plots []tvImportPlot
	poQuery := `
		SELECT po.id, ` + geomExpr + `
		FROM plotobservations AS po
		INNER JOIN plots AS p ON p.id = po.plotid` + locJoin + `
		WHERE COALESCE(po.deleted, 0) = 0
		ORDER BY po.id`

	poRows, err := db.QueryContext(ctx, poQuery)
	if err != nil {
		return nil, fmt.Errorf("read Turboveg 3 plotobservations: %w", err)
	}
	plotIdxByObsID := map[int]int{}
	wktByObsID := map[int]string{}
	releveFallbackByObsID := map[int]string{}
	for poRows.Next() {
		var obsID int
		var wkt string
		if err := poRows.Scan(&obsID, &wkt); err != nil {
			poRows.Close()
			return nil, err
		}
		plotIdxByObsID[obsID] = len(plots)
		wktByObsID[obsID] = wkt
		plots = append(plots, tvImportPlot{
			Header: tvPlotHeader{
				PlotID: obsID,
			},
		})
	}
	if err := poRows.Err(); err != nil {
		poRows.Close()
		return nil, err
	}
	if err := poRows.Close(); err != nil {
		return nil, err
	}

	// Pivot plotobservationheaderdata
	if tbls["plotobservationheaderdata"] && tbls["fields"] {
		dtExpr := "''"
		dtJoin := ""
		if tbls["datatypes"] {
			dtExpr = `UPPER(COALESCE(dt.name, ''))`
			dtJoin = ` LEFT JOIN datatypes AS dt ON dt.id = f.datatypeid`
		}
		ldCodeExpr := "''"
		ldDescExpr := "''"
		ldJoin := ""
		if tbls["lookupdata"] {
			ldCodeExpr = `COALESCE(ld.code, '')`
			ldDescExpr = `COALESCE(ld.description1, '')`
			ldJoin = ` LEFT JOIN lookupdata AS ld ON ld.id = poh.lookupdataid`
		}
		hdrQuery := `
			SELECT
				poh.plotobservationid,
				UPPER(TRIM(f.fieldname)),
				` + dtExpr + `,
				COALESCE(poh.valuestring, ''),
				poh.valueinteger,
				poh.valuefloat,
				` + ldCodeExpr + `,
				` + ldDescExpr + `
			FROM plotobservationheaderdata AS poh
			INNER JOIN plotobservations AS po ON po.id = poh.plotobservationid
			INNER JOIN fields AS f ON f.id = poh.fieldid` + dtJoin + ldJoin + `
			WHERE COALESCE(po.deleted, 0) = 0 AND COALESCE(poh.deleted, 0) = 0
			ORDER BY poh.plotobservationid, poh.id`
		hRows, err := db.QueryContext(ctx, hdrQuery)
		if err != nil {
			return nil, fmt.Errorf("read Turboveg 3 plotobservationheaderdata: %w", err)
		}
		for hRows.Next() {
			var obsID int
			var fieldName, dtName, valStr, ldCode, ldDesc string
			var valInt sql.NullInt64
			var valFloat sql.NullFloat64
			if err := hRows.Scan(&obsID, &fieldName, &dtName, &valStr, &valInt, &valFloat, &ldCode, &ldDesc); err != nil {
				hRows.Close()
				return nil, err
			}
			idx, ok := plotIdxByObsID[obsID]
			if !ok {
				continue
			}
			h := &plots[idx].Header

			strVal := strings.TrimSpace(valStr)
			if strVal == "" && strings.TrimSpace(ldDesc) != "" {
				strVal = strings.TrimSpace(ldDesc)
			} else if strVal == "" && strings.TrimSpace(ldCode) != "" {
				strVal = strings.TrimSpace(ldCode)
			}
			if strVal == "" && valInt.Valid {
				strVal = strconv.FormatInt(valInt.Int64, 10)
			} else if strVal == "" && valFloat.Valid {
				strVal = strconv.FormatFloat(valFloat.Float64, 'f', -1, 64)
			}

			floatPtr := func() *float64 {
				if valFloat.Valid {
					v := valFloat.Float64
					return &v
				}
				if valInt.Valid {
					v := float64(valInt.Int64)
					return &v
				}
				if f, pErr := strconv.ParseFloat(strings.TrimSpace(valStr), 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
					return &f
				}
				return nil
			}
			intPtr := func() *int {
				if valInt.Valid {
					v := int(valInt.Int64)
					return &v
				}
				if valFloat.Valid {
					v := int(math.Round(valFloat.Float64))
					return &v
				}
				if deg, ok := parseCompassAspectDegrees(strVal); ok {
					return &deg
				}
				return nil
			}

			switch fieldName {
			case "VPRO_PLOT", "PLOT_NUMBER", "PLOTNUMBER":
				if strVal != "" {
					h.PlotNumber = strVal
				}
			case "RELEVE_NR":
				if strVal != "" {
					releveFallbackByObsID[obsID] = strVal
				}
			case "FIELD_NR", "FIELD_NUMBER", "NR_IN_TAB":
				if h.FieldNumber == "" || fieldName == "FIELD_NR" {
					h.FieldNumber = strVal
				}
			case "PROJECT_ID", "PROJECT":
				if h.ProjectID == "" || fieldName == "PROJECT_ID" {
					if strings.TrimSpace(valStr) != "" {
						h.ProjectID = strings.TrimSpace(valStr)
					} else {
						h.ProjectID = strVal
					}
				}
			case "DATE":
				if strings.TrimSpace(valStr) != "" {
					h.Date = normalizeTVDateString(valStr)
				} else if valFloat.Valid {
					h.Date = delphiDateTimeToISODate(valFloat.Float64)
				} else if valInt.Valid {
					if dtName != "DATE" && valInt.Int64 >= 10000101 && valInt.Int64 <= 99991231 {
						h.Date = normalizeTVDateString(strconv.FormatInt(valInt.Int64, 10))
					} else {
						h.Date = delphiDateTimeToISODate(float64(valInt.Int64))
					}
				}
			case "SURF_AREA", "PLOT_SIZE":
				if v := floatPtr(); v != nil {
					h.PlotSize = v
				}
			case "ALTITUDE", "ELEVATION":
				if v := intPtr(); v != nil {
					h.Elevation = v
				}
			case "EXPOSITION", "ASPECT":
				if v := intPtr(); v != nil {
					h.Aspect = v
				}
			case "INCLINATIO", "INCLINATION", "SLOPE":
				if v := floatPtr(); v != nil {
					h.Slope = v
				}
			case "AUTHOR", "SURVEYOR":
				if h.Surveyor == "" || fieldName == "SURVEYOR" {
					h.Surveyor = strVal
				}
			case "LATITUDE", "DEG_LAT":
				if v := floatPtr(); v != nil {
					h.Latitude = v
				}
			case "LONGITUDE", "DEG_LON":
				if v := floatPtr(); v != nil {
					h.Longitude = v
				}
			case "ZONE":
				h.Zone = strVal
			case "SUBZONE":
				h.SubZone = strVal
			case "SITE_SERIES":
				h.SiteSeries = strVal
			case "SITE_UNIT":
				h.SiteUnit = strVal
			case "SYNTAXON":
				if h.SiteUnit == "" {
					if strings.TrimSpace(ldCode) != "" {
						h.SiteUnit = strings.TrimSpace(ldCode)
					} else {
						h.SiteUnit = strVal
					}
				}
			case "MOISTURE", "MOISTURE_REGIME":
				h.MoistureRegime = strVal
			case "NUTRIENT", "NUTRIENT_REGIME":
				h.NutrientRegime = strVal
			case "MESO_SLOPE", "MESO_SLOPE_POSITION":
				h.MesoSlopePosition = strVal
			case "SURF_SHAPE", "SURFACE_SHAPE":
				h.SurfaceShape = strVal
			case "DRAINAGE", "SOIL_DRAINAGE":
				h.SoilDrainage = strVal
			case "HUMUS_FORM":
				h.HumusForm = strVal
			case "LOCATION", "LOCALITY":
				if h.Location == "" || fieldName == "LOCATION" {
					h.Location = strVal
				}
			case "SITE_NOTES":
				h.SiteNotes = strVal
			case "REMARKS":
				if h.SiteNotes == "" {
					h.SiteNotes = strVal
				}
			case "VEG_NOTES":
				h.VegNotes = strVal
			case "SOIL_NOTES":
				h.SoilNotes = strVal
			}
		}
		if err := hRows.Err(); err != nil {
			hRows.Close()
			return nil, err
		}
		if err := hRows.Close(); err != nil {
			return nil, err
		}
	}

	// Finalize plot numbers and WKT POINT coordinate fallback
	for obsID, idx := range plotIdxByObsID {
		h := &plots[idx].Header
		if strings.TrimSpace(h.PlotNumber) == "" {
			if rel := strings.TrimSpace(releveFallbackByObsID[obsID]); rel != "" {
				h.PlotNumber = rel
			} else {
				h.PlotNumber = strconv.Itoa(obsID)
			}
		}
		if h.Latitude == nil || h.Longitude == nil {
			if lon, lat, ok := parseWKTPointLonLat(wktByObsID[obsID]); ok {
				if h.Longitude == nil {
					h.Longitude = &lon
				}
				if h.Latitude == nil {
					h.Latitude = &lat
				}
			}
		}
	}

	// Load speciesobservationdata attributes keyed by speciesobservationid
	type soAttr struct {
		height    *float64
		collected string
		vproLayer string
		vproCode  string
	}
	attrsBySOID := map[int]*soAttr{}
	if tbls["speciesobservationdata"] && tbls["fields"] {
		sodRows, err := db.QueryContext(ctx, `
			SELECT
				sod.speciesobservationid,
				UPPER(TRIM(f.fieldname)),
				COALESCE(sod.valuestring, ''),
				sod.valueinteger,
				sod.valuefloat
			FROM speciesobservationdata AS sod
			INNER JOIN fields AS f ON f.id = sod.fieldid
			WHERE COALESCE(sod.deleted, 0) = 0
			ORDER BY sod.speciesobservationid, sod.id`)
		if err != nil {
			return nil, fmt.Errorf("read Turboveg 3 speciesobservationdata: %w", err)
		}
		for sodRows.Next() {
			var soID int
			var fName, vStr string
			var vInt sql.NullInt64
			var vFloat sql.NullFloat64
			if err := sodRows.Scan(&soID, &fName, &vStr, &vInt, &vFloat); err != nil {
				sodRows.Close()
				return nil, err
			}
			a := attrsBySOID[soID]
			if a == nil {
				a = &soAttr{}
				attrsBySOID[soID] = a
			}
			switch fName {
			case "HEIGHT":
				if vFloat.Valid {
					v := vFloat.Float64
					a.height = &v
				} else if vInt.Valid {
					v := float64(vInt.Int64)
					a.height = &v
				} else if f, pErr := strconv.ParseFloat(strings.TrimSpace(vStr), 64); pErr == nil {
					a.height = &f
				}
			case "COLLECTED":
				a.collected = strings.TrimSpace(vStr)
			case "VPRO_LAYER":
				a.vproLayer = strings.TrimSpace(vStr)
			case "VPRO_CODE":
				a.vproCode = strings.TrimSpace(vStr)
			}
		}
		if err := sodRows.Err(); err != nil {
			sodRows.Close()
			return nil, err
		}
		if err := sodRows.Close(); err != nil {
			return nil, err
		}
	}

	// Read active speciesobservations joined with taxa, scalar synonym target lookup (never multiplying rows when tv2speciesid repeats), vegetationlayers, and covercodes
	soRows, err := db.QueryContext(ctx, `
		SELECT
			so.id,
			so.plotobservationid,
			so.vegetationlayerid,
			COALESCE(vl.abbreviation, ''),
			cc.percentage_mean,
			COALESCE(cc.code, ''),
			COALESCE(t.scientificname, ''),
			COALESCE((
				SELECT vt.scientificname
				FROM taxa AS vt
				WHERE t.issynonym IS NOT NULL AND t.issynonym <> 0
					AND t.tv2validid IS NOT NULL
					AND vt.taxonlistid = t.taxonlistid
					AND vt.tv2speciesid = t.tv2validid
					AND vt.id <> t.id
				ORDER BY CASE WHEN COALESCE(vt.issynonym, 0) = 0 THEN 0 ELSE 1 END, vt.id ASC
				LIMIT 1
			), ''),
			COALESCE(t.lettercode, ''),
			COALESCE(t.metadata, ''),
			COALESCE(so.metadata, '')
		FROM speciesobservations AS so
		INNER JOIN plotobservations AS po ON po.id = so.plotobservationid
		INNER JOIN taxa AS t ON t.id = so.taxonid
		LEFT JOIN vegetationlayers AS vl ON vl.id = so.vegetationlayerid
		INNER JOIN covercodes AS cc ON cc.id = so.covercodeid
		WHERE COALESCE(po.deleted, 0) = 0 AND COALESCE(so.deleted, 0) = 0
		ORDER BY so.plotobservationid, so.id`)
	if err != nil {
		return nil, fmt.Errorf("read Turboveg 3 speciesobservations: %w", err)
	}
	defer soRows.Close()

	for soRows.Next() {
		var soID, obsID, vegLayerID int
		var vlAbbr, ccCode, sciName, validSciName, letterCode, taxaMetaRaw, soMetaRaw string
		var ccMean sql.NullFloat64
		if err := soRows.Scan(
			&soID, &obsID, &vegLayerID, &vlAbbr,
			&ccMean, &ccCode, &sciName, &validSciName, &letterCode,
			&taxaMetaRaw, &soMetaRaw,
		); err != nil {
			return nil, err
		}
		pIdx, ok := plotIdxByObsID[obsID]
		if !ok {
			continue
		}

		var cov float64
		if ccMean.Valid {
			cov = ccMean.Float64
		} else if parsed, pErr := strconv.ParseFloat(strings.TrimSpace(ccCode), 64); pErr == nil {
			cov = parsed
		} else if bb, ok := defaultBraunBlanquetMidpoint(ccCode); ok {
			cov = bb
		} else {
			return nil, fmt.Errorf("unresolvable Turboveg 3 cover code %q on speciesobservation %d", ccCode, soID)
		}

		var explicitVProCode, vproLayer string
		var heightPtr *float64
		var collected string
		if a := attrsBySOID[soID]; a != nil {
			explicitVProCode = a.vproCode
			vproLayer = a.vproLayer
			heightPtr = a.height
			collected = a.collected
		}
		if explicitVProCode == "" || vproLayer == "" {
			for _, rawJSON := range []string{soMetaRaw, taxaMetaRaw} {
				if strings.TrimSpace(rawJSON) == "" {
					continue
				}
				var m map[string]any
				if json.Unmarshal([]byte(rawJSON), &m) == nil {
					if explicitVProCode == "" {
						if vc, ok := m["vpro_code"].(string); ok {
							explicitVProCode = strings.TrimSpace(vc)
						}
					}
					if vproLayer == "" {
						if vl, ok := m["vpro_layer"].(string); ok {
							vproLayer = strings.TrimSpace(vl)
						}
					}
				}
			}
		}

		vproCode, lf := resolver.resolveTaxon(explicitVProCode, sciName, validSciName, letterCode)
		if strings.TrimSpace(vproCode) == "" {
			continue
		}
		isWoody := isWoodyVProLifeform(lf)
		tvLayerRaw := strconv.Itoa(vegLayerID)
		if vegLayerID < 0 && strings.TrimSpace(vlAbbr) != "" {
			tvLayerRaw = vlAbbr
		}

		plots[pIdx].Observations = append(plots[pIdx].Observations, tvImportObs{
			SpeciesCode: vproCode,
			TVLayerRaw:  tvLayerRaw,
			VProLayer:   vproLayer,
			IsWoody:     isWoody,
			Cover:       cov,
			Height:      heightPtr,
			Collected:   collected,
		})
	}
	return plots, soRows.Err()
}
