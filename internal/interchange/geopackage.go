package interchange

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	CRSEPSG4326 = "EPSG:4326"
	CRSEPSG3005 = "EPSG:3005"

	// GRS80 ellipsoid & BC Albers (EPSG:3005) parameters
	grs80SemiMajor   = 6378137.0
	grs80InvFlat     = 298.257222101
	bcAlbersPhi1Deg  = 50.0
	bcAlbersPhi2Deg  = 58.5
	bcAlbersPhi0Deg  = 45.0
	bcAlbersLam0Deg  = -126.0
	bcAlbersFalseE   = 1000000.0
	bcAlbersFalseN   = 0.0
)

var (
	grs80Flat = 1.0 / grs80InvFlat
	grs80E2   = 2*grs80Flat - grs80Flat*grs80Flat
	grs80E    = math.Sqrt(grs80E2)

	bcAlbersM1   = albersM(bcAlbersPhi1Deg * math.Pi / 180.0)
	bcAlbersM2   = albersM(bcAlbersPhi2Deg * math.Pi / 180.0)
	bcAlbersQ0   = albersQ(bcAlbersPhi0Deg * math.Pi / 180.0)
	bcAlbersQ1   = albersQ(bcAlbersPhi1Deg * math.Pi / 180.0)
	bcAlbersQ2   = albersQ(bcAlbersPhi2Deg * math.Pi / 180.0)
	bcAlbersN    = (bcAlbersM1*bcAlbersM1 - bcAlbersM2*bcAlbersM2) / (bcAlbersQ2 - bcAlbersQ1)
	bcAlbersC    = bcAlbersM1*bcAlbersM1 + bcAlbersN*bcAlbersQ1
	bcAlbersRho0 = grs80SemiMajor * math.Sqrt(bcAlbersC-bcAlbersN*bcAlbersQ0) / bcAlbersN
	bcAlbersLam0 = bcAlbersLam0Deg * math.Pi / 180.0
)

func albersM(phi float64) float64 {
	s := math.Sin(phi)
	return math.Cos(phi) / math.Sqrt(1.0-grs80E2*s*s)
}

func albersQ(phi float64) float64 {
	s := math.Sin(phi)
	es := grs80E * s
	return (1.0 - grs80E2) * (s/(1.0-grs80E2*s*s) - (1.0/(2.0*grs80E))*math.Log((1.0-es)/(1.0+es)))
}

// ProjectNAD83ToBCAlbers implements Snyder's exact GRS80 Albers Equal-Area Conic forward projection (EPSG:3005).
func ProjectNAD83ToBCAlbers(latDeg, lonDeg float64) (easting, northing float64) {
	phi := latDeg * math.Pi / 180.0
	lam := lonDeg * math.Pi / 180.0
	q := albersQ(phi)
	radicand := bcAlbersC - bcAlbersN*q
	if radicand < 0 {
		radicand = 0
	}
	rho := grs80SemiMajor * math.Sqrt(radicand) / bcAlbersN
	theta := bcAlbersN * (lam - bcAlbersLam0)
	easting = bcAlbersFalseE + rho*math.Sin(theta)
	northing = bcAlbersFalseN + bcAlbersRho0 - rho*math.Cos(theta)
	return easting, northing
}

// ProjectBCAlbersToNAD83 implements Snyder's iterative inverse GRS80 Albers projection (eqs. 14-19, 3-16).
func ProjectBCAlbersToNAD83(easting, northing float64) (latDeg, lonDeg float64) {
	dx := easting - bcAlbersFalseE
	dy := bcAlbersRho0 - (northing - bcAlbersFalseN)
	rho := math.Hypot(dx, dy)
	if bcAlbersN < 0 {
		rho = -rho
		dx = -dx
		dy = -dy
	}
	theta := 0.0
	if rho != 0 {
		theta = math.Atan2(dx, dy)
	}
	q := (bcAlbersC - (rho*bcAlbersN/grs80SemiMajor)*(rho*bcAlbersN/grs80SemiMajor)) / bcAlbersN
	qMax := (1.0 - grs80E2) * (1.0/(1.0-grs80E2) - (1.0/(2.0*grs80E))*math.Log((1.0-grs80E)/(1.0+grs80E)))
	arg := q / qMax
	if arg > 1 {
		arg = 1
	} else if arg < -1 {
		arg = -1
	}
	phi := math.Asin(arg)
	for iter := 0; iter < 12; iter++ {
		s := math.Sin(phi)
		es := grs80E * s
		oneMinusE2S2 := 1.0 - grs80E2*s*s
		dPhi := (oneMinusE2S2 * oneMinusE2S2 / (2.0 * math.Cos(phi))) *
			(q/(1.0-grs80E2) - s/oneMinusE2S2 + (1.0/(2.0*grs80E))*math.Log((1.0-es)/(1.0+es)))
		phi += dPhi
		if math.Abs(dPhi) < 1e-12 {
			break
		}
	}
	lam := bcAlbersLam0 + theta/bcAlbersN
	return phi * 180.0 / math.Pi, lam * 180.0 / math.Pi
}

