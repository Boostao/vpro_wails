package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"
)

type EnvironmentReportField struct {
	Source  string `json:"source"`
	Key     string `json:"key"`
	Label   string `json:"label"`
	Heading bool   `json:"heading"`
}

type EnvironmentReportName struct {
	RowID string              `json:"rowId"`
	Value ProjectMetadataCell `json:"value"`
}

type EnvironmentReportPlot struct {
	PlotNumber string                `json:"plotNumber"`
	Status     string                `json:"status"`
	Values     []ProjectMetadataCell `json:"values"`
}

type EnvironmentReportUnit struct {
	Code           string                  `json:"code"`
	LongName       *string                 `json:"longName"`
	NameStatus     string                  `json:"nameStatus"`
	NameCandidates []EnvironmentReportName `json:"nameCandidates"`
	Plots          []EnvironmentReportPlot `json:"plots"`
}

type EnvironmentReportDiagnostic struct {
	Code       string  `json:"code"`
	Unit       *string `json:"unit"`
	PlotNumber *string `json:"plotNumber"`
	Count      int     `json:"count"`
}

type EnvironmentReport struct {
	Project     string                        `json:"project"`
	SU          string                        `json:"su"`
	Title       string                        `json:"title"`
	Fields      []EnvironmentReportField      `json:"fields"`
	Units       []EnvironmentReportUnit       `json:"units"`
	Diagnostics []EnvironmentReportDiagnostic `json:"diagnostics"`
}

