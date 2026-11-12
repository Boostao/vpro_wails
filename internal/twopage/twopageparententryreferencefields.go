package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

type TwoPageEntryReferenceField struct {
	sourceform.SiviParentSharedReferencePolicy
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

func twoPageEntryReferenceFields(form string) ([]TwoPageEntryReferenceField, error) {
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
	policies := map[string]sourceform.SiviParentSharedReferencePolicy{}
	for _, field := range sourceform.SiviParentSharedReferenceFields {
		policies[field.Column] = field
	}
	for _, field := range vprocore.ParentCodeDescriptors {
		policies[field.Name] = sourceform.SiviParentSharedReferencePolicy{Column: field.Name, List: field.List, Reader: "parent", Maximum: field.Maximum, Required: false}
	}
	for _, field := range vprocore.GeologyHeaderFields(vprocore.FS882Header{}) {
		policies[field.Name] = sourceform.SiviParentSharedReferencePolicy{Column: field.Name, List: "BedrockType", Reader: "geology", Maximum: field.Maximum, Required: false}
	}
	for _, field := range vprocore.QualityHeaderFields(vprocore.FS882Header{}) {
		policies[field.Name] = sourceform.SiviParentSharedReferencePolicy{Column: field.Name, List: "PlotQualitySite", Reader: "quality", Maximum: field.Maximum, Required: false}
	}
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		list := "SoilClassGroup"
		if field.Name == "SoilClassSubGroup" {
			list = "SoilClassSubgroup"
		}
		policies[field.Name] = sourceform.SiviParentSharedReferencePolicy{Column: field.Name, List: list, Reader: "soil", Maximum: field.Maximum, Required: false}
	}
	for _, field := range []sourceform.SiviParentSharedReferencePolicy{
		{Column: "ProjectID", List: "ProjectID", Reader: "project", Maximum: 30, Required: false},
		{Column: "PlotType", List: "PlotType", Reader: "family", Maximum: 10, Required: false},
		{Column: "LocationAccuracy", List: "accuracy", Reader: "family", Maximum: 6, Required: false},
		{Column: "Ecosection", List: "ecosection", Reader: "ecosection", Maximum: 3, Required: false},
		{Column: "SiteSeries", List: "SiteSeries", Reader: "bec-series", Maximum: 5, Required: false},
		{Column: "TransDistrib", List: "TransDistrib", Reader: "family", Maximum: 3, Required: false},
		{Column: "SurfaceTopographyType", List: "SurfaceTopography", Reader: "family", Maximum: 3, Required: false},
		{Column: "SurfaceTopographySize", List: "SurfaceTopographySize", Reader: "family", Maximum: 2, Required: false},
		{Column: "BECSiteUnit", List: "BECSiteUnit", Reader: "master-unit", Maximum: 100, Required: false},
		{Column: "UserSiteUnit", List: "UserSiteUnit", Reader: "working-unit", Maximum: 100, Required: false},
		{Column: "SV_FullCruiseCard", List: "MensurationType", Reader: "family", Maximum: 50, Required: false},
		{Column: "BEC_Use", List: "BEC_Use", Reader: "family", Maximum: 255, Required: false},
	} {
		policies[field.Column] = field
	}
	fields := []TwoPageEntryReferenceField{}
	seen := map[string]bool{}
	for _, control := range sources[form].Forms[0].Fields {
		if control.Type != "ComboBox" || control.Binding == "" {
			continue
		}
		policy, exists := policies[control.Binding]
		if !exists || seen[control.Binding] {
			return nil, fmt.Errorf("complete-entry reference %s has an unreviewed reader or duplicate owner", control.Binding)
		}
		policy.Required = false
		if limit, present := control.Properties["LimitToList"]; present {
			if limit.Value != "NotDefault" {
				return nil, fmt.Errorf("complete-entry reference %s has an unreviewed LimitToList property", control.Binding)
			}
			policy.Required = true
		}
		storage := "text"
		if physical, present := xl[control.Binding]; present {
			if physical.kind == "integer" && control.Binding == "LocationAccuracy" {
				storage = "integer"
			} else if physical.kind != "text" || physical.maximum != policy.Maximum {
				return nil, fmt.Errorf("complete-entry reference %s disagrees with its physical policy", control.Binding)
			}
		} else if extra, present := sourceform.TwoPageParentExtraFields[control.Binding]; present {
			if extra.Domain != "text" || extra.Maximum != policy.Maximum {
				return nil, fmt.Errorf("complete-entry extra reference %s disagrees with its physical policy", control.Binding)
			}
		} else if control.Binding != "PlotType" {
			return nil, fmt.Errorf("complete-entry reference %s has no reviewed physical scope", control.Binding)
		}
		fields = append(fields, TwoPageEntryReferenceField{policy, storage})
		seen[control.Binding] = true
	}
	want := 56
	if form == "FS882-8x6XL-CHARS" {
		want = 55
	}
	if len(fields) != want {
		return nil, errors.New("complete-entry bound ComboBox reference scope changed")
	}
	slices.SortFunc(fields, func(left, right TwoPageEntryReferenceField) int {
		return strings.Compare(left.Column, right.Column)
	})
	return fields, nil
}
