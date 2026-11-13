package interchange

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

var tvFallbackUUIDSeq uint64

func newTurboVegUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		seq := atomic.AddUint64(&tvFallbackUUIDSeq, 1)
		now := uint64(time.Now().UnixNano())
		for i := 0; i < 8; i++ {
			b[i] = byte(now >> (8 * i))
			b[8+i] = byte(seq >> (8 * i))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}

func truncateUTF16Units(s string, maxUnits int) string {
	if maxUnits <= 0 || s == "" {
		return s
	}
	units := utf16.Encode([]rune(s))
	if len(units) <= maxUnits {
		return s
	}
	cut := maxUnits
	if cut > 0 && units[cut-1] >= 0xD800 && units[cut-1] <= 0xDBFF {
		cut--
	}
	if cut == 0 {
		return ""
	}
	return string(utf16.Decode(units[:cut]))
}

func nullTruncatedUTF16(s string, maxUnits int) any {
	trimmed := strings.TrimSpace(s)
	if maxUnits > 0 {
		trimmed = truncateUTF16Units(trimmed, maxUnits)
	}
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func validateFiniteSingleFloat(v float64, label string, nonNegative bool) (float64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > math.MaxFloat32 || (nonNegative && v < 0) {
		return 0, fmt.Errorf("%s %v violates IEEE-754 SINGLE bounds", label, v)
	}
	return float64(float32(v)), nil
}

func validateUTF8Text(s, label string) error {
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return fmt.Errorf("%s contains malformed UTF-8 or NUL byte", label)
	}
	return nil
}
func nullEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func accumulateImportedSingleCover(covers map[string]float64, col string, obsCov float64, plotNum, species string) error {
	if math.IsNaN(obsCov) || math.IsInf(obsCov, 0) || obsCov < 0 || math.Abs(obsCov) > math.MaxFloat32 || float32(obsCov) >= 100 {
		return fmt.Errorf("plot %s species %s cover %v violates IEEE-754 SINGLE [0, 100) bounds", plotNum, species, obsCov)
	}
	obsSingle := float64(float32(obsCov))
	prev, hasPrev := covers[col]
	if !hasPrev {
		covers[col] = obsSingle
		return nil
	}
	sumSingle := float32(prev + obsSingle)
	if sumSingle >= 100 {
		covers[col] = 99.0
	} else {
		covers[col] = float64(sumSingle)
	}
	return nil
}

const (
	TurboVegFormatXML    = "xml"
	TurboVegFormatSQLite = "sqlite"
)

// Official Alterra Turboveg 3 fixed vegetationlayers (0..10, non-extensible per Alterra specification).
type tvVegetationLayerDef struct {
	ID           int
	Name         string
	Abbreviation string
	Description  string
}

var canonicalTV3VegetationLayers = []tvVegetationLayerDef{
	{0, "-", "-", "no layer defined"},
	{1, "Tree layer 1", "t1", "Tree layer 1"},
	{2, "Tree layer 2", "t2", "Tree layer 2"},
	{3, "Tree layer 3", "t3", "Tree layer 3"},
	{4, "Shrub layer 1", "s1", "Shrub layer 1"},
	{5, "Shrub layer 2", "s2", "Shrub layer 2"},
	{6, "Herb layer", "hl", "Herb layer"},
	{7, "Juvenile layer", "jl", "Juvenile layer"},
	{8, "Seedling layer", "sl", "Seedling layer"},
	{9, "Cryptogam layer", "ml", "Cryptogam layer"},
	{10, "Epiphytes", "ep", "Epiphytes"},
}

type turboVegLayerCrosswalk struct {
	VProLayer   string
	TVLayerID   int
	TVAbbr      string
	VProCover   string
	VProHeight  string
	IsPrimary   bool
	Description string
}

var turboVegCrosswalk = []turboVegLayerCrosswalk{
	{"A1", 1, "t1", "Cover1", "Height1", true, "Dominant tree layer (A1 -> t1)"},
	{"A2", 2, "t2", "Cover2", "Height2", true, "Main canopy tree layer (A2 -> t2)"},
	{"A3", 3, "t3", "Cover3", "Height3", true, "Sub-canopy tree layer (A3 -> t3)"},
	{"A", 1, "t1", "TotalA", "HeightA", false, "Combined tree stratum (TotalA -> t1 + VPRO_LAYER=A)"},
	{"B1", 4, "s1", "Cover4", "Height4", true, "Tall shrub layer (B1 -> s1)"},
	{"B2", 5, "s2", "Cover5", "Height5", true, "Low shrub layer (B2 -> s2)"},
	{"B2a", 5, "s2", "Cover5a", "Height5a", false, "CHARS extended shrub sub-layer (B2a -> s2 + VPRO_LAYER=B2a)"},
	{"B2b", 5, "s2", "Cover5b", "Height5b", false, "CHARS extended shrub sub-layer (B2b -> s2 + VPRO_LAYER=B2b)"},
	{"B2c", 5, "s2", "Cover5c", "Height5c", false, "CHARS extended shrub sub-layer (B2c -> s2 + VPRO_LAYER=B2c)"},
	{"B", 4, "s1", "TotalB", "HeightB", false, "Combined shrub stratum (TotalB -> s1 + VPRO_LAYER=B)"},
	{"C", 6, "hl", "Cover6", "Height6", true, "Herbaceous layer (C -> hl)"},
	{"D", 9, "ml", "Cover7", "", true, "Terrestrial moss/bryoid layer (D -> ml)"},
	{"Dw", 9, "ml", "Cover8", "", false, "Epixylic bryoid layer on decaying wood (Dw -> ml + VPRO_LAYER=Dw)"},
	{"Dr", 9, "ml", "Cover9", "", false, "Epilithic bryoid layer on rock (Dr -> ml + VPRO_LAYER=Dr)"},
	{"Ep", 10, "ep", "Cover10", "", true, "Epiphyte layer on trees/shrubs (Ep -> ep)"},
}

