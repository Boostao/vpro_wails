package main

import "database/sql"

type parentCodeField struct {
	becHeaderField
	list string
}

var parentCodeDescriptors = parentCodeFields(FS882Header{})

func parentCodeFields(h FS882Header) []parentCodeField {
	return []parentCodeField{
		{becHeaderField{"CoarseFragLith1", "coarseFragLith1", "Env", 12, h.CoarseFragLith1}, "BedrockType"},
		{becHeaderField{"CoarseFragLith2", "coarseFragLith2", "Env", 12, h.CoarseFragLith2}, "BedrockType"},
		{becHeaderField{"CoarseFragLith3", "coarseFragLith3", "Env", 12, h.CoarseFragLith3}, "BedrockType"},
		{becHeaderField{"FloodingRegimeDur", "floodingRegimeDur", "Env", 2, h.FloodingRegimeDur}, "FloodingRegimeDur"},
		{becHeaderField{"FloodingRegimeFreq", "floodingRegimeFreq", "Env", 7, h.FloodingRegimeFreq}, "FloodingRegimeFreq"},
		{becHeaderField{"GeoMorProSubSurf", "geoMorProSubSurf", "Env", 3, h.GeoMorProSubSurf}, "GeoMorPro"},
		{becHeaderField{"GeoMorProSurf", "geoMorProSurf", "Env", 3, h.GeoMorProSurf}, "GeoMorPro"},
		{becHeaderField{"HumusForm", "humusForm", "Env", 4, h.HumusForm}, "HumusForm"},
		{becHeaderField{"HumusFormPhase", "humusFormPhase", "Env", 50, h.HumusFormPhase}, "HumusFormPhase"},
		{becHeaderField{"HydroGeoSubSystem", "hydroGeoSubSystem", "Env", 2, h.HydroGeoSubSystem}, "HydrogeoSubsystem"},
		{becHeaderField{"HydroGeoSystem", "hydroGeoSystem", "Env", 3, h.HydroGeoSystem}, "HydrogeoSystem"},
		{becHeaderField{"RealmClass", "realmClass", "Env", 5, h.RealmClass}, "RealmClass"},
		{becHeaderField{"RootRestrictingType", "rootRestrictingType", "Env", 1, h.RootRestrictingType}, "RootRestrictingType"},
		{becHeaderField{"RootZoneParticleSize", "rootZoneParticleSize", "Env", 6, h.RootZoneParticleSize}, "RootZoneParticleSize"},
		{becHeaderField{"SurfaceExpSubSurf", "surfaceExpSubSurf", "Env", 3, h.SurfaceExpSubSurf}, "SurfaceExp"},
		{becHeaderField{"SurfaceExpSurf", "surfaceExpSurf", "Env", 3, h.SurfaceExpSurf}, "SurfaceExp"},
		{becHeaderField{"SurficialMaterialSubSurf", "surficialMaterialSubSurf", "Env", 6, h.SurficialMaterialSubSurf}, "SurficialMaterial"},
		{becHeaderField{"SurficialMaterialSurf", "surficialMaterialSurf", "Env", 6, h.SurficialMaterialSurf}, "SurficialMaterial"},
		{becHeaderField{"TerrainTextureSubSurf", "terrainTextureSubSurf", "Env", 3, h.TerrainTextureSubSurf}, "TerrainTexture"},
		{becHeaderField{"TerrainTextureSurf", "terrainTextureSurf", "Env", 3, h.TerrainTextureSurf}, "TerrainTexture"},
		{becHeaderField{"WaterSource", "waterSource", "Env", 5, h.WaterSource}, "WaterSource"},
	}
}

func parentCodeHeaderFields(h FS882Header) []becHeaderField {
	fields := parentCodeFields(h)
	result := make([]becHeaderField, len(fields))
	for i, field := range fields {
		result[i] = field.becHeaderField
	}
	return result
}

func isParentCodeProperty(property string) bool {
	for _, field := range parentCodeDescriptors {
		if property == field.property {
			return true
		}
	}
	return false
}

func parentCodeListMaximum(list string) (int, bool) {
	if list == "SoilDrainage" {
		return 5, true // Reference-only while the distinct LimitToList constraint is held.
	}
	for _, field := range parentCodeDescriptors {
		if list == field.list {
			return field.maximum, true
		}
	}
	return 0, false
}

func validateParentCodeHeaderValues(h FS882Header, old *FS882Header) error {
	return validateCodeHeaderValues(h, old, parentCodeHeaderFields, false)
}

func validateParentCodeHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, validateParentCodeHeaderValues)
}

func validateParentCodeRestoreValue(table, column string, value any) error {
	return validateCodeRestoreValue(table, column, value, parentCodeHeaderFields(FS882Header{}))
}
