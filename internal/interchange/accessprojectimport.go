package interchange

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/accessimport"
	"github.com/boostao/vpro-wails/internal/accessreader"
)

type AccessImportTableSummary struct {
	Suffix      string `json:"suffix"`
	SourceTable string `json:"sourceTable"`
	Description string `json:"description"`
	RowCount    int    `json:"rowCount"`
	ColumnCount int    `json:"columnCount"`
	Action      string `json:"action"`
	Present     bool   `json:"present"`
}

type AccessProjectCandidate struct {
	SourceProject     string                     `json:"sourceProject"`
	SuggestedTarget   string                     `json:"suggestedTarget"`
	SuggestedDestPath string                     `json:"suggestedDestPath"`
	SourceVersion     string                     `json:"sourceVersion"`
	ConversionMode    string                     `json:"conversionMode"`
	Compatible        bool                       `json:"compatible"`
	StatusMessage     string                     `json:"statusMessage"`
	PlotCount         int                        `json:"plotCount"`
	Tables            []AccessImportTableSummary `json:"tables"`
	OptionalSUs       []string                   `json:"optionalSUs"`
}

type AccessDatabaseInspection struct {
	SourcePath   string                   `json:"sourcePath"`
	SourceSHA256 string                   `json:"sourceSha256"`
	Projects     []AccessProjectCandidate `json:"projects"`
}

type AccessProjectImportRequest struct {
	SourcePath        string `json:"sourcePath"`
	SourceProject     string `json:"sourceProject"`
	TargetProject     string `json:"targetProject"`
	DestinationPath   string `json:"destinationPath"`
	ActivateOnSuccess bool   `json:"activateOnSuccess"`
}

type AccessProjectImportResult struct {
	SourcePath              string                     `json:"sourcePath"`
	SourceProject           string                     `json:"sourceProject"`
	SourceVersion           string                     `json:"sourceVersion"`
	ConversionMode          string                     `json:"conversionMode"`
	TargetProject           string                     `json:"targetProject"`
	DestinationPath         string                     `json:"destinationPath"`
	DestinationSHA          string                     `json:"destinationSha256"`
	PlotCount               int                        `json:"plotCount"`
	QuarantinedMalformedRow int                        `json:"quarantinedMalformedRows"`
	QuarantinedOrphanRows   int                        `json:"quarantinedOrphanRows"`
	Tables                  []AccessImportTableSummary `json:"tables"`
	ImportedSUs             []string                   `json:"importedSUs"`
	Activated               bool                       `json:"activated"`
	State                   *vprocore.ProjectState     `json:"state,omitempty"`
}

var vp07EnvMapping = [][2]string{
	{"PlotNumber", "PlotNumber"}, {"FieldNumber", "FieldNumber"}, {"ProjectID", "ProjectID"},
	{"FSRegionDistrict", "FSRegionDistrict"}, {"Date", "Date"}, {"SiteSurveyor", "SiteSurveyor"},
	{"PlotRepresenting", "PlotRepresenting"}, {"Location", "Location"}, {"Ecosection", "Ecosection"},
	{"NtsMapSheet", "NtsMapSheet"}, {"Longitude", "Longitude"}, {"Latitude", "Latitude"},
	{"UTMZone", "UTMZone"}, {"UTMEasting", "UTMEasting"}, {"UTMNorthing", "UTMNorthing"},
	{"LocationAccuracy", "LocationAccuracy"}, {"AirPhotoNum", "AirPhotoNum"}, {"XCoord", "XCoord"},
	{"YCoord", "YCoord"}, {"Zone", "Zone"}, {"SubZone", "SubZone"}, {"SiteSeries", "SiteSeries"},
	{"SiteModifier1", "SiteModifier1"}, {"SiteModifier2", "SiteModifier2"}, {"TransDistrib", "TransDistrib"},
	{"RealmClass", "RealmClass"}, {"MapUnit", "MapUnit"}, {"SnowCoverregime", "SnowCoverregime"},
	{"MoistureRegime", "MoistureRegime"}, {"NutrientRegime", "NutrientRegime"},
	{"SuccessionalStatus", "SuccessionalStatus"}, {"StructuralStage", "StructuralStage"},
	{"StructuralStageMod", "StructuralStageMod"}, {"StandAge", "StandAge"}, {"Elevation", "Elevation"},
	{"SlopeGradient", "SlopeGradient"}, {"Aspect", "Aspect"}, {"MesoSlopePosition", "MesoSlopePosition"},
	{"SurfaceShape", "SurfaceShape"}, {"SurfaceTopographyType", "SurfaceTopographyType"},
	{"SurfaceTopographySize", "SurfaceTopographySize"}, {"WaterSource", "WaterSource"}, {"Photo", "Photo"},
	{"Exposure1", "Exposure1"}, {"Exposure2", "Exposure2"}, {"SiteDisturbance1", "SiteDisturbance1"},
	{"SiteDisturbance2", "SiteDisturbance2"}, {"SiteDisturbance3", "SiteDisturbance3"},
	{"SubstrateDecWood", "SubstrateDecWood"}, {"SubstrateBedRock", "SubstrateBedRock"},
	{"SubstrateRocks", "SubstrateRocks"}, {"SubstrateMineralSoil", "SubstrateMineralSoil"},
	{"SubstrateOrganicMatter", "SubstrateOrganicMatter"}, {"SubstrateWater", "SubstrateWater"},
	{"SiteNotes", "SiteNotes"}, {"SoilSurveyor", "SoilSurveyor"}, {"BedrockGeology1", "BedrockGeology1"},
	{"BedrockGeology2", "BedrockGeology2"}, {"BedrockGeology3", "BedrockGeology3"},
	{"CoarseFragLith1", "CoarseFragLith1"}, {"CoarseFragLith2", "CoarseFragLith2"},
	{"CoarseFragLith3", "CoarseFragLith3"}, {"TerrainTextureSurf", "TerrainTextureSurf"},
	{"SurficialMaterialSurf", "SurficialMaterialSurf"}, {"SurfaceExpSurf", "SurfaceExpSurf"},
	{"GeoMorProSurf", "GeoMorProSurf"}, {"TerrainTextureSubSurf", "TerrainTextureSubSurf"},
	{"SurficialMaterialSubSurf", "SurficialMaterialSubSurf"}, {"SurfaceExpSubSurf", "SurfaceExpSubSurf"},
	{"GeoMorProSubSurf", "GeoMorProSubSurf"}, {"FloodingRegimeFreq", "FloodingRegimeFreq"},
	{"MoistureRegimeSub", "MoistureRegimeSub"}, {"FloodingRegimeDur", "FloodingRegimeDur"},
	{"SoilDrainage", "SoilDrainage"}, {"SeepageDepth", "SeepageDepth"},
	{"RootRestrictingType", "RootRestrictingType"}, {"RootRestrictingDepth", "RootRestrictingDepth"},
	{"RootZoneParticleSize", "RootZoneParticleSize"}, {"RootingDepth", "RootingDepth"},
	{"SoilClassSubGroup", "SoilClassSubGroup"}, {"SoilClassGroup", "SoilClassGroup"},
	{"HumusForm", "HumusForm"}, {"HumusFormPhase", "HumusFormPhase"},
	{"pHMethodCodeMineral", "pHMethodCodeMineral"}, {"pHMethodCodeOrganic", "pHMethodCodeOrganic"},
	{"SoilNotes", "SoilNotes"}, {"VegSurveyor", "VegSurveyor"}, {"StrataCoverTree", "StrataCoverTree"},
	{"StrataCoverShrub", "StrataCoverShrub"}, {"StrataCoverHerb", "StrataCoverHerb"},
	{"StrataCoverMoss", "StrataCoverMoss"}, {"VegNotes", "VegNotes"},
	{"HydroGeoSystem", "HydroGeoSystem"}, {"HydroGeoSubSystem", "HydroGeoSubSystem"},
	{"SpeciesListComplete", "SpeciesListComplete"}, {"Temporary", "Temporary"}, {"Flag", "Flag"},
	{"SV_PolygonNumber", "PolygonNumber"}, {"SV_FloodPlain", "FloodPlain"},
	{"SV_StandAgeEstMeas", "StandAgeEstMeas"}, {"SV_StandHeight", "StandHeight"},
	{"SV_StandHeightEstMeas", "StandHeightEstMeas"}, {"SV_CanopyComposition", "CanopyComposition"},
	{"SV_SoilDepth", "SoilDepth"}, {"SV_RootZoneTexture", "RootZoneTexture"},
	{"SV_PercentCoarseFrags", "PercentCoarseFrags"}, {"SV_GleyingMottlingCM", "GleyingMottlingCM"},
	{"SV_WaterTableCM", "WaterTableCM"}, {"SV_FullCruiseCard", "FullCruiseCard"},
	{"SV_AhorizonType", "AhorizonType"}, {"SV_AhorizonDepth", "AhorizonDepth"},
	{"ActiveLayerDepth", "ActiveLayerDepth"},
}

