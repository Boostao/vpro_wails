package reportcore

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"

	"unicode/utf8"
)

type EnvironmentReportPlot struct {
	PlotNumber string                         `json:"plotNumber"`
	Status     string                         `json:"status"`
	Values     []vprocore.ProjectMetadataCell `json:"values"`
}

type EnvironmentReportUnit struct {
	Code           string                           `json:"code"`
	LongName       *string                          `json:"longName"`
	NameStatus     string                           `json:"nameStatus"`
	NameCandidates []vprocore.EnvironmentReportName `json:"nameCandidates"`
	Plots          []EnvironmentReportPlot          `json:"plots"`
}

type EnvironmentReportDiagnostic struct {
	Code       string  `json:"code"`
	Unit       *string `json:"unit"`
	PlotNumber *string `json:"plotNumber"`
	Count      int     `json:"count"`
}

type EnvironmentReport struct {
	Project     string                            `json:"project"`
	SU          string                            `json:"su"`
	Title       string                            `json:"title"`
	Fields      []vprocore.EnvironmentReportField `json:"fields"`
	Units       []EnvironmentReportUnit           `json:"units"`
	Diagnostics []EnvironmentReportDiagnostic     `json:"diagnostics"`
}

func LongEnvironmentFields() []vprocore.EnvironmentReportField {
	return []vprocore.EnvironmentReportField{
		{Source: "Env", Key: "PlotNumber", Label: "Plot", Heading: false},
		{Source: "Env", Key: "FieldNumber", Label: "Site Number", Heading: false},
		{Source: "Env", Key: "FSRegionDistrict", Label: "FSRegionDistrict", Heading: false},
		{Source: "Admin", Key: "SitePlotQuality", Label: "Plot Quality", Heading: false},
		{Source: "", Key: "", Label: "GENERAL LOCATION", Heading: true},
		{Source: "Env", Key: "Zone", Label: "Biogeoclimatic Zone", Heading: false},
		{Source: "Env", Key: "SubZone", Label: "SubZone", Heading: false},
		{Source: "Env", Key: "SiteSeries", Label: "Site Series", Heading: false},
		{Source: "Admin", Key: "UserSiteUnit", Label: "Assigned Site Unit", Heading: false},
		{Source: "Env", Key: "Location", Label: "Location", Heading: false},
		{Source: "Env", Key: "NtsMapSheet", Label: "NTS Map Sheet", Heading: false},
		{Source: "Env", Key: "Longitude", Label: "Longitude", Heading: false},
		{Source: "Env", Key: "Latitude", Label: "Latitude", Heading: false},
		{Source: "", Key: "", Label: "SITE", Heading: true},
		{Source: "Env", Key: "Elevation", Label: "Elevation(m)", Heading: false},
		{Source: "Env", Key: "SlopeGradient", Label: "Slope Gradient(%)", Heading: false},
		{Source: "Env", Key: "Aspect", Label: "Aspect (degrees)", Heading: false},
		{Source: "Env", Key: "MesoSlopePosition", Label: "Meso Slope Position", Heading: false},
		{Source: "Env", Key: "SurfaceShape", Label: "Surface Shape", Heading: false},
		{Source: "Env", Key: "SurfaceTopographyType", Label: "Surface Topography Type", Heading: false},
		{Source: "Env", Key: "MoistureRegime", Label: "Moisture Regime", Heading: false},
		{Source: "Env", Key: "NutrientRegime", Label: "Nutrient Regime", Heading: false},
		{Source: "Env", Key: "Exposure1", Label: "Exposure1", Heading: false},
		{Source: "Env", Key: "Exposure2", Label: "Exposure2", Heading: false},
		{Source: "Env", Key: "SiteDisturbance1", Label: "Site Disturbance 1", Heading: false},
		{Source: "Env", Key: "SiteDisturbance2", Label: "Site Disturbance 2", Heading: false},
		{Source: "Env", Key: "SiteDisturbance3", Label: "Site Disturbance 3", Heading: false},
		{Source: "Env", Key: "SubstrateDecWood", Label: "Substrate Decaying Wood(%)", Heading: false},
		{Source: "Env", Key: "SubstrateBedRock", Label: "Substrate Bedrock(%)", Heading: false},
		{Source: "Env", Key: "SubstrateRocks", Label: "Substrate Rocks(%)", Heading: false},
		{Source: "Env", Key: "SubstrateMineralSoil", Label: "Substrate Mineral Soil(%)", Heading: false},
		{Source: "Env", Key: "SubstrateOrganicMatter", Label: "Substrate Organic Matter(%)", Heading: false},
		{Source: "Env", Key: "SubstrateWater", Label: "Substrate Water(%)", Heading: false},
		{Source: "", Key: "", Label: "SOIL", Heading: true},
		{Source: "Env", Key: "SoilClassGroup", Label: "Soil Great Group", Heading: false},
		{Source: "Env", Key: "SoilClassSubGroup", Label: "Soil Subgroup", Heading: false},
		{Source: "Env", Key: "BedrockGeology1", Label: "Bedrock Geology 1", Heading: false},
		{Source: "Env", Key: "BedrockGeology2", Label: "Bedrock Geology 2", Heading: false},
		{Source: "Env", Key: "BedrockGeology3", Label: "Bedrock Geology3", Heading: false},
		{Source: "Env", Key: "CoarseFragLith1", Label: "Coarse Frag Lith 1", Heading: false},
		{Source: "Env", Key: "CoarseFragLith2", Label: "Coarse Frag Lith2", Heading: false},
		{Source: "Env", Key: "CoarseFragLith3", Label: "Coarse Frag Lith3", Heading: false},
		{Source: "Env", Key: "TerrainTextureSurf", Label: "Terrain Texture Surface", Heading: false},
		{Source: "Env", Key: "TerrainTextureSubSurf", Label: "Terrain Texture Sub Surface", Heading: false},
		{Source: "Env", Key: "SurficialMaterialSurf", Label: "Surficial Material Surface", Heading: false},
		{Source: "Env", Key: "SurficialMaterialSubSurf", Label: "Surficial Material Sub Surface", Heading: false},
		{Source: "Env", Key: "SurfaceExpSurf", Label: "Surface Expression Surface", Heading: false},
		{Source: "Env", Key: "SurfaceExpSubSurf", Label: "Surface Expression Sub Surface", Heading: false},
		{Source: "Env", Key: "GeoMorProSurf", Label: "Geomorphological Process Surface", Heading: false},
		{Source: "Env", Key: "GeoMorProSubSurf", Label: "Geomorphological Process Sub Surface", Heading: false},
		{Source: "Env", Key: "RootZoneParticleSize", Label: "Root Zone Particle Size", Heading: false},
		{Source: "Env", Key: "RootingDepth", Label: "Rooting Depth(cm)", Heading: false},
		{Source: "Env", Key: "RootRestrictingType", Label: "RootRestrictingType", Heading: false},
		{Source: "Env", Key: "RootRestrictingDepth", Label: "Root Restricting Depth(cm)", Heading: false},
		{Source: "Env", Key: "SeepageDepth", Label: "Seepage Depth(cm)", Heading: false},
		{Source: "Env", Key: "SoilDrainage", Label: "Soil Drainage", Heading: false},
		{Source: "Env", Key: "HumusForm", Label: "Humus Form (MOF 81)", Heading: false},
		{Source: "Env", Key: "HumusFormPhase", Label: "Humus Form Phase", Heading: false},
		{Source: "Admin", Key: "HumusThickness", Label: "Humus Thickness", Heading: false},
		{Source: "", Key: "", Label: "VEGETATION", Heading: true},
		{Source: "Env", Key: "StandAge", Label: "Stand Age", Heading: false},
		{Source: "Env", Key: "SuccessionalStatus", Label: "Successional Status", Heading: false},
		{Source: "Env", Key: "StructuralStage", Label: "Structural Stage", Heading: false},
		{Source: "Env", Key: "StrataCoverTree", Label: "Strata Cover Tree(%)", Heading: false},
		{Source: "Env", Key: "StrataCoverShrub", Label: "Strata Cover Shrub(%)", Heading: false},
		{Source: "Env", Key: "StrataCoverHerb", Label: "Strata Cover Herb(%)", Heading: false},
		{Source: "Env", Key: "StrataCoverMoss", Label: "Strata Cover Moss(%)", Heading: false},
		{Source: "", Key: "", Label: "OTHER", Heading: true},
		{Source: "Env", Key: "HydroGeoSystem", Label: "System", Heading: false},
		{Source: "Env", Key: "HydroGeoSubSystem", Label: "Subsystem", Heading: false},
		{Source: "Env", Key: "WaterSource", Label: "Water Source", Heading: false},
		{Source: "Env", Key: "FloodingRegimeFreq", Label: "Flood Frequency", Heading: false},
	}
}

