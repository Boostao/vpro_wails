package cloudsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// ColumnSpec defines a canonical column across VP08 SQLite and PostgreSQL schemas.
type ColumnSpec struct {
	Name           string
	PGType         string // "TEXT" | "BIGINT" | "DOUBLE PRECISION" | "SMALLINT"
	SQLiteType     string
	IsAccessBool   bool
	IsSingleCover  bool
	UTF16MaxLength int
}

func quotePGIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func columnDDL(col ColumnSpec) string {
	ddl := quotePGIdent(col.Name) + " " + col.PGType
	if col.IsAccessBool {
		ddl += fmt.Sprintf(" CHECK (%s IN (-1, 0))", quotePGIdent(col.Name))
	}
	return ddl
}

func columnNames(cols []ColumnSpec) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	return names
}

func quotedColumnCSV(cols []ColumnSpec) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = quotePGIdent(c.Name)
	}
	return strings.Join(parts, ", ")
}

// sqliteSelectColumnsCSV wraps PGType=="TEXT" columns in CAST(... AS BLOB) when typeof='text'
// so mattn/go-sqlite3 never parses TIMESTAMP-declared text cells into Go time.Time (+0000 UTC / 0001-01-01).
func sqliteSelectColumnsCSV(cols []ColumnSpec) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		q := quotePGIdent(c.Name)
		if c.PGType == "TEXT" {
			parts[i] = fmt.Sprintf("CASE WHEN typeof(%s) = 'text' THEN CAST(%s AS BLOB) ELSE %s END", q, q, q)
		} else {
			parts[i] = q
		}
	}
	return strings.Join(parts, ", ")
}

// NarrowSingleCover narrows float64 to IEEE-754 float32 BEFORE evaluating < 100.
func NarrowSingleCover(value float64) (float32, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || math.Abs(value) > math.MaxFloat32 {
		return 0, false
	}
	narrowed := float32(value)
	if narrowed < 0 || narrowed >= 100 {
		return narrowed, false
	}
	return narrowed, true
}

func utf16CodeUnits(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func ValidateUTF16Bound(label, s string, maxUnits int) error {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return fmt.Errorf("%s contains malformed UTF-8 or NUL byte", label)
	}
	if maxUnits > 0 && utf16CodeUnits(s) > maxUnits {
		return fmt.Errorf("%s exceeds %d UTF-16 code units", label, maxUnits)
	}
	return nil
}

// ValidateLMH25HorizonDepth enforces LMH25 directional soil horizon depth rules:
// Upland L/F/H humus horizons measure upward from 0 cm (UpperDepth > LowerDepth);
// Wetland Of/Om/Oh and Mineral A/B/C/R/W measure downward (UpperDepth < LowerDepth).
func ValidateLMH25HorizonDepth(tableSuffix, horizon string, upper, lower *float64) error {
	if upper == nil || lower == nil {
		return nil
	}
	u, l := *upper, *lower
	if math.IsNaN(u) || math.IsInf(u, 0) || math.IsNaN(l) || math.IsInf(l, 0) {
		return fmt.Errorf("%s depth contains NaN/Inf", tableSuffix)
	}
	switch tableSuffix {
	case "Humus":
		hz := strings.ToUpper(strings.TrimSpace(horizon))
		if strings.HasPrefix(hz, "OF") || strings.HasPrefix(hz, "OM") || strings.HasPrefix(hz, "OH") {
			if u >= l {
				return fmt.Errorf("LMH25 wetland %s horizon requires UpperDepth < LowerDepth (got %g..%g)", strings.TrimSpace(horizon), u, l)
			}
		} else if strings.HasPrefix(hz, "L") || strings.HasPrefix(hz, "F") || strings.HasPrefix(hz, "H") {
			if u <= l {
				return fmt.Errorf("LMH25 upland %s horizon requires UpperDepth > LowerDepth (got %g..%g)", strings.TrimSpace(horizon), u, l)
			}
		}
	case "Mineral":
		if u >= l {
			return fmt.Errorf("LMH25 mineral %s horizon requires UpperDepth < LowerDepth (got %g..%g)", strings.TrimSpace(horizon), u, l)
		}
	}
	return nil
}

