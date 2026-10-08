package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

type twoPageEntryReferenceField struct {
	siviParentSharedReferencePolicy
	storage string
}

func validateTwoPageEntryReferenceSource(form string, controls []fs882layout.Field) error {
	if form != "FS882-8x6XL" && form != "FS882-8x6XL-CHARS" {
		return errors.New("complete-entry reference source requires an exact normal or CHARS form")
	}
	expected := map[string]string{}
	for _, column := range strings.Fields(`ProjectID PlotType FSRegionDistrict LocationAccuracy Ecosection
		Zone SubZone SiteSeries RealmClass TransDistrib MoistureRegime NutrientRegime SuccessionalStatus
		StructuralStage MesoSlopePosition SurfaceShape SurfaceTopographyType SurfaceTopographySize
		SiteDisturbance1 SiteDisturbance2 SiteDisturbance3 Exposure1 Exposure2 BECSiteUnit UserSiteUnit
		SV_FullCruiseCard SitePlotQuality VegPlotQuality SoilPlotQuality BedrockGeology1 BedrockGeology2
		BedrockGeology3 CoarseFragLith1 CoarseFragLith2 CoarseFragLith3 TerrainTextureSurf SurficialMaterialSurf
		SurfaceExpSurf GeoMorProSurf TerrainTextureSubSurf SurficialMaterialSubSurf SurfaceExpSubSurf
		GeoMorProSubSurf SoilClassSubGroup SoilClassGroup HumusForm HumusFormPhase HydroGeoSystem
		HydroGeoSubSystem RootRestrictingType WaterSource SoilDrainage RootZoneParticleSize FloodingRegimeFreq
		FloodingRegimeDur`) {
		expected[column] = "ComboBox"
	}
	if form == "FS882-8x6XL" {
		expected["BEC_Use"] = "ComboBox"
	}
	seen := map[string]bool{}
	for _, control := range controls {
		if control.Binding == "" {
			continue
		}
		kind, known := expected[control.Binding]
		if !known {
			if control.Type == "ComboBox" || control.Binding == "BEC_Use" {
				return fmt.Errorf("complete-entry reference source has an unreviewed ComboBox %s", control.Binding)
			}
			continue
		}
		if seen[control.Binding] || control.Type != kind {
			return fmt.Errorf("complete-entry reference source %s has a duplicate or different control type", control.Binding)
		}
		seen[control.Binding] = true
	}
	if len(seen) != len(expected) {
		return errors.New("complete-entry reference source is missing reviewed controls")
	}
	return nil
}

func twoPageEntryReferenceFields(form string) ([]twoPageEntryReferenceField, error) {
	if _, err := twoPageParentBindings(form); err != nil {
		return nil, err
	}
	sources, err := twoPageParentSources()
	if err != nil {
		return nil, err
	}
	if err := validateTwoPageEntryReferenceSource(form, sources[form].Forms[0].Fields); err != nil {
		return nil, err
	}
	xl, err := twoPageXLFields(form)
	if err != nil {
		return nil, err
	}
	policies := map[string]siviParentSharedReferencePolicy{}
	for _, field := range siviParentSharedReferenceFields {
		policies[field.column] = field
	}
	for _, field := range parentCodeDescriptors {
		policies[field.name] = siviParentSharedReferencePolicy{field.name, field.list, "parent", field.maximum, false}
	}
	for _, field := range geologyHeaderFields(FS882Header{}) {
		policies[field.name] = siviParentSharedReferencePolicy{field.name, "BedrockType", "geology", field.maximum, false}
	}
	for _, field := range qualityHeaderFields(FS882Header{}) {
		policies[field.name] = siviParentSharedReferencePolicy{field.name, "PlotQualitySite", "quality", field.maximum, false}
	}
	for _, field := range soilHeaderFields(FS882Header{}) {
		list := "SoilClassGroup"
		if field.name == "SoilClassSubGroup" {
			list = "SoilClassSubgroup"
		}
		policies[field.name] = siviParentSharedReferencePolicy{field.name, list, "soil", field.maximum, false}
	}
	for _, field := range []siviParentSharedReferencePolicy{
		{"ProjectID", "ProjectID", "project", 30, false},
		{"PlotType", "PlotType", "family", 10, false},
		{"LocationAccuracy", "accuracy", "family", 6, false},
		{"Ecosection", "ecosection", "ecosection", 3, false},
		{"SiteSeries", "SiteSeries", "bec-series", 5, false},
		{"TransDistrib", "TransDistrib", "family", 3, false},
		{"SurfaceTopographyType", "SurfaceTopography", "family", 3, false},
		{"SurfaceTopographySize", "SurfaceTopographySize", "family", 2, false},
		{"BECSiteUnit", "BECSiteUnit", "master-unit", 100, false},
		{"UserSiteUnit", "UserSiteUnit", "working-unit", 100, false},
		{"SV_FullCruiseCard", "MensurationType", "family", 50, false},
		{"BEC_Use", "BEC_Use", "family", 255, false},
	} {
		policies[field.column] = field
	}
	fields := []twoPageEntryReferenceField{}
	seen := map[string]bool{}
	for _, control := range sources[form].Forms[0].Fields {
		if control.Type != "ComboBox" || control.Binding == "" {
			continue
		}
		policy, exists := policies[control.Binding]
		if !exists || seen[control.Binding] {
			return nil, fmt.Errorf("complete-entry reference %s has an unreviewed reader or duplicate owner", control.Binding)
		}
		policy.required = false
		if limit, present := control.Properties["LimitToList"]; present {
			if limit.Value != "NotDefault" {
				return nil, fmt.Errorf("complete-entry reference %s has an unreviewed LimitToList property", control.Binding)
			}
			policy.required = true
		}
		storage := "text"
		if physical, present := xl[control.Binding]; present {
			if physical.kind == "integer" && control.Binding == "LocationAccuracy" {
				storage = "integer"
			} else if physical.kind != "text" || physical.maximum != policy.maximum {
				return nil, fmt.Errorf("complete-entry reference %s disagrees with its physical policy", control.Binding)
			}
		} else if extra, present := twoPageParentExtraFields[control.Binding]; present {
			if extra.domain != "text" || extra.maximum != policy.maximum {
				return nil, fmt.Errorf("complete-entry extra reference %s disagrees with its physical policy", control.Binding)
			}
		} else if control.Binding != "PlotType" {
			return nil, fmt.Errorf("complete-entry reference %s has no reviewed physical scope", control.Binding)
		}
		fields = append(fields, twoPageEntryReferenceField{policy, storage})
		seen[control.Binding] = true
	}
	want := 56
	if form == "FS882-8x6XL-CHARS" {
		want = 55
	}
	if len(fields) != want {
		return nil, errors.New("complete-entry bound ComboBox reference scope changed")
	}
	slices.SortFunc(fields, func(left, right twoPageEntryReferenceField) int {
		return strings.Compare(left.column, right.column)
	})
	return fields, nil
}