var vp07AdminFromEnvMapping = [][2]string{
	{"Plot", "PlotNumber"}, {"StartDate", "StartDate"}, {"PlotType", "PlotType"},
	{"PlotSize", "PlotSize"}, {"ProvinceStateTerritory", "ProvinceStateTerritory"},
	{"SitePlotQuality", "SitePlotQuality"}, {"VegPlotQuality", "VegPlotQuality"},
	{"SoilPlotQuality", "SoilPlotQuality"}, {"UpdatedFromCards", "UpdatedFromCards"},
	{"EnteredBy", "EnteredBy"}, {"UserSiteUnit", "UserSiteUnit"}, {"BECSiteUnit", "BECSiteUnit"},
	{"SiteUnitShortName", "SiteUnitShortName"}, {"SiteUnitLongName", "SiteUnitLongName"},
	{"OfficeNotes", "OfficeNotes"}, {"HumusThickness", "HumusThickness"},
	{"GIS_BGC", "GIS_BGC"}, {"GIS_BGC_ver", "GIS_BGC_ver"},
	{"StrataCoverTotal", "StrataCoverTotal"},
}

var vp06EnvMapping = [][2]string{
	{"PlotNumber", "PlotNumber"}, {"FieldNumber", "FieldNumber"}, {"ProjectID", "ProjectID"},
	{"FSRegionDistrict", "FSRegionDistrict"}, {"Date", "Date"}, {"SiteSurveyor", "SiteSurveyor"},
	{"PlotRepresenting", "PlotRepresenting"}, {"Location", "Location"}, {"Ecosection", "Ecosection"},
	{"NtsMapSheet", "NtsMapSheet"}, {"Longitude", "Longitude"}, {"Latitude", "Latitude"},
	{"UTMZone", "UTMZone"}, {"UTMEasting", "UTMEasting"}, {"UTMNorthing", "UTMNorthing"},
	{"LocationAccuracy", "LocationAccuracy"}, {"AirPhotoNum", "AirPhotoNum"}, {"XCoord", "XCoord"},
	{"YCoord", "YCoord"}, {"Zone", "Zone"}, {"SubZone", "SubZone"}, {"SiteSeries", "SiteSeries"},
	{"TransDistrib", "TransDistrib"}, {"RealmClass", "RealmClass"}, {"MapUnit", "MapUnit"},
	{"MoistureRegime", "MoistureRegime"}, {"NutrientRegime", "NutrientRegime"},
	{"SuccessionalStatus", "SuccessionalStatus"}, {"StructuralStage", "StructuralStage"},
	{"StructuralStageMod", "StructuralStageMod"}, {"StandAge", "StandAge"}, {"Elevation", "Elevation"},
	{"SlopeGradient", "SlopeGradient"}, {"Aspect", "Aspect"}, {"MesoSlopePosition", "MesoSlopePosition"},
	{"SurfaceShape", "SurfaceShape"}, {"SurfaceTopographyType", "SurfaceTopographyType"},
	{"SurfaceTopographySize", "SurfaceTopographySize"}, {"WaterSource", "WaterSource"}, {"Photo", "Photo"},
	{"Exposure1", "Exposure1"}, {"Exposure2", "Exposure2"}, {"SiteDisturbance1", "SiteDisturbance1"},
	{"SiteDisturbance2", "SiteDisturbance2"}, {"SiteDisturbance3", "SiteDisturbance3"},
	{"SubstrateDecWood", "SubstrateDecWood"}, {"SubstrateBedRock", "SubstrateBedRock"},
	{"SubstrateRocks", "SubstrateRocks"}, {"SubstrateMineralSoil", "SubstrateMineralSoil"},
	{"SubstrateOrganicMatter", "SubstrateOrganicMatter"}, {"SubstrateWater", "SubstrateWater"},
	{"SiteNotes", "SiteNotes"}, {"SoilSurveyor", "SoilSurveyor"}, {"BedrockGeology1", "BedrockGeology1"},
	{"BedrockGeology2", "BedrockGeology2"}, {"BedrockGeology3", "BedrockGeology3"},
	{"CoarseFragLith1", "CoarseFragLith1"}, {"CoarseFragLith2", "CoarseFragLith2"},
	{"CoarseFragLith3", "CoarseFragLith3"}, {"TerrainTextureSurf", "TerrainTextureSurf"},
	{"SurficialMaterialSurf", "SurficialMaterialSurf"}, {"SurfaceExpSurf", "SurfaceExpSurf"},
	{"GeoMorProSurf", "GeoMorProSurf"}, {"TerrainTextureSubSurf", "TerrainTextureSubSurf"},
	{"SurficialMaterialSubSurf", "SurficialMaterialSubSurf"}, {"SurfaceExpSubSurf", "SurfaceExpSubSurf"},
	{"GeoMorProSubSurf", "GeoMorProSubSurf"}, {"FloodingRegimeFreq", "FloodingRegimeFreq"},
	{"MoistureRegimeSub", "MoistureRegimeSub"}, {"FloodingRegimeDur", "FloodingRegimeDur"},
	{"SoilDrainage", "SoilDrainage"}, {"SeepageDepth", "SeepageDepth"},
	{"RootRestrictingType", "RootRestrictingType"}, {"RootRestrictingDepth", "RootRestrictingDepth"},
	{"RootZoneParticleSize", "RootZoneParticleSize"}, {"RootingDepth", "RootingDepth"},
	{"SoilClassSubGroup", "SoilClassSubGroup"}, {"SoilClassGroup", "SoilClassGroup"},
	{"HumusForm", "HumusForm"}, {"HumusFormPhase", "HumusFormPhase"},
	{"pHMethodCodeMineral", "pHMethodCodeMineral"}, {"pHMethodCodeOrganic", "pHMethodCodeOrganic"},
	{"SoilNotes", "SoilNotes"}, {"VegSurveyor", "VegSurveyor"}, {"StrataCoverTree", "StrataCoverTree"},
	{"StrataCoverShrub", "StrataCoverShrub"}, {"StrataCoverHerb", "StrataCoverHerb"},
	{"StrataCoverMoss", "StrataCoverMoss"}, {"VegNotes", "VegNotes"},
	{"HydroGeoSystem", "HydroGeoSystem"}, {"HydroGeoSubSystem", "HydroGeoSubSystem"},
	{"SpeciesListComplete", "SpeciesListComplete"}, {"Temporary", "Temporary"}, {"Flag", "Flag"},
}

var vp06AdminFromEnvMapping = [][2]string{
	{"Plot", "PlotNumber"},
	{"SitePlotQuality", "SitePlotQuality"}, {"VegPlotQuality", "VegPlotQuality"},
	{"SoilPlotQuality", "SoilPlotQuality"}, {"UpdatedFromCards", "UpdatedFromCards"},
	{"EnteredBy", "EnteredBy"}, {"UserSiteUnit", "UserSiteUnit"}, {"BECSiteUnit", "BECSiteUnit"},
	{"SiteUnitShortName", "SiteUnitShortName"}, {"SiteUnitLongName", "SiteUnitLongName"},
	{"OfficeNotes", "OfficeNotes"}, {"HumusThickness", "HumusThickness"},
}

var vp05EnvMapping = [][2]string{
	{"PlotNumber", "PlotNumber"}, {"FieldNumber", "SiteNumber"}, {"ProjectID", "ProjectName"},
	{"FSRegionDistrict", "Region"}, {"Date", "Date"},
	{"SiteSurveyor", "SiteSurveyor"}, {"PlotRepresenting", "PlotRepresenting"},
	{"Location", "Location"}, {"Ecosection", "Ecosection"}, {"NtsMapSheet", "NtsMapSheet"},
	{"UTMZone", "UTMZone"}, {"UTMEasting", "UTMEasting"}, {"UTMNorthing", "UTMNorthing"},
	{"LocationAccuracy", "LocationAccuracy"}, {"AirPhotoNum", "AirPhotoNum"},
	{"XCoord", "XCoord"}, {"YCoord", "YCoord"}, {"Zone", "Zone"}, {"SubZone", "SubZone"},
	{"SiteSeries", "SiteSeries"}, {"TransDistrib", "TransDistrib"}, {"RealmClass", "RealmClass"},
	{"MapUnit", "MapUnit"}, {"MoistureRegime", "MoistureRegime"}, {"NutrientRegime", "NutrientRegime"},
	{"SuccessionalStatus", "SuccessionalStatus"}, {"StructuralStage", "StructuralStage"},
	{"StructuralStageMod", "StructuralStageMod"}, {"StandAge", "StandAge"}, {"Elevation", "Elevation"},
	{"SlopeGradient", "SlopeGradient"}, {"Aspect", "Aspect"}, {"MesoSlopePosition", "MesoSlopePosition"},
	{"SurfaceShape", "SurfaceTopography"}, {"WaterSource", "WaterSource"},
	{"Exposure1", "Exposure1"}, {"Exposure2", "Exposure2"},
	{"SiteDisturbance1", "SiteDisturbance1"}, {"SiteDisturbance2", "SiteDisturbance2"},
	{"SubstrateDecWood", "SubstrateDecWood"}, {"SubstrateBedRock", "SubstrateBedRock"},
	{"SubstrateRocks", "SubstrateRocks"}, {"SubstrateMineralSoil", "SubstrateMineralSoil"},
	{"SubstrateOrganicMatter", "SubstrateOrganicMatter"}, {"SubstrateWater", "SubstrateWater"},
	{"SurfaceTopographyType", "SurfaceTopographyType"}, {"SiteNotes", "SiteNotes"},
	{"SoilSurveyor", "SoilSurveyor"}, {"BedrockGeology1", "BedrockGeology"},
	{"CoarseFragLith1", "CoarseFragLith"}, {"TerrainTextureSurf", "TerrainTextureSurf"},
	{"SurficialMaterialSurf", "SurficialMaterialSurf"}, {"SurfaceExpSurf", "SurfaceExpSurf"},
	{"GeoMorProSurf", "GeoMorProSurf"}, {"TerrainTextureSubSurf", "TerrainTextureSubSurf"},
	{"SurficialMaterialSubSurf", "SurficialMaterialSubSurf"}, {"SurfaceExpSubSurf", "SurfaceExpSubSurf"},
	{"GeoMorProSubSurf", "GeoMorProSubSurf"}, {"FloodingRegimeFreq", "FloodingRegimeFreq"},
	{"MoistureRegimeSub", "MoistureRegimeSub"}, {"FloodingRegimeDur", "FloodingRegimeDur"},
	{"SurfaceTopographySize", "SurfaceTopographySize"}, {"SoilDrainage", "SoilDrainage"},
	{"SeepageDepth", "SeepageDepth"}, {"RootRestrictingType", "RootRestrictingType"},
	{"RootRestrictingDepth", "RootRestrictingDepth"}, {"RootZoneParticleSize", "RootZoneParticleSize"},
	{"RootingDepth", "RootingDepth"}, {"SoilClassSubGroup", "SoilClassSubGroup"},
	{"SoilClassGroup", "SoilClassGroup"}, {"HumusForm", "HumusForm"},
	{"HumusFormPhase", "HumusFormPhase"},
	{"pHMethodCodeMineral", "pHMethodCodeMineral"}, {"pHMethodCodeOrganic", "pHMethodCodeOrganic"},
	{"SoilNotes", "SoilNotes"}, {"VegSurveyor", "VegSurveyor"},
	{"StrataCoverTree", "StrataCoverTree"}, {"StrataCoverShrub", "StrataCoverShrub"},
	{"StrataCoverHerb", "StrataCoverHerb"}, {"StrataCoverMoss", "StrataCoverMoss"},
	{"VegNotes", "VegNotes"}, {"Photo", "PhotoRollNum"},
	{"HydroGeoSystem", "HydroGeoSystem"}, {"HydroGeoSubSystem", "HydroGeoSubSystem"},
	{"Flag", "Flag"},
}

