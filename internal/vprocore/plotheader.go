package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type HeaderField struct {
	Property string `json:"-" yaml:"-"`
	Member   string `json:"-" yaml:"-"`
	Table    string `json:"-" yaml:"-"`
	Column   string `json:"-" yaml:"-"`
}

// Allowlist verified against Forms/FS882-6x4XL.txt ControlSource and
// Tables_Def/Sample_{Env,Admin}_CreateSQL.txt. FIELD NOTES binds SiteNotes;
// Microtop. type/size bind SurfaceTopographyType/Size. optLockData is unbound:
// its AfterUpdate saves the record then toggles AllowEdits on this form/children.
// UTM coordinates and slope are SINGLE/SQLite REAL; StartDate is INTEGER
// (the form labels it "Yr."). LocationAccuracy is INTEGER, not a text code.
// All 98 unique parent ControlSource columns plus the two Arctic extended site
// columns (SnowCoverregime, ActiveLayerDepth) exist in the packaged Env/Admin
// schema. Metadata-only and unbound/computed controls are not header fields.
var HeaderFields = []HeaderField{
	{"plotNumber", "PlotNumber", "Env", "PlotNumber"},
	{"projectId", "ProjectID", "Env", "ProjectID"},
	{"surveyor", "Surveyor", "Env", "SiteSurveyor"},
	{"date", "Date", "Env", "Date"},
	{"fieldNo", "FieldNo", "Env", "FieldNumber"},
	{"generalLocation", "GeneralLocation", "Env", "Location"},
	{"mapSheet", "MapSheet", "Env", "NtsMapSheet"},
	{"utmZone", "UTMZone", "Env", "UTMZone"},
	{"easting", "Easting", "Env", "UTMEasting"},
	{"northing", "Northing", "Env", "UTMNorthing"},
	{"accuracy", "Accuracy", "Env", "LocationAccuracy"},
	{"plotRepresenting", "PlotRepresenting", "Env", "PlotRepresenting"},
	{"siteSeries", "SiteSeries", "Env", "SiteSeries"},
	{"transition", "Transition", "Env", "TransDistrib"},
	{"mapUnit", "MapUnit", "Env", "MapUnit"},
	{"moistureRegime", "MoistureRegime", "Env", "MoistureRegime"},
	{"nutrientRegime", "NutrientRegime", "Env", "NutrientRegime"},
	{"successional", "Successional", "Env", "SuccessionalStatus"},
	{"structuralStage", "StructuralStage", "Env", "StructuralStage"},
	{"standAge", "StandAge", "Env", "StandAge"},
	{"elevation", "Elevation", "Env", "Elevation"},
	{"slope", "Slope", "Env", "SlopeGradient"},
	{"aspect", "Aspect", "Env", "Aspect"},
	{"mesoSlopePos", "MesoSlopePos", "Env", "MesoSlopePosition"},
	{"surfaceShape", "SurfaceShape", "Env", "SurfaceShape"},
	{"microtopType", "MicrotopType", "Env", "SurfaceTopographyType"},
	{"microtopSize", "MicrotopSize", "Env", "SurfaceTopographySize"},
	{"fieldNotes", "FieldNotes", "Env", "SiteNotes"},
	{"officeNotes", "OfficeNotes", "Admin", "OfficeNotes"},
	{"startDate", "StartDate", "Admin", "StartDate"},
	{"activeLayerDepth", "ActiveLayerDepth", "Env", "ActiveLayerDepth"},
	{"airPhotoNum", "AirPhotoNum", "Env", "AirPhotoNum"},
	{"becSiteUnit", "BECSiteUnit", "Admin", "BECSiteUnit"},
	{"bedrockGeology1", "BedrockGeology1", "Env", "BedrockGeology1"},
	{"bedrockGeology2", "BedrockGeology2", "Env", "BedrockGeology2"},
	{"bedrockGeology3", "BedrockGeology3", "Env", "BedrockGeology3"},
	{"coarseFragLith1", "CoarseFragLith1", "Env", "CoarseFragLith1"},
	{"coarseFragLith2", "CoarseFragLith2", "Env", "CoarseFragLith2"},
	{"coarseFragLith3", "CoarseFragLith3", "Env", "CoarseFragLith3"},
	{"ecosection", "Ecosection", "Env", "Ecosection"},
	{"enteredBy", "EnteredBy", "Admin", "EnteredBy"},
	{"exposure1", "Exposure1", "Env", "Exposure1"},
	{"exposure2", "Exposure2", "Env", "Exposure2"},
	{"fsRegionDistrict", "FSRegionDistrict", "Env", "FSRegionDistrict"},
	{"floodingRegimeDur", "FloodingRegimeDur", "Env", "FloodingRegimeDur"},
	{"floodingRegimeFreq", "FloodingRegimeFreq", "Env", "FloodingRegimeFreq"},
	{"geoMorProSubSurf", "GeoMorProSubSurf", "Env", "GeoMorProSubSurf"},
	{"geoMorProSurf", "GeoMorProSurf", "Env", "GeoMorProSurf"},
	{"humusForm", "HumusForm", "Env", "HumusForm"},
	{"humusFormPhase", "HumusFormPhase", "Env", "HumusFormPhase"},
	{"humusThickness", "HumusThickness", "Admin", "HumusThickness"},
	{"hydroGeoSubSystem", "HydroGeoSubSystem", "Env", "HydroGeoSubSystem"},
	{"hydroGeoSystem", "HydroGeoSystem", "Env", "HydroGeoSystem"},
	{"latitude", "Latitude", "Env", "Latitude"},
	{"longitude", "Longitude", "Env", "Longitude"},
	{"photo", "Photo", "Env", "Photo"},
	{"realmClass", "RealmClass", "Env", "RealmClass"},
	{"rootRestrictingDepth", "RootRestrictingDepth", "Env", "RootRestrictingDepth"},
	{"rootRestrictingType", "RootRestrictingType", "Env", "RootRestrictingType"},
	{"rootZoneParticleSize", "RootZoneParticleSize", "Env", "RootZoneParticleSize"},
	{"rootingDepth", "RootingDepth", "Env", "RootingDepth"},
	{"seepageDepth", "SeepageDepth", "Env", "SeepageDepth"},
	{"siteDisturbance1", "SiteDisturbance1", "Env", "SiteDisturbance1"},
	{"siteDisturbance2", "SiteDisturbance2", "Env", "SiteDisturbance2"},
	{"siteDisturbance3", "SiteDisturbance3", "Env", "SiteDisturbance3"},
	{"sitePlotQuality", "SitePlotQuality", "Admin", "SitePlotQuality"},
	{"snowCoverRegime", "SnowCoverRegime", "Env", "SnowCoverregime"},
	{"soilClassGroup", "SoilClassGroup", "Env", "SoilClassGroup"},
	{"soilClassSubGroup", "SoilClassSubGroup", "Env", "SoilClassSubGroup"},
	{"soilDrainage", "SoilDrainage", "Env", "SoilDrainage"},
	{"soilNotes", "SoilNotes", "Env", "SoilNotes"},
	{"soilPlotQuality", "SoilPlotQuality", "Admin", "SoilPlotQuality"},
	{"soilSurveyor", "SoilSurveyor", "Env", "SoilSurveyor"},
	{"speciesListComplete", "SpeciesListComplete", "Env", "SpeciesListComplete"},
	{"strataCoverHerb", "StrataCoverHerb", "Env", "StrataCoverHerb"},
	{"strataCoverMoss", "StrataCoverMoss", "Env", "StrataCoverMoss"},
	{"strataCoverShrub", "StrataCoverShrub", "Env", "StrataCoverShrub"},
	{"strataCoverTree", "StrataCoverTree", "Env", "StrataCoverTree"},
	{"subZone", "SubZone", "Env", "SubZone"},
	{"substrateBedRock", "SubstrateBedRock", "Env", "SubstrateBedRock"},
	{"substrateDecWood", "SubstrateDecWood", "Env", "SubstrateDecWood"},
	{"substrateMineralSoil", "SubstrateMineralSoil", "Env", "SubstrateMineralSoil"},
	{"substrateOrganicMatter", "SubstrateOrganicMatter", "Env", "SubstrateOrganicMatter"},
	{"substrateRocks", "SubstrateRocks", "Env", "SubstrateRocks"},
	{"substrateWater", "SubstrateWater", "Env", "SubstrateWater"},
	{"surfaceExpSubSurf", "SurfaceExpSubSurf", "Env", "SurfaceExpSubSurf"},
	{"surfaceExpSurf", "SurfaceExpSurf", "Env", "SurfaceExpSurf"},
	{"surficialMaterialSubSurf", "SurficialMaterialSubSurf", "Env", "SurficialMaterialSubSurf"},
	{"surficialMaterialSurf", "SurficialMaterialSurf", "Env", "SurficialMaterialSurf"},
	{"terrainTextureSubSurf", "TerrainTextureSubSurf", "Env", "TerrainTextureSubSurf"},
	{"terrainTextureSurf", "TerrainTextureSurf", "Env", "TerrainTextureSurf"},
	{"updatedFromCards", "UpdatedFromCards", "Admin", "UpdatedFromCards"},
	{"userSiteUnit", "UserSiteUnit", "Admin", "UserSiteUnit"},
	{"vegNotes", "VegNotes", "Env", "VegNotes"},
	{"vegPlotQuality", "VegPlotQuality", "Admin", "VegPlotQuality"},
	{"vegSurveyor", "VegSurveyor", "Env", "VegSurveyor"},
	{"waterSource", "WaterSource", "Env", "WaterSource"},
	{"xCoord", "XCoord", "Env", "XCoord"},
	{"yCoord", "YCoord", "Env", "YCoord"},
	{"zone", "Zone", "Env", "Zone"},
	{"locked", "Locked", "", ""},
}

func QuoteHeaderIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func HeaderCapabilities(db sqlutil.HeaderDB, project string) (map[string]bool, error) {
	if !ProjectNamePattern.MatchString(project) {
		return nil, errors.New("invalid project name")
	}
	columns := make(map[string]map[string]bool)
	for _, table := range []string{"Env", "Admin"} {
		rows, err := db.Query(`PRAGMA table_info(` + QuoteHeaderIdentifier(project+"_"+table) + `)`)
		if err != nil {
			return nil, err
		}
		columns[table] = make(map[string]bool)
		for rows.Next() {
			var cid, notNull, pk int
			var name, typ string
			var def sql.NullString
			if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
				rows.Close()
				return nil, err
			}
			columns[table][strings.ToLower(name)] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	caps := make(map[string]bool, len(HeaderFields))
	for _, field := range HeaderFields {
		caps[field.Property] = field.Column != "" && columns[field.Table][strings.ToLower(field.Column)]
	}
	caps["plotNumber"] = caps["plotNumber"] && columns["Admin"]["plot"]
	return caps, nil
}

// GetHeaderCapabilities returns exact DTO JSON keys, not guessed schema aliases.
// locked is always false: the Access option is runtime UI state, not stored data.
func (s *PlotService) GetHeaderCapabilities() (map[string]bool, error) {
	db, project, release, err := s.GetActiveReadDB()
	if err != nil {
		return nil, err
	}
	defer release()
	return HeaderCapabilities(s.ReadDB(db), project)
}

func ReadHeader(db sqlutil.HeaderDB, project, plot string, caps map[string]bool) (*FS882Header, error) {
	if !caps["plotNumber"] {
		return nil, errors.New("project is missing verified Env/Admin identity columns")
	}
	h := &FS882Header{}
	value := reflect.ValueOf(h).Elem()
	var selectFields []string
	var destinations []any
	for _, field := range HeaderFields {
		if caps[field.Property] {
			column := QuoteHeaderIdentifier(field.Table) + "." + QuoteHeaderIdentifier(field.Column)
			column = headerReadColumn(column, value.FieldByName(field.Member))
			selectFields = append(selectFields, column)
			destinations = append(destinations, value.FieldByName(field.Member).Addr().Interface())
		}
	}
	query := `SELECT ` + strings.Join(selectFields, ",") +
		` FROM ` + QuoteHeaderIdentifier(project+"_Env") + ` AS "Env" INNER JOIN ` +
		QuoteHeaderIdentifier(project+"_Admin") + ` AS "Admin" ON "Env"."PlotNumber" = "Admin"."Plot" WHERE "Env"."PlotNumber" = ?`
	if err := db.QueryRow(query, plot).Scan(destinations...); err != nil {
		return nil, err
	}
	return h, nil
}

// GetPlot reads supported nullable fields; unsupported properties remain nil/false.
func (s *PlotService) GetPlot(plotNumber string) (*FS882Header, error) {
	db, project, release, err := s.GetActiveReadDB()
	if err != nil {
		return nil, err
	}
	defer release()
	caps, err := HeaderCapabilities(s.ReadDB(db), project)
	if err != nil {
		return nil, err
	}
	h, err := ReadHeader(s.ReadDB(db), project, plotNumber, caps)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("plot %q not found", plotNumber)
	}
	return h, err
}

