package sourceform

const SiviCHARSParentEntryHistoryTable = "__VPRO_SIVICHARSParentEntryHistory"

func TwoPageEntryProjectHistoryTable(table string) bool {
	return table == "__VPRO_TwoPageParentEntryHistory" || table == "__VPRO_TwoPageCHARSParentEntryHistory"
}

type twoPageParentExtraField struct {
	Owner, Domain string `json:"-" yaml:"-"`
	Maximum       int    `json:"-" yaml:"-"`
}

var TwoPageParentExtraFields = map[string]twoPageParentExtraField{
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
