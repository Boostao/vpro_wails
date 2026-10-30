package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/accessimport"
	"github.com/boostao/vpro-wails/internal/accessreader"
)

type AccessImportTableSummary struct {
	Suffix        string `json:"suffix"`
	SourceTable   string `json:"sourceTable"`
	Description   string `json:"description"`
	RowCount      int    `json:"rowCount"`
	ColumnCount   int    `json:"columnCount"`
	Action        string `json:"action"`
	Present       bool   `json:"present"`
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
	SourcePath       string                     `json:"sourcePath"`
	SourceProject    string                     `json:"sourceProject"`
	SourceVersion    string                     `json:"sourceVersion"`
	ConversionMode   string                     `json:"conversionMode"`
	TargetProject    string                     `json:"targetProject"`
	DestinationPath  string                     `json:"destinationPath"`
	DestinationSHA   string                     `json:"destinationSha256"`
	PlotCount        int                        `json:"plotCount"`
	Tables           []AccessImportTableSummary `json:"tables"`
	ImportedSUs      []string                   `json:"importedSUs"`
	Activated        bool                       `json:"activated"`
	State            *ProjectState              `json:"state,omitempty"`
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
	if strings.EqualFold(candidate, "Sample") || strings.EqualFold(candidate, "None") {
		candidate += "_Imp"
		if len(candidate) > 31 {
			candidate = candidate[:31]
		}
	}
	return candidate
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
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", "", fmt.Errorf("read Access database %q: %w", resolved, err)
	}
	digest := sha256.Sum256(data)
	return resolved, hex.EncodeToString(digest[:]), nil
}