// NormalizeWesternLongitude normalizes positive Western Hemisphere longitudes (e.g., 125.4 -> -125.4)
// and validates BC / Canadian Arctic latitude & longitude bounds.
func NormalizeWesternLongitude(lat, lon float64) (normLat, normLon float64, wasFlipped, valid bool) {
	normLat, normLon = lat, lon
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return lat, lon, false, false
	}
	if normLon >= 100.0 && normLon <= 170.0 {
		normLon = -normLon
		wasFlipped = true
	}
	if normLat < 45.0 || normLat > 85.0 || normLon < -170.0 || normLon > -100.0 {
		return normLat, normLon, wasFlipped, false
	}
	return normLat, normLon, wasFlipped, true
}

// EncodeGeoPackagePoint encodes a standard OGC GeoPackage Binary (GP v0) Little-Endian Point with XY envelope.
func EncodeGeoPackagePoint(srsID int32, x, y float64) []byte {
	// Header: 'G','P' (2) + version 0 (1) + flags 0x03 [LE + XY envelope] (1) + srs_id (4) + envelope [minX,maxX,minY,maxY] (32) = 40 bytes
	// WKB Point: byteOrder 1 (1) + wkbPoint 1 (4) + X (8) + Y (8) = 21 bytes -> total 61 bytes
	buf := make([]byte, 61)
	buf[0] = 'G'
	buf[1] = 'P'
	buf[2] = 0x00
	buf[3] = 0x03 // bit 0 = 1 (little-endian GP header), bits 1..3 = 1 (XY envelope)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(srsID))
	binary.LittleEndian.PutUint64(buf[8:16], math.Float64bits(x))
	binary.LittleEndian.PutUint64(buf[16:24], math.Float64bits(x))
	binary.LittleEndian.PutUint64(buf[24:32], math.Float64bits(y))
	binary.LittleEndian.PutUint64(buf[32:40], math.Float64bits(y))
	buf[40] = 0x01 // WKB little-endian
	binary.LittleEndian.PutUint32(buf[41:45], 1) // wkbPoint = 1
	binary.LittleEndian.PutUint64(buf[45:53], math.Float64bits(x))
	binary.LittleEndian.PutUint64(buf[53:61], math.Float64bits(y))
	return buf
}

// DecodeGeoPackagePoint parses a GeoPackage Binary Point for verification and tests.
func DecodeGeoPackagePoint(blob []byte) (srsID int32, x, y float64, err error) {
	if len(blob) < 29 || blob[0] != 'G' || blob[1] != 'P' {
		return 0, 0, 0, errors.New("invalid GeoPackage binary magic")
	}
	flags := blob[3]
	leHeader := (flags & 0x01) != 0
	envCode := (flags >> 1) & 0x07
	envBytes := 0
	switch envCode {
	case 0:
		envBytes = 0
	case 1:
		envBytes = 32
	case 2, 3:
		envBytes = 48
	case 4:
		envBytes = 64
	default:
		return 0, 0, 0, fmt.Errorf("invalid GeoPackage envelope indicator %d", envCode)
	}
	if leHeader {
		srsID = int32(binary.LittleEndian.Uint32(blob[4:8]))
	} else {
		srsID = int32(binary.BigEndian.Uint32(blob[4:8]))
	}
	wkbStart := 8 + envBytes
	if len(blob) < wkbStart+21 {
		return 0, 0, 0, errors.New("truncated GeoPackage WKB Point")
	}
	wkb := blob[wkbStart:]
	wkbLE := wkb[0] == 1
	var wkbType uint32
	if wkbLE {
		wkbType = binary.LittleEndian.Uint32(wkb[1:5])
		x = math.Float64frombits(binary.LittleEndian.Uint64(wkb[5:13]))
		y = math.Float64frombits(binary.LittleEndian.Uint64(wkb[13:21]))
	} else {
		wkbType = binary.BigEndian.Uint32(wkb[1:5])
		x = math.Float64frombits(binary.BigEndian.Uint64(wkb[5:13]))
		y = math.Float64frombits(binary.BigEndian.Uint64(wkb[13:21]))
	}
	if wkbType != 1 {
		return 0, 0, 0, fmt.Errorf("expected WKB Point (1), got %d", wkbType)
	}
	return srsID, x, y, nil
}