func headerReadColumn(column string, member reflect.Value) string {
	kind := member.Kind()
	if kind == reflect.Pointer {
		kind = member.Type().Elem().Kind()
	}
	switch kind {
	case reflect.String:
		// Bypass SQLite DATETIME affinity conversion for stored DTO strings.
		return "CAST(" + column + " AS TEXT)"
	case reflect.Bool:
		// Access true is -1; the SQLite BOOLEAN driver conversion loses it.
		return "CASE WHEN " + column + " IS NULL THEN NULL WHEN CAST(" + column + " AS INTEGER) = 0 THEN 0 ELSE 1 END"
	default:
		return column
	}
}

func HeaderValue(h FS882Header, field HeaderField) any {
	v := reflect.ValueOf(h).FieldByName(field.Member)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return v.Elem().Interface()
	}
	return v.Interface()
}

func HeaderAuditValue(value any) any {
	if value == nil {
		return nil
	}
	return fmt.Sprint(HeaderStorageValue(value))
}

func HeaderStorageValue(value any) any {
	if flag, ok := value.(bool); ok {
		if flag {
			return -1
		}
		return 0
	}
	return value
}

// V7mdlAudit.AuditTrail: strength 1 edits values, 2 adds values, exactly 3
// also records deletions. Access NULL comparisons do not count as value edits.
func AuditHeaderChange(before, after any, strength int) bool {
	if reflect.DeepEqual(before, after) {
		return false
	}
	if before == nil {
		return after != nil && strength >= 2
	}
	if after == nil {
		return strength == 3
	}
	return strength >= 1
}