func (s *ContextService) InspectAccessDatabase(ctx context.Context, contextID, sourcePath string) (AccessDatabaseInspection, error) {
	var projectsDir string
	if _, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		projectsDir = filepath.Join(plots.projects.root, "projects")
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
			if projectNamePattern.MatchString(prefix) {
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
				cols, rows, readErr := db.ReadAll(srcTbl.Name)
				if readErr != nil {
					allPresent = false
					candidate.StatusMessage = fmt.Sprintf("Failed reading %s: %v", srcTbl.Name, readErr)
				} else {
					summary.ColumnCount = len(cols)
					summary.RowCount = len(rows)
					if suffix == "Env" {
						candidate.PlotCount = len(rows)
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

func insertMappedRows(ctx context.Context, tx *sql.Tx, targetTable string, targetToSource [][2]string, colIndex map[string]int, rows [][]any, computed map[string]func([]any) any) error {
	targetCols := make([]string, 0, len(targetToSource)+len(computed))
	sourceIndices := make([]int, len(targetToSource))
	for i, pair := range targetToSource {
		idx, ok := colIndex[strings.ToLower(pair[1])]
		if !ok {
			if strings.EqualFold(pair[1], "Collected") {
				sourceIndices[i] = -1
			} else {
				return fmt.Errorf("source table for %s is missing column %q", targetTable, pair[1])
			}
		} else {
			sourceIndices[i] = idx
		}
		targetCols = append(targetCols, quoteHeaderIdentifier(pair[0]))
	}
	computedKeys := make([]string, 0, len(computed))
	for k := range computed {
		computedKeys = append(computedKeys, k)
	}
	slices.Sort(computedKeys)
	for _, k := range computedKeys {
		targetCols = append(targetCols, quoteHeaderIdentifier(k))
	}
	marks := make([]string, len(targetCols))
	for i := range marks {
		marks[i] = "?"
	}
	stmtSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteHeaderIdentifier(targetTable),
		strings.Join(targetCols, ", "),
		strings.Join(marks, ", "))
	stmt, err := tx.PrepareContext(ctx, stmtSQL)
	if err != nil {
		return fmt.Errorf("prepare insert into %s: %w", targetTable, err)
	}
	defer stmt.Close()
	for rIdx, row := range rows {
		args := make([]any, 0, len(targetCols))
		for _, idx := range sourceIndices {
			if idx < 0 {
				args = append(args, nil)
			} else {
				args = append(args, row[idx])
			}
		}
		for _, k := range computedKeys {
			args = append(args, computed[k](row))
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("insert row %d into %s: %w", rIdx+1, targetTable, err)
		}
	}
	return nil
}

func identityColumnPairs(columns []string) [][2]string {
	pairs := make([][2]string, len(columns))
	for i, c := range columns {
		pairs[i] = [2]string{c, c}
	}
	return pairs
}

func (s *ContextService) ImportAccessProject(ctx context.Context, contextID string, request AccessProjectImportRequest) (_ AccessProjectImportResult, resultErr error) {
	targetProject := strings.TrimSpace(request.TargetProject)
	if !projectNamePattern.MatchString(targetProject) || strings.EqualFold(targetProject, "None") ||
		strings.EqualFold(targetProject, "Sample") || strings.Contains(strings.ToLower(targetProject), "master") {
		return AccessProjectImportResult{}, errors.New("target project name must start with a letter, use only letters/digits/underscore (max 31 chars), and not be None, Sample, or master")
	}
	destPath := strings.TrimSpace(request.DestinationPath)
	if destPath != "" && !strings.EqualFold(filepath.Ext(destPath), ".db") {
		destPath += ".db"
	}
	resolvedDest, parentInfo, err := freshSQLiteDestination(destPath, "Project")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	resolvedSource, sourceDigestBefore, err := resolveAccessSourcePath(request.SourcePath)
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	review, err := s.readNewProjectTemplates(ctx, contextID)
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	ddlPlan, err := planNewProjectDDL(ctx, targetProject, review)
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

	readAdapted := func(suffix string) ([]accessreader.Column, map[string]int, [][]any, string, error) {
		tbl, exists := byLower[strings.ToLower(sourceProject+"_"+suffix)]
		if !exists {
			return nil, nil, nil, "", fmt.Errorf("missing required table %s_%s", sourceProject, suffix)
		}
		cols, rows, err := db.ReadAll(tbl.Name)
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("read %s: %w", tbl.Name, err)
		}
		idx, adapted, err := adaptAccessRows(ctx, cols, rows)
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("adapt %s: %w", tbl.Name, err)
		}
		return cols, idx, adapted, tbl.Name, nil
	}

	envCols, envIdx, envRows, envActualName, err := readAdapted("Env")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	vegCols, vegIdx, vegRows, vegActualName, err := readAdapted("Veg")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	otherCols, otherIdx, otherRows, otherActualName, err := readAdapted("Other")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	humusCols, humusIdx, humusRows, humusActualName, err := readAdapted("Humus")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	mineralCols, mineralIdx, mineralRows, mineralActualName, err := readAdapted("Mineral")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	auditCols, auditIdx, auditRows, auditActualName, err := readAdapted("Audit")
	if err != nil {
		return AccessProjectImportResult{}, err
	}

	var metaCols []accessreader.Column
	var metaIdx map[string]int
	var metaRows [][]any
	var metaActualName string
	var adminCols []accessreader.Column
	var adminIdx map[string]int
	var adminRows [][]any
	var adminActualName string

	if version == "VP08" {
		metaCols, metaIdx, metaRows, metaActualName, err = readAdapted("Metadata")
		if err != nil {
			return AccessProjectImportResult{}, err
		}
		adminCols, adminIdx, adminRows, adminActualName, err = readAdapted("Admin")
		if err != nil {
			return AccessProjectImportResult{}, err
		}
	} else if version == "VP07" {
		if _, exists := byLower[strings.ToLower(sourceProject+"_Metadata")]; !exists {
			return AccessProjectImportResult{}, fmt.Errorf("VP07 import requires table %s_Metadata", sourceProject)
		}
	}

	type suImportPayload struct {
		name string
		idx  map[string]int
		rows [][]any
	}
	var suPayloads []suImportPayload
	for _, tbl := range tables {
		if tbl.Linked || len(tbl.Name) <= 3 || !strings.EqualFold(tbl.Name[len(tbl.Name)-3:], "_SU") ||
			strings.HasPrefix(strings.ToUpper(tbl.Name), "USYS") {
			continue
		}
		suPrefix := tbl.Name[:len(tbl.Name)-3]
		if !projectNamePattern.MatchString(suPrefix) {
			continue
		}
		cols, rows, readErr := db.ReadAll(tbl.Name)
		if readErr != nil {
			continue
		}
		idx, adapted, adaptErr := adaptAccessRows(ctx, cols, rows)
		if adaptErr != nil {
			continue
		}
		if _, hasPlot := idx["plotnumber"]; hasPlot {
			if _, hasSU := idx["siteunit"]; hasSU {
				suPayloads = append(suPayloads, suImportPayload{name: suPrefix, idx: idx, rows: adapted})
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
		}
	}()
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		return AccessProjectImportResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		return AccessProjectImportResult{}, err
	}

	sqlDB, err := sql.Open("sqlite3", sqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	sqlDB.SetMaxOpenConns(1)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		sqlDB.Close()
		return AccessProjectImportResult{}, err
	}

	writeTables := func() error {
		for _, tblPlan := range ddlPlan.Tables {
			for _, mapping := range tblPlan.Schema {
				if mapping.SQL != nil {
					if _, err := tx.ExecContext(ctx, *mapping.SQL); err != nil {
						return fmt.Errorf("create schema for %s: %w", tblPlan.TargetTable, err)
					}
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, suffix := range coreTables {
			if _, err := tx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP08')`, targetProject+"_"+suffix); err != nil {
				return err
			}
		}

		switch version {
		case "VP08":
			for _, item := range []struct {
				suffix string
				cols   []accessreader.Column
				idx    map[string]int
				rows   [][]any
			}{
				{"Env", envCols, envIdx, envRows},
				{"Veg", vegCols, vegIdx, vegRows},
				{"Other", otherCols, otherIdx, otherRows},
				{"Humus", humusCols, humusIdx, humusRows},
				{"Mineral", mineralCols, mineralIdx, mineralRows},
				{"Audit", auditCols, auditIdx, auditRows},
				{"Metadata", metaCols, metaIdx, metaRows},
				{"Admin", adminCols, adminIdx, adminRows},
			} {
				names := make([]string, len(item.cols))
				for i, c := range item.cols {
					names[i] = c.Name
				}
				if err := insertMappedRows(ctx, tx, targetProject+"_"+item.suffix, identityColumnPairs(names), item.idx, item.rows, nil); err != nil {
					return err
				}
			}
		case "VP07":
			if err := insertMappedRows(ctx, tx, targetProject+"_Env", vp07EnvMapping, envIdx, envRows, nil); err != nil {
				return err
			}
			if err := insertMappedRows(ctx, tx, targetProject+"_Admin", vp07AdminFromEnvMapping, envIdx, envRows, nil); err != nil {
				return err
			}
		case "VP06":
			if err := insertMappedRows(ctx, tx, targetProject+"_Env", vp06EnvMapping, envIdx, envRows, nil); err != nil {
				return err
			}
			if err := insertMappedRows(ctx, tx, targetProject+"_Admin", vp06AdminFromEnvMapping, envIdx, envRows, nil); err != nil {
				return err
			}
		case "VP05":
			lonDegIdx, hasLonDeg := envIdx["longitudedegrees"]
			lonMinIdx := envIdx["longitudeminutes"]
			lonSecIdx := envIdx["longitudeseconds"]
			latDegIdx, hasLatDeg := envIdx["latitudedegrees"]
			latMinIdx := envIdx["latitudeminutes"]
			latSecIdx := envIdx["latitudeseconds"]
			if !hasLonDeg || !hasLatDeg {
				return errors.New("VP05 Env table is missing LongitudeDegrees or LatitudeDegrees")
			}
			computed := map[string]func([]any) any{
				"Longitude": func(r []any) any {
					return convertLongLatToDeg(r[lonDegIdx], r[lonMinIdx], r[lonSecIdx])
				},
				"Latitude": func(r []any) any {
					return convertLongLatToDeg(r[latDegIdx], r[latMinIdx], r[latSecIdx])
				},
			}
			if err := insertMappedRows(ctx, tx, targetProject+"_Env", vp05EnvMapping, envIdx, envRows, computed); err != nil {
				return err
			}
			if err := insertMappedRows(ctx, tx, targetProject+"_Admin", vp05AdminFromEnvMapping, envIdx, envRows, nil); err != nil {
				return err
			}
		}

		if version != "VP08" {
			for _, item := range []struct {
				suffix string
				cols   []string
				idx    map[string]int
				rows   [][]any
			}{
				{"Veg", legacyVegColumns, vegIdx, vegRows},
				{"Other", legacyOtherColumns, otherIdx, otherRows},
				{"Humus", legacyHumusColumns, humusIdx, humusRows},
				{"Mineral", legacyMineralColumns, mineralIdx, mineralRows},
				{"Audit", legacyAuditColumns, auditIdx, auditRows},
			} {
				if err := insertMappedRows(ctx, tx, targetProject+"_"+item.suffix, identityColumnPairs(item.cols), item.idx, item.rows, nil); err != nil {
					return err
				}
			}
		}

		for _, su := range suPayloads {
			suTable := quoteHeaderIdentifier(su.name + "_SU")
			if _, err := tx.ExecContext(ctx, `CREATE TABLE `+suTable+`("PlotNumber" VARCHAR,"SiteUnit" VARCHAR);
				CREATE UNIQUE INDEX `+quoteHeaderIdentifier("uidx_"+su.name+"_SU_PlotNumber")+` ON `+suTable+`("PlotNumber");
				CREATE INDEX `+quoteHeaderIdentifier("idx_"+su.name+"_SU_SiteUnit")+` ON `+suTable+`("SiteUnit")`); err != nil {
				return err
			}
			if err := insertMappedRows(ctx, tx, su.name+"_SU", [][2]string{{"PlotNumber", "PlotNumber"}, {"SiteUnit", "SiteUnit"}}, su.idx, su.rows, nil); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP04')`, su.name+"_SU"); err != nil {
				return err
			}
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
	if err := sqlDB.Close(); err != nil {
		return AccessProjectImportResult{}, err
	}

	// Verify source Access file was not mutated during import
	_, sourceDigestAfter, err := resolveAccessSourcePath(resolvedSource)
	if err != nil || sourceDigestAfter != sourceDigestBefore {
		return AccessProjectImportResult{}, errors.New("source Access database changed during import; publication aborted")
	}

	// Verify staged SQLite file passes VP08 inspection before linking
	inspected, err := inspectFileContext(ctx, tmpPath)
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
	if err := os.Link(tmpPath, resolvedDest); err != nil {
		return AccessProjectImportResult{}, fmt.Errorf("publish SQLite database %s: %w", resolvedDest, err)
	}
	published = true
	_ = os.Remove(tmpPath)

	destBytes, err := os.ReadFile(resolvedDest)
	if err != nil {
		return AccessProjectImportResult{}, err
	}
	destDigest := sha256.Sum256(destBytes)

	importedSUs := make([]string, len(suPayloads))
	for i, su := range suPayloads {
		importedSUs[i] = su.name
	}
	summaries := []AccessImportTableSummary{
		{Suffix: "Env", SourceTable: envActualName, Description: "VP08", RowCount: len(envRows), ColumnCount: 112, Present: true, Action: "Imported"},
		{Suffix: "Veg", SourceTable: vegActualName, Description: "VP08", RowCount: len(vegRows), ColumnCount: 44, Present: true, Action: "Imported"},
		{Suffix: "Other", SourceTable: otherActualName, Description: "VP08", RowCount: len(otherRows), ColumnCount: 11, Present: true, Action: "Imported"},
		{Suffix: "Humus", SourceTable: humusActualName, Description: "VP08", RowCount: len(humusRows), ColumnCount: 18, Present: true, Action: "Imported"},
		{Suffix: "Mineral", SourceTable: mineralActualName, Description: "VP08", RowCount: len(mineralRows), ColumnCount: 28, Present: true, Action: "Imported"},
		{Suffix: "Audit", SourceTable: auditActualName, Description: "VP08", RowCount: len(auditRows), ColumnCount: 11, Present: true, Action: "Imported"},
		{Suffix: "Metadata", SourceTable: metaActualName, Description: "VP08", RowCount: len(metaRows), ColumnCount: 75, Present: version == "VP08", Action: "Imported"},
		{Suffix: "Admin", SourceTable: adminActualName, Description: "VP08", RowCount: len(envRows), ColumnCount: 20, Present: version == "VP08", Action: "Imported"},
	}

	result := AccessProjectImportResult{
		SourcePath:      resolvedSource,
		SourceProject:   sourceProject,
		SourceVersion:   version,
		ConversionMode:  fmt.Sprintf("%s -> VP08", version),
		TargetProject:   targetProject,
		DestinationPath: resolvedDest,
		DestinationSHA:  hex.EncodeToString(destDigest[:]),
		PlotCount:       len(envRows),
		Tables:          summaries,
		ImportedSUs:     importedSUs,
	}

	if request.ActivateOnSuccess {
		state, switchErr := s.SwitchContext(contextID, ContextSelection{
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
		result.Activated = true
		result.State = &state
	} else {
		state, stateErr := s.projects.GetState(ctx)
		if stateErr == nil {
			result.State = &state
		}
	}
	return result, nil
}