const (
	wktWGS84 = `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563]],PRIMEM["Greenwich",0],UNIT["degree",0.0174532925199433]]`
	wktNAD83 = `GEOGCS["NAD83",DATUM["North_American_Datum_1983",SPHEROID["GRS 1980",6378137,298.257222101]],PRIMEM["Greenwich",0],UNIT["degree",0.0174532925199433]]`
	wktBCAlbers = `PROJCS["NAD83 / BC Albers",GEOGCS["NAD83",DATUM["North_American_Datum_1983",SPHEROID["GRS 1980",6378137,298.257222101]],PRIMEM["Greenwich",0],UNIT["degree",0.0174532925199433]],PROJECTION["Albers_Conic_Equal_Area"],PARAMETER["standard_parallel_1",50],PARAMETER["standard_parallel_2",58.5],PARAMETER["latitude_of_center",45],PARAMETER["longitude_of_center",-126],PARAMETER["false_easting",1000000],PARAMETER["false_northing",0],UNIT["metre",1]]`
)

type GeoPackageExportRequest struct {
	DestinationPath        string `json:"destinationPath"`
	CRS                    string `json:"crs"` // "EPSG:3005" | "EPSG:4326"
	IncludeAttributeTables bool   `json:"includeAttributeTables"`
	IncludeUnlocatedPlots  bool   `json:"includeUnlocatedPlots"`
	LumpTable              string `json:"lumpTable,omitempty"`
	LumpDBPath             string `json:"lumpDbPath,omitempty"`
}