var vp05AdminFromEnvMapping = [][2]string{
	{"Plot", "PlotNumber"},
	{"SitePlotQuality", "PlotQuality"}, {"UserSiteUnit", "AssignedSiteUnit"},
	{"SiteUnitShortName", "SiteUnitShortName"}, {"SiteUnitLongName", "SiteUnitLongName"},
	{"HumusThickness", "HumusThickness"},
}

var legacyAuditColumns = []string{
	"Project", "User", "PlotNumber", "Table", "EditField", "EditWhen",
	"BeforeEdit", "AfterEdit", "Restore", "Flag", "ID",
}

var legacyHumusColumns = []string{
	"PlotNumber", "Horizon", "UpperDepth", "LowerDepth", "HumusStructureDegree",
	"HumusStructureKind", "MycelAbundance", "FecalAbundance", "RootsAbundance",
	"RootsSize", "vonPost", "HumusFormpH", "Consistence", "Character", "Fauna",
	"Comment", "Flag", "ID",
}

var legacyMineralColumns = []string{
	"PlotNumber", "Horizon", "UpperDepth", "LowerDepth", "PitDepthLimit", "Colour",
	"ASP", "Texture", "PercentCoarseFragsGravel", "PercentCoarseFragsCobbles",
	"PercentCoarseFragsStones", "PercentCoarseFragsTotal", "PercentCoarseFragsShape",
	"RootsAbundance", "RootsSize", "MineralStructureClass", "MineralStructureKind",
	"MineralFormpH", "MottlesAbundance", "MottlesSize", "MottlesContrast",
	"ClayFilmsFreq", "ClayFilmThickness", "Effervescence", "Porosity",
	"Comments", "Flag", "ID",
}

var legacyOtherColumns = []string{
	"PlotNumber", "DataName", "DataItem", "UserItem1", "UserItem2", "UserItem3",
	"UserFlag1", "UserFlag2", "UserFlag3", "Flag", "ID",
}

var legacyVegColumns = []string{
	"PlotNumber", "Species", "Cover1", "Height1", "Cover2", "Height2", "Cover3",
	"Height3", "Cover4", "Height4", "Cover5", "Height5", "Cover6", "Height6",
	"Cover7", "Cover8", "Cover9", "Cover10", "TotalA", "TotalB", "Collected",
	"Flag", "ID",
}

func convertLongLatToDeg(degVal, minVal, secVal any) any {
	deg, ok := toFloat64(degVal)
	if !ok {
		return nil
	}
	minutes, _ := toFloat64(minVal)
	seconds, _ := toFloat64(secVal)
	return deg + ((minutes + (seconds / 60.0)) / 60.0)
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func sanitizeProjectSuggestion(raw string) string {
	var builder strings.Builder
	for _, r := range raw {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			builder.WriteRune(r)
		} else if r == '-' || r == ' ' {
			builder.WriteRune('_')
		}
	}
	candidate := builder.String()
	if candidate == "" || !((candidate[0] >= 'A' && candidate[0] <= 'Z') || (candidate[0] >= 'a' && candidate[0] <= 'z')) {
		candidate = "P_" + candidate
	}
	if len(candidate) > 31 {
		candidate = candidate[:31]
	}
	if strings.EqualFold(candidate, "Sample") || strings.EqualFold(candidate, "None") || strings.EqualFold(candidate, "Master") {
		candidate += "_Imp"
		if len(candidate) > 31 {
			candidate = candidate[:31]
		}
	}
	return candidate
}

func streamFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func resolveAccessSourcePath(requested string) (string, string, error) {
	if requested == "" || !utf8.ValidString(requested) || strings.ContainsRune(requested, 0) || !filepath.IsAbs(requested) {
		return "", "", errors.New("Access database path must be an explicit absolute path")
	}
	ext := strings.ToLower(filepath.Ext(requested))
	if ext != ".mdb" && ext != ".accdb" {
		return "", "", errors.New("Access database path must end in .mdb or .accdb")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(requested))
	if err != nil {
		return "", "", fmt.Errorf("resolve Access database %q: %w", requested, err)
	}
	digest, err := streamFileSHA256(resolved)
	if err != nil {
		return "", "", fmt.Errorf("read Access database %q: %w", resolved, err)
	}
	return resolved, digest, nil
}