var vproSublayerCodeToTV = func() map[string]turboVegLayerCrosswalk {
	m := make(map[string]turboVegLayerCrosswalk, len(turboVegCrosswalk))
	for _, cw := range turboVegCrosswalk {
		m[cw.VProLayer] = cw
	}
	return m
}()

func isWoodyVProLifeform(lf *int) bool {
	if lf == nil {
		return false
	}
	switch *lf {
	case 1, 2, 3, 4, 12:
		return true
	default:
		return false
	}
}

func taxonGroupForLifeform(lf *int) int {
	if lf == nil {
		return 1
	}
	switch *lf {
	case 9, 10:
		return 3 // Mosses & liverworts
	case 11:
		return 4 // Lichens
	case 13:
		return 6 // Algae
	default:
		return 1 // Vascular plants
	}
}

func taxonRankForSciName(sci string) int {
	lower := strings.ToLower(sci)
	if strings.Contains(lower, " var. ") {
		return 4 // Variety
	}
	if strings.Contains(lower, " ssp. ") || strings.Contains(lower, " subsp. ") {
		return 3 // Subspecies
	}
	if strings.HasSuffix(lower, " spp.") || strings.HasSuffix(lower, " sp.") || !strings.Contains(strings.TrimSpace(sci), " ") {
		return 1 // Genus
	}
	return 2 // Species
}

func resolveIncomingTurboVegLayer(tvLayer, vproLayer string, isWoody bool) (coverCol, heightCol string) {
	switch strings.ToUpper(strings.TrimSpace(vproLayer)) {
	case "A1":
		return "Cover1", "Height1"
	case "A2":
		return "Cover2", "Height2"
	case "A3":
		return "Cover3", "Height3"
	case "A", "TOTALA":
		return "TotalA", "HeightA"
	case "B1":
		return "Cover4", "Height4"
	case "B2":
		return "Cover5", "Height5"
	case "B2A", "5A":
		return "Cover5a", "Height5a"
	case "B2B", "5B":
		return "Cover5b", "Height5b"
	case "B2C", "5C":
		return "Cover5c", "Height5c"
	case "B", "TOTALB":
		return "TotalB", "HeightB"
	case "C":
		return "Cover6", "Height6"
	case "D", "DH":
		return "Cover7", ""
	case "DW":
		return "Cover8", ""
	case "DR":
		return "Cover9", ""
	case "EP":
		return "Cover10", ""
	}

	switch strings.ToLower(strings.TrimSpace(tvLayer)) {
	case "1", "t1":
		return "Cover1", "Height1"
	case "2", "t2":
		return "Cover2", "Height2"
	case "3", "t3":
		return "Cover3", "Height3"
	case "4", "s1":
		return "Cover4", "Height4"
	case "5", "s2":
		return "Cover5", "Height5"
	case "6", "hl":
		return "Cover6", "Height6"
	case "7", "jl":
		if isWoody {
			return "Cover5", "Height5"
		}
		return "Cover6", "Height6"
	case "8", "sl":
		return "Cover6", "Height6"
	case "9", "ml":
		return "Cover7", ""
	case "10", "ep":
		return "Cover10", ""
	default:
		return "Cover6", "Height6"
	}
}

type TurboVegExportRequest struct {
	Format          string `json:"format"` // "xml" | "sqlite"
	DestinationPath string `json:"destinationPath"`
	LumpTable       string `json:"lumpTable,omitempty"`
	LumpDBPath      string `json:"lumpDbPath,omitempty"`
}

type TurboVegExportOutcome struct {
	Format           string `json:"format"`
	Path             string `json:"path"`
	SHA256           string `json:"sha256"`
	PlotCount        int    `json:"plotCount"`
	SpeciesCount     int    `json:"speciesCount"`
	ObservationCount int    `json:"observationCount"`
}

type TurboVegImportRequest struct {
	SourcePath        string `json:"sourcePath"`
	Format            string `json:"format,omitempty"` // "xml" | "sqlite" (inferred from extension if omitted)
	TargetProject     string `json:"targetProject"`
	DestinationPath   string `json:"destinationPath,omitempty"`
	ActivateOnSuccess bool   `json:"activateOnSuccess"`
}

type TurboVegImportOutcome struct {
	SourcePath       string                 `json:"sourcePath"`
	Format           string                 `json:"format"`
	TargetProject    string                 `json:"targetProject"`
	DestinationPath  string                 `json:"destinationPath"`
	DestinationSHA   string                 `json:"destinationSha256"`
	PlotCount        int                    `json:"plotCount"`
	SpeciesCount     int                    `json:"speciesCount"`
	ObservationCount int                    `json:"observationCount"`
	Activated        bool                   `json:"activated"`
	State            *vprocore.ProjectState `json:"state,omitempty"`
}

type tvPlotHeader struct {
	PlotID            int
	GUID              string
	PlotNumber        string
	FieldNumber       string
	ProjectID         string
	Date              string
	Latitude          *float64
	Longitude         *float64
	Elevation         *int
	Slope             *float64
	Aspect            *int
	PlotSize          *float64
	Zone              string
	SubZone           string
	SiteSeries        string
	SiteUnit          string
	MoistureRegime    string
	NutrientRegime    string
	MesoSlopePosition string
	SurfaceShape      string
	SoilDrainage      string
	HumusForm         string
	Surveyor          string
	Location          string
	SiteNotes         string
	VegNotes          string
	SoilNotes         string
}

