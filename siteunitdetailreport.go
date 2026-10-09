package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type SiteUnitSummaryField struct {
	Source  string `json:"source"`
	Key     string `json:"key"`
	Label   string `json:"label"`
	Section string `json:"section"`
	Kind    string `json:"kind"`
}

type SiteUnitSummaryPlot struct {
	PlotNumber string `json:"plotNumber"`
	SURowID    string `json:"suRowId"`
	EnvRowID   string `json:"envRowId"`
	AdminRowID string `json:"adminRowId"`
}

type SiteUnitSummaryMembership struct {
	RowID      string              `json:"rowId"`
	PlotNumber ProjectMetadataCell `json:"plotNumber"`
	SiteUnit   ProjectMetadataCell `json:"siteUnit"`
	JoinedRows int                 `json:"joinedRows"`
	Status     string              `json:"status"`
}

type SiteUnitSummaryUnit struct {
	Code           string                  `json:"code"`
	LongName       *string                 `json:"longName"`
	NameStatus     string                  `json:"nameStatus"`
	NameCandidates []EnvironmentReportName `json:"nameCandidates"`
	Plots          []SiteUnitSummaryPlot   `json:"plots"`
	Values         []string                `json:"values"`
}

type SiteUnitSummaryReport struct {
	Project     string                      `json:"project"`
	SU          string                      `json:"su"`
	Method      int                         `json:"method"`
	QuerySource string                      `json:"querySource"`
	Fields      []SiteUnitSummaryField      `json:"fields"`
	Units       []SiteUnitSummaryUnit       `json:"units"`
	Memberships []SiteUnitSummaryMembership `json:"memberships"`
}

func siteUnitSummaryFields() []SiteUnitSummaryField {
	return []SiteUnitSummaryField{
		{"Env", "Zone", "Biogeoclimatic zone", "SITE", "category"},
		{"Env", "SubZone", "Biogeoclimatic unit", "SITE", "bgc-unit"},
		{"Env", "Elevation", "Elevation", "SITE", "numeric"},
		{"Env", "Aspect", "Aspect", "SITE", "aspect"},
		{"Env", "SlopeGradient", "Slope Gradient(%)", "SITE", "numeric"},
		{"Env", "MesoSlopePosition", "Meso Slope Position", "SITE", "category"},
		{"Env", "MoistureRegime", "Moisture Regime", "SITE", "moisture"},
		{"Env", "NutrientRegime", "Nutrient Regime", "SITE", "category"},
		{"Env", "SiteDisturbance2", "Site Disturbance 1", "SITE", "category"},
		{"Env", "SiteDisturbance2", "Site Disturbance 2", "SITE", "category"},
		{"Env", "Exposure1", "Exposure", "SITE", "category"},
		{"Env", "SurfaceTopographyType", "Micro Topography Type", "SITE", "category"},
		{"Env", "SubstrateOrganicMatter", "%Substrate Org. Matter", "SITE", "numeric"},
		{"Env", "SubstrateRocks", "%Substrate Rocks", "SITE", "numeric"},
		{"Env", "SubstrateDecWood", "%Substrate Dec. Wood", "SITE", "numeric"},
		{"Env", "SubstrateMineralSoil", "%Substrate Mineral Soil", "SITE", "numeric"},
		{"Env", "SubstrateBedRock", "%Substrate Bedrock", "SITE", "numeric"},
		{"Env", "SubstrateWater", "%Substrate Water", "SITE", "numeric"},
		{"Env", "HydroGeoSystem", "Hydr Geo System", "SITE", "category"},
		{"Env", "HydroGeoSubSystem", "Hydro Geo Sub System", "SITE", "category"},
		{"Env", "StandAge", "Stand Age", "SITE", "numeric"},
		{"Env", "SuccessionalStatus", "Successional Status", "VEGETATION", "category"},
		{"Env", "StructuralStage", "Structure Stage", "VEGETATION", "category"},
		{"Env", "StrataCoverTree", "Tree Layer Cover", "VEGETATION", "numeric"},
		{"Env", "StrataCoverShrub", "Shrub Layer Cover", "VEGETATION", "numeric"},
		{"Env", "StrataCoverHerb", "Herb Layer Cover", "VEGETATION", "numeric"},
		{"Env", "StrataCoverMoss", "Moss LayerCover", "VEGETATION", "numeric"},
		{"Env", "SoilClassGroup", "Soil Great Group", "SOILS", "category"},
		{"Env", "HumusForm", "Humus Form", "SOILS", "category"},
		{"Admin", "HumusThickness", "Humus Depth", "SOILS", "numeric"},
		{"Env", "SoilDrainage", "Soil Drainage", "SOILS", "category"},
		{"Env", "SeepageDepth", "Seepage(cm)", "SOILS", "numeric"},
		{"Env", "SurficialMaterialSurf", "Surficial Material", "SOILS", "category"},
		{"Env", "RootZoneParticleSize", "Root Zone Particle Size", "SOILS", "category"},
		{"Env", "RootingDepth", "Rooting Depth", "SOILS", "numeric"},
		{"Env", "RootRestrictingType", "Rooting Restricting Type", "SOILS", "category"},
		{"Env", "BedrockGeology1", "Bedrock Type 1", "SOILS", "category"},
		{"Env", "BedrockGeology2", "Bedrock Type 2", "SOILS", "category"},
		{"Env", "BedrockGeology3", "Bedrock Type 3", "SOILS", "category"},
	}
}