func InspectAccessDatabase(s *vprocore.ContextService, ctx context.Context, contextID, sourcePath string) (AccessDatabaseInspection, error) {
	var projectsDir string
	if _, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		projectsDir = filepath.Join(plots.Projects.Root, "projects")
		return struct{}{}, nil
	}); err != nil {
		return AccessDatabaseInspection{}, err
	}
	resolved, digest, err := resolveAccessSourcePath(sourcePath)
	if err != nil {
		return AccessDatabaseInspection{}, err
	}
	db, err := accessreader.Open(resolved)
	if err != nil {
		return AccessDatabaseInspection{}, err
	}
	tables, err := db.Tables()
	if err != nil {
		return AccessDatabaseInspection{}, err
	}
	byLower := make(map[string]accessreader.Table, len(tables))
	var optionalSUs []string
	for _, tbl := range tables {
		if tbl.Linked {
			continue
		}
		byLower[strings.ToLower(tbl.Name)] = tbl
		if len(tbl.Name) > 3 && strings.EqualFold(tbl.Name[len(tbl.Name)-3:], "_SU") &&
			!strings.HasPrefix(strings.ToUpper(tbl.Name), "USYS") {
			prefix := tbl.Name[:len(tbl.Name)-3]
			if vprocore.ProjectNamePattern.MatchString(prefix) {
				optionalSUs = append(optionalSUs, prefix)
			}
		}
	}
	slices.Sort(optionalSUs)

	var candidates []AccessProjectCandidate
	for _, tbl := range tables {
		if err := ctx.Err(); err != nil {
			return AccessDatabaseInspection{}, err
		}
		if tbl.Linked || len(tbl.Name) <= 4 || !strings.EqualFold(tbl.Name[len(tbl.Name)-4:], "_Env") {
			continue
		}
		if strings.EqualFold(tbl.Name, "Filtered_Env") || strings.EqualFold(tbl.Name, "USysEnv") {
			continue
		}
		prefix := tbl.Name[:len(tbl.Name)-4]
		version := strings.TrimSpace(tbl.Description)
		if version == "" {
			version = "Unknown"
		}
		suggested := sanitizeProjectSuggestion(prefix)
		destPath := filepath.Join(projectsDir, suggested+".db")
		for idx := 2; ; idx++ {
			if _, statErr := os.Stat(destPath); errors.Is(statErr, os.ErrNotExist) {
				break
			}
			suffix := fmt.Sprintf("_%d", idx)
			base := suggested
			if len(base)+len(suffix) > 31 {
				base = base[:31-len(suffix)]
			}
			suggested = base + suffix
			destPath = filepath.Join(projectsDir, suggested+".db")
		}

		candidate := AccessProjectCandidate{
			SourceProject:     prefix,
			SuggestedTarget:   suggested,
			SuggestedDestPath: destPath,
			SourceVersion:     version,
			OptionalSUs:       optionalSUs,
		}
		var requiredSuffixes []string
		switch version {
		case "VP08":
			candidate.ConversionMode = "Direct VP08 8-table import (lossless VPro 19/64 schema)"
			requiredSuffixes = []string{"Env", "Veg", "Other", "Humus", "Mineral", "Audit", "Metadata", "Admin"}
		case "VP07":
			candidate.ConversionMode = "Autoconvert VP07 (VPro 15) -> VP08 (ImportVPro07 SV_* rename + Admin split)"
			requiredSuffixes = []string{"Env", "Veg", "Other", "Humus", "Mineral", "Audit", "Metadata"}
		case "VP06":
			candidate.ConversionMode = "Autoconvert VP06 (VPro 07/13) -> VP08 (ImportVPro06 Env/Admin split)"
			requiredSuffixes = []string{"Env", "Veg", "Other", "Humus", "Mineral", "Audit"}
		case "VP05":
			candidate.ConversionMode = "Autoconvert VP05 (VPro XP/03) -> VP08 (ImportVPro05 DMS->Decimal Deg + column mapping + Admin synthesis)"
			requiredSuffixes = []string{"Env", "Veg", "Other", "Humus", "Mineral", "Audit"}
		default:
			candidate.ConversionMode = fmt.Sprintf("Unsupported legacy version %q (supported: VP05, VP06, VP07, VP08)", version)
			candidate.Compatible = false
			candidate.StatusMessage = candidate.ConversionMode
			candidates = append(candidates, candidate)
			continue
		}

		allPresent := true
		for _, suffix := range []string{"Env", "Veg", "Other", "Humus", "Mineral", "Audit", "Metadata", "Admin"} {
			srcTbl, exists := byLower[strings.ToLower(prefix+"_"+suffix)]
			required := slices.Contains(requiredSuffixes, suffix)
			summary := AccessImportTableSummary{
				Suffix:      suffix,
				SourceTable: prefix + "_" + suffix,
				Present:     exists,
			}
			if exists {
				summary.SourceTable = srcTbl.Name
				summary.Description = srcTbl.Description
				reader, readErr := db.Scan(srcTbl.Name)
				if readErr != nil {
					allPresent = false
					candidate.StatusMessage = fmt.Sprintf("Failed opening %s: %v", srcTbl.Name, readErr)
				} else {
					reader.EnableRowQuarantine()
					rowCnt := 0
					for reader.Next() {
						rowCnt++
					}
					iterErr := reader.Err()
					colCnt := len(reader.Columns)
					_ = reader.Close()
					if iterErr != nil {
						allPresent = false
						candidate.StatusMessage = fmt.Sprintf("Failed reading %s: %v", srcTbl.Name, iterErr)
					} else {
						summary.ColumnCount = colCnt
						summary.RowCount = rowCnt
						if suffix == "Env" {
							candidate.PlotCount = rowCnt
						}
					}
				}
				if version == "VP08" {
					summary.Action = "Copy rows directly"
				} else if suffix == "Env" {
					summary.Action = fmt.Sprintf("Convert %s Env -> VP08 Env + Admin", version)
				} else if suffix == "Metadata" && version == "VP07" {
					summary.Action = "Create empty VP08 Metadata table (ImportVPro07 contract)"
				} else {
					summary.Action = fmt.Sprintf("Map %s %s columns -> VP08 %s", version, suffix, suffix)
				}
			} else if !required {
				if suffix == "Admin" {
					summary.Action = fmt.Sprintf("Synthesize VP08 Admin from %s_Env (%d plots)", prefix, candidate.PlotCount)
				} else if suffix == "Metadata" {
					summary.Action = "Create empty VP08 Metadata table"
				}
			} else {
				allPresent = false
				summary.Action = "Missing required source table"
				if candidate.StatusMessage == "" {
					candidate.StatusMessage = fmt.Sprintf("Missing required table %s_%s", prefix, suffix)
				}
			}
			candidate.Tables = append(candidate.Tables, summary)
		}
		candidate.Compatible = allPresent && candidate.StatusMessage == ""
		if candidate.Compatible {
			candidate.StatusMessage = fmt.Sprintf("Ready to import (%d plots)", candidate.PlotCount)
		}
		candidates = append(candidates, candidate)
	}
	slices.SortFunc(candidates, func(a, b AccessProjectCandidate) int {
		return strings.Compare(a.SourceProject, b.SourceProject)
	})
	return AccessDatabaseInspection{
		SourcePath:   resolved,
		SourceSHA256: digest,
		Projects:     candidates,
	}, nil
}

func adaptAccessRows(ctx context.Context, cols []accessreader.Column, rows []accessreader.Row) (map[string]int, [][]any, error) {
	srcCols := make([]accessimport.SourceColumn, len(cols))
	colIndex := make(map[string]int, len(cols))
	for i, c := range cols {
		srcCols[i] = accessimport.SourceColumn{Name: c.Name, Type: c.Type}
		colIndex[strings.ToLower(c.Name)] = i
	}
	adapted := make([][]any, len(rows))
	for rIdx, rawRow := range rows {
		vals, err := accessimport.SQLiteRow(ctx, srcCols, []any(rawRow))
		if err != nil {
			return nil, nil, fmt.Errorf("row %d: %w", rIdx+1, err)
		}
		adapted[rIdx] = vals
	}
	return colIndex, adapted, nil
}

func querySQLiteTableColumns(ctx context.Context, tx *sql.Tx, tableName string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", vprocore.QuoteHeaderIdentifier(tableName)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make(map[string]string)
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[strings.ToLower(name)] = name
	}
	return cols, rows.Err()
}

type streamInsertSpec struct {
	targetTable        string
	targetToSource     [][2]string // nil means intersect source columns with target SQLite columns
	computed           map[string]func([]any, map[string]int) any
	columnOverrides    map[string]func(any) any
	requireEnvPlot     bool
	plotSourceCol      string
	dedupePlotKey      bool
	recordEnvPlots     map[string]struct{}
	childIdentityTable string
}

type streamTableMetrics struct {
	actualSourceTable string
	columnCount       int
	importedRows      int
	malformedRows     int
	orphanRows        int
}