type tvSpeciesObs struct {
	SpeciesNR   int
	SpeciesCode string
	SciName     string
	TVLayerID   int
	TVLayerRaw  string
	VProLayer   string
	IsWoody     bool
	Cover       float64
	Height      *float64
	Collected   string
}

type tvDataset struct {
	ProjectName string
	Species     []tvSpeciesItem
	Plots       []tvPlotBundle
}

type tvSpeciesItem struct {
	SpeciesNR      int
	SpeciesCode    string
	LetterCode     string
	ScientificName string
	EnglishName    string
	Lifeform       *int
	Authority      string
}

type tvPlotBundle struct {
	Header       tvPlotHeader
	Observations []tvSpeciesObs
}

type speciesResolvedInfo struct {
	Code     string
	Lifeform *int
}

type speciesImportResolver struct {
	byActiveCode  map[string]speciesResolvedInfo
	bySynonymCode map[string]speciesResolvedInfo
	byUserCode    map[string]speciesResolvedInfo
	bySciFull     map[string]speciesResolvedInfo
	bySciCanon    map[string]speciesResolvedInfo
}

func normalizeScientificFull(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "×", "x")
	s = strings.ReplaceAll(s, "subsp.", "ssp.")
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// canonicalScientificTaxon extracts normalized "genus species [ssp.|var.|f. epithet]"
// stripping trailing botanical authorities or "agg." / "s.l." qualifiers when present.
func canonicalScientificTaxon(raw string) string {
	norm := normalizeScientificFull(raw)
	if norm == "" {
		return ""
	}
	tokens := strings.Fields(norm)
	if len(tokens) == 1 {
		return tokens[0]
	}
	if tokens[1] == "x" && len(tokens) >= 3 {
		base := tokens[0] + " x " + tokens[2]
		if len(tokens) >= 5 && (tokens[3] == "ssp." || tokens[3] == "var." || tokens[3] == "f.") {
			return base + " " + tokens[3] + " " + tokens[4]
		}
		return base
	}
	if tokens[1] == "spp." || tokens[1] == "sp." {
		return tokens[0] + " spp."
	}
	base := tokens[0] + " " + tokens[1]
	for i := 2; i+1 < len(tokens); i++ {
		if tokens[i] == "ssp." || tokens[i] == "var." || tokens[i] == "f." {
			return base + " " + tokens[i] + " " + tokens[i+1]
		}
	}
	return base
}