func planSiteUnitSummary(ctx context.Context, project, suName string, method, maxJoinedRows int,
	env, admin, su, master ProjectMetadataTable) (SiteUnitSummaryReport, error) {
	fail := func(err error) (SiteUnitSummaryReport, error) { return SiteUnitSummaryReport{}, err }
	if method != 1 && method != 2 {
		return fail(errors.New("Summary Environment requires Mean (1) or Interquartile (2)"))
	}
	scope, err := prepareSiteUnitDetailScope(ctx, project, suName, siteUnitDetailSelectedSU, maxJoinedRows, env, admin, su)
	if err != nil {
		return fail(err)
	}
	report := SiteUnitSummaryReport{Project: project, SU: suName, Method: method, QuerySource: string(scope.QuerySource),
		Fields: siteUnitSummaryFields(), Units: []SiteUnitSummaryUnit{}, Memberships: []SiteUnitSummaryMembership{}}
	required := map[string][]string{"Env": {"PlotNumber", "Zone"}, "Admin": {"Plot"}}
	for _, field := range report.Fields {
		required[field.Source] = append(required[field.Source], field.Key)
	}
	columns := map[string]map[string]int{}
	for name, table := range map[string]ProjectMetadataTable{"Env": env, "Admin": admin} {
		columns[name], err = siteUnitTransferColumns(table, required[name]...)
		if err != nil {
			return fail(fmt.Errorf("Summary Environment %s schema: %w", name, err))
		}
	}
	masterColumns, err := siteUnitTransferColumns(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fail(err)
	}
	masterRows, err := siteUnitTransferIndex(master, masterColumns["SiteSeries"])
	if err != nil {
		return fail(err)
	}
	rows := map[string]map[string]ProjectMetadataRow{"Env": {}, "Admin": {}}
	for name, table := range map[string]ProjectMetadataTable{"Env": env, "Admin": admin} {
		for _, row := range table.Rows {
			rows[name][row.RowID] = row
		}
	}
	for _, membership := range scope.Memberships {
		report.Memberships = append(report.Memberships, SiteUnitSummaryMembership{membership.RowID,
			cloneSiteUnitCell(membership.PlotNumber), cloneSiteUnitCell(membership.SiteUnit), membership.JoinedRows, membership.Status})
	}
	for _, scopeUnit := range scope.Units {
		names, err := resolveReportUnitNames(ctx, masterRows[scopeUnit.Code], masterColumns["SiteSeriesLongName"], "Summary Environment")
		if err != nil {
			return fail(err)
		}
		unit := SiteUnitSummaryUnit{Code: scopeUnit.Code, LongName: names.LongName, NameStatus: names.Status,
			NameCandidates: names.Candidates, Plots: []SiteUnitSummaryPlot{}, Values: []string{}}
		for _, plot := range scopeUnit.Plots {
			unit.Plots = append(unit.Plots, SiteUnitSummaryPlot{plot.PlotNumber, plot.SURowID, plot.EnvRowID, plot.AdminRowID})
		}
		for _, field := range report.Fields {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			cells := make([]ProjectMetadataCell, 0, len(scopeUnit.Plots))
			zones := make([]ProjectMetadataCell, 0, len(scopeUnit.Plots))
			for _, plot := range scopeUnit.Plots {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				id := plot.EnvRowID
				if field.Source == "Admin" {
					id = plot.AdminRowID
				}
				cells = append(cells, rows[field.Source][id].Cells[columns[field.Source][field.Key]])
				if field.Kind == "bgc-unit" {
					zones = append(zones, rows["Env"][plot.EnvRowID].Cells[columns["Env"]["Zone"]])
				}
			}
			var value string
			if field.Kind == "numeric" {
				value, err = summarizeSiteUnitNumeric(cells, field.Key, method)
			} else {
				value, err = summarizeSiteUnitCategories(cells, zones, field.Kind)
			}
			if err != nil {
				return fail(fmt.Errorf("Summary Environment unit %q %s.%s: %w", unit.Code, field.Source, field.Key, err))
			}
			unit.Values = append(unit.Values, value)
		}
		report.Units = append(report.Units, unit)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return report, nil
}

func summarizeSiteUnitCategories(cells, zones []ProjectMetadataCell, kind string) (string, error) {
	counts, nulls := map[string]int{}, 0
	eligible := map[string]bool{}
	for i, cell := range cells {
		if _, err := metadataCellValue(cell); err != nil {
			return "", err
		}
		if kind == "bgc-unit" {
			if len(zones) != len(cells) {
				return "", errors.New("BGC unit requires matched zone observations")
			}
			zone, err := environmentReportIdentity(zones[i])
			if err != nil {
				return "", err
			}
			subzone, err := environmentReportIdentity(cell)
			if err != nil {
				return "", err
			}
			// Access & treats one NULL operand as empty, but both NULL as NULL.
			if zone == nil {
				if subzone == nil {
					nulls++
				} else {
					counts[*subzone]++
				}
				continue
			}
			value := *zone
			if subzone != nil {
				value += *subzone
			}
			eligible[value] = true
			counts[value]++
			continue
		}
		if cell.Storage == "null" {
			nulls++
			continue
		}
		var value string
		if kind == "aspect" {
			raw, err := metadataCellValue(cell)
			if err != nil {
				return "", err
			}
			var degrees float64
			switch n := raw.(type) {
			case int64:
				degrees = float64(n)
			case float64:
				degrees = n
			default:
				return "", errors.New("aspect requires original numeric storage; historical text is not coerced")
			}
			if math.IsInf(float64(float32(degrees)), 0) {
				return "", errors.New("aspect exceeds source Single range")
			}
			value = siteUnitSummaryAspect(float32(degrees))
		} else {
			text, err := environmentReportIdentity(cell)
			if err != nil {
				return "", err
			}
			value = *text
		}
		counts[value]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		if kind != "bgc-unit" || eligible[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var result strings.Builder
	for _, key := range keys {
		label := key
		if kind == "moisture" {
			label = map[string]string{"0": "VX", "1": "X", "2": "SX", "3": "SM", "4": "M", "5": "SHG", "6": "HG", "7": "SHD", "8": "HD"}[key]
		}
		fmt.Fprintf(&result, "%s(%d)  ", label, counts[key])
	}
	if nulls > 0 {
		fmt.Fprintf(&result, "(Null %d)", nulls)
	}
	return result.String(), nil
}

func siteUnitSummaryAspect(degrees float32) string {
	labels := []string{"NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW", "N"}
	for i, label := range labels {
		lower := float32(11.25 + float64(i)*22.5)
		upper := lower + 22.5
		if i == 15 {
			upper = 360
		}
		if degrees >= lower && degrees <= upper {
			return label
		}
	}
	if degrees >= 0 && degrees <= 11.25 {
		return "N"
	}
	if degrees == 999 {
		return "Level"
	}
	return "???"
}