func adaptedPlotKey(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

type accessRowSource interface {
	ColumnsList() []accessreader.Column
	EnableRowQuarantine()
	Next() bool
	Row() accessreader.Row
	Err() error
	QuarantinedRows() []accessreader.QuarantinedRow
	Close() error
}

type accessReaderAdapter struct {
	*accessreader.Reader
}

func (a accessReaderAdapter) ColumnsList() []accessreader.Column            { return a.Reader.Columns }
func (a accessReaderAdapter) QuarantinedRows() []accessreader.QuarantinedRow { return a.Reader.Quarantined }

func streamAccessTableIntoSQLite(
	ctx context.Context,
	db *accessreader.DB,
	tx *sql.Tx,
	quarantineStmt *sql.Stmt,
	childIdStmt *sql.Stmt,
	sourceTable string,
	validEnvPlots map[string]struct{},
	specs []streamInsertSpec,
) (streamTableMetrics, error) {
	reader, err := db.Scan(sourceTable)
	if err != nil {
		return streamTableMetrics{}, fmt.Errorf("open %s: %w", sourceTable, err)
	}
	defer reader.Close()
	return streamAccessSourceIntoSQLite(ctx, accessReaderAdapter{reader}, tx, quarantineStmt, childIdStmt, sourceTable, validEnvPlots, specs)
}

func streamAccessSourceIntoSQLite(
	ctx context.Context,
	reader accessRowSource,
	tx *sql.Tx,
	quarantineStmt *sql.Stmt,
	childIdStmt *sql.Stmt,
	sourceTable string,
	validEnvPlots map[string]struct{},
	specs []streamInsertSpec,
) (streamTableMetrics, error) {
	var metrics streamTableMetrics
	reader.EnableRowQuarantine()
	cols := reader.ColumnsList()
	metrics.actualSourceTable = sourceTable
	metrics.columnCount = len(cols)

	srcCols := make([]accessimport.SourceColumn, len(cols))
	colIndex := make(map[string]int, len(cols))
	for i, c := range cols {
		srcCols[i] = accessimport.SourceColumn{Name: c.Name, Type: c.Type}
		colIndex[strings.ToLower(c.Name)] = i
	}
	colPlan, err := accessimport.PlanColumns(ctx, srcCols)
	if err != nil {
		return metrics, err
	}
	idIdx, hasIDCol := colIndex["id"]
	if !hasIDCol {
		idIdx = -1
	}

	type preparedTarget struct {
		spec          streamInsertSpec
		stmt          *sql.Stmt
		sourceIndices []int
		overrides     []func(any) any
		computedKeys  []string
		argsBuf       []any
		seenPlots     map[string]struct{}
		plotIdx       int
	}
	targets := make([]preparedTarget, len(specs))
	for sIdx, spec := range specs {
		mapping := spec.targetToSource
		if mapping == nil {
			dstCols, err := querySQLiteTableColumns(ctx, tx, spec.targetTable)
			if err != nil {
				return metrics, err
			}
			for _, c := range cols {
				if canonical, ok := dstCols[strings.ToLower(c.Name)]; ok {
					mapping = append(mapping, [2]string{canonical, c.Name})
				}
			}
		}
		if len(spec.computed) > 0 {
			computedLower := make(map[string]struct{}, len(spec.computed))
			for k := range spec.computed {
				computedLower[strings.ToLower(k)] = struct{}{}
			}
			filtered := make([][2]string, 0, len(mapping))
			for _, pair := range mapping {
				if _, overridden := computedLower[strings.ToLower(pair[0])]; overridden {
					continue
				}
				filtered = append(filtered, pair)
			}
			mapping = filtered
		}
		overrideByLower := make(map[string]func(any) any, len(spec.columnOverrides))
		for k, fn := range spec.columnOverrides {
			overrideByLower[strings.ToLower(k)] = fn
		}
		targetCols := make([]string, 0, len(mapping)+len(spec.computed))
		sourceIndices := make([]int, len(mapping))
		overrides := make([]func(any) any, len(mapping))
		for i, pair := range mapping {
			overrideFn := overrideByLower[strings.ToLower(pair[0])]
			idx, ok := colIndex[strings.ToLower(pair[1])]
			if !ok {
				if strings.EqualFold(pair[1], "Collected") || (strings.EqualFold(pair[0], "Project") && overrideFn != nil) {
					sourceIndices[i] = -1
				} else {
					return metrics, fmt.Errorf("source table %s for %s is missing column %q", sourceTable, spec.targetTable, pair[1])
				}
			} else {
				sourceIndices[i] = idx
			}
			overrides[i] = overrideFn
			targetCols = append(targetCols, vprocore.QuoteHeaderIdentifier(pair[0]))
		}
		var computedKeys []string
		for k := range spec.computed {
			computedKeys = append(computedKeys, k)
		}
		slices.Sort(computedKeys)
		for _, k := range computedKeys {
			targetCols = append(targetCols, vprocore.QuoteHeaderIdentifier(k))
		}
		marks := make([]string, len(targetCols))
		for i := range marks {
			marks[i] = "?"
		}
		stmtSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			vprocore.QuoteHeaderIdentifier(spec.targetTable),
			strings.Join(targetCols, ", "),
			strings.Join(marks, ", "))
		stmt, err := tx.PrepareContext(ctx, stmtSQL)
		if err != nil {
			return metrics, fmt.Errorf("prepare insert into %s: %w", spec.targetTable, err)
		}
		defer stmt.Close()

		plotCol := spec.plotSourceCol
		if plotCol == "" {
			plotCol = "PlotNumber"
		}
		pIdx, hasPlot := colIndex[strings.ToLower(plotCol)]
		if !hasPlot {
			pIdx = -1
		}
		var seen map[string]struct{}
		if spec.dedupePlotKey || spec.recordEnvPlots != nil {
			seen = make(map[string]struct{})
		}
		targets[sIdx] = preparedTarget{
			spec:          spec,
			stmt:          stmt,
			sourceIndices: sourceIndices,
			overrides:     overrides,
			computedKeys:  computedKeys,
			argsBuf:       make([]any, len(sourceIndices)+len(computedKeys)),
			seenPlots:     seen,
			plotIdx:       pIdx,
		}
	}

	reserveQuarantinedID := func(tg *preparedTarget, row []any) {
		if childIdStmt == nil || tg.spec.childIdentityTable == "" || idIdx < 0 || idIdx >= len(row) {
			return
		}
		if idVal, ok := row[idIdx].(int64); ok && idVal > 0 {
			_, _ = childIdStmt.ExecContext(ctx, tg.spec.childIdentityTable, idVal)
		}
	}

	adapted := make([]any, len(colPlan))
	rowOrdinal := 0
	for reader.Next() {
		rowOrdinal++
		if rowOrdinal&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return metrics, err
			}
		}
		rawRow := reader.Row()
		var adaptErr error
		for cIdx, cellVal := range rawRow {
			adapted[cIdx], adaptErr = accessimport.SQLiteValue(cellVal)
			if adaptErr != nil {
				break
			}
		}
		if adaptErr != nil {
			metrics.malformedRows++
			if quarantineStmt != nil {
				_, _ = quarantineStmt.ExecContext(ctx, sourceTable, rowOrdinal, "", "malformed_value: "+adaptErr.Error(), "{}")
			}
			continue
		}

		skipRow := false
		for tIdx := range targets {
			tg := &targets[tIdx]
			plotKey := ""
			if tg.plotIdx >= 0 && adapted[tg.plotIdx] != nil {
				plotKey = adaptedPlotKey(adapted[tg.plotIdx])
			}
			if tg.spec.requireEnvPlot && validEnvPlots != nil {
				if _, ok := validEnvPlots[plotKey]; !ok {
					metrics.orphanRows++
					reserveQuarantinedID(tg, adapted)
					if quarantineStmt != nil {
						payload, _ := json.Marshal(adapted)
						_, _ = quarantineStmt.ExecContext(ctx, sourceTable, rowOrdinal, plotKey, "orphan_missing_env_plot", string(payload))
					}
					skipRow = true
					break
				}
			}
			if tg.seenPlots != nil && plotKey != "" {
				if _, dup := tg.seenPlots[plotKey]; dup {
					metrics.malformedRows++
					reserveQuarantinedID(tg, adapted)
					if quarantineStmt != nil {
						payload, _ := json.Marshal(adapted)
						_, _ = quarantineStmt.ExecContext(ctx, sourceTable, rowOrdinal, plotKey, "duplicate_plot_key", string(payload))
					}
					skipRow = true
					break
				}
			}
		}
		if skipRow {
			continue
		}

		insertedPrimary := false
		for tIdx := range targets {
			tg := &targets[tIdx]
			args := tg.argsBuf
			nSrc := len(tg.sourceIndices)
			for i, idx := range tg.sourceIndices {
				var v any
				if idx >= 0 {
					v = adapted[idx]
				}
				if fn := tg.overrides[i]; fn != nil {
					v = fn(v)
				}
				args[i] = v
			}
			for i, k := range tg.computedKeys {
				args[nSrc+i] = tg.spec.computed[k](adapted, colIndex)
			}
			if _, execErr := tg.stmt.ExecContext(ctx, args...); execErr != nil {
				metrics.malformedRows++
				reserveQuarantinedID(tg, adapted)
				if quarantineStmt != nil {
					payload, _ := json.Marshal(adapted)
					_, _ = quarantineStmt.ExecContext(ctx, sourceTable, rowOrdinal, "", "sqlite_insert_constraint: "+execErr.Error(), string(payload))
				}
				break
			}
			if tg.plotIdx >= 0 && adapted[tg.plotIdx] != nil {
				pk := adaptedPlotKey(adapted[tg.plotIdx])
				if tg.seenPlots != nil && pk != "" {
					tg.seenPlots[pk] = struct{}{}
				}
				if tg.spec.recordEnvPlots != nil && pk != "" {
					tg.spec.recordEnvPlots[pk] = struct{}{}
				}
			}
			if tIdx == 0 {
				insertedPrimary = true
			}
		}
		if insertedPrimary {
			metrics.importedRows++
		}
	}
	if err := reader.Err(); err != nil {
		return metrics, fmt.Errorf("scan %s: %w", sourceTable, err)
	}
	for _, q := range reader.QuarantinedRows() {
		metrics.malformedRows++
		if quarantineStmt != nil {
			_, _ = quarantineStmt.ExecContext(ctx, sourceTable, q.RowNumber, "", "access_decode: "+q.Reason, "{}")
		}
	}
	return metrics, nil
}

