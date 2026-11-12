package vprocore

import "database/sql"

type parentCodeField struct {
	BecHeaderField
	List string `json:"-" yaml:"-"`
}

var ParentCodeDescriptors = ParentCodeFields(FS882Header{})

func ParentCodeFields(h FS882Header) []parentCodeField {
	return []parentCodeField{
		{BecHeaderField{"CoarseFragLith1", "coarseFragLith1", "Env", 12, h.CoarseFragLith1}, "BedrockType"},
		{BecHeaderField{"CoarseFragLith2", "coarseFragLith2", "Env", 12, h.CoarseFragLith2}, "BedrockType"},
		{BecHeaderField{"CoarseFragLith3", "coarseFragLith3", "Env", 12, h.CoarseFragLith3}, "BedrockType"},
		{BecHeaderField{"FloodingRegimeDur", "floodingRegimeDur", "Env", 2, h.FloodingRegimeDur}, "FloodingRegimeDur"},
		{BecHeaderField{"FloodingRegimeFreq", "floodingRegimeFreq", "Env", 7, h.FloodingRegimeFreq}, "FloodingRegimeFreq"},
		{BecHeaderField{"GeoMorProSubSurf", "geoMorProSubSurf", "Env", 3, h.GeoMorProSubSurf}, "GeoMorPro"},
		{BecHeaderField{"GeoMorProSurf", "geoMorProSurf", "Env", 3, h.GeoMorProSurf}, "GeoMorPro"},
		{BecHeaderField{"HumusForm", "humusForm", "Env", 4, h.HumusForm}, "HumusForm"},
		{BecHeaderField{"HumusFormPhase", "humusFormPhase", "Env", 50, h.HumusFormPhase}, "HumusFormPhase"},
		{BecHeaderField{"HydroGeoSubSystem", "hydroGeoSubSystem", "Env", 2, h.HydroGeoSubSystem}, "HydrogeoSubsystem"},
		{BecHeaderField{"HydroGeoSystem", "hydroGeoSystem", "Env", 3, h.HydroGeoSystem}, "HydrogeoSystem"},
		{BecHeaderField{"RealmClass", "realmClass", "Env", 5, h.RealmClass}, "RealmClass"},
		{BecHeaderField{"RootRestrictingType", "rootRestrictingType", "Env", 1, h.RootRestrictingType}, "RootRestrictingType"},
		{BecHeaderField{"RootZoneParticleSize", "rootZoneParticleSize", "Env", 6, h.RootZoneParticleSize}, "RootZoneParticleSize"},
		{BecHeaderField{"SurfaceExpSubSurf", "surfaceExpSubSurf", "Env", 3, h.SurfaceExpSubSurf}, "SurfaceExp"},
		{BecHeaderField{"SurfaceExpSurf", "surfaceExpSurf", "Env", 3, h.SurfaceExpSurf}, "SurfaceExp"},
		{BecHeaderField{"SurficialMaterialSubSurf", "surficialMaterialSubSurf", "Env", 6, h.SurficialMaterialSubSurf}, "SurficialMaterial"},
		{BecHeaderField{"SurficialMaterialSurf", "surficialMaterialSurf", "Env", 6, h.SurficialMaterialSurf}, "SurficialMaterial"},
		{BecHeaderField{"TerrainTextureSubSurf", "terrainTextureSubSurf", "Env", 3, h.TerrainTextureSubSurf}, "TerrainTexture"},
		{BecHeaderField{"TerrainTextureSurf", "terrainTextureSurf", "Env", 3, h.TerrainTextureSurf}, "TerrainTexture"},
		{BecHeaderField{"WaterSource", "waterSource", "Env", 5, h.WaterSource}, "WaterSource"},
	}
}

func ParentCodeHeaderFields(h FS882Header) []BecHeaderField {
	fields := ParentCodeFields(h)
	result := make([]BecHeaderField, len(fields))
	for i, field := range fields {
		result[i] = field.BecHeaderField
	}
	return result
}

func IsParentCodeProperty(property string) bool {
	for _, field := range ParentCodeDescriptors {
		if property == field.Property {
			return true
		}
	}
	return false
}

func validateParentCodeHeaderValues(h FS882Header, old *FS882Header) error {
	return validateCodeHeaderValues(h, old, ParentCodeHeaderFields, false)
}

func validateParentCodeHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, validateParentCodeHeaderValues)
}

func ValidateParentCodeRestoreValue(table, column string, value any) error {
	return validateCodeRestoreValue(table, column, value, ParentCodeHeaderFields(FS882Header{}))
}