func (r *GeoPackageExportRequest) UnmarshalJSON(data []byte) error {
	type plain GeoPackageExportRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type GeoPackageExportOutcome struct {
	Path                     string   `json:"path"`
	SHA256                   string   `json:"sha256"`
	CRS                      string   `json:"crs"`
	SRSID                    int32    `json:"srsId"`
	FeatureCount             int      `json:"featureCount"`
	LocatedPlotCount         int      `json:"locatedPlotCount"`
	SkippedUnlocatedCount    int      `json:"skippedUnlocatedCount"`
	NormalizedWestLongitudes int      `json:"normalizedWestLongitudes"`
	AttributeTables          []string `json:"attributeTables"`
}

func ExportGeoPackage(s *vprocore.ContextService, ctx context.Context, contextID string, request GeoPackageExportRequest) (GeoPackageExportOutcome, error) {
	if strings.TrimSpace(request.DestinationPath) == "" {
		return GeoPackageExportOutcome{}, errors.New("GeoPackage export requires a non-empty destinationPath")
	}
	crs := strings.ToUpper(strings.TrimSpace(request.CRS))
	if crs == "" {
		crs = CRSEPSG3005
	}
	var targetSRSID int32
	switch crs {
	case CRSEPSG3005:
		targetSRSID = 3005
	case CRSEPSG4326:
		targetSRSID = 4326
	default:
		return GeoPackageExportOutcome{}, fmt.Errorf("unsupported GeoPackage CRS %q (supported: EPSG:3005, EPSG:4326)", request.CRS)
	}

	dest := strings.TrimSpace(request.DestinationPath)
	if !strings.HasSuffix(strings.ToLower(dest), ".gpkg") {
		dest += ".gpkg"
	}
	absDest, err := filepath.Abs(filepath.Clean(dest))
	if err != nil {
		return GeoPackageExportOutcome{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absDest), 0o755); err != nil {
		return GeoPackageExportOutcome{}, err
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (GeoPackageExportOutcome, error) {
		owner := plots.Projects.Sqlite
		lumpMap, err := loadOptionalLumpMap(ctx, owner.Selection.ProjectPath, request.LumpTable, request.LumpDBPath)
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return GeoPackageExportOutcome{}, err
		}
		defer owner.Mu.Unlock()
		readTx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}
		defer func() { _ = readTx.Rollback() }()

		project := owner.Selection.Project
		_, suByPlot, err := loadScopedPlotSiteUnits(ctx, readTx, owner)
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}

		envCols, envRows, err := readRawTableForGPKG(ctx, readTx, "project", project+"_Env",
			`EXISTS (SELECT 1 FROM USysEnv AS u WHERE typeof(u.PlotNumber)=typeof(t."PlotNumber") AND CAST(u.PlotNumber AS BLOB)=CAST(t."PlotNumber" AS BLOB))`,
			`t."PlotNumber"`)
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}
		adminCols, adminRows, err := readRawTableForGPKG(ctx, readTx, "project", project+"_Admin",
			`EXISTS (SELECT 1 FROM USysEnv AS u WHERE typeof(u.PlotNumber)=typeof(t."Plot") AND CAST(u.PlotNumber AS BLOB)=CAST(t."Plot" AS BLOB))`,
			`t."Plot"`)
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}

		var vegObs []longVegObservation
		var humusCols, mineralCols []gpkgColSpec
		var humusRows, mineralRows [][]any
		if request.IncludeAttributeTables {
			sppMeta, mErr := loadSpeciesLookupMeta(ctx, readTx)
			if mErr != nil {
				return GeoPackageExportOutcome{}, mErr
			}
			vegObs, err = extractLongVegObservations(ctx, readTx, owner, suByPlot, sppMeta, lumpMap)
			if err != nil {
				return GeoPackageExportOutcome{}, err
			}
			humusCols, humusRows, err = readRawTableForGPKG(ctx, readTx, "project", project+"_Humus",
				`EXISTS (SELECT 1 FROM USysEnv AS u WHERE typeof(u.PlotNumber)=typeof(t."PlotNumber") AND CAST(u.PlotNumber AS BLOB)=CAST(t."PlotNumber" AS BLOB))`,
				`t."PlotNumber", t."UpperDepth" DESC, t."ID"`)
			if err != nil {
				return GeoPackageExportOutcome{}, err
			}
			mineralCols, mineralRows, err = readRawTableForGPKG(ctx, readTx, "project", project+"_Mineral",
				`EXISTS (SELECT 1 FROM USysEnv AS u WHERE typeof(u.PlotNumber)=typeof(t."PlotNumber") AND CAST(u.PlotNumber AS BLOB)=CAST(t."PlotNumber" AS BLOB))`,
				`t."PlotNumber", t."UpperDepth" ASC, t."ID"`)
			if err != nil {
				return GeoPackageExportOutcome{}, err
			}
		}
		if err := readTx.Commit(); err != nil {
			return GeoPackageExportOutcome{}, err
		}

		// Write staged .gpkg SQLite file
		tmpFile, err := os.CreateTemp(filepath.Dir(absDest), ".vpro-gpkg-*.gpkg")
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}
		tmpPath := tmpFile.Name()
		published := false
		defer func() {
			if !published {
				_ = os.Remove(tmpPath)
			}
		}()
		_ = tmpFile.Close()

		gpkgDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}
		gpkgDB.SetMaxOpenConns(1)

		// Set GeoPackage 1.3.1 magic header PRAGMAs before creating tables
		if _, err := gpkgDB.ExecContext(ctx, `PRAGMA application_id = 1196444487; PRAGMA user_version = 10300;`); err != nil {
			gpkgDB.Close()
			return GeoPackageExportOutcome{}, err
		}

		tx, err := gpkgDB.BeginTx(ctx, nil)
		if err != nil {
			gpkgDB.Close()
			return GeoPackageExportOutcome{}, err
		}

		outcome, buildErr := populateGeoPackageTx(ctx, tx, targetSRSID, crs, request, suByPlot, envCols, envRows, adminCols, adminRows, vegObs, humusCols, humusRows, mineralCols, mineralRows)
		if buildErr != nil {
			_ = tx.Rollback()
			_ = gpkgDB.Close()
			return GeoPackageExportOutcome{}, buildErr
		}
		if err := tx.Commit(); err != nil {
			_ = gpkgDB.Close()
			return GeoPackageExportOutcome{}, err
		}
		if err := gpkgDB.Close(); err != nil {
			return GeoPackageExportOutcome{}, err
		}

		if err := os.Rename(tmpPath, absDest); err != nil {
			return GeoPackageExportOutcome{}, err
		}
		published = true

		fileBytes, err := os.ReadFile(absDest)
		if err != nil {
			return GeoPackageExportOutcome{}, err
		}
		sum := sha256.Sum256(fileBytes)
		outcome.Path = absDest
		outcome.SHA256 = hex.EncodeToString(sum[:])
		return outcome, nil
	})
}