func ImportAccessProject(s *vprocore.ContextService, ctx context.Context, contextID string, request AccessProjectImportRequest) (_ AccessProjectImportResult, resultErr error) {
	targetProject := strings.TrimSpace(request.TargetProject)
	if !vprocore.ProjectNamePattern.MatchString(targetProject) || strings.EqualFold(targetProject, "None") ||
		strings.EqualFold(targetProject, "Sample") || strings.EqualFold(targetProject, "Master") {
		return AccessProjectImportResult{}, errors.New("target project name must start with a letter, use only letters/digits/underscore (max 31 chars), and not be None, Sample, or Master")
	}
	destPath := strings.TrimSpace(request.DestinationPath)
	if destPath != "" && !strings.EqualFold(filepath.Ext(destPath), ".db") {
		destPath += ".db"
	}
	resolvedDest, parentInfo, err := vprocore.FreshSQLiteDestination(destPath, "Project")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	resolvedSource, sourceDigestBefore, err := resolveAccessSourcePath(request.SourcePath)
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	review, err := s.ReadNewProjectTemplates(ctx, contextID)
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	ddlPlan, err := vprocore.PlanNewProjectDDL(ctx, targetProject, review)
	if err != nil {
		return AccessProjectImportResult{}, err
	}

	db, err := accessreader.Open(resolvedSource)
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	tables, err := db.Tables()
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	byLower := make(map[string]accessreader.Table, len(tables))
	for _, tbl := range tables {
		if !tbl.Linked {
			byLower[strings.ToLower(tbl.Name)] = tbl
		}
	}
	sourceProject := strings.TrimSpace(request.SourceProject)
	envTbl, ok := byLower[strings.ToLower(sourceProject+"_Env")]
	if !ok {
		return AccessProjectImportResult{}, fmt.Errorf("source project %q (_Env table) was not found in %s", sourceProject, filepath.Base(resolvedSource))
	}
	version := strings.TrimSpace(envTbl.Description)
	if version != "VP08" && version != "VP07" && version != "VP06" && version != "VP05" {
		return AccessProjectImportResult{}, fmt.Errorf("source project %s has unsupported version %q (supported: VP05, VP06, VP07, VP08)", sourceProject, version)
	}

	requireSourceTable := func(suffix string) (string, error) {
		tbl, exists := byLower[strings.ToLower(sourceProject+"_"+suffix)]
		if !exists {
			return "", fmt.Errorf("missing required table %s_%s", sourceProject, suffix)
		}
		return tbl.Name, nil
	}

	envActualName, err := requireSourceTable("Env")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	vegActualName, err := requireSourceTable("Veg")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	otherActualName, err := requireSourceTable("Other")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	humusActualName, err := requireSourceTable("Humus")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	mineralActualName, err := requireSourceTable("Mineral")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	auditActualName, err := requireSourceTable("Audit")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	var metaActualName, adminActualName string
	if version == "VP08" {
		if metaActualName, err = requireSourceTable("Metadata"); err != nil {
			return AccessProjectImportResult{}, err
		}
		if adminActualName, err = requireSourceTable("Admin"); err != nil {
			return AccessProjectImportResult{}, err
		}
	} else if version == "VP07" {
		if _, exists := byLower[strings.ToLower(sourceProject+"_Metadata")]; !exists {
			return AccessProjectImportResult{}, fmt.Errorf("VP07 import requires table %s_Metadata", sourceProject)
		}
	}

	type companionSourceTable struct {
		kind        string // "Hierarchy" | "Lump" | "Profile"
		prefix      string
		sourceTable string
		description string
	}
	var suSourceTables []struct {
		prefix      string
		table       string
		description string
	}
	var companionTables []companionSourceTable

	hasAllColumnsCI := func(cols []accessreader.Column, required ...string) bool {
		colMap := make(map[string]string, len(cols))
		for _, c := range cols {
			colMap[strings.ToLower(c.Name)] = c.Type
		}
		for _, req := range required {
			if _, ok := colMap[strings.ToLower(req)]; !ok {
				return false
			}
		}
		return true
	}

	for _, tbl := range tables {
		if tbl.Linked || strings.HasPrefix(strings.ToUpper(tbl.Name), "USYS") {
			continue
		}
		upperName := strings.ToUpper(tbl.Name)
		switch {
		case len(tbl.Name) > 3 && strings.HasSuffix(upperName, "_SU"):
			suPrefix := tbl.Name[:len(tbl.Name)-3]
			if !vprocore.ProjectNamePattern.MatchString(suPrefix) {
				continue
			}
			r, rErr := db.Scan(tbl.Name)
			if rErr != nil {
				continue
			}
			ok := hasAllColumnsCI(r.Columns, "PlotNumber", "SiteUnit")
			_ = r.Close()
			if ok {
				desc := strings.TrimSpace(tbl.Description)
				if desc == "" {
					desc = "VP04"
				}
				suSourceTables = append(suSourceTables, struct {
					prefix      string
					table       string
					description string
				}{prefix: suPrefix, table: tbl.Name, description: desc})
			}

		case len(tbl.Name) > len("_Hierarchy") && strings.HasSuffix(upperName, "_HIERARCHY"):
			prefix := tbl.Name[:len(tbl.Name)-len("_Hierarchy")]
			if !vprocore.ProjectNamePattern.MatchString(prefix) {
				continue
			}
			r, rErr := db.Scan(tbl.Name)
			if rErr != nil {
				continue
			}
			colTypes := map[string]string{}
			for _, c := range r.Columns {
				colTypes[strings.ToLower(c.Name)] = c.Type
			}
			_ = r.Close()
			if colTypes["id"] == "integer" && colTypes["name"] == "text" && colTypes["parent"] == "integer" && colTypes["level"] == "integer" {
				desc := strings.TrimSpace(tbl.Description)
				if desc == "" {
					desc = "VP04"
				}
				companionTables = append(companionTables, companionSourceTable{
					kind: "Hierarchy", prefix: prefix, sourceTable: tbl.Name, description: desc,
				})
			}

		case len(tbl.Name) > len("_Lump") && strings.HasSuffix(upperName, "_LUMP"):
			prefix := tbl.Name[:len(tbl.Name)-len("_Lump")]
			if !vprocore.ProjectNamePattern.MatchString(prefix) {
				continue
			}
			r, rErr := db.Scan(tbl.Name)
			if rErr != nil {
				continue
			}
			ok := hasAllColumnsCI(r.Columns, "LumpCode", "SppCode")
			_ = r.Close()
			if ok {
				desc := strings.TrimSpace(tbl.Description)
				if desc == "" {
					desc = "VP05"
				}
				companionTables = append(companionTables, companionSourceTable{
					kind: "Lump", prefix: prefix, sourceTable: tbl.Name, description: desc,
				})
			}

		case len(tbl.Name) > len("_Profile") && strings.HasSuffix(upperName, "_PROFILE"):
			prefix := tbl.Name[:len(tbl.Name)-len("_Profile")]
			if !vprocore.ProjectNamePattern.MatchString(prefix) {
				continue
			}
			r, rErr := db.Scan(tbl.Name)
			if rErr != nil {
				continue
			}
			ok := hasAllColumnsCI(r.Columns, vprocore.PlotProfileColumns...)
			_ = r.Close()
			if ok {
				desc := strings.TrimSpace(tbl.Description)
				if desc == "" {
					desc = "VP05-2"
				}
				companionTables = append(companionTables, companionSourceTable{
					kind: "Profile", prefix: prefix, sourceTable: tbl.Name, description: desc,
				})
			}
		}
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(resolvedDest), ".vpro-access-import-*.db")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
			_ = os.Remove(tmpPath + "-wal")
			_ = os.Remove(tmpPath + "-shm")
		}
	}()
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		return AccessProjectImportResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		return AccessProjectImportResult{}, err
	}

	sqlDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_foreign_keys=0&_journal_mode=MEMORY&_synchronous=OFF")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	sqlDB.SetMaxOpenConns(1)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		sqlDB.Close()
		return AccessProjectImportResult{}, err
	}

	var envM, vegM, otherM, humusM, mineralM, auditM, metaM, adminM streamTableMetrics
	var importedSUs []string
	totalMalformed := 0
	totalOrphan := 0

	writeTables := func() error {
		var deferredIndexes []string
		for _, tblPlan := range ddlPlan.Tables {
			for _, mapping := range tblPlan.Schema {
				if mapping.SQL != nil {
					stmtText := strings.TrimSpace(*mapping.SQL)
					if strings.HasPrefix(strings.ToUpper(stmtText), "CREATE INDEX") || strings.HasPrefix(strings.ToUpper(stmtText), "CREATE UNIQUE INDEX") {
						deferredIndexes = append(deferredIndexes, stmtText)
						continue
					}
					if _, err := tx.ExecContext(ctx, stmtText); err != nil {
						return fmt.Errorf("create schema for %s: %w", tblPlan.TargetTable, err)
					}
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, suffix := range vprocore.CoreTables {
			if _, err := tx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP08')`, targetProject+"_"+suffix); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ImportQuarantine" (
			"ID" INTEGER PRIMARY KEY AUTOINCREMENT,
			"SourceTable" TEXT NOT NULL,
			"SourceRow" INTEGER NOT NULL,
			"PlotNumber" TEXT NOT NULL,
			"Reason" TEXT NOT NULL,
			"PayloadJSON" TEXT NOT NULL
		)`); err != nil {
			return err
		}
		quarantineStmt, err := tx.PrepareContext(ctx, `INSERT INTO "__VPRO_ImportQuarantine" ("SourceTable","SourceRow","PlotNumber","Reason","PayloadJSON") VALUES (?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer quarantineStmt.Close()

		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
			"ChildTable" TEXT NOT NULL,
			"ID" INTEGER NOT NULL,
			PRIMARY KEY ("ChildTable", "ID")
		)`); err != nil {
			return err
		}
		childIdStmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer childIdStmt.Close()

		coalesceFalseBool := func(v any) any {
			if v == nil {
				return int64(0)
			}
			return v
		}
		auditOverrides := map[string]func(any) any{
			"Project": func(any) any { return targetProject },
			"Restore": coalesceFalseBool,
			"Flag":    coalesceFalseBool,
		}
		auditComputed := map[string]func([]any, map[string]int) any{
			"Project": func([]any, map[string]int) any { return targetProject },
		}

		validEnvPlots := make(map[string]struct{}, 1024)
		accumulate := func(m streamTableMetrics) {
			totalMalformed += m.malformedRows
			totalOrphan += m.orphanRows
		}

		switch version {
		case "VP08":
			if envM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, envActualName, nil, []streamInsertSpec{
				{targetTable: targetProject + "_Env", dedupePlotKey: true, recordEnvPlots: validEnvPlots},
			}); err != nil {
				return err
			}
			accumulate(envM)
			if vegM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, vegActualName, validEnvPlots, []streamInsertSpec{
				{targetTable: targetProject + "_Veg", requireEnvPlot: true, childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_Veg")},
			}); err != nil {
				return err
			}
			accumulate(vegM)
			if otherM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, otherActualName, validEnvPlots, []streamInsertSpec{
				{targetTable: targetProject + "_Other", requireEnvPlot: true, childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_Other")},
			}); err != nil {
				return err
			}
			accumulate(otherM)
			if humusM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, humusActualName, validEnvPlots, []streamInsertSpec{
				{targetTable: targetProject + "_Humus", requireEnvPlot: true, childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_Humus")},
			}); err != nil {
				return err
			}
			accumulate(humusM)
			if mineralM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, mineralActualName, validEnvPlots, []streamInsertSpec{
				{targetTable: targetProject + "_Mineral", requireEnvPlot: true, childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_Mineral")},
			}); err != nil {
				return err
			}
			accumulate(mineralM)
			if auditM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, auditActualName, nil, []streamInsertSpec{
				{targetTable: targetProject + "_Audit", computed: auditComputed, columnOverrides: auditOverrides, childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_Audit")},
			}); err != nil {
				return err
			}
			accumulate(auditM)
			if metaM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, metaActualName, nil, []streamInsertSpec{
				{targetTable: targetProject + "_Metadata", childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_Metadata")},
			}); err != nil {
				return err
			}
			accumulate(metaM)
			if adminM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, adminActualName, validEnvPlots, []streamInsertSpec{
				{targetTable: targetProject + "_Admin", requireEnvPlot: true, plotSourceCol: "Plot", dedupePlotKey: true},
			}); err != nil {
				return err
			}
			accumulate(adminM)
		case "VP07":
			if envM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, envActualName, nil, []streamInsertSpec{
				{targetTable: targetProject + "_Env", targetToSource: vp07EnvMapping, dedupePlotKey: true, recordEnvPlots: validEnvPlots},
				{targetTable: targetProject + "_Admin", targetToSource: vp07AdminFromEnvMapping},
			}); err != nil {
				return err
			}
			accumulate(envM)
			adminM = streamTableMetrics{actualSourceTable: envActualName, columnCount: 20, importedRows: envM.importedRows}
		case "VP06":
			if envM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, envActualName, nil, []streamInsertSpec{
				{targetTable: targetProject + "_Env", targetToSource: vp06EnvMapping, dedupePlotKey: true, recordEnvPlots: validEnvPlots},
				{targetTable: targetProject + "_Admin", targetToSource: vp06AdminFromEnvMapping},
			}); err != nil {
				return err
			}
			accumulate(envM)
			adminM = streamTableMetrics{actualSourceTable: envActualName, columnCount: 20, importedRows: envM.importedRows}
		case "VP05":
			rCheck, err := db.Scan(envActualName)
			if err != nil {
				return err
			}
			hasLonDeg, hasLatDeg := false, false
			for _, c := range rCheck.Columns {
				if strings.EqualFold(c.Name, "LongitudeDegrees") {
					hasLonDeg = true
				} else if strings.EqualFold(c.Name, "LatitudeDegrees") {
					hasLatDeg = true
				}
			}
			_ = rCheck.Close()
			if !hasLonDeg || !hasLatDeg {
				return errors.New("VP05 Env table is missing LongitudeDegrees or LatitudeDegrees")
			}
			getCol := func(r []any, idx map[string]int, name string) any {
				if i, ok := idx[strings.ToLower(name)]; ok && i >= 0 && i < len(r) {
					return r[i]
				}
				return nil
			}
			computed := map[string]func([]any, map[string]int) any{
				"Longitude": func(r []any, idx map[string]int) any {
					return convertLongLatToDeg(getCol(r, idx, "LongitudeDegrees"), getCol(r, idx, "LongitudeMinutes"), getCol(r, idx, "LongitudeSeconds"))
				},
				"Latitude": func(r []any, idx map[string]int) any {
					return convertLongLatToDeg(getCol(r, idx, "LatitudeDegrees"), getCol(r, idx, "LatitudeMinutes"), getCol(r, idx, "LatitudeSeconds"))
				},
			}
			if envM, err = streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, envActualName, nil, []streamInsertSpec{
				{targetTable: targetProject + "_Env", targetToSource: vp05EnvMapping, computed: computed, dedupePlotKey: true, recordEnvPlots: validEnvPlots},
				{targetTable: targetProject + "_Admin", targetToSource: vp05AdminFromEnvMapping},
			}); err != nil {
				return err
			}
			accumulate(envM)
			adminM = streamTableMetrics{actualSourceTable: envActualName, columnCount: 20, importedRows: envM.importedRows}
		}

		if version != "VP08" {
			legacyPairs := []struct {
				suffix string
				src    string
				cols   []string
				reqEnv bool
				dst    *streamTableMetrics
			}{
				{"Veg", vegActualName, legacyVegColumns, true, &vegM},
				{"Other", otherActualName, legacyOtherColumns, true, &otherM},
				{"Humus", humusActualName, legacyHumusColumns, true, &humusM},
				{"Mineral", mineralActualName, legacyMineralColumns, true, &mineralM},
				{"Audit", auditActualName, legacyAuditColumns, false, &auditM},
			}
			for _, item := range legacyPairs {
				pairs := make([][2]string, len(item.cols))
				for i, c := range item.cols {
					pairs[i] = [2]string{c, c}
				}
				var envSet map[string]struct{}
				if item.reqEnv {
					envSet = validEnvPlots
				}
				spec := streamInsertSpec{
					targetTable:        targetProject + "_" + item.suffix,
					targetToSource:     pairs,
					requireEnvPlot:     item.reqEnv,
					childIdentityTable: vprocore.QuoteHeaderIdentifier(targetProject + "_" + item.suffix),
				}
				if item.suffix == "Audit" {
					spec.computed = auditComputed
					spec.columnOverrides = auditOverrides
				}
				m, sErr := streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, childIdStmt, item.src, envSet, []streamInsertSpec{spec})
				if sErr != nil {
					return sErr
				}
				*item.dst = m
				accumulate(m)
			}
		}

		tableExistsCI := func(name string) (bool, error) {
			var exists bool
			err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE)`, name).Scan(&exists)
			return exists, err
		}

		for _, su := range suSourceTables {
			targetName := su.prefix + "_SU"
			exists, exErr := tableExistsCI(targetName)
			if exErr != nil {
				return exErr
			}
			if exists {
				continue
			}
			suTable := vprocore.QuoteHeaderIdentifier(targetName)
			if _, err := tx.ExecContext(ctx, `CREATE TABLE `+suTable+`("PlotNumber" VARCHAR,"SiteUnit" VARCHAR);
				CREATE UNIQUE INDEX `+vprocore.QuoteHeaderIdentifier("uidx_"+su.prefix+"_SU_PlotNumber")+` ON `+suTable+`("PlotNumber");
				CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+su.prefix+"_SU_SiteUnit")+` ON `+suTable+`("SiteUnit")`); err != nil {
				return err
			}
			suM, suErr := streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, nil, su.table, validEnvPlots, []streamInsertSpec{
				{
					targetTable:    targetName,
					targetToSource: [][2]string{{"PlotNumber", "PlotNumber"}, {"SiteUnit", "SiteUnit"}},
					requireEnvPlot: true,
					dedupePlotKey:  true,
				},
			})
			if suErr != nil {
				return suErr
			}
			accumulate(suM)
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO "_table_metadata" ("table_name", "description") VALUES (?, ?)`, targetName, su.description); err != nil {
				return err
			}
			importedSUs = append(importedSUs, su.prefix)
		}

		for _, comp := range companionTables {
			targetName := comp.prefix + "_" + comp.kind
			exists, exErr := tableExistsCI(targetName)
			if exErr != nil {
				return exErr
			}
			if exists {
				continue
			}
			qTbl := vprocore.QuoteHeaderIdentifier(targetName)
			switch comp.kind {
			case "Hierarchy":
				if _, err := tx.ExecContext(ctx, `CREATE TABLE `+qTbl+` (
					"ID" INTEGER PRIMARY KEY,
					"Name" VARCHAR,
					"Parent" INTEGER,
					"Level" SMALLINT,
					"Tag" VARCHAR,
					"MyOrder" VARCHAR,
					"ChildID" SMALLINT,
					"StartChild" SMALLINT,
					"LastChild" SMALLINT,
					"Flag" BOOLEAN
				);
				CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+targetName+"_Parent")+` ON `+qTbl+` ("Parent");
				CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+targetName+"_Level")+` ON `+qTbl+` ("Level")`); err != nil {
					return err
				}
			case "Lump":
				if _, err := tx.ExecContext(ctx, `CREATE TABLE `+qTbl+` (
					"LumpCode" VARCHAR,
					"SppCode" VARCHAR,
					"Use" BOOLEAN
				);
				CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+targetName+"_LumpCode")+` ON `+qTbl+` ("LumpCode");
				CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+targetName+"_SppCode")+` ON `+qTbl+` ("SppCode")`); err != nil {
					return err
				}
			case "Profile":
				if _, err := tx.ExecContext(ctx, `CREATE TABLE `+qTbl+` (
					"Order" SMALLINT,
					"Table" VARCHAR,
					"Field" VARCHAR,
					"Operator" VARCHAR,
					"Layer" VARCHAR,
					"Species" VARCHAR,
					"Criteria" VARCHAR,
					"Operation" VARCHAR,
					"PlotCount" INTEGER
				)`); err != nil {
					return err
				}
			}
			cM, cErr := streamAccessTableIntoSQLite(ctx, db, tx, quarantineStmt, nil, comp.sourceTable, nil, []streamInsertSpec{
				{targetTable: targetName},
			})
			if cErr != nil {
				return cErr
			}
			accumulate(cM)
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO "_table_metadata" ("table_name", "description") VALUES (?, ?)`, targetName, comp.description); err != nil {
				return err
			}
		}

		for _, idxSQL := range deferredIndexes {
			if _, err := tx.ExecContext(ctx, idxSQL); err != nil {
				return fmt.Errorf("create deferred index: %w", err)
			}
		}
		for _, suffix := range []string{"Env", "Veg"} {
			qTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_" + suffix)
			qIdx := vprocore.QuoteHeaderIdentifier("idx_vpro_nocase_" + targetProject + "_" + suffix + "_PlotNumber")
			_, _ = tx.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s ("PlotNumber" COLLATE NOCASE ASC, "PlotNumber" COLLATE BINARY ASC)`, qIdx, qTbl))
			qBin := vprocore.QuoteHeaderIdentifier("idx_vpro_bin_" + targetProject + "_" + suffix + "_PlotNumber")
			_, _ = tx.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s ("PlotNumber")`, qBin, qTbl))
		}
		qAdm := vprocore.QuoteHeaderIdentifier(targetProject + "_Admin")
		qAdmIdx := vprocore.QuoteHeaderIdentifier("idx_vpro_bin_" + targetProject + "_Admin_Plot")
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s ("Plot")`, qAdmIdx, qAdm)); err != nil {
			return err
		}
		qAud := vprocore.QuoteHeaderIdentifier(targetProject + "_Audit")
		qAudIdx := vprocore.QuoteHeaderIdentifier("idx_vpro_bin_" + targetProject + "_Audit_PlotNumber")
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s ("PlotNumber")`, qAudIdx, qAud)); err != nil {
			return err
		}
		if err := seedImportedChildIdentities(ctx, tx, sourceProject, targetProject); err != nil {
			return err
		}
		return nil
	}

	if err := writeTables(); err != nil {
		_ = tx.Rollback()
		_ = sqlDB.Close()
		return AccessProjectImportResult{}, err
	}
	if err := tx.Commit(); err != nil {
		_ = sqlDB.Close()
		return AccessProjectImportResult{}, err
	}
	_, _ = sqlDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE); PRAGMA journal_mode = DELETE;")
	if err := sqlDB.Close(); err != nil {
		return AccessProjectImportResult{}, err
	}

	// Verify source Access file was not mutated during import
	_, sourceDigestAfter, err := resolveAccessSourcePath(resolvedSource)
	if err != nil || sourceDigestAfter != sourceDigestBefore {
		return AccessProjectImportResult{}, errors.New("source Access database changed during import; publication aborted")
	}

	// Verify staged SQLite file passes VP08 inspection before linking
	inspected, err := vprocore.InspectFileContext(ctx, tmpPath)
	if err != nil {
		return AccessProjectImportResult{}, fmt.Errorf("verify staged SQLite project: %w", err)
	}
	compatible := false
	for _, p := range inspected {
		if p.Name == targetProject && p.Compatible {
			compatible = true
			break
		}
	}
	if !compatible {
		return AccessProjectImportResult{}, errors.New("staged SQLite database did not pass VP08 compatibility validation")
	}

	currentParent, err := os.Stat(filepath.Dir(resolvedDest))
	if err != nil || !os.SameFile(parentInfo, currentParent) {
		return AccessProjectImportResult{}, errors.New("destination parent directory changed during import; publication aborted")
	}
	if err := sqlutil.LinkOrCreateExclusive(tmpPath, resolvedDest); err != nil {
		return AccessProjectImportResult{}, fmt.Errorf("publish SQLite database %s: %w", resolvedDest, err)
	}
	published = true
	_ = os.Remove(tmpPath)

	if err := vprocore.InitializeSQLiteWAL(ctx, resolvedDest); err != nil {
		_ = os.Remove(resolvedDest + "-wal")
		_ = os.Remove(resolvedDest + "-shm")
		_ = os.Remove(resolvedDest)
		return AccessProjectImportResult{}, fmt.Errorf("initialize SQLite WAL on %s: %w", resolvedDest, err)
	}

	destDigestHex, err := streamFileSHA256(resolvedDest)
	if err != nil {
		return AccessProjectImportResult{}, err
	}

	summaries := []AccessImportTableSummary{
		{Suffix: "Env", SourceTable: envActualName, Description: "VP08", RowCount: envM.importedRows, ColumnCount: 112, Present: true, Action: "Imported"},
		{Suffix: "Veg", SourceTable: vegActualName, Description: "VP08", RowCount: vegM.importedRows, ColumnCount: 44, Present: true, Action: "Imported"},
		{Suffix: "Other", SourceTable: otherActualName, Description: "VP08", RowCount: otherM.importedRows, ColumnCount: 11, Present: true, Action: "Imported"},
		{Suffix: "Humus", SourceTable: humusActualName, Description: "VP08", RowCount: humusM.importedRows, ColumnCount: 18, Present: true, Action: "Imported"},
		{Suffix: "Mineral", SourceTable: mineralActualName, Description: "VP08", RowCount: mineralM.importedRows, ColumnCount: 28, Present: true, Action: "Imported"},
		{Suffix: "Audit", SourceTable: auditActualName, Description: "VP08", RowCount: auditM.importedRows, ColumnCount: 11, Present: true, Action: "Imported"},
		{Suffix: "Metadata", SourceTable: metaActualName, Description: "VP08", RowCount: metaM.importedRows, ColumnCount: 75, Present: version == "VP08", Action: "Imported"},
		{Suffix: "Admin", SourceTable: adminActualName, Description: "VP08", RowCount: adminM.importedRows, ColumnCount: 20, Present: true, Action: "Imported"},
	}

	result := AccessProjectImportResult{
		SourcePath:              resolvedSource,
		SourceProject:           sourceProject,
		SourceVersion:           version,
		ConversionMode:          fmt.Sprintf("%s -> VP08", version),
		TargetProject:           targetProject,
		DestinationPath:         resolvedDest,
		DestinationSHA:          destDigestHex,
		PlotCount:               envM.importedRows,
		QuarantinedMalformedRow: totalMalformed,
		QuarantinedOrphanRows:   totalOrphan,
		Tables:                  summaries,
		ImportedSUs:             importedSUs,
	}

	if request.ActivateOnSuccess {
		state, switchErr := s.SwitchContext(contextID, vprocore.ContextSelection{
			Project:       targetProject,
			ProjectPath:   resolvedDest,
			SU:            "None",
			SUPath:        "",
			Hierarchy:     "None",
			HierarchyPath: "",
		})
		if switchErr != nil {
			return result, fmt.Errorf("project imported to %s, but switching context failed: %w", resolvedDest, switchErr)
		}
		_ = s.Projects.EnsureActiveProjectWAL(ctx)
		result.Activated = true
		result.State = &state
	} else {
		state, stateErr := s.Projects.GetState(ctx)
		if stateErr == nil {
			result.State = &state
		}
	}
	return result, nil
}

func resolveAuditChildSuffixes(rawTable, sourceProject, targetProject string) []string {
	t := strings.ToLower(strings.TrimSpace(rawTable))
	if t == "" {
		return []string{"Veg", "Humus", "Mineral", "Other", "Metadata"}
	}
	for _, pfx := range []string{strings.ToLower(sourceProject) + "_", strings.ToLower(targetProject) + "_", "_"} {
		if strings.HasPrefix(t, pfx) {
			t = strings.TrimPrefix(t, pfx)
			break
		}
	}
	switch t {
	case "veg", "vegetation":
		return []string{"Veg"}
	case "humus", "soilhumus", "organic":
		return []string{"Humus"}
	case "mineral", "soilmineral":
		return []string{"Mineral"}
	case "other", "vegother":
		return []string{"Other"}
	case "metadata", "projectmetadata":
		return []string{"Metadata"}
	case "env", "environment", "admin", "su", "hierarchy", "lump", "profile",
		"open", "close", "on", "off", "usysallspecs", "usystableoflists":
		return nil
	default:
		// Unrecognized legacy audit labels (e.g. 'Unknown') with positive IDs reserve across child tables conservatively.
		return []string{"Veg", "Humus", "Mineral", "Other", "Metadata"}
	}
}

func seedImportedChildIdentities(ctx context.Context, tx *sql.Tx, sourceProject, targetProject string) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL,
		"ID" INTEGER NOT NULL,
		PRIMARY KEY ("ChildTable", "ID")
	)`); err != nil {
		return err
	}

	for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other", "Metadata", "Audit"} {
		qChild := vprocore.QuoteHeaderIdentifier(targetProject + "_" + suffix)
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO "__VPRO_ChildIdentity" ("ChildTable", "ID")
			 SELECT ?, "ID" FROM `+qChild+` WHERE typeof("ID") = 'integer' AND "ID" > 0`,
			qChild,
		); err != nil {
			return err
		}
	}

	qAudit := vprocore.QuoteHeaderIdentifier(targetProject + "_Audit")
	rows, err := tx.QueryContext(ctx,
		`SELECT DISTINCT COALESCE(CAST("Table" AS TEXT), ''), "ID"
		 FROM `+qAudit+`
		 WHERE typeof("ID") = 'integer' AND "ID" > 0`)
	if err != nil {
		return err
	}
	type auditIDPair struct {
		tbl string
		id  int64
	}
	var pairs []auditIDPair
	for rows.Next() {
		var tbl string
		var id int64
		if err := rows.Scan(&tbl, &id); err != nil {
			rows.Close()
			return err
		}
		pairs = append(pairs, auditIDPair{tbl: tbl, id: id})
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}

	if len(pairs) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range pairs {
			for _, suffix := range resolveAuditChildSuffixes(p.tbl, sourceProject, targetProject) {
				qChild := vprocore.QuoteHeaderIdentifier(targetProject + "_" + suffix)
				if _, err := stmt.ExecContext(ctx, qChild, p.id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

