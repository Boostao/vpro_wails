package main

const siviCHARSParentEntryHistoryTable = "__VPRO_SIVICHARSParentEntryHistory"

func twoPageEntryProjectHistoryTable(table string) bool {
	return table == "__VPRO_TwoPageParentEntryHistory" || table == "__VPRO_TwoPageCHARSParentEntryHistory"
}

type twoPageParentExtraField struct {
	owner, domain string
	maximum       int
}

var twoPageParentExtraFields = map[string]twoPageParentExtraField{
	"SV_WaterTableCM":        {"Env", "single", 0},
	"SV_FullCruiseCard":      {"Env", "text", 50},
	"ActiveLayerDepth":       {"Env", "single", 0},
	"PlotSize":               {"Admin", "single", 0},
	"ProvinceStateTerritory": {"Admin", "text", 255},
	"SiteUnitLongName":       {"Admin", "text", 100},
	"GIS_BGC":                {"Admin", "text", 255},
	"GIS_BGC_VER":            {"Admin", "integer", 0},
	"BEC_Use":                {"Admin", "text", 255},
}