func loadSpeciesImportResolver(ctx context.Context, tx *sql.Tx) (*speciesImportResolver, error) {
	r := &speciesImportResolver{
		byActiveCode:  make(map[string]speciesResolvedInfo, 12000),
		bySynonymCode: make(map[string]speciesResolvedInfo, 6000),
		byUserCode:    make(map[string]speciesResolvedInfo, 256),
		bySciFull:     make(map[string]speciesResolvedInfo, 16000),
		bySciCanon:    make(map[string]speciesResolvedInfo, 16000),
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT COALESCE(Code,''), COALESCE(OldCode,''), COALESCE(ScientificName,''), Lifeform, COALESCE(Codetype,'')
		FROM VLists.USysAllSpecs
		WHERE Code IS NOT NULL AND TRIM(Code) <> ''
		ORDER BY CASE
			WHEN UPPER(COALESCE(Codetype,''))='U' THEN 0
			WHEN UPPER(COALESCE(Codetype,''))<>'S' THEN 1
			ELSE 2
		END, rowid`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var code, oldCode, sci, ctype string
		var lf sql.NullInt64
		if err := rows.Scan(&code, &oldCode, &sci, &lf, &ctype); err != nil {
			rows.Close()
			return nil, err
		}
		uCode := strings.ToUpper(strings.TrimSpace(code))
		if uCode == "" {
			continue
		}
		var lfPtr *int
		if lf.Valid {
			v := int(lf.Int64)
			lfPtr = &v
		}
		info := speciesResolvedInfo{Code: uCode, Lifeform: lfPtr}
		isSyn := strings.EqualFold(strings.TrimSpace(ctype), "S")
		if !isSyn {
			if _, exists := r.byActiveCode[uCode]; !exists {
				r.byActiveCode[uCode] = info
			}
		} else {
			uOld := strings.ToUpper(strings.TrimSpace(oldCode))
			if uOld != "" {
				if _, exists := r.bySynonymCode[uOld]; !exists {
					r.bySynonymCode[uOld] = info
				}
			}
		}
		if full := normalizeScientificFull(sci); full != "" {
			if _, exists := r.bySciFull[full]; !exists {
				r.bySciFull[full] = info
			}
		}
		if canon := canonicalScientificTaxon(sci); canon != "" {
			if _, exists := r.bySciCanon[canon]; !exists {
				r.bySciCanon[canon] = info
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	uRows, err := tx.QueryContext(ctx, `
		SELECT COALESCE(Code,''), COALESCE(ScientificName,''), LifeForm, COALESCE(Codetype,'')
		FROM VUser.USysUserSpp
		WHERE Code IS NOT NULL AND TRIM(Code) <> ''`)
	if err != nil {
		return nil, err
	}
	for uRows.Next() {
		var code, sci, ctype string
		var lf sql.NullInt64
		if err := uRows.Scan(&code, &sci, &lf, &ctype); err != nil {
			uRows.Close()
			return nil, err
		}
		if strings.EqualFold(strings.TrimSpace(ctype), "S") {
			continue
		}
		uCode := strings.ToUpper(strings.TrimSpace(code))
		if uCode == "" {
			continue
		}
		var lfPtr *int
		if lf.Valid {
			v := int(lf.Int64)
			lfPtr = &v
		}
		info := speciesResolvedInfo{Code: uCode, Lifeform: lfPtr}
		if _, exists := r.byUserCode[uCode]; !exists {
			r.byUserCode[uCode] = info
		}
		if full := normalizeScientificFull(sci); full != "" {
			if _, exists := r.bySciFull[full]; !exists {
				r.bySciFull[full] = info
			}
		}
		if canon := canonicalScientificTaxon(sci); canon != "" {
			if _, exists := r.bySciCanon[canon]; !exists {
				r.bySciCanon[canon] = info
			}
		}
	}
	if err := uRows.Err(); err != nil {
		uRows.Close()
		return nil, err
	}
	return r, uRows.Close()
}

func (r *speciesImportResolver) resolveTaxon(explicitVProCode, sciName, validSciName, letterCode string) (code string, lifeform *int) {
	if v := strings.ToUpper(strings.TrimSpace(explicitVProCode)); v != "" {
		if r != nil {
			if info, ok := r.byActiveCode[v]; ok {
				return v, info.Lifeform
			}
			if info, ok := r.byUserCode[v]; ok {
				return v, info.Lifeform
			}
			if info, ok := r.bySynonymCode[v]; ok {
				return v, info.Lifeform
			}
		}
		return v, nil
	}

	if r != nil {
		for _, candidateSci := range []string{validSciName, sciName} {
			if full := normalizeScientificFull(candidateSci); full != "" {
				if info, ok := r.bySciFull[full]; ok {
					return info.Code, info.Lifeform
				}
			}
			if canon := canonicalScientificTaxon(candidateSci); canon != "" {
				if info, ok := r.bySciCanon[canon]; ok {
					return info.Code, info.Lifeform
				}
			}
		}
		if lc := strings.ToUpper(strings.TrimSpace(letterCode)); lc != "" {
			if info, ok := r.byActiveCode[lc]; ok {
				return info.Code, info.Lifeform
			}
			if info, ok := r.bySynonymCode[lc]; ok {
				return info.Code, info.Lifeform
			}
			if info, ok := r.byUserCode[lc]; ok {
				return info.Code, info.Lifeform
			}
		}
	}

	if lc := strings.ToUpper(strings.TrimSpace(letterCode)); lc != "" {
		return lc, nil
	}
	for _, candidateSci := range []string{validSciName, sciName} {
		if abbr := abbreviateSpeciesName(candidateSci); abbr != "" {
			return abbr, nil
		}
	}
	return "", nil
}

func enrichExportSpeciesMetaWithOldCodes(ctx context.Context, tx *sql.Tx, sppMeta map[string]speciesLookupMeta) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT COALESCE(OldCode,''), COALESCE(ScientificName,''), COALESCE(EnglishName,''), COALESCE(Authority,''), Lifeform
		FROM VLists.USysAllSpecs
		WHERE OldCode IS NOT NULL AND TRIM(OldCode) <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var oldCode, sci, eng, auth string
		var lf sql.NullInt64
		if err := rows.Scan(&oldCode, &sci, &eng, &auth, &lf); err != nil {
			return err
		}
		uOld := strings.ToUpper(strings.TrimSpace(oldCode))
		if uOld == "" {
			continue
		}
		if _, exists := sppMeta[uOld]; exists {
			continue
		}
		var lfPtr *int
		if lf.Valid {
			v := int(lf.Int64)
			lfPtr = &v
		}
		sppMeta[uOld] = speciesLookupMeta{
			ScientificName: strings.TrimSpace(sci),
			EnglishName:    strings.TrimSpace(eng),
			Authority:      strings.TrimSpace(auth),
			Lifeform:       lfPtr,
		}
	}
	return rows.Err()
}

func extractTVDatasetFromContext(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, lumpMap map[string]string) (tvDataset, int, error) {
	_, suByPlot, err := loadScopedPlotSiteUnits(ctx, tx, owner)
	if err != nil {
		return tvDataset{}, 0, err
	}
	sppMeta, err := loadSpeciesLookupMeta(ctx, tx)
	if err != nil {
		return tvDataset{}, 0, err
	}
	if err := enrichExportSpeciesMetaWithOldCodes(ctx, tx, sppMeta); err != nil {
		return tvDataset{}, 0, err
	}
	obs, err := extractLongVegObservations(ctx, tx, owner, suByPlot, sppMeta, lumpMap)
	if err != nil {
		return tvDataset{}, 0, err
	}

	hRows, err := tx.QueryContext(ctx, `
		SELECT
			PlotNumber,
			COALESCE(FieldNumber,''),
			COALESCE(ProjectID,''),
			COALESCE(CAST("Date" AS TEXT),''),
			Latitude,
			Longitude,
			Elevation,
			SlopeGradient,
			Aspect,
			PlotSize,
			COALESCE("Zone",''),
			COALESCE(SubZone,''),
			COALESCE(SiteSeries,''),
			COALESCE(MoistureRegime,''),
			COALESCE(NutrientRegime,''),
			COALESCE(MesoSlopePosition,''),
			COALESCE(SurfaceShape,''),
			COALESCE(SoilDrainage,''),
			COALESCE(HumusForm,''),
			COALESCE(SiteSurveyor,''),
			COALESCE(Location,''),
			COALESCE(SiteNotes,''),
			COALESCE(VegNotes,''),
			COALESCE(SoilNotes,'')
		FROM USysEnv
		ORDER BY PlotNumber`)
	if err != nil {
		return tvDataset{}, 0, fmt.Errorf("read USysEnv for TurboVeg export: %w", err)
	}
	defer hRows.Close()

	var headers []tvPlotHeader
	plotIdxByNum := map[string]int{}
	for hRows.Next() {
		var h tvPlotHeader
		var lat, lon, slope, pSize sql.NullFloat64
		var elev, aspect sql.NullInt64
		if err := hRows.Scan(
			&h.PlotNumber, &h.FieldNumber, &h.ProjectID, &h.Date,
			&lat, &lon, &elev, &slope, &aspect, &pSize,
			&h.Zone, &h.SubZone, &h.SiteSeries, &h.MoistureRegime, &h.NutrientRegime,
			&h.MesoSlopePosition, &h.SurfaceShape, &h.SoilDrainage, &h.HumusForm,
			&h.Surveyor, &h.Location, &h.SiteNotes, &h.VegNotes, &h.SoilNotes,
		); err != nil {
			return tvDataset{}, 0, err
		}
		if lat.Valid {
			v := lat.Float64
			h.Latitude = &v
		}
		if lon.Valid {
			v := lon.Float64
			h.Longitude = &v
		}
		if elev.Valid {
			v := int(elev.Int64)
			h.Elevation = &v
		}
		if slope.Valid {
			v := slope.Float64
			h.Slope = &v
		}
		if aspect.Valid {
			v := int(aspect.Int64)
			h.Aspect = &v
		}
		if pSize.Valid {
			v := pSize.Float64
			h.PlotSize = &v
		}
		h.SiteUnit = suByPlot[h.PlotNumber]
		h.PlotID = len(headers) + 1
		h.GUID = newTurboVegUUID()
		plotIdxByNum[h.PlotNumber] = len(headers)
		headers = append(headers, h)
	}
	if err := hRows.Err(); err != nil {
		return tvDataset{}, 0, err
	}

	distinctCodes := map[string]struct{}{}
	for _, o := range obs {
		distinctCodes[o.Species] = struct{}{}
	}
	sortedCodes := make([]string, 0, len(distinctCodes))
	for c := range distinctCodes {
		sortedCodes = append(sortedCodes, c)
	}
	slices.Sort(sortedCodes)

	sppNRByCode := make(map[string]int, len(sortedCodes))
	sppDefs := make([]tvSpeciesItem, len(sortedCodes))
	for i, code := range sortedCodes {
		nr := i + 1
		sppNRByCode[code] = nr
		m := sppMeta[code]
		sci := strings.TrimSpace(m.ScientificName)
		if sci == "" {
			sci = code
		}
		sppDefs[i] = tvSpeciesItem{
			SpeciesNR:      nr,
			SpeciesCode:    code,
			LetterCode:     truncateUTF16Units(code, 7),
			ScientificName: sci,
			EnglishName:    m.EnglishName,
			Lifeform:       m.Lifeform,
			Authority:      m.Authority,
		}
	}

	bundles := make([]tvPlotBundle, len(headers))
	for i, h := range headers {
		bundles[i] = tvPlotBundle{Header: h}
	}

	totalObs := 0
	for _, o := range obs {
		pIdx, ok := plotIdxByNum[o.PlotNumber]
		if !ok {
			continue
		}
		cw, ok := vproSublayerCodeToTV[o.Layer]
		tvLayerID := 6
		vproLayer := o.Layer
		if ok {
			tvLayerID = cw.TVLayerID
			vproLayer = cw.VProLayer
		}
		cov := o.Cover
		if float32(cov) >= 100 {
			cov = 99.99
		}
		sci := strings.TrimSpace(o.ScientificName)
		if sci == "" {
			sci = sppMeta[o.Species].ScientificName
		}
		if sci == "" {
			sci = o.Species
		}
		bundles[pIdx].Observations = append(bundles[pIdx].Observations, tvSpeciesObs{
			SpeciesNR:   sppNRByCode[o.Species],
			SpeciesCode: o.Species,
			SciName:     sci,
			TVLayerID:   tvLayerID,
			TVLayerRaw:  strconv.Itoa(tvLayerID),
			VProLayer:   vproLayer,
			IsWoody:     isWoodyVProLifeform(o.Lifeform),
			Cover:       cov,
			Height:      o.Height,
			Collected:   o.Collected,
		})
		totalObs++
	}

	projName := strings.TrimSpace(owner.Selection.Project)
	if projName == "" {
		projName = "VProExport"
	}

	return tvDataset{
		ProjectName: projName,
		Species:     sppDefs,
		Plots:       bundles,
	}, totalObs, nil
}

func ExportTurboVeg(s *vprocore.ContextService, ctx context.Context, contextID string, request TurboVegExportRequest) (TurboVegExportOutcome, error) {
	if strings.TrimSpace(request.DestinationPath) == "" {
		return TurboVegExportOutcome{}, errors.New("TurboVeg export requires a non-empty destinationPath")
	}
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if format == "" {
		if strings.HasSuffix(strings.ToLower(request.DestinationPath), ".xml") {
			format = TurboVegFormatXML
		} else {
			format = TurboVegFormatSQLite
		}
	}
	if format != TurboVegFormatXML && format != TurboVegFormatSQLite {
		return TurboVegExportOutcome{}, fmt.Errorf("unsupported TurboVeg export format %q (expected xml or sqlite)", request.Format)
	}

	dest := strings.TrimSpace(request.DestinationPath)
	if format == TurboVegFormatXML && !strings.HasSuffix(strings.ToLower(dest), ".xml") {
		dest += ".xml"
	} else if format == TurboVegFormatSQLite && !strings.HasSuffix(strings.ToLower(dest), ".sqlite") && !strings.HasSuffix(strings.ToLower(dest), ".db") {
		dest += ".sqlite"
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (TurboVegExportOutcome, error) {
		owner := plots.Projects.Sqlite
		lumpMap, err := loadOptionalLumpMap(ctx, owner.Selection.ProjectPath, request.LumpTable, request.LumpDBPath)
		if err != nil {
			return TurboVegExportOutcome{}, err
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return TurboVegExportOutcome{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return TurboVegExportOutcome{}, err
		}
		defer func() { _ = tx.Rollback() }()

		ds, obsCount, err := extractTVDatasetFromContext(ctx, tx, owner, lumpMap)
		if err != nil {
			return TurboVegExportOutcome{}, err
		}
		if err := tx.Commit(); err != nil {
			return TurboVegExportOutcome{}, err
		}

		var writtenPath, shaHex string
		if format == TurboVegFormatXML {
			xmlBytes, encErr := encodeTurboVegXML(ctx, ds)
			if encErr != nil {
				return TurboVegExportOutcome{}, encErr
			}
			writtenPath, shaHex, err = writeAtomicExportFile(dest, xmlBytes)
			if err != nil {
				return TurboVegExportOutcome{}, err
			}
		} else {
			writtenPath, shaHex, err = writeTurboVeg3SQLite(ctx, dest, ds)
			if err != nil {
				return TurboVegExportOutcome{}, err
			}
		}

		return TurboVegExportOutcome{
			Format:           format,
			Path:             writtenPath,
			SHA256:           shaHex,
			PlotCount:        len(ds.Plots),
			SpeciesCount:     len(ds.Species),
			ObservationCount: obsCount,
		}, nil
	})
}

func loadActiveContextSpeciesResolver(s *vprocore.ContextService, ctx context.Context, contextID string) (*speciesImportResolver, error) {
	if s == nil {
		return nil, nil
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*speciesImportResolver, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback() }()
		res, err := loadSpeciesImportResolver(ctx, tx)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return res, nil
	})
}

func ImportTurboVeg(s *vprocore.ContextService, ctx context.Context, contextID string, request TurboVegImportRequest) (TurboVegImportOutcome, error) {
	src := strings.TrimSpace(request.SourcePath)
	if src == "" || !utf8.ValidString(src) || strings.ContainsRune(src, 0) {
		return TurboVegImportOutcome{}, errors.New("TurboVeg import requires a valid sourcePath")
	}
	absSrc, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return TurboVegImportOutcome{}, err
	}
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if format == "" {
		if strings.EqualFold(filepath.Ext(absSrc), ".xml") {
			format = TurboVegFormatXML
		} else {
			format = TurboVegFormatSQLite
		}
	}
	if format != TurboVegFormatXML && format != TurboVegFormatSQLite {
		return TurboVegImportOutcome{}, fmt.Errorf("unsupported TurboVeg import format %q", format)
	}

	resolver, err := loadActiveContextSpeciesResolver(s, ctx, contextID)
	if err != nil {
		return TurboVegImportOutcome{}, err
	}

	var ds tvDataset
	switch format {
	case TurboVegFormatXML:
		ds, err = parseTurboVegXMLFile(ctx, absSrc, resolver)
	case TurboVegFormatSQLite:
		ds, err = readTurboVeg3SQLiteFile(ctx, absSrc, resolver)
	}
	if err != nil {
		return TurboVegImportOutcome{}, err
	}
	if len(ds.Plots) == 0 {
		return TurboVegImportOutcome{}, errors.New("TurboVeg source contains zero plots")
	}

	sppSet := map[string]struct{}{}
	obsCount := 0
	for _, pb := range ds.Plots {
		for _, o := range pb.Observations {
			sppSet[strings.ToUpper(strings.TrimSpace(o.SpeciesCode))] = struct{}{}
			obsCount++
		}
	}

	destPath, destSHA, activated, newState, err := stageAndPublishVP08Project(
		s, ctx, contextID, request.TargetProject, request.DestinationPath, request.ActivateOnSuccess,
		func(ctx context.Context, tx *sql.Tx, project string) error {
			return populateVP08FromTVDataset(ctx, tx, project, ds)
		},
	)
	if err != nil {
		return TurboVegImportOutcome{}, err
	}

	return TurboVegImportOutcome{
		SourcePath:       absSrc,
		Format:           format,
		TargetProject:    strings.TrimSpace(request.TargetProject),
		DestinationPath:  destPath,
		DestinationSHA:   destSHA,
		PlotCount:        len(ds.Plots),
		SpeciesCount:     len(sppSet),
		ObservationCount: obsCount,
		Activated:        activated,
		State:            newState,
	}, nil
}

func abbreviateSpeciesName(sci string) string {
	parts := strings.Fields(strings.ToUpper(sci))
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return truncateUTF16Units(parts[0], 8)
	}
	g := truncateUTF16Units(parts[0], 4)
	sp := truncateUTF16Units(parts[1], 3)
	return truncateUTF16Units(g+sp, 8)
}

func populateVP08FromTVDataset(ctx context.Context, tx *sql.Tx, project string, ds tvDataset) error {
	envTbl := vprocore.QuoteHeaderIdentifier(project + "_Env")
	adminTbl := vprocore.QuoteHeaderIdentifier(project + "_Admin")
	vegTbl := vprocore.QuoteHeaderIdentifier(project + "_Veg")
	metaTbl := vprocore.QuoteHeaderIdentifier(project + "_Metadata")
	auditTbl := vprocore.QuoteHeaderIdentifier(project + "_Audit")

	envStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+envTbl+` (
			"PlotNumber","FieldNumber","ProjectID","Date","Latitude","Longitude",
			"Elevation","SlopeGradient","Aspect","Zone","SubZone","SiteSeries",
			"MoistureRegime","NutrientRegime","MesoSlopePosition","SurfaceShape",
			"SoilDrainage","HumusForm","SiteSurveyor","Location","SiteNotes","VegNotes","SoilNotes"
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer envStmt.Close()

	adminStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+adminTbl+` ("Plot","PlotType","PlotSize","UserSiteUnit") VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer adminStmt.Close()

	vegColumns := []string{
		"ID", "PlotNumber", "Species", "Collected",
		"Cover1", "Height1", "Cover2", "Height2", "Cover3", "Height3", "TotalA", "HeightA",
		"Cover4", "Height4", "Cover5", "Height5", "Cover5a", "Height5a", "Cover5b", "Height5b", "Cover5c", "Height5c", "TotalB", "HeightB",
		"Cover6", "Height6", "Cover7", "Cover8", "Cover9", "Cover10",
	}
	qVegCols := make([]string, len(vegColumns))
	qMarks := make([]string, len(vegColumns))
	colPos := map[string]int{}
	for i, c := range vegColumns {
		qVegCols[i] = vprocore.QuoteHeaderIdentifier(c)
		qMarks[i] = "?"
		colPos[c] = i
	}
	vegStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+vegTbl+` (`+strings.Join(qVegCols, ", ")+`) VALUES (`+strings.Join(qMarks, ", ")+`)`)
	if err != nil {
		return err
	}
	defer vegStmt.Close()

	var nextVegID int64 = 1
	suRows := [][2]string{}
	seenRawPlots := map[string]bool{}
	reservedLiteralShort := map[string]bool{}
	for _, pb := range ds.Plots {
		rawPlot := strings.TrimSpace(pb.Header.PlotNumber)
		if rawPlot == "" {
			return fmt.Errorf("TurboVeg plot %d has an empty plot number", pb.Header.PlotID)
		}
		if seenRawPlots[rawPlot] {
			return fmt.Errorf("duplicate TurboVeg source plot number %q", rawPlot)
		}
		seenRawPlots[rawPlot] = true
		if len(utf16.Encode([]rune(rawPlot))) <= 7 {
			reservedLiteralShort[rawPlot] = true
		}
	}

	usedTargetPlots := map[string]bool{}
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for plotIdx, pb := range ds.Plots {
		if plotIdx%128 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		h := pb.Header
		rawPlot := strings.TrimSpace(h.PlotNumber)
		for _, tf := range [][2]string{
			{"PlotNumber", h.PlotNumber},
			{"FieldNumber", h.FieldNumber},
			{"ProjectID", h.ProjectID},
			{"Date", h.Date},
			{"Zone", h.Zone},
			{"SubZone", h.SubZone},
			{"SiteSeries", h.SiteSeries},
			{"SiteUnit", h.SiteUnit},
			{"MoistureRegime", h.MoistureRegime},
			{"NutrientRegime", h.NutrientRegime},
			{"MesoSlopePosition", h.MesoSlopePosition},
			{"SurfaceShape", h.SurfaceShape},
			{"SoilDrainage", h.SoilDrainage},
			{"HumusForm", h.HumusForm},
			{"Surveyor", h.Surveyor},
			{"Location", h.Location},
			{"SiteNotes", h.SiteNotes},
			{"VegNotes", h.VegNotes},
			{"SoilNotes", h.SoilNotes},
		} {
			if err := validateUTF8Text(tf[1], tf[0]); err != nil {
				return err
			}
		}
		var slopeSingle *float64
		if h.Slope != nil {
			sv, err := validateFiniteSingleFloat(*h.Slope, "SlopeGradient", true)
			if err != nil {
				return fmt.Errorf("plot %s: %w", rawPlot, err)
			}
			slopeSingle = &sv
		}
		pNum := allocateUniqueTVPlotNumber(rawPlot, usedTargetPlots, reservedLiteralShort)
		usedTargetPlots[pNum] = true

		projID := h.ProjectID
		if strings.TrimSpace(projID) == "" {
			projID = project
		}
		projID = truncateUTF16Units(projID, 20)

		if _, err := envStmt.ExecContext(ctx,
			pNum,
			nullTruncatedUTF16(h.FieldNumber, 50),
			projID,
			nullTruncatedUTF16(h.Date, 40),
			h.Latitude, h.Longitude,
			h.Elevation, slopeSingle, h.Aspect,
			nullTruncatedUTF16(h.Zone, 4),
			nullTruncatedUTF16(h.SubZone, 8),
			nullTruncatedUTF16(h.SiteSeries, 5),
			nullTruncatedUTF16(h.MoistureRegime, 3),
			nullTruncatedUTF16(h.NutrientRegime, 2),
			nullTruncatedUTF16(h.MesoSlopePosition, 3),
			nullTruncatedUTF16(h.SurfaceShape, 3),
			nullTruncatedUTF16(h.SoilDrainage, 5),
			nullTruncatedUTF16(h.HumusForm, 4),
			nullTruncatedUTF16(h.Surveyor, 30),
			nullTruncatedUTF16(h.Location, 255),
			nullEmpty(h.SiteNotes), nullEmpty(h.VegNotes), nullEmpty(h.SoilNotes),
		); err != nil {
			return fmt.Errorf("insert plot %s into _Env: %w", pNum, err)
		}
		if _, err := adminStmt.ExecContext(ctx, pNum, "FS882", h.PlotSize, nullTruncatedUTF16(h.SiteUnit, 100)); err != nil {
			return fmt.Errorf("insert plot %s into _Admin: %w", pNum, err)
		}
		if suUnit := truncateUTF16Units(strings.TrimSpace(h.SiteUnit), 255); suUnit != "" {
			suRows = append(suRows, [2]string{pNum, suUnit})
		}

		type wideRow struct {
			spp       string
			collected string
			covers    map[string]float64
			heights   map[string]float64
		}
		bySpp := map[string]*wideRow{}
		var sppOrder []string
		for _, o := range pb.Observations {
			if err := validateUTF8Text(o.SpeciesCode, "Species"); err != nil {
				return err
			}
			if err := validateUTF8Text(o.Collected, "Collected"); err != nil {
				return err
			}
			code := truncateUTF16Units(strings.ToUpper(strings.TrimSpace(o.SpeciesCode)), 8)
			if code == "" || o.Cover == 0 {
				continue
			}
			wr := bySpp[code]
			if wr == nil {
				wr = &wideRow{spp: code, covers: map[string]float64{}, heights: map[string]float64{}}
				bySpp[code] = wr
				sppOrder = append(sppOrder, code)
			}
			if wr.collected == "" && strings.TrimSpace(o.Collected) != "" {
				wr.collected = truncateUTF16Units(strings.TrimSpace(o.Collected), 1)
			}
			cCol, hCol := resolveIncomingTurboVegLayer(o.TVLayerRaw, o.VProLayer, o.IsWoody)
			if err := accumulateImportedSingleCover(wr.covers, cCol, o.Cover, pNum, code); err != nil {
				return err
			}
			if o.Height != nil && hCol != "" {
				hv, err := validateFiniteSingleFloat(*o.Height, hCol, true)
				if err != nil {
					return fmt.Errorf("plot %s species %s: %w", pNum, code, err)
				}
				if hv > wr.heights[hCol] {
					wr.heights[hCol] = hv
				}
			}
		}

		for _, code := range sppOrder {
			wr := bySpp[code]
			args := make([]any, len(vegColumns))
			args[colPos["ID"]] = nextVegID
			nextVegID++
			args[colPos["PlotNumber"]] = pNum
			args[colPos["Species"]] = wr.spp
			args[colPos["Collected"]] = nullEmpty(wr.collected)
			for cCol, cv := range wr.covers {
				if pos, ok := colPos[cCol]; ok {
					args[pos] = cv
				}
			}
			for hCol, hv := range wr.heights {
				if pos, ok := colPos[hCol]; ok {
					if hCol == "HeightB" {
						args[pos] = strconv.FormatFloat(hv, 'f', -1, 64)
					} else {
						args[pos] = hv
					}
				}
			}
			if _, err := vegStmt.ExecContext(ctx, args...); err != nil {
				return fmt.Errorf("insert veg %s/%s: %w", pNum, wr.spp, err)
			}
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen","Restore","Flag")
			VALUES (?, 'VPro', ?, 'Environment', 'Imported TurboVeg record', ?, 0, 0)`,
			project, pNum, nowStr); err != nil {
			return fmt.Errorf("insert audit for plot %s: %w", pNum, err)
		}
	}

	if len(suRows) > 0 {
		suTbl := vprocore.QuoteHeaderIdentifier(project + "_SU")
		if _, err := tx.ExecContext(ctx, `
			CREATE TABLE `+suTbl+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR);
			CREATE UNIQUE INDEX `+vprocore.QuoteHeaderIdentifier("uidx_"+project+"_SU_PlotNumber")+` ON `+suTbl+`("PlotNumber");
			CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+project+"_SU_SiteUnit")+` ON `+suTbl+`("SiteUnit");
			INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP04');`, project+"_SU"); err != nil {
			return err
		}
		for _, pair := range suRows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+suTbl+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, pair[0], pair[1]); err != nil {
				return err
			}
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO `+metaTbl+` ("ID", "ProjectID", "ProjectTitle", "NumberOfFS882Plots", "VegCoverMethod", "Datum")
		VALUES (1, ?, ?, ?, '1', '2')`,
		project, project+" (TurboVeg Import)", len(usedTargetPlots))
	return err
}

func allocateUniqueTVPlotNumber(rawPlot string, usedTargetPlots, reservedLiteralShort map[string]bool) string {
	cand := truncateUTF16Units(rawPlot, 7)
	if cand != "" && !usedTargetPlots[cand] && (cand == rawPlot || !reservedLiteralShort[cand]) {
		return cand
	}
	for seq := 1; ; seq++ {
		suffix := "~" + strconv.Itoa(seq)
		var disambig string
		if maxPrefix := 7 - len(suffix); maxPrefix > 0 {
			disambig = truncateUTF16Units(rawPlot, maxPrefix) + suffix
		} else {
			b36 := strings.ToUpper(strconv.FormatInt(int64(seq), 36))
			if len(b36) < 7 {
				b36 = strings.Repeat("0", 7-len(b36)) + b36
			}
			disambig = truncateUTF16Units(b36, 7)
		}
		if disambig != "" && !usedTargetPlots[disambig] && !reservedLiteralShort[disambig] {
			return disambig
		}
	}
}