// CanonicalizeSQLiteToPG converts a dynamic SQLite value into a typed PostgreSQL parameter
// preserving byte-exact text, IEEE-754 float64 bit identity, 64-bit integers, and Access -1/0 booleans.
func CanonicalizeSQLiteToPG(col ColumnSpec, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	if col.IsAccessBool {
		switch v := raw.(type) {
		case int64:
			if v != 0 {
				return int16(-1), nil
			}
			return int16(0), nil
		case int:
			if v != 0 {
				return int16(-1), nil
			}
			return int16(0), nil
		case float64:
			if v != 0 {
				return int16(-1), nil
			}
			return int16(0), nil
		case bool:
			if v {
				return int16(-1), nil
			}
			return int16(0), nil
		case []byte:
			return parseBoolBytes(string(v))
		case string:
			return parseBoolBytes(v)
		default:
			return nil, fmt.Errorf("unsupported boolean value %T for %s", raw, col.Name)
		}
	}

	switch col.PGType {
	case "TEXT":
		switch v := raw.(type) {
		case string:
			if !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
				return nil, fmt.Errorf("column %s contains invalid UTF-8", col.Name)
			}
			return v, nil
		case []byte:
			s := string(v)
			if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
				return nil, fmt.Errorf("column %s contains invalid UTF-8 bytes", col.Name)
			}
			return s, nil
		case time.Time:
			if v.IsZero() {
				return nil, nil
			}
			if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
				return v.Format("2006-01-02"), nil
			}
			return v.Format("2006-01-02 15:04:05"), nil
		case int64:
			return strconv.FormatInt(v, 10), nil
		case float64:
			return strconv.FormatFloat(v, 'g', -1, 64), nil
		case bool:
			if v {
				return "-1", nil
			}
			return "0", nil
		default:
			return fmt.Sprint(v), nil
		}

	case "BIGINT", "SMALLINT":
		var iv int64
		switch v := raw.(type) {
		case int64:
			iv = v
		case int:
			iv = int64(v)
		case int16:
			iv = int64(v)
		case int32:
			iv = int64(v)
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, nil
			}
			iv = int64(math.Round(v))
		case bool:
			if v {
				iv = -1
			}
		case []byte:
			s := strings.TrimSpace(string(v))
			if s == "" {
				return nil, nil
			}
			parsed, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				f, ferr := strconv.ParseFloat(s, 64)
				if ferr != nil {
					return nil, nil
				}
				iv = int64(math.Round(f))
			} else {
				iv = parsed
			}
		case string:
			s := strings.TrimSpace(v)
			if s == "" {
				return nil, nil
			}
			parsed, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				f, ferr := strconv.ParseFloat(s, 64)
				if ferr != nil {
					return nil, nil
				}
				iv = int64(math.Round(f))
			} else {
				iv = parsed
			}
		default:
			return nil, fmt.Errorf("unsupported integer value %T for %s", raw, col.Name)
		}
		if col.PGType == "SMALLINT" {
			if iv < math.MinInt16 || iv > math.MaxInt16 {
				return nil, fmt.Errorf("column %s value %d overflows SMALLINT", col.Name, iv)
			}
			return int16(iv), nil
		}
		return iv, nil

	case "DOUBLE PRECISION":
		switch v := raw.(type) {
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, nil
			}
			return v, nil
		case float32:
			f := float64(v)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, nil
			}
			return f, nil
		case int64:
			return float64(v), nil
		case int:
			return float64(v), nil
		case []byte:
			s := strings.TrimSpace(string(v))
			if s == "" {
				return nil, nil
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, nil
			}
			return f, nil
		case string:
			s := strings.TrimSpace(v)
			if s == "" {
				return nil, nil
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, nil
			}
			return f, nil
		default:
			return nil, fmt.Errorf("unsupported float value %T for %s", raw, col.Name)
		}
	}
	return raw, nil
}

func parseBoolBytes(raw string) (any, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	if strings.EqualFold(s, "true") || s == "-1" || s == "1" {
		return int16(-1), nil
	}
	if strings.EqualFold(s, "false") || s == "0" {
		return int16(0), nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid boolean literal %q", raw)
	}
	if n != 0 {
		return int16(-1), nil
	}
	return int16(0), nil
}

// CanonicalizePGToSQLite normalizes a value scanned from PostgreSQL for insertion into SQLite VP08.
func CanonicalizePGToSQLite(col ColumnSpec, raw any) any {
	if raw == nil {
		return nil
	}
	if col.IsAccessBool {
		switch v := raw.(type) {
		case int16:
			if v != 0 {
				return int64(-1)
			}
			return int64(0)
		case int32:
			if v != 0 {
				return int64(-1)
			}
			return int64(0)
		case int64:
			if v != 0 {
				return int64(-1)
			}
			return int64(0)
		case bool:
			if v {
				return int64(-1)
			}
			return int64(0)
		}
	}
	switch v := raw.(type) {
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case float32:
		return float64(v)
	default:
		return raw
	}
}

const GenesisAuditHash = "0000000000000000000000000000000000000000000000000000000000000000"

