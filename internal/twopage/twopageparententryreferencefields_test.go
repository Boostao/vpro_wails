package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
)

import (
	"reflect"
	"testing"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func TestTwoPageEntryReferenceSourceRejectsCountPreservingSubstitutions(t *testing.T) {
	sources, err := twoPageParentSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		original := sources[form].Forms[0].Fields
		if err := validateTwoPageEntryReferenceSource(form, original); err != nil {
			t.Fatal(err)
		}
		for _, failure := range []string{"scope substitution", "BEC_Use substitution", "different type", "duplicate", "missing"} {
			controls := append([]fs882layout.Field(nil), original...)
			zone, becUse, unrelated := -1, -1, -1
			for i, control := range controls {
				switch control.Binding {
				case "Zone":
					zone = i
				case "BEC_Use":
					becUse = i
				case "FieldNumber":
					unrelated = i
				}
			}
			if zone < 0 || unrelated < 0 || (form == "FS882-8x6XL" && becUse < 0) {
				t.Fatal("independent source substitution controls missing")
			}
			switch failure {
			case "scope substitution":
				controls[zone].Type, controls[unrelated].Type = "TextBox", "ComboBox"
			case "BEC_Use substitution":
				if form == "FS882-8x6XL-CHARS" {
					controls[zone].Type, controls[unrelated].Type = "TextBox", "ComboBox"
					controls[unrelated].Binding = "BEC_Use"
				} else {
					controls[becUse].Type, controls[unrelated].Type = "TextBox", "ComboBox"
				}
			case "different type":
				controls[zone].Type = "OptionGroup"
			case "duplicate":
				controls = append(controls, controls[zone])
			case "missing":
				controls = append(controls[:zone], controls[zone+1:]...)
			}
			if err := validateTwoPageEntryReferenceSource(form, controls); err == nil {
				t.Fatal("unreviewed source shape passed count-only sealing", form, failure)
			}
		}
		if form == "FS882-8x6XL-CHARS" {
			for _, kind := range []string{"TextBox", "ComboBox"} {
				controls := append([]fs882layout.Field(nil), original...)
				controls = append(controls, fs882layout.Field{Binding: "BEC_Use", Type: kind})
				if err := validateTwoPageEntryReferenceSource(form, controls); err == nil {
					t.Fatal("CHARS inherited absent source binding", kind)
				}
			}
		}
	}
}

func TestTwoPageEntryReferenceFieldsSourceAndPhysicalPolicies(t *testing.T) {
	expected := map[string]TwoPageEntryReferenceField{
		"LocationAccuracy":      {sourceform.SiviParentSharedReferencePolicy{Column: "LocationAccuracy", List: "accuracy", Reader: "family", Maximum: 6, Required: false}, "integer"},
		"PlotType":              {sourceform.SiviParentSharedReferencePolicy{Column: "PlotType", List: "PlotType", Reader: "family", Maximum: 10, Required: false}, "text"},
		"ProjectID":             {sourceform.SiviParentSharedReferencePolicy{Column: "ProjectID", List: "ProjectID", Reader: "project", Maximum: 30, Required: false}, "text"},
		"Ecosection":            {sourceform.SiviParentSharedReferencePolicy{Column: "Ecosection", List: "ecosection", Reader: "ecosection", Maximum: 3, Required: false}, "text"},
		"SurfaceTopographyType": {sourceform.SiviParentSharedReferencePolicy{Column: "SurfaceTopographyType", List: "SurfaceTopography", Reader: "family", Maximum: 3, Required: false}, "text"},
		"SV_FullCruiseCard":     {sourceform.SiviParentSharedReferencePolicy{Column: "SV_FullCruiseCard", List: "MensurationType", Reader: "family", Maximum: 50, Required: false}, "text"},
		"BedrockGeology3":       {sourceform.SiviParentSharedReferencePolicy{Column: "BedrockGeology3", List: "BedrockType", Reader: "geology", Maximum: 4, Required: false}, "text"},
		"CoarseFragLith1":       {sourceform.SiviParentSharedReferencePolicy{Column: "CoarseFragLith1", List: "BedrockType", Reader: "parent", Maximum: 12, Required: false}, "text"},
		"MesoSlopePosition":     {sourceform.SiviParentSharedReferencePolicy{Column: "MesoSlopePosition", List: "MesoSlopePosition", Reader: "family", Maximum: 3, Required: true}, "text"},
		"SurfaceShape":          {sourceform.SiviParentSharedReferencePolicy{Column: "SurfaceShape", List: "SurfaceShape", Reader: "family", Maximum: 3, Required: true}, "text"},
	}
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		fields, err := twoPageEntryReferenceFields(form)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]TwoPageEntryReferenceField{}
		required := []string{}
		for _, field := range fields {
			if _, exists := seen[field.Column]; exists {
				t.Fatal("reference field duplicated", field.Column)
			}
			seen[field.Column] = field
			if field.Required {
				required = append(required, field.Column)
			}
		}
		if !reflect.DeepEqual(required, []string{"Exposure1", "Exposure2", "MesoSlopePosition", "SoilDrainage", "SuccessionalStatus", "SurfaceShape"}) {
			t.Fatal("source mandatory membership changed", form, required)
		}
		for column, want := range expected {
			if got, exists := seen[column]; !exists || !reflect.DeepEqual(got, want) {
				t.Fatal("independent source list/physical expectation changed", form, column, got, want)
			}
		}
		becUse, exists := seen["BEC_Use"]
		if form == "FS882-8x6XL" {
			if len(fields) != 56 || !exists || becUse.List != "BEC_Use" || becUse.Maximum != 255 {
				t.Fatal("normal source ComboBox BEC_Use disappeared", fields)
			}
		} else if len(fields) != 55 || exists {
			t.Fatal("CHARS inherited absent BEC_Use binding as a reference control", fields)
		}
	}
	if got, err := twoPageEntryReferenceFields("FS882-6x4XL"); err == nil || got != nil {
		t.Fatal("foreign source inherited complete-entry reference policies", got, err)
	}
}