func PlanLongEnvironment(ctx context.Context, project, suName, title string, env, admin, su, master vprocore.ProjectMetadataTable) (EnvironmentReport, error) {
	fail := func(err error) (EnvironmentReport, error) { return EnvironmentReport{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if project == "" || suName == "" || suName == "None" || !utf8.ValidString(project) || !utf8.ValidString(suName) || !utf8.ValidString(title) {
		return fail(errors.New("Long Environment requires explicit valid project/SU identities and title"))
	}
	report := EnvironmentReport{Project: project, SU: suName, Title: title, Fields: LongEnvironmentFields(),
		Units: []EnvironmentReportUnit{}, Diagnostics: []EnvironmentReportDiagnostic{}}
	required := map[string][]string{"Env": {}, "Admin": {"Plot"}}
	for _, field := range report.Fields {
		if !field.Heading {
			required[field.Source] = append(required[field.Source], field.Key)
		}
	}
	envColumns, err := vprocore.SiteUnitTransferColumns(env, required["Env"]...)
	if err != nil {
		return fail(fmt.Errorf("Long Environment Env schema: %w", err))
	}
	adminColumns, err := vprocore.SiteUnitTransferColumns(admin, required["Admin"]...)
	if err != nil {
		return fail(fmt.Errorf("Long Environment Admin schema: %w", err))
	}
	suColumns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Long Environment selected SU schema: %w", err))
	}
	masterColumns, err := vprocore.SiteUnitTransferColumns(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fail(fmt.Errorf("Long Environment master name schema: %w", err))
	}
	envRows, err := vprocore.SiteUnitTransferIndex(env, envColumns["PlotNumber"])
	if err != nil {
		return fail(err)
	}
	adminRows, err := vprocore.SiteUnitTransferIndex(admin, adminColumns["Plot"])
	if err != nil {
		return fail(err)
	}
	for _, rows := range []map[string][]vprocore.ProjectMetadataRow{envRows, adminRows} {
		for _, matches := range rows {
			if len(matches) > 1 {
				return fail(errors.New("Long Environment rejects ambiguous physical Env/Admin plot links"))
			}
		}
	}
	masterRows, err := vprocore.SiteUnitTransferIndex(master, masterColumns["SiteSeries"])
	if err != nil {
		return fail(err)
	}
	memberships, classifications := map[string]map[string]int{}, map[string]string{}
	excluded := 0
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := EnvironmentReportIdentity(row.Cells[suColumns["PlotNumber"]])
		if err != nil {
			return fail(err)
		}
		unit, err := EnvironmentReportIdentity(row.Cells[suColumns["SiteUnit"]])
		if err != nil {
			return fail(err)
		}
		if plot == nil || unit == nil {
			excluded++
			continue
		}
		if previous, exists := classifications[*plot]; exists && previous != *unit {
			return fail(errors.New("Long Environment rejects conflicting selected-SU classifications for one plot"))
		}
		classifications[*plot] = *unit
		if memberships[*unit] == nil {
			memberships[*unit] = map[string]int{}
		}
		memberships[*unit][*plot]++
	}
	if excluded > 0 {
		report.Diagnostics = append(report.Diagnostics, EnvironmentReportDiagnostic{Code: "null_membership", Count: excluded})
	}
	for _, code := range vprocore.SortedEnvironmentReportKeys(memberships) {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		names, err := ResolveReportUnitNames(ctx, masterRows[code], masterColumns["SiteSeriesLongName"], "Long Environment")
		if err != nil {
			return fail(err)
		}
		unit := EnvironmentReportUnit{Code: code, LongName: names.LongName, NameStatus: names.Status,
			NameCandidates: names.Candidates, Plots: []EnvironmentReportPlot{}}
		if unit.NameStatus != "unique" {
			report.Diagnostics = append(report.Diagnostics, EnvironmentReportDiagnostic{Code: "unit_name_" + unit.NameStatus, Unit: &unit.Code, Count: len(unit.NameCandidates)})
		}
		for _, plot := range vprocore.SortedEnvironmentReportKeys(memberships[code]) {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			entry := EnvironmentReportPlot{PlotNumber: plot, Status: "complete", Values: make([]vprocore.ProjectMetadataCell, len(report.Fields))}
			hasEnv, hasAdmin := len(envRows[plot]) == 1, len(adminRows[plot]) == 1
			if !hasEnv {
				entry.Status = "missing_env"
			}
			if !hasAdmin {
				entry.Status = "missing_admin"
				if !hasEnv {
					entry.Status = "missing_env_and_admin"
				}
			}
			for _, issue := range []struct {
				missing bool
				code    string
			}{{!hasEnv, "missing_env"}, {!hasAdmin, "missing_admin"}, {memberships[code][plot] > 1, "duplicate_membership"}} {
				if issue.missing {
					count := 1
					if issue.code == "duplicate_membership" {
						count = memberships[code][plot] - 1
					}
					report.Diagnostics = append(report.Diagnostics, EnvironmentReportDiagnostic{issue.code, &unit.Code, &entry.PlotNumber, count})
				}
			}
			for i, field := range report.Fields {
				cell := vprocore.ProjectMetadataCell{Storage: "null"}
				// The source outer-joins an Env/Admin inner projection, so either missing row blanks all fields.
				if !field.Heading && hasEnv && hasAdmin {
					if field.Source == "Admin" {
						cell = adminRows[plot][0].Cells[adminColumns[field.Key]]
					} else {
						cell = envRows[plot][0].Cells[envColumns[field.Key]]
					}
					if _, err := vprocore.MetadataCellValue(cell); err != nil {
						return fail(fmt.Errorf("Long Environment %s.%s value: %w", field.Source, field.Key, err))
					}
				}
				entry.Values[i] = vprocore.CloneSiteUnitCell(cell)
			}
			unit.Plots = append(unit.Plots, entry)
		}
		report.Units = append(report.Units, unit)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return report, nil
}

func EnvironmentReportIdentity(cell vprocore.ProjectMetadataCell) (*string, error) {
	if _, err := vprocore.MetadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage != "text" && cell.Storage != "null" {
		return nil, errors.New("Long Environment membership identities require original text/NULL without coercion")
	}
	return cell.Text, nil
}