type gpkgColSpec struct {
	Name     string
	GPKGType string
	IsBool   bool
	IsText   bool
}

func mapSQLiteToGPKGType(decl string) (string, bool, bool) {
	u := strings.ToUpper(strings.TrimSpace(decl))
	switch {
	case strings.Contains(u, "BOOL") || strings.Contains(u, "BIT"):
		return "BOOLEAN", true, false
	case strings.Contains(u, "SMALLINT") || strings.Contains(u, "INT"):
		return "INTEGER", false, false
	case strings.Contains(u, "DOUB") || strings.Contains(u, "REAL") || strings.Contains(u, "FLOA") || strings.Contains(u, "SING") || strings.Contains(u, "NUM"):
		return "DOUBLE", false, false
	default:
		return "TEXT", false, true
	}
}

func readRawTableForGPKG(ctx context.Context, tx *sql.Tx, alias, table, whereClause, orderClause string) ([]gpkgColSpec, [][]any, error) {
	pRows, err := tx.QueryContext(ctx, `PRAGMA `+vprocore.QuoteHeaderIdentifier(alias)+`.table_info(`+vprocore.QuoteHeaderIdentifier(table)+`)`)
	if err != nil {
		return nil, nil, err
	}
	var specs []gpkgColSpec
	for pRows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt any
		if err := pRows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			pRows.Close()
			return nil, nil, err
		}
		gType, isBool, isText := mapSQLiteToGPKGType(ctype)
		specs = append(specs, gpkgColSpec{Name: name, GPKGType: gType, IsBool: isBool, IsText: isText})
	}
	if err := pRows.Close(); err != nil {
		return nil, nil, err
	}

	exprs := make([]string, len(specs))
	for i, sp := range specs {
		q := `t.` + vprocore.QuoteHeaderIdentifier(sp.Name)
		if sp.IsBool {
			exprs[i] = `CASE WHEN ` + q + ` IS NULL THEN NULL WHEN typeof(` + q + `)='text' THEN ` + q + ` ELSE CAST(` + q + ` AS INTEGER) END`
		} else if sp.IsText {
			exprs[i] = `CASE WHEN ` + q + ` IS NULL THEN NULL ELSE CAST(` + q + ` AS TEXT) END`
		} else {
			exprs[i] = q
		}
	}

	sqlStr := `SELECT ` + strings.Join(exprs, ", ") + ` FROM ` + vprocore.QuoteHeaderIdentifier(alias) + `.` + vprocore.QuoteHeaderIdentifier(table) + ` AS t`
	if whereClause != "" {
		sqlStr += ` WHERE ` + whereClause
	}
	if orderClause != "" {
		sqlStr += ` ORDER BY ` + orderClause
	}

	rows, err := tx.QueryContext(ctx, sqlStr)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var data [][]any
	for rows.Next() {
		vals := make([]any, len(specs))
		ptrs := make([]any, len(specs))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		for i, sp := range specs {
			if vals[i] == nil {
				continue
			}
			if sp.IsBool {
				if isAccessTruthy(vals[i]) {
					vals[i] = int64(1)
				} else {
					vals[i] = int64(0)
				}
			} else if b, ok := vals[i].([]byte); ok {
				vals[i] = string(b)
			}
		}
		data = append(data, vals)
	}
	return specs, data, rows.Err()
}

func asFloat64Value(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	default:
		return 0, false
	}
}

