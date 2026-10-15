package main

func summaryVegetationEnvironmentalCategoryField(function string) (string, bool) {
	switch function {
	case "SVStructureStage":
		return "StructuralStage", true
	case "SVSuccessional":
		return "SuccessionalStatus", true
	case "SVSurfaceTopographyType":
		return "SurfaceTopographyType", true
	case "SVSnr":
		return "NutrientRegime", true
	case "SVMesoSlopePosition":
		return "MesoSlopePosition", true
	case "SVHydroGeoSubSystem":
		return "HydroGeoSubSystem", true
	case "SVHydroGeoSystem":
		return "HydroGeoSystem", true
	case "SVSoilDrainage":
		return "SoilDrainage", true
	case "SVHumusForm":
		return "HumusForm", true
	case "SVRootRestrictingType":
		return "RootRestrictingType", true
	case "SVSoilClassSubGroup":
		return "SoilClassSubGroup", true
	case "SVSoilClassGroup":
		return "SoilClassGroup", true
	case "SVRootZonePartSize":
		return "RootZoneParticleSize", true
	case "SVSurficialMaterialSurf":
		return "SurficialMaterialSurf", true
	case "SVExposure":
		return "Exposure1", true
	case "SVBioGeoClimaticZone":
		return "Zone", true
	case "SVBioGeoClimaticUnit":
		return "Zone", true
	case "SVAspect":
		return "Aspect", true
	case "SVSmr":
		return "MoistureRegime", true
	default:
		return "", false
	}
}

func summaryVegetationEnvironmentalTextField(function string) (string, bool) {
	if function == "SVSmr" || function == "SVBioGeoClimaticUnit" || function == "SVAspect" {
		return "", false
	}
	return summaryVegetationEnvironmentalCategoryField(function)
}