// SavePlot is a full replacement of supported header values (nil clears SQL
// NULL), not a patch. Env, Admin and audits commit or roll back together.
func (s *PlotService) SavePlot(h FS882Header) error {
	return s.saveHeader(h, headerUpsert)
}

type headerSaveMode int

const (
	headerUpsert headerSaveMode = iota
	headerCreate
	HeaderUpdate
)

func (s *PlotService) CreatePlot(h FS882Header) error {
	return s.saveHeader(h, headerCreate)
}

func (s *PlotService) UpdatePlot(h FS882Header) error {
	return s.saveHeader(h, HeaderUpdate)
}

func (s *PlotService) saveHeader(h FS882Header, mode headerSaveMode) error {
	if err := s.RequireContextEdit(); err != nil {
		return err
	}
	s.Mu.RLock()
	user, strength := s.CurrentUser, s.AuditStrength
	s.Mu.RUnlock()
	masterAllowed := MasterBECAllowed(user)
	if strings.TrimSpace(h.PlotNumber) == "" {
		return errors.New("PlotNumber is required")
	}
	db, project, release, err := s.GetActiveDB()
	if err != nil {
		return err
	}
	defer release()
	if err := validateBECHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateQualityHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := ValidateSubstrateHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := s.validateSiteCodeHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateRegionHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateSoilHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateGeologyHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateParentCodeHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateOrdinaryHeaderBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := s.validateSoilDrainageBeforeTransaction(db, project, h, mode); err != nil {
		return err
	}
	if err := validateCodeHeaderBeforeTransaction(db, project, h, mode, func(next FS882Header, old *FS882Header) error {
		return validateMasterBECHeaderValues(next, old, masterAllowed)
	}); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	caps, err := HeaderCapabilities(tx, project)
	if err != nil {
		return err
	}
	if !caps["plotNumber"] {
		return errors.New("project is missing verified Env/Admin identity columns")
	}
	for _, field := range HeaderFields {
		member := reflect.ValueOf(h).FieldByName(field.Member)
		if !caps[field.Property] && !member.IsZero() {
			return fmt.Errorf("unsupported header property %q in active project", field.Property)
		}
		if member.Kind() == reflect.Pointer && !member.IsNil() && member.Elem().Kind() == reflect.Float64 {
			number := member.Elem().Float()
			if math.IsNaN(number) || math.IsInf(number, 0) {
				return fmt.Errorf("header property %q must be finite", field.Property)
			}
		}
	}
	var envCount, adminCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+QuoteHeaderIdentifier(project+"_Env")+` WHERE "PlotNumber" = ?`, h.PlotNumber).Scan(&envCount); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+QuoteHeaderIdentifier(project+"_Admin")+` WHERE "Plot" = ?`, h.PlotNumber).Scan(&adminCount); err != nil {
		return err
	}
	if envCount != adminCount || envCount > 1 {
		return fmt.Errorf("plot %q has inconsistent Env/Admin identity rows", h.PlotNumber)
	}
	if mode == headerCreate && envCount != 0 {
		return fmt.Errorf("plot %q already exists; choose another plot number", h.PlotNumber)
	}
	if mode == HeaderUpdate && envCount != 1 {
		return fmt.Errorf("plot %q no longer exists; reload before saving", h.PlotNumber)
	}
	creating := envCount == 0
	old := &FS882Header{}
	if !creating {
		old, err = ReadHeader(tx, project, h.PlotNumber, caps)
		if err != nil {
			return err
		}
	}
	if err := validateBECHeaderValues(h, old); err != nil {
		return err
	}
	qualityOld := old
	if creating {
		qualityOld = nil
	}
	if err := validateQualityHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := ValidateSubstrateHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := s.validateSiteCodeHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := validateRegionHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := validateSoilHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := validateGeologyHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := validateParentCodeHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := validateOrdinaryHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := s.validateSoilDrainageHeaderValues(h, qualityOld); err != nil {
		return err
	}
	if err := validateMasterBECHeaderValues(h, qualityOld, masterAllowed); err != nil {
		return err
	}
	for _, table := range []string{"Env", "Admin"} {
		key := "PlotNumber"
		if table == "Admin" {
			key = "Plot"
		}
		columns := []string{QuoteHeaderIdentifier(key)}
		marks := []string{"?"}
		var assignments []string
		values := []any{h.PlotNumber}
		for _, field := range HeaderFields {
			if field.Table != table || field.Property == "plotNumber" || !caps[field.Property] {
				continue
			}
			if !creating && (field.Property == "zone" || field.Property == "subZone" || field.Property == "siteSeries" || field.Property == "userSiteUnit" || field.Property == "becSiteUnit" || field.Property == "speciesListComplete" || field.Property == "updatedFromCards" || field.Property == "soilDrainage" || isQualityProperty(field.Property) || isSubstrateProperty(field.Property) || isSiteCodeProperty(field.Property) || isRegionProperty(field.Property) || isSoilProperty(field.Property) || isGeologyProperty(field.Property) || IsParentCodeProperty(field.Property) || IsOrdinaryProperty(field.Property)) &&
				reflect.DeepEqual(HeaderValue(h, field), HeaderValue(*old, field)) {
				continue
			}
			column := QuoteHeaderIdentifier(field.Column)
			columns = append(columns, column)
			marks = append(marks, "?")
			assignments = append(assignments, column+" = ?")
			values = append(values, HeaderStorageValue(HeaderValue(h, field)))
		}
		target := QuoteHeaderIdentifier(project + "_" + table)
		query := `INSERT INTO ` + target + ` (` + strings.Join(columns, ",") + `) VALUES (` + strings.Join(marks, ",") + `)`
		if !creating {
			if len(assignments) == 0 {
				continue
			}
			query = `UPDATE ` + target + ` SET ` + strings.Join(assignments, ",") + ` WHERE ` + QuoteHeaderIdentifier(key) + ` = ?`
			values = append(values[1:], h.PlotNumber)
		}
		if _, err := tx.Exec(query, values...); err != nil {
			return fmt.Errorf("save %s: %w", table, err)
		}
	}
	if (!creating && strength >= 1) || (creating && strength >= 2) {
		query := `INSERT INTO ` + QuoteHeaderIdentifier(project+"_Audit") +
			` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag") VALUES (?,?,?,?,?,?,?,?,0,0)`
		now := time.Now().Format("2006-01-02 15:04:05")
		for _, field := range HeaderFields {
			if !caps[field.Property] || field.Property == "plotNumber" {
				continue
			}
			before, after := HeaderValue(*old, field), HeaderValue(h, field)
			if !AuditHeaderChange(before, after, strength) {
				continue
			}
			if _, err := tx.Exec(query, project, user, h.PlotNumber, "_"+field.Table, field.Column, now, HeaderAuditValue(before), HeaderAuditValue(after)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