func populateGeoPackageTx(
	ctx context.Context,
	tx *sql.Tx,
	targetSRSID int32,
	crs string,
	request GeoPackageExportRequest,
	suByPlot map[string]string,
	envCols []gpkgColSpec,
	envRows [][]any,
	adminCols []gpkgColSpec,
	adminRows [][]any,
	vegObs []longVegObservation,
	humusCols []gpkgColSpec,
	humusRows [][]any,
	mineralCols []gpkgColSpec,
	mineralRows [][]any,
) (GeoPackageExportOutcome, error) {
	outcome := GeoPackageExportOutcome{
		CRS:             crs,
		SRSID:           targetSRSID,
		AttributeTables: []string{},
	}

	// 1. OGC GeoPackage 1.3 system metadata tables
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE gpkg_spatial_ref_sys (
			srs_name TEXT NOT NULL,
			srs_id INTEGER NOT NULL PRIMARY KEY,
			organization TEXT NOT NULL,
			organization_coordsys_id INTEGER NOT NULL,
			definition TEXT NOT NULL,
			description TEXT
		);
		CREATE TABLE gpkg_contents (
			table_name TEXT NOT NULL PRIMARY KEY,
			data_type TEXT NOT NULL,
			identifier TEXT UNIQUE,
			description TEXT DEFAULT '',
			last_change DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			min_x DOUBLE,
			min_y DOUBLE,
			max_x DOUBLE,
			max_y DOUBLE,
			srs_id INTEGER,
			CONSTRAINT fk_gc_r_srs_id FOREIGN KEY (srs_id) REFERENCES gpkg_spatial_ref_sys(srs_id)
		);
		CREATE TABLE gpkg_geometry_columns (
			table_name TEXT NOT NULL,
			column_name TEXT NOT NULL,
			geometry_type_name TEXT NOT NULL,
			srs_id INTEGER NOT NULL,
			z TINYINT NOT NULL,
			m TINYINT NOT NULL,
			CONSTRAINT pk_geom_cols PRIMARY KEY (table_name, column_name),
			CONSTRAINT fk_gc_tn FOREIGN KEY (table_name) REFERENCES gpkg_contents(table_name),
			CONSTRAINT fk_gc_srs FOREIGN KEY (srs_id) REFERENCES gpkg_spatial_ref_sys(srs_id)
		);
	`); err != nil {
		return outcome, fmt.Errorf("create gpkg metadata tables: %w", err)
	}

	srsEntries := []struct {
		name string
		id   int32
		org  string
		code int32
		def  string
		desc string
	}{
		{"Undefined Cartesian SRS", -1, "NONE", -1, "undefined", "undefined cartesian coordinate reference system"},
		{"Undefined Geographic SRS", 0, "NONE", 0, "undefined", "undefined geographic coordinate reference system"},
		{"WGS 84 geodetic", 4326, "EPSG", 4326, wktWGS84, "WGS 84 / NAD83 geographic 2D"},
		{"NAD83", 4269, "EPSG", 4269, wktNAD83, "North American Datum 1983"},
		{"NAD83 / BC Albers", 3005, "EPSG", 3005, wktBCAlbers, "BC Ministry of Forests standard Albers Equal-Area Conic"},
	}
	for _, srs := range srsEntries {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO gpkg_spatial_ref_sys (srs_name, srs_id, organization, organization_coordsys_id, definition, description)
			VALUES (?, ?, ?, ?, ?, ?)`,
			srs.name, srs.id, srs.org, srs.code, srs.def, srs.desc); err != nil {
			return outcome, err
		}
	}

	// Index Admin rows by Plot
	adminPlotIdx := -1
	for i, c := range adminCols {
		if strings.EqualFold(c.Name, "Plot") {
			adminPlotIdx = i
			break
		}
	}
	adminByPlot := map[string][]any{}
	if adminPlotIdx >= 0 {
		for _, r := range adminRows {
			if p, ok := r[adminPlotIdx].(string); ok {
				adminByPlot[p] = r
			}
		}
	}

	// Build unified plots feature table columns (Env + non-duplicate Admin + SiteUnit)
	seenCol := map[string]bool{"fid": true, "geom": true, "siteunit": true}
	var mergedSpecs []struct {
		name    string
		gType   string
		fromEnv bool
		idx     int
	}
	envPlotIdx, latIdx, lonIdx := -1, -1, -1
	for i, c := range envCols {
		lower := strings.ToLower(c.Name)
		if lower == "plotnumber" {
			envPlotIdx = i
		} else if lower == "latitude" {
			latIdx = i
		} else if lower == "longitude" {
			lonIdx = i
		}
		if seenCol[lower] {
			continue
		}
		seenCol[lower] = true
		mergedSpecs = append(mergedSpecs, struct {
			name    string
			gType   string
			fromEnv bool
			idx     int
		}{name: c.Name, gType: c.GPKGType, fromEnv: true, idx: i})
	}
	for i, c := range adminCols {
		lower := strings.ToLower(c.Name)
		if lower == "plot" || seenCol[lower] {
			continue
		}
		seenCol[lower] = true
		mergedSpecs = append(mergedSpecs, struct {
			name    string
			gType   string
			fromEnv bool
			idx     int
		}{name: c.Name, gType: c.GPKGType, fromEnv: false, idx: i})
	}

	colDefs := []string{`"fid" INTEGER PRIMARY KEY AUTOINCREMENT`, `"geom" POINT`, `"SiteUnit" TEXT`}
	insertCols := []string{`"geom"`, `"SiteUnit"`}
	placeholders := []string{"?", "?"}
	for _, ms := range mergedSpecs {
		q := vprocore.QuoteHeaderIdentifier(ms.name)
		colDefs = append(colDefs, q+" "+ms.gType)
		insertCols = append(insertCols, q)
		placeholders = append(placeholders, "?")
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE "plots" (`+strings.Join(colDefs, ", ")+`)`); err != nil {
		return outcome, fmt.Errorf("create plots feature table: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO "plots" (`+strings.Join(insertCols, ", ")+`) VALUES (`+strings.Join(placeholders, ", ")+`)`)
	if err != nil {
		return outcome, err
	}
	defer stmt.Close()

	hasBounds := false
	var minX, minY, maxX, maxY float64
	locatedPlotSet := map[string]bool{}

	for _, eRow := range envRows {
		plotNum := ""
		if envPlotIdx >= 0 && eRow[envPlotIdx] != nil {
			plotNum = fmt.Sprint(eRow[envPlotIdx])
		}
		var geomBlob any
		hasValidCoord := false
		if latIdx >= 0 && lonIdx >= 0 && eRow[latIdx] != nil && eRow[lonIdx] != nil {
			latVal, ok1 := asFloat64Value(eRow[latIdx])
			lonVal, ok2 := asFloat64Value(eRow[lonIdx])
			if ok1 && ok2 {
				nLat, nLon, flipped, valid := NormalizeWesternLongitude(latVal, lonVal)
				if valid {
					hasValidCoord = true
					if flipped {
						outcome.NormalizedWestLongitudes++
					}
					var x, y float64
					if targetSRSID == 3005 {
						x, y = ProjectNAD83ToBCAlbers(nLat, nLon)
					} else {
						x, y = nLon, nLat
					}
					geomBlob = EncodeGeoPackagePoint(targetSRSID, x, y)
					if !hasBounds {
						minX, maxX, minY, maxY = x, x, y, y
						hasBounds = true
					} else {
						if x < minX {
							minX = x
						}
						if x > maxX {
							maxX = x
						}
						if y < minY {
							minY = y
						}
						if y > maxY {
							maxY = y
						}
					}
				}
			}
		}

		if !hasValidCoord {
			outcome.SkippedUnlocatedCount++
			if !request.IncludeUnlocatedPlots {
				continue
			}
		} else {
			outcome.LocatedPlotCount++
		}
		locatedPlotSet[plotNum] = true

		aRow := adminByPlot[plotNum]
		args := make([]any, 0, 2+len(mergedSpecs))
		args = append(args, geomBlob, suByPlot[plotNum])
		for _, ms := range mergedSpecs {
			if ms.fromEnv {
				val := eRow[ms.idx]
				// Normalize exported Longitude attribute when positive West was flipped
				if strings.EqualFold(ms.name, "Longitude") && hasValidCoord && val != nil {
					if fv, ok := asFloat64Value(val); ok && fv >= 100 && fv <= 170 {
						val = -fv
					}
				}
				args = append(args, val)
			} else if aRow != nil && ms.idx < len(aRow) {
				args = append(args, aRow[ms.idx])
			} else {
				args = append(args, nil)
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return outcome, fmt.Errorf("insert plot %s into GeoPackage: %w", plotNum, err)
		}
		outcome.FeatureCount++
	}

	nowISO := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	var bMinX, bMinY, bMaxX, bMaxY any
	if hasBounds {
		bMinX, bMinY, bMaxX, bMaxY = minX, minY, maxX, maxY
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, min_x, min_y, max_x, max_y, srs_id)
		VALUES ('plots', 'features', 'plots', 'VPro BEC Ecological Plots (Env + Admin + SiteUnit)', ?, ?, ?, ?, ?, ?)`,
		nowISO, bMinX, bMinY, bMaxX, bMaxY, targetSRSID); err != nil {
		return outcome, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m)
		VALUES ('plots', 'geom', 'POINT', ?, 0, 0)`, targetSRSID); err != nil {
		return outcome, err
	}

	// Optional non-spatial attribute tables (data_type = 'attributes')
	if request.IncludeAttributeTables {
		// 1. veg
		if _, err := tx.ExecContext(ctx, `
			CREATE TABLE "veg" (
				"fid" INTEGER PRIMARY KEY AUTOINCREMENT,
				"PlotNumber" TEXT NOT NULL,
				"SiteUnit" TEXT,
				"Layer" TEXT NOT NULL,
				"Species" TEXT NOT NULL,
				"ScientificName" TEXT,
				"Lifeform" INTEGER,
				"Cover" DOUBLE NOT NULL,
				"Height" DOUBLE,
				"Collected" TEXT
			)`); err != nil {
			return outcome, err
		}
		vegStmt, err := tx.PrepareContext(ctx, `INSERT INTO "veg" ("PlotNumber","SiteUnit","Layer","Species","ScientificName","Lifeform","Cover","Height","Collected") VALUES (?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return outcome, err
		}
		for _, vo := range vegObs {
			if !request.IncludeUnlocatedPlots && !locatedPlotSet[vo.PlotNumber] {
				continue
			}
			var lfAny, htAny any
			if vo.Lifeform != nil {
				lfAny = *vo.Lifeform
			}
			if vo.Height != nil {
				htAny = *vo.Height
			}
			if _, err := vegStmt.ExecContext(ctx, vo.PlotNumber, vo.SiteUnit, vo.Layer, vo.Species, vo.ScientificName, lfAny, vo.Cover, htAny, vo.Collected); err != nil {
				vegStmt.Close()
				return outcome, err
			}
		}
		vegStmt.Close()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, srs_id)
			VALUES ('veg', 'attributes', 'veg', 'VPro Long Vegetation Observations', ?, 0)`, nowISO); err != nil {
			return outcome, err
		}
		outcome.AttributeTables = append(outcome.AttributeTables, "veg")

		// 2. humus & 3. mineral
		for _, attrTbl := range []struct {
			name string
			desc string
			cols []gpkgColSpec
			rows [][]any
		}{
			{"humus", "VPro Soil Humus Horizons", humusCols, humusRows},
			{"mineral", "VPro Soil Mineral Horizons", mineralCols, mineralRows},
		} {
			if err := writeAttributeTableGPKG(ctx, tx, attrTbl.name, attrTbl.desc, nowISO, attrTbl.cols, attrTbl.rows, locatedPlotSet, request.IncludeUnlocatedPlots); err != nil {
				return outcome, err
			}
			outcome.AttributeTables = append(outcome.AttributeTables, attrTbl.name)
		}
	}

	return outcome, nil
}