func longEnvironmentFields() []EnvironmentReportField {
	return []EnvironmentReportField{
		{"Env", "PlotNumber", "Plot", false},
		{"Env", "FieldNumber", "Site Number", false},
		{"Env", "FSRegionDistrict", "FSRegionDistrict", false},
		{"Admin", "SitePlotQuality", "Plot Quality", false},
		{"", "", "GENERAL LOCATION", true},
		{"Env", "Zone", "Biogeoclimatic Zone", false},
		{"Env", "SubZone", "SubZone", false},
		{"Env", "SiteSeries", "Site Series", false},
		{"Admin", "UserSiteUnit", "Assigned Site Unit", false},
		{"Env", "Location", "Location", false},
		{"Env", "NtsMapSheet", "NTS Map Sheet", false},
		{"Env", "Longitude", "Longitude", false},
		{"Env", "Latitude", "Latitude", false},
		{"", "", "SITE", true},
		{"Env", "Elevation", "Elevation(m)", false},
		{"Env", "SlopeGradient", "Slope Gradient(%)", false},
		{"Env", "Aspect", "Aspect (degrees)", false},
		{"Env", "MesoSlopePosition", "Meso Slope Position", false},
		{"Env", "SurfaceShape", "Surface Shape", false},
		{"Env", "SurfaceTopographyType", "Surface Topography Type", false},
		{"Env", "MoistureRegime", "Moisture Regime", false},
		{"Env", "NutrientRegime", "Nutrient Regime", false},
		{"Env", "Exposure1", "Exposure1", false},
		{"Env", "Exposure2", "Exposure2", false},
		{"Env", "SiteDisturbance1", "Site Disturbance 1", false},
		{"Env", "SiteDisturbance2", "Site Disturbance 2", false},
		{"Env", "SiteDisturbance3", "Site Disturbance 3", false},
		{"Env", "SubstrateDecWood", "Substrate Decaying Wood(%)", false},
		{"Env", "SubstrateBedRock", "Substrate Bedrock(%)", false},
		{"Env", "SubstrateRocks", "Substrate Rocks(%)", false},
		{"Env", "SubstrateMineralSoil", "Substrate Mineral Soil(%)", false},
		{"Env", "SubstrateOrganicMatter", "Substrate Organic Matter(%)", false},
		{"Env", "SubstrateWater", "Substrate Water(%)", false},
		{"", "", "SOIL", true},
		{"Env", "SoilClassGroup", "Soil Great Group", false},
		{"Env", "SoilClassSubGroup", "Soil Subgroup", false},
		{"Env", "BedrockGeology1", "Bedrock Geology 1", false},
		{"Env", "BedrockGeology2", "Bedrock Geology 2", false},
		{"Env", "BedrockGeology3", "Bedrock Geology3", false},
		{"Env", "CoarseFragLith1", "Coarse Frag Lith 1", false},
		{"Env", "CoarseFragLith2", "Coarse Frag Lith2", false},
		{"Env", "CoarseFragLith3", "Coarse Frag Lith3", false},
		{"Env", "TerrainTextureSurf", "Terrain Texture Surface", false},
		{"Env", "TerrainTextureSubSurf", "Terrain Texture Sub Surface", false},
		{"Env", "SurficialMaterialSurf", "Surficial Material Surface", false},
		{"Env", "SurficialMaterialSubSurf", "Surficial Material Sub Surface", false},
		{"Env", "SurfaceExpSurf", "Surface Expression Surface", false},
		{"Env", "SurfaceExpSubSurf", "Surface Expression Sub Surface", false},
		{"Env", "GeoMorProSurf", "Geomorphological Process Surface", false},
		{"Env", "GeoMorProSubSurf", "Geomorphological Process Sub Surface", false},
		{"Env", "RootZoneParticleSize", "Root Zone Particle Size", false},
		{"Env", "RootingDepth", "Rooting Depth(cm)", false},
		{"Env", "RootRestrictingType", "RootRestrictingType", false},
		{"Env", "RootRestrictingDepth", "Root Restricting Depth(cm)", false},
		{"Env", "SeepageDepth", "Seepage Depth(cm)", false},
		{"Env", "SoilDrainage", "Soil Drainage", false},
		{"Env", "HumusForm", "Humus Form (MOF 81)", false},
		{"Env", "HumusFormPhase", "Humus Form Phase", false},
		{"Admin", "HumusThickness", "Humus Thickness", false},
		{"", "", "VEGETATION", true},
		{"Env", "StandAge", "Stand Age", false},
		{"Env", "SuccessionalStatus", "Successional Status", false},
		{"Env", "StructuralStage", "Structural Stage", false},
		{"Env", "StrataCoverTree", "Strata Cover Tree(%)", false},
		{"Env", "StrataCoverShrub", "Strata Cover Shrub(%)", false},
		{"Env", "StrataCoverHerb", "Strata Cover Herb(%)", false},
		{"Env", "StrataCoverMoss", "Strata Cover Moss(%)", false},
		{"", "", "OTHER", true},
		{"Env", "HydroGeoSystem", "System", false},
		{"Env", "HydroGeoSubSystem", "Subsystem", false},
		{"Env", "WaterSource", "Water Source", false},
		{"Env", "FloodingRegimeFreq", "Flood Frequency", false},
	}
}