// ComputeAuditRowHash computes a deterministic tamper-evident SHA-256 chain link over a VP08 Audit record.
func ComputeAuditRowHash(prevHash, mergeTxID string, values []any) string {
	h := sha256.New()
	writePart := func(s string) {
		_, _ = h.Write([]byte(strconv.Itoa(len(s))))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte("|"))
	}
	writePart(prevHash)
	writePart(mergeTxID)
	for _, val := range values {
		if val == nil {
			writePart("<NULL>")
			continue
		}
		switch v := val.(type) {
		case string:
			writePart("S:" + v)
		case int16:
			writePart("I:" + strconv.FormatInt(int64(v), 10))
		case int32:
			writePart("I:" + strconv.FormatInt(int64(v), 10))
		case int64:
			writePart("I:" + strconv.FormatInt(v, 10))
		case int:
			writePart("I:" + strconv.Itoa(v))
		default:
			writePart("V:" + fmt.Sprint(v))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func tCol(name, sqliteType string, maxUTF16 int) ColumnSpec {
	return ColumnSpec{Name: name, PGType: "TEXT", SQLiteType: sqliteType, UTF16MaxLength: maxUTF16}
}

func iCol(name, sqliteType string) ColumnSpec {
	return ColumnSpec{Name: name, PGType: "BIGINT", SQLiteType: sqliteType}
}

func fCol(name, sqliteType string, isCover bool) ColumnSpec {
	return ColumnSpec{Name: name, PGType: "DOUBLE PRECISION", SQLiteType: sqliteType, IsSingleCover: isCover}
}

func bCol(name string) ColumnSpec {
	return ColumnSpec{Name: name, PGType: "SMALLINT", SQLiteType: "BOOLEAN", IsAccessBool: true}
}

// VP08EnvColumns defines all 112 canonical VP08 <Project>_Env columns ("PlotNumber" + 111 columns).
var VP08EnvColumns = []ColumnSpec{
	tCol("PlotNumber", "VARCHAR", 7),
	tCol("FieldNumber", "VARCHAR", 50),
	tCol("ProjectID", "VARCHAR", 30),
	tCol("FSRegionDistrict", "VARCHAR", 7),
	tCol("Date", "TIMESTAMP", 0),
	tCol("SiteSurveyor", "VARCHAR", 0),
	tCol("PlotRepresenting", "VARCHAR", 0),
	tCol("Location", "VARCHAR", 0),
	tCol("Ecosection", "VARCHAR", 3),
	tCol("NtsMapSheet", "VARCHAR", 8),
	fCol("Longitude", "DOUBLE", false),
	fCol("Latitude", "DOUBLE", false),
	tCol("UTMZone", "VARCHAR", 0),
	fCol("UTMEasting", "REAL", false),
	fCol("UTMNorthing", "REAL", false),
	iCol("LocationAccuracy", "SMALLINT"),
	tCol("AirPhotoNum", "VARCHAR", 20),
	fCol("XCoord", "REAL", false),
	fCol("YCoord", "REAL", false),
	tCol("Zone", "VARCHAR", 4),
	tCol("SubZone", "VARCHAR", 8),
	tCol("SiteSeries", "VARCHAR", 5),
	tCol("SiteModifier1", "VARCHAR", 0),
	tCol("SiteModifier2", "VARCHAR", 0),
	tCol("TransDistrib", "VARCHAR", 0),
	tCol("RealmClass", "VARCHAR", 5),
	tCol("MapUnit", "VARCHAR", 30),
	tCol("SnowCoverregime", "VARCHAR", 1),
	tCol("MoistureRegime", "VARCHAR", 0),
	tCol("NutrientRegime", "VARCHAR", 0),
	tCol("SuccessionalStatus", "VARCHAR", 0),
	tCol("StructuralStage", "VARCHAR", 0),
	tCol("StructuralStageMod", "VARCHAR", 0),
	iCol("StandAge", "SMALLINT"),
	iCol("Elevation", "SMALLINT"),
	fCol("SlopeGradient", "REAL", false),
	iCol("Aspect", "SMALLINT"),
	tCol("MesoSlopePosition", "VARCHAR", 0),
	tCol("SurfaceShape", "VARCHAR", 0),
	tCol("SurfaceTopographyType", "VARCHAR", 0),
	tCol("SurfaceTopographySize", "VARCHAR", 0),
	tCol("WaterSource", "VARCHAR", 5),
	tCol("Photo", "VARCHAR", 50),
	tCol("Exposure1", "VARCHAR", 2),
	tCol("Exposure2", "VARCHAR", 2),
	tCol("SiteDisturbance1", "VARCHAR", 8),
	tCol("SiteDisturbance2", "VARCHAR", 8),
	tCol("SiteDisturbance3", "VARCHAR", 8),
	fCol("SubstrateDecWood", "REAL", false),
	fCol("SubstrateBedRock", "REAL", false),
	fCol("SubstrateRocks", "REAL", false),
	fCol("SubstrateMineralSoil", "REAL", false),
	fCol("SubstrateOrganicMatter", "REAL", false),
	fCol("SubstrateWater", "REAL", false),
	tCol("SiteNotes", "TEXT", 0),
	tCol("SoilSurveyor", "VARCHAR", 30),
	tCol("BedrockGeology1", "VARCHAR", 4),
	tCol("BedrockGeology2", "VARCHAR", 4),
	tCol("BedrockGeology3", "VARCHAR", 4),
	tCol("CoarseFragLith1", "VARCHAR", 12),
	tCol("CoarseFragLith2", "VARCHAR", 12),
	tCol("CoarseFragLith3", "VARCHAR", 12),
	tCol("TerrainTextureSurf", "VARCHAR", 3),
	tCol("SurficialMaterialSurf", "VARCHAR", 6),
	tCol("SurfaceExpSurf", "VARCHAR", 3),
	tCol("GeoMorProSurf", "VARCHAR", 3),
	tCol("TerrainTextureSubSurf", "VARCHAR", 3),
	tCol("SurficialMaterialSubSurf", "VARCHAR", 6),
	tCol("SurfaceExpSubSurf", "VARCHAR", 3),
	tCol("GeoMorProSubSurf", "VARCHAR", 3),
	tCol("FloodingRegimeFreq", "VARCHAR", 7),
	tCol("MoistureRegimeSub", "VARCHAR", 0),
	tCol("FloodingRegimeDur", "VARCHAR", 2),
	tCol("SoilDrainage", "VARCHAR", 5),
	iCol("SeepageDepth", "SMALLINT"),
	tCol("RootRestrictingType", "VARCHAR", 1),
	iCol("RootRestrictingDepth", "SMALLINT"),
	tCol("RootZoneParticleSize", "VARCHAR", 6),
	iCol("RootingDepth", "SMALLINT"),
	tCol("SoilClassSubGroup", "VARCHAR", 4),
	tCol("SoilClassGroup", "VARCHAR", 4),
	tCol("HumusForm", "VARCHAR", 4),
	tCol("HumusFormPhase", "VARCHAR", 50),
	tCol("pHMethodCodeMineral", "VARCHAR", 0),
	tCol("pHMethodCodeOrganic", "VARCHAR", 0),
	tCol("SoilNotes", "TEXT", 0),
	tCol("VegSurveyor", "VARCHAR", 30),
	fCol("StrataCoverTree", "REAL", true),
	fCol("StrataCoverShrub", "REAL", true),
	fCol("StrataCoverHerb", "REAL", true),
	fCol("StrataCoverMoss", "REAL", true),
	tCol("VegNotes", "TEXT", 0),
	tCol("HydroGeoSystem", "VARCHAR", 3),
	tCol("HydroGeoSubSystem", "VARCHAR", 2),
	bCol("SpeciesListComplete"),
	tCol("Temporary", "VARCHAR", 0),
	bCol("Flag"),
	tCol("SV_PolygonNumber", "VARCHAR", 0),
	bCol("SV_FloodPlain"),
	tCol("SV_StandAgeEstMeas", "VARCHAR", 0),
	fCol("SV_StandHeight", "REAL", false),
	tCol("SV_StandHeightEstMeas", "VARCHAR", 0),
	tCol("SV_CanopyComposition", "VARCHAR", 0),
	fCol("SV_SoilDepth", "REAL", false),
	tCol("SV_RootZoneTexture", "VARCHAR", 0),
	fCol("SV_PercentCoarseFrags", "REAL", false),
	fCol("SV_GleyingMottlingCM", "REAL", false),
	fCol("SV_WaterTableCM", "REAL", false),
	tCol("SV_FullCruiseCard", "VARCHAR", 0),
	tCol("SV_AhorizonType", "VARCHAR", 0),
	fCol("SV_AhorizonDepth", "REAL", false),
	fCol("ActiveLayerDepth", "REAL", false),
}

// VP08AdminColumns defines all 20 canonical VP08 <Project>_Admin columns ("Plot" + 19 columns).
var VP08AdminColumns = []ColumnSpec{
	tCol("Plot", "VARCHAR", 7),
	iCol("StartDate", "SMALLINT"),
	tCol("PlotType", "VARCHAR", 10),
	fCol("PlotSize", "REAL", false),
	tCol("ProvinceStateTerritory", "VARCHAR", 0),
	tCol("SitePlotQuality", "VARCHAR", 15),
	tCol("VegPlotQuality", "VARCHAR", 15),
	tCol("SoilPlotQuality", "VARCHAR", 15),
	bCol("UpdatedFromCards"),
	tCol("EnteredBy", "VARCHAR", 50),
	tCol("UserSiteUnit", "VARCHAR", 100),
	tCol("BECSiteUnit", "VARCHAR", 100),
	tCol("SiteUnitShortName", "VARCHAR", 0),
	tCol("SiteUnitLongName", "VARCHAR", 0),
	tCol("OfficeNotes", "TEXT", 0),
	fCol("HumusThickness", "REAL", false),
	tCol("GIS_BGC", "VARCHAR", 0),
	iCol("GIS_BGC_VER", "SMALLINT"),
	tCol("BEC_Use", "VARCHAR", 0),
	fCol("StrataCoverTotal", "REAL", true),
}

// VP08VegColumns defines all 44 canonical VP08 <Project>_Veg columns.
var VP08VegColumns = []ColumnSpec{
	tCol("PlotNumber", "VARCHAR", 7),
	tCol("Species", "VARCHAR", 8),
	tCol("Layer", "VARCHAR", 2),
	fCol("Cover1", "REAL", true),
	fCol("Height1", "REAL", false),
	fCol("Cover2", "REAL", true),
	fCol("Height2", "REAL", false),
	fCol("Cover3", "REAL", true),
	fCol("Height3", "REAL", false),
	fCol("TotalA", "REAL", true),
	fCol("HeightA", "REAL", false),
	fCol("Cover4", "REAL", true),
	fCol("Height4", "REAL", false),
	fCol("Cover5", "REAL", true),
	fCol("Height5", "REAL", false),
	fCol("Cover5a", "REAL", true),
	fCol("Height5a", "REAL", false),
	fCol("Cover5b", "REAL", true),
	fCol("Height5b", "REAL", false),
	fCol("Cover5c", "REAL", true),
	fCol("Height5c", "REAL", false),
	fCol("TotalB", "REAL", true),
	tCol("HeightB", "VARCHAR", 0),
	fCol("Cover6", "REAL", true),
	fCol("Height6", "REAL", false),
	fCol("Cover7", "REAL", true),
	fCol("Cover8", "REAL", true),
	fCol("Cover9", "REAL", true),
	fCol("Cover10", "REAL", true),
	tCol("Collected", "VARCHAR", 1),
	bCol("Flag"),
	iCol("ID", "INTEGER"),
	iCol("LL", "INTEGER"),
	iCol("AF", "SMALLINT"),
	iCol("DC", "SMALLINT"),
	iCol("UT", "SMALLINT"),
	iCol("VI", "SMALLINT"),
	iCol("PV", "INTEGER"),
	iCol("PG", "SMALLINT"),
	iCol("FFA", "SMALLINT"),
	iCol("Cultural1", "SMALLINT"),
	iCol("Cultural2", "SMALLINT"),
	iCol("Other1", "SMALLINT"),
	iCol("Other2", "SMALLINT"),
}

// VP08HumusColumns defines all 18 canonical VP08 <Project>_Humus columns.
var VP08HumusColumns = []ColumnSpec{
	tCol("PlotNumber", "VARCHAR", 7),
	tCol("Horizon", "VARCHAR", 8),
	fCol("UpperDepth", "REAL", false),
	fCol("LowerDepth", "REAL", false),
	tCol("HumusStructureDegree", "VARCHAR", 0),
	tCol("HumusStructureKind", "VARCHAR", 0),
	tCol("MycelAbundance", "VARCHAR", 0),
	tCol("FecalAbundance", "VARCHAR", 0),
	tCol("RootsAbundance", "VARCHAR", 0),
	tCol("RootsSize", "VARCHAR", 0),
	iCol("vonPost", "SMALLINT"),
	fCol("HumusFormpH", "REAL", false),
	tCol("Consistence", "VARCHAR", 0),
	tCol("Character", "VARCHAR", 0),
	tCol("Fauna", "VARCHAR", 0),
	tCol("Comment", "TEXT", 0),
	bCol("Flag"),
	iCol("ID", "INTEGER"),
}

// VP08MineralColumns defines all 28 canonical VP08 <Project>_Mineral columns.
var VP08MineralColumns = []ColumnSpec{
	tCol("PlotNumber", "VARCHAR", 7),
	tCol("Horizon", "VARCHAR", 8),
	fCol("UpperDepth", "REAL", false),
	fCol("LowerDepth", "REAL", false),
	tCol("PitDepthLimit", "VARCHAR", 0),
	tCol("Colour", "VARCHAR", 14),
	iCol("ASP", "SMALLINT"),
	tCol("Texture", "VARCHAR", 4),
	iCol("PercentCoarseFragsGravel", "SMALLINT"),
	iCol("PercentCoarseFragsCobbles", "SMALLINT"),
	iCol("PercentCoarseFragsStones", "SMALLINT"),
	iCol("PercentCoarseFragsTotal", "SMALLINT"),
	tCol("PercentCoarseFragsShape", "VARCHAR", 0),
	tCol("RootsAbundance", "VARCHAR", 0),
	tCol("RootsSize", "VARCHAR", 0),
	tCol("MineralStructureClass", "VARCHAR", 0),
	tCol("MineralStructureKind", "VARCHAR", 0),
	fCol("MineralFormpH", "REAL", false),
	tCol("MottlesAbundance", "VARCHAR", 0),
	tCol("MottlesSize", "VARCHAR", 0),
	tCol("MottlesContrast", "VARCHAR", 0),
	tCol("ClayFilmsFreq", "VARCHAR", 0),
	tCol("ClayFilmThickness", "VARCHAR", 0),
	tCol("Effervescence", "VARCHAR", 0),
	tCol("Porosity", "VARCHAR", 0),
	tCol("Comments", "TEXT", 0),
	bCol("Flag"),
	iCol("ID", "INTEGER"),
}

// VP08OtherColumns defines all 11 canonical VP08 <Project>_Other columns.
var VP08OtherColumns = []ColumnSpec{
	tCol("PlotNumber", "VARCHAR", 7),
	tCol("DataName", "VARCHAR", 0),
	tCol("DataItem", "VARCHAR", 0),
	tCol("UserItem1", "VARCHAR", 0),
	tCol("UserItem2", "VARCHAR", 0),
	tCol("UserItem3", "VARCHAR", 0),
	bCol("UserFlag1"),
	bCol("UserFlag2"),
	bCol("UserFlag3"),
	bCol("Flag"),
	iCol("ID", "INTEGER"),
}

// VP08MetadataColumns defines all 75 canonical VP08 <Project>_Metadata columns ("ProjectID" + 74 columns).
var VP08MetadataColumns = []ColumnSpec{
	tCol("ProjectID", "VARCHAR", 30),
	iCol("StartDate", "SMALLINT"),
	iCol("EndDate", "SMALLINT"),
	tCol("ProjectTitle", "VARCHAR", 0),
	tCol("CoordinatingAgency", "VARCHAR", 0),
	tCol("ProponentFunder", "VARCHAR", 0),
	tCol("FieldCompanyAgency", "VARCHAR", 0),
	tCol("FieldLeader", "VARCHAR", 0),
	tCol("FieldDataCollectionTeam", "VARCHAR", 0),
	tCol("ProjectPurpose", "VARCHAR", 0),
	tCol("GeographicStudyArea", "VARCHAR", 0),
	tCol("GeographicStudyRegion", "VARCHAR", 0),
	iCol("NumberOfFS882Plots", "INTEGER"),
	iCol("NumberOfSiteVisits", "SMALLINT"),
	tCol("ProjectType", "VARCHAR", 0),
	tCol("ProjectTypeOther", "VARCHAR", 0),
	tCol("EcosysCollectionStandard", "VARCHAR", 0),
	tCol("EcosysCollectionStandardOther", "VARCHAR", 0),
	tCol("VegCoverMethod", "VARCHAR", 0),
	tCol("VegCoverMethodOther", "VARCHAR", 0),
	tCol("PlotMethod", "VARCHAR", 0),
	tCol("PlotMethodOther", "VARCHAR", 0),
	tCol("MensurationMethod", "VARCHAR", 0),
	tCol("MensurationMethodOther", "VARCHAR", 0),
	tCol("ExtraVegFieldDescription", "VARCHAR", 0),
	tCol("DataCustodian", "VARCHAR", 0),
	tCol("StorageLocation", "VARCHAR", 0),
	iCol("CollectedSite", "SMALLINT"),
	tCol("DataQualitySite", "VARCHAR", 0),
	iCol("CollectedVeg", "SMALLINT"),
	tCol("DataQualityVeg", "VARCHAR", 0),
	iCol("CollectedSoil", "SMALLINT"),
	tCol("DataQualitySoil", "VARCHAR", 0),
	iCol("CollectedTerrain", "SMALLINT"),
	tCol("DataQualityTerrain", "VARCHAR", 0),
	iCol("CollectedMens", "SMALLINT"),
	tCol("DataQualityMens", "VARCHAR", 0),
	iCol("CollectedCWD", "SMALLINT"),
	tCol("DataQualityCWD", "VARCHAR", 0),
	iCol("CollectedWildTree", "SMALLINT"),
	tCol("DataQualityWildTree", "VARCHAR", 0),
	iCol("CollectedSoilChem", "SMALLINT"),
	tCol("DataQualitySoilChem", "VARCHAR", 0),
	iCol("CollectedWildlifeHabitatAssessment", "SMALLINT"),
	tCol("DataQualityWildlifeHabitatAssessment", "VARCHAR", 0),
	tCol("CollectedCompleteOther", "VARCHAR", 0),
	tCol("CollectedPartialOther", "VARCHAR", 0),
	tCol("CollectedNoneOther", "VARCHAR", 0),
	tCol("GeoRefMethod", "VARCHAR", 0),
	tCol("GeoRefMethodOther", "VARCHAR", 0),
	tCol("Datum", "VARCHAR", 0),
	tCol("DatumOther", "VARCHAR", 0),
	tCol("CoordinateSystem", "VARCHAR", 0),
	tCol("CoordinateSystemOther", "VARCHAR", 0),
	tCol("AllSpecs", "VARCHAR", 0),
	tCol("TableOfLists", "VARCHAR", 0),
	tCol("CoverA1Description", "VARCHAR", 0),
	tCol("CoverA2Description", "VARCHAR", 0),
	tCol("CoverA3Description", "VARCHAR", 0),
	tCol("CoverADescription", "VARCHAR", 0),
	tCol("CoverB1Description", "VARCHAR", 0),
	tCol("CoverB2Description", "VARCHAR", 0),
	tCol("CoverB2aDescription", "VARCHAR", 0),
	tCol("CoverB2bDescription", "VARCHAR", 0),
	tCol("CoverB2cDescription", "VARCHAR", 0),
	tCol("CoverBDescription", "VARCHAR", 0),
	tCol("CoverCDescription", "VARCHAR", 0),
	tCol("CoverDDescription", "VARCHAR", 0),
	tCol("Cover8Description", "VARCHAR", 0),
	tCol("Cover9Description", "VARCHAR", 0),
	tCol("Cover10Description", "VARCHAR", 0),
	iCol("BAPID", "INTEGER"),
	tCol("DateLastEdited", "TIMESTAMP", 0),
	tCol("Notes", "TEXT", 0),
	iCol("ID", "INTEGER"),
}

// VP08AuditColumns defines all 11 canonical VP08 <Project>_Audit columns.
var VP08AuditColumns = []ColumnSpec{
	tCol("Project", "VARCHAR", 0),
	tCol("User", "VARCHAR", 0),
	tCol("PlotNumber", "VARCHAR", 0),
	tCol("Table", "VARCHAR", 0),
	tCol("EditField", "VARCHAR", 0),
	tCol("EditWhen", "TIMESTAMP", 0),
	tCol("BeforeEdit", "TEXT", 0),
	tCol("AfterEdit", "TEXT", 0),
	bCol("Restore"),
	bCol("Flag"),
	iCol("ID", "INTEGER"),
}

func VP08ColumnsBySuffix(suffix string) []ColumnSpec {
	switch suffix {
	case "Env":
		return VP08EnvColumns
	case "Admin":
		return VP08AdminColumns
	case "Veg":
		return VP08VegColumns
	case "Humus":
		return VP08HumusColumns
	case "Mineral":
		return VP08MineralColumns
	case "Other":
		return VP08OtherColumns
	case "Metadata":
		return VP08MetadataColumns
	case "Audit":
		return VP08AuditColumns
	default:
		return nil
	}
}

// CustodianProtectedAdminFields are BECMaster classification columns gated by custodian review.
var CustodianProtectedAdminFields = map[string]bool{
	"BECSiteUnit": true,
	"BEC_Use":     true,
	"GIS_BGC":     true,
}

// Standards table column specifications (Schedule A 3.g)

var StandardsUSysAllSpecsColumns = []ColumnSpec{
	tCol("YearOfChange", "TEXT", 0),
	tCol("ChangeDescription", "TEXT", 0),
	tCol("APCFamily", "TEXT", 0),
	tCol("FamilyCode", "TEXT", 0),
	tCol("OldCode", "TEXT", 0),
	tCol("Code", "TEXT", 0),
	tCol("SplCode", "TEXT", 0),
	tCol("TreeCode", "TEXT", 0),
	tCol("ScientificName", "TEXT", 0),
	iCol("Lifeform", "INTEGER"),
	tCol("Authority", "TEXT", 0),
	tCol("EnglishName_Old", "TEXT", 0),
	tCol("EnglishName", "TEXT", 0),
	tCol("common_name_pb", "TEXT", 0),
	tCol("Notes_on_names", "TEXT", 0),
	tCol("ShortenedGuideName", "TEXT", 0),
	tCol("Native", "TEXT", 0),
	tCol("Codetype", "TEXT", 0), // TEXT preserves 2-char 'XX' codes safely
	tCol("Province", "TEXT", 0),
	iCol("ID", "INTEGER"),
	tCol("TaxonLevel", "TEXT", 0),
	iCol("IsAttribute", "BOOLEAN"),
	iCol("Report", "INTEGER"),
	tCol("SplCode_calc", "TEXT", 0),
	iCol("Codelength", "INTEGER"),
	tCol("CombinedEnglishName", "TEXT", 0),
}

var StandardsUSysTableOfListsColumns = []ColumnSpec{
	tCol("ListName", "TEXT", 0),
	tCol("ListFilter", "TEXT", 0),
	fCol("ItemOrder", "REAL", false),
	tCol("Item", "TEXT", 0),
	tCol("ItemDescription", "TEXT", 0),
	tCol("FieldUsedIn", "TEXT", 0),
	tCol("ValidateLoops", "TEXT", 0),
	iCol("Validate", "BOOLEAN"),
	tCol("Note", "TEXT", 0),
	iCol("Flag", "BOOLEAN"),
}

var StandardsBECSiteSeriesColumns = []ColumnSpec{
	tCol("RowID", "TEXT", 0),
	tCol("SourceID", "TEXT", 0),
	tCol("SSCode", "TEXT", 0),
	tCol("Name", "TEXT", 0),
	tCol("Region", "TEXT", 0),
	tCol("Zone", "TEXT", 0),
	tCol("SubZone", "TEXT", 0),
	tCol("BaseSubZone", "TEXT", 0),
	tCol("Variant", "TEXT", 0),
	tCol("Phase", "TEXT", 0),
	tCol("SiteSeries", "TEXT", 0),
	tCol("SiteSeriesPhase", "TEXT", 0),
	tCol("Variation", "TEXT", 0),
	tCol("Seral", "TEXT", 0),
	tCol("Description", "TEXT", 0),
	tCol("PlantAssociation", "TEXT", 0),
	tCol("Comments", "TEXT", 0),
	fCol("ReferenceID", "REAL", false),
	tCol("AddedDate", "TEXT", 0),
	tCol("ExpiredDate", "TEXT", 0),
	tCol("OriginalSourceID", "TEXT", 0),
	tCol("MergedBGC", "TEXT", 0),
	tCol("Suballiance", "TEXT", 0),
	tCol("Alliance", "TEXT", 0),
	tCol("MissingNpeNa", "TEXT", 0),
	tCol("TransferID", "TEXT", 0),
	iCol("Flag", "BOOLEAN"),
}

var StandardsMasterSiteUnitListColumns = []ColumnSpec{
	iCol("ID", "INTEGER"),
	tCol("SiteSeries", "TEXT", 0),
	tCol("SiteSeriesLongName", "TEXT", 0),
	tCol("SiteSeriesScientificName", "TEXT", 0),
	iCol("Level", "INTEGER"),
}

var StandardsUSysZoneListColumns = []ColumnSpec{
	tCol("Zone", "TEXT", 0),
	tCol("SubZone", "TEXT", 0),
	tCol("ZoneDescription", "TEXT", 0),
	tCol("SubZoneVarDescription", "TEXT", 0),
}

var StandardsProjectMetadataColumns = []ColumnSpec{
	tCol("ProjectID", "TEXT", 0),
	tCol("ProjectTitle", "TEXT", 0),
	tCol("CoordinatingAgency", "TEXT", 0),
	tCol("ProponentFunder", "TEXT", 0),
	tCol("FieldCompanyAgency", "TEXT", 0),
	tCol("FieldLeader", "TEXT", 0),
	tCol("FieldDataCollectionTeam", "TEXT", 0),
	tCol("ProjectPurpose", "TEXT", 0),
	tCol("GeographicStudyArea", "TEXT", 0),
	tCol("GeographicStudyRegion", "TEXT", 0),
	tCol("GeographicStudyDistrict", "TEXT", 0),
	tCol("StartDate", "TIMESTAMP", 0),
	tCol("EndDate", "TIMESTAMP", 0),
	iCol("NumberOfFS882Plots", "INTEGER"),
	iCol("NumberOfSiteVisits", "INTEGER"),
	iCol("CollectedSite", "INTEGER"),
	iCol("CollectedVeg", "INTEGER"),
	iCol("CollectedSoil", "INTEGER"),
	iCol("CollectedTerrain", "INTEGER"),
	iCol("CollectedMens", "INTEGER"),
	iCol("CollectedCWD", "INTEGER"),
	iCol("CollectedWildTree", "INTEGER"),
	iCol("CollectedSoilChem", "INTEGER"),
	iCol("CollectedOther", "INTEGER"),
	tCol("CollectedCompleteOtherName", "TEXT", 0),
	tCol("CollectedPartialOtherName", "TEXT", 0),
	tCol("CollectedNoneOtherName", "TEXT", 0),
	iCol("GeoRefMethod", "INTEGER"),
	iCol("Map", "INTEGER"),
	iCol("Datum", "INTEGER"),
	iCol("CoordinateSystem", "INTEGER"),
	iCol("EcosysCollectionStandard", "INTEGER"),
	tCol("EcosysCollectionStandardOtherName", "TEXT", 0),
	iCol("VegCoverMethod", "INTEGER"),
	tCol("VegCoverMethodOtherName", "TEXT", 0),
	iCol("MensurationMethod", "INTEGER"),
	tCol("MensurationMethodOtherName", "TEXT", 0),
	tCol("ExtraVegFieldDescription", "TEXT", 0),
	iCol("PlotMethod", "INTEGER"),
	tCol("DataCustodian", "TEXT", 0),
	tCol("StorageLocation", "TEXT", 0),
	tCol("Notes", "TEXT", 0),
}