func writeAttributeTableGPKG(
	ctx context.Context,
	tx *sql.Tx,
	tableName, desc, nowISO string,
	cols []gpkgColSpec,
	rows [][]any,
	locatedPlots map[string]bool,
	includeUnlocated bool,
) error {
	defs := []string{`"fid" INTEGER PRIMARY KEY AUTOINCREMENT`}
	insCols := make([]string, 0, len(cols))
	marks := make([]string, 0, len(cols))
	plotIdx := -1
	for i, c := range cols {
		if strings.EqualFold(c.Name, "PlotNumber") {
			plotIdx = i
		}
		colOut := c.Name
		if strings.EqualFold(colOut, "fid") {
			colOut = "src_fid"
		}
		q := vprocore.QuoteHeaderIdentifier(colOut)
		defs = append(defs, q+" "+c.GPKGType)
		insCols = append(insCols, q)
		marks = append(marks, "?")
	}
	qTbl := vprocore.QuoteHeaderIdentifier(tableName)
	if _, err := tx.ExecContext(ctx, `CREATE TABLE `+qTbl+` (`+strings.Join(defs, ", ")+`)`); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO `+qTbl+` (`+strings.Join(insCols, ", ")+`) VALUES (`+strings.Join(marks, ", ")+`)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if !includeUnlocated && plotIdx >= 0 && r[plotIdx] != nil {
			if !locatedPlots[fmt.Sprint(r[plotIdx])] {
				continue
			}
		}
		if _, err := stmt.ExecContext(ctx, r...); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, srs_id)
		VALUES (?, 'attributes', ?, ?, ?, 0)`, tableName, tableName, desc, nowISO)
	return err
}