func planLongEnvironment(ctx context.Context, project, suName, title string, env, admin, su, master ProjectMetadataTable) (EnvironmentReport, error) {
	fail := func(err error) (EnvironmentReport, error) { return EnvironmentReport{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if project == "" || suName == "" || suName == "None" || !utf8.ValidString(project) || !utf8.ValidString(suName) || !utf8.ValidString(title) {
		return fail(errors.New("Long Environment requires explicit valid project/SU identities and title"))
	}
	report := EnvironmentReport{Project: project, SU: suName, Title: title, Fields: longEnvironmentFields(),
		Units: []EnvironmentReportUnit{}, Diagnostics: []EnvironmentReportDiagnostic{}}
	required := map[string][]string{"Env": {}, "Admin": {"Plot"}}
	for _, field := range report.Fields {
		if !field.Heading {
			required[field.Source] = append(required[field.Source], field.Key)
		}
	}
	envColumns, err := siteUnitTransferColumns(env, required["Env"]...)
	if err != nil {
		return fail(fmt.Errorf("Long Environment Env schema: %w", err))
	}
	adminColumns, err := siteUnitTransferColumns(admin, required["Admin"]...)
	if err != nil {
		return fail(fmt.Errorf("Long Environment Admin schema: %w", err))
	}
	suColumns, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Long Environment selected SU schema: %w", err))
	}
	masterColumns, err := siteUnitTransferColumns(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fail(fmt.Errorf("Long Environment master name schema: %w", err))
	}
	envRows, err := siteUnitTransferIndex(env, envColumns["PlotNumber"])
	if err != nil {
		return fail(err)
	}
	adminRows, err := siteUnitTransferIndex(admin, adminColumns["Plot"])
	if err != nil {
		return fail(err)
	}
	for _, rows := range []map[string][]ProjectMetadataRow{envRows, adminRows} {
		for _, matches := range rows {
			if len(matches) > 1 {
				return fail(errors.New("Long Environment rejects ambiguous physical Env/Admin plot links"))
			}
		}
	}
	masterRows, err := siteUnitTransferIndex(master, masterColumns["SiteSeries"])
	if err != nil {
		return fail(err)
	}
	memberships, classifications := map[string]map[string]int{}, map[string]string{}
	excluded := 0
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := environmentReportIdentity(row.Cells[suColumns["PlotNumber"]])
		if err != nil {
			return fail(err)
		}
		unit, err := environmentReportIdentity(row.Cells[suColumns["SiteUnit"]])
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
	for _, code := range sortedEnvironmentReportKeys(memberships) {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit := EnvironmentReportUnit{Code: code, NameStatus: "missing", NameCandidates: []EnvironmentReportName{}, Plots: []EnvironmentReportPlot{}}
		names, unsupported := map[string]bool{}, false
		for _, row := range masterRows[code] {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			cell := row.Cells[masterColumns["SiteSeriesLongName"]]
			if _, err := metadataCellValue(cell); err != nil {
				return fail(fmt.Errorf("Long Environment master name value: %w", err))
			}
			unit.NameCandidates = append(unit.NameCandidates, EnvironmentReportName{row.RowID, cloneSiteUnitCell(cell)})
			switch cell.Storage {
			case "text":
				names[*cell.Text] = true
			case "null":
			default:
				unsupported = true
			}
		}
		sort.Slice(unit.NameCandidates, func(i, j int) bool { return unit.NameCandidates[i].RowID < unit.NameCandidates[j].RowID })
		switch {
		case unsupported:
			unit.NameStatus = "unsupported_storage"
		case len(names) > 1:
			unit.NameStatus = "conflicting"
		case len(names) == 1:
			unit.NameStatus = "unique"
			name := sortedEnvironmentReportKeys(names)[0]
			unit.LongName = &name
		}
		if unit.NameStatus != "unique" {
			report.Diagnostics = append(report.Diagnostics, EnvironmentReportDiagnostic{Code: "unit_name_" + unit.NameStatus, Unit: &unit.Code, Count: len(unit.NameCandidates)})
		}
		for _, plot := range sortedEnvironmentReportKeys(memberships[code]) {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			entry := EnvironmentReportPlot{PlotNumber: plot, Status: "complete", Values: make([]ProjectMetadataCell, len(report.Fields))}
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
				cell := ProjectMetadataCell{Storage: "null"}
				// The source outer-joins an Env/Admin inner projection, so either missing row blanks all fields.
				if !field.Heading && hasEnv && hasAdmin {
					if field.Source == "Admin" {
						cell = adminRows[plot][0].Cells[adminColumns[field.Key]]
					} else {
						cell = envRows[plot][0].Cells[envColumns[field.Key]]
					}
					if _, err := metadataCellValue(cell); err != nil {
						return fail(fmt.Errorf("Long Environment %s.%s value: %w", field.Source, field.Key, err))
					}
				}
				entry.Values[i] = cloneSiteUnitCell(cell)
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

func environmentReportIdentity(cell ProjectMetadataCell) (*string, error) {
	if _, err := metadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage != "text" && cell.Storage != "null" {
		return nil, errors.New("Long Environment membership identities require original text/NULL without coercion")
	}
	return cell.Text, nil
}

func sortedEnvironmentReportKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
