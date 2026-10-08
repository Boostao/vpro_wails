package main

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func longVegetationOptionsFixture(t *testing.T) configValues {
	t.Helper()
	values, err := decodeConfig(desktopDefaults)
	if err != nil {
		t.Fatal(err)
	}
	return values
}

func TestDecodeLongVegetationQualityCriteriaPreservesDefaultInitAndRuntimeTypes(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	before, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	criteria, err := decodeLongVegetationQualityCriteria(values)
	if err != nil || criteria != vegetationQualityCriteria("Poor", 7) {
		t.Fatal("source minimum/NULL defaults lost", criteria, err)
	}
	after, err := yaml.Marshal(values)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("quality decoder rewrote stored YAML types", err)
	}
	for i, domain := range []string{"Site", "Veg", "Soil"} {
		for _, value := range []any{true, false, -1, 0, "True", "False"} {
			values := longVegetationOptionsFixture(t)
			fields := values["ReportOptions"].(map[string]any)
			fields["DataQualityFilter"+domain+"NullLV"] = value
			criteria, err := decodeLongVegetationQualityCriteria(values)
			want := value == true || value == -1 || value == "True"
			if err != nil || criteria[i].IncludeNull != want || fields["DataQualityFilter"+domain+"NullLV"] != value {
				t.Fatal("explicit source bool/string flag lost or normalized in storage", domain, value, criteria, err)
			}
		}
		for _, label := range []string{"", "   ", "Good", strings.Repeat("x", 270), "  Literal \u00e9  "} {
			values := longVegetationOptionsFixture(t)
			values["ReportOptions"].(map[string]any)["DataQualityFilter"+domain+"LV"] = label
			criteria, err := decodeLongVegetationQualityCriteria(values)
			if err != nil || criteria[i].Minimum != label {
				t.Fatal("quality minimum label trimmed/coerced/bounded without source evidence", criteria, err)
			}
		}
	}
}

func TestDecodeLongVegetationQualityCriteriaRejectsMissingNullAndMalformedSettings(t *testing.T) {
	for _, domain := range []string{"Site", "Veg", "Soil"} {
		for suffix, badValues := range map[string][]any{
			"NullLV": {nil, 1, "true", "FALSE", " True ", uint64(0), float64(0), []string{"True"}},
			"LV":     {nil, 1, true, []string{"Poor"}, string([]byte{0xff}), "Poor\x00"},
		} {
			key := "DataQualityFilter" + domain + suffix
			for _, invalid := range badValues {
				values := longVegetationOptionsFixture(t)
				values["ReportOptions"].(map[string]any)[key] = invalid
				criteria, err := decodeLongVegetationQualityCriteria(values)
				if err == nil || !strings.Contains(err.Error(), key) || criteria != ([3]vegetationQualityCriterion{}) {
					t.Fatal("invalid criterion accepted or leaked partial settings", key, invalid, criteria, err)
				}
			}
			values := longVegetationOptionsFixture(t)
			delete(values["ReportOptions"].(map[string]any), key)
			if criteria, err := decodeLongVegetationQualityCriteria(values); err == nil || criteria != ([3]vegetationQualityCriterion{}) {
				t.Fatal("missing quality preference silently defaulted", key, criteria, err)
			}
		}
	}
	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["DataQualityFilterSiteNullLV"] = nil
	if _, err := decodeLongVegetationOptions(values); err != nil {
		t.Fatal("inactive quality criteria caused unrelated default preview failure", err)
	}
}

func TestDecodeLongVegetationOptionsDefaults(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	got, err := decodeLongVegetationOptions(values)
	want := longVegetationOptions{Title: "Long Vegetation Report", Average: "all-plots",
		ConstantSpeciesList: true, Order: "presence", ShowEnglishName: true}
	if err != nil || got != want {
		t.Fatalf("defaults: %#v, %v; want %#v", got, err, want)
	}
	if values["ReportOptions"].(map[string]any)["LVReportSummary"] != -1 {
		t.Fatal("source summary preference changed")
	}
}

func TestDecodeLongVegetationOptionsSourceEnums(t *testing.T) {
	// USysLongVegOptions: Calculate 10/20; Order 10/20; Show 0/1/2.
	for _, average := range []int{10, 20} {
		for _, order := range []int{10, 20} {
			for _, show := range []int{0, 1, 2} {
				for _, constant := range []any{-1, 0, true, false} {
					t.Run(fmt.Sprintf("%d/%d/%d/%v", average, order, show, constant), func(t *testing.T) {
						values := longVegetationOptionsFixture(t)
						fields := values["ReportOptions"].(map[string]any)
						fields["LVAvgType"], fields["LVOrderBy"], fields["LVShowEnglishName"] = average, order, show
						fields["LVConstantSppList"] = constant
						got, err := decodeLongVegetationOptions(values)
						wantConstant := constant == -1 || constant == true
						if err != nil || got.Average != map[int]string{10: "all-plots", 20: "observations"}[average] ||
							got.Order != map[int]string{10: "species", 20: "presence"}[order] ||
							got.ShowEnglishName != (show == 1) || got.ShowSpeciesCode != (show == 2) || got.ConstantSpeciesList != wantConstant {
							t.Fatalf("source modes: %#v, %v", got, err)
						}
					})
				}
			}
		}
	}
	for _, enforce := range []any{false, 0} {
		values := longVegetationOptionsFixture(t)
		values["ReportOptions"].(map[string]any)["DataQualityFilterEnforceLV"] = enforce
		if _, err := decodeLongVegetationOptions(values); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDecodeLongVegetationOptionsUnavailableModes(t *testing.T) {
	for key, modes := range map[string][]any{
		"LVUnitGroups":      {2, 3},
		"LVShowEnglishName": {4},
	} {
		for _, mode := range modes {
			t.Run(fmt.Sprintf("%s/%v", key, mode), func(t *testing.T) {
				values := longVegetationOptionsFixture(t)
				values["ReportOptions"].(map[string]any)[key] = mode
				got, err := decodeLongVegetationOptions(values)
				if err == nil || !strings.Contains(err.Error(), key) || got != (longVegetationOptions{}) {
					t.Fatalf("unavailable mode silently accepted: %#v, %v", got, err)
				}
			})
		}
	}
}

func TestDecodeLongVegetationOptionsStrictTypesAndMissing(t *testing.T) {
	fields := map[string][]any{
		"LVReportTitle":              {0, false, []any{"title"}, "bad\x00title", string([]byte{0xff})},
		"LVAvgType":                  {0, 1, 11, 19, 21, 10.0, "10", true},
		"LVOrderBy":                  {0, 11, 19, 21, 20.0, "20", false},
		"LVGroupBy":                  {0, 5, 1.0, "1", true},
		"LVUnitGroups":               {0, 4, 1.0, "1", false},
		"LVShowEnglishName":          {-1, 3, 1.0, "1", true},
		"LVConstantSppList":          {1, -2, -1.0, "True", " -1 ", []any{true}},
		"DataQualityFilterEnforceLV": {1, -2, 0.0, "False", "0", []any{false}},
		"LVPresenceGreaterThan":      {"0", " 2 ", false, []any{1}, math.NaN(), math.Inf(1), math.Inf(-1), int64(2)},
		"LVCoverGreaterThan":         {"0", true, map[string]any{}, math.NaN(), math.Inf(1), math.Inf(-1), uint64(1<<63 + 1)},
	}
	for key, invalid := range fields {
		invalid = append(invalid, nil)
		for i, value := range invalid {
			t.Run(fmt.Sprintf("%s/%d", key, i), func(t *testing.T) {
				values := longVegetationOptionsFixture(t)
				values["ReportOptions"].(map[string]any)[key] = value
				got, err := decodeLongVegetationOptions(values)
				if err == nil || !strings.Contains(err.Error(), key) || got != (longVegetationOptions{}) {
					t.Fatalf("invalid value accepted: %#v, %v", got, err)
				}
			})
		}
		t.Run(key+"/missing", func(t *testing.T) {
			values := longVegetationOptionsFixture(t)
			delete(values["ReportOptions"].(map[string]any), key)
			if _, err := decodeLongVegetationOptions(values); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("missing setting defaulted: %v", err)
			}
		})
	}
	for _, values := range []configValues{nil, {}, {"ReportOptions": nil}, {"ReportOptions": []any{}}, {"ReportOptions": "wrong"}} {
		if _, err := decodeLongVegetationOptions(values); err == nil || !strings.Contains(err.Error(), "ReportOptions") {
			t.Fatalf("malformed section accepted: %v", err)
		}
	}
}

func TestDecodeLongVegetationOptionsUnicodeAndThresholds(t *testing.T) {
	for _, title := range []string{"", "  Forêt 🌲 e\u0301 中文\nrapport  ", strings.Repeat("🌲", 1000)} {
		values := longVegetationOptionsFixture(t)
		fields := values["ReportOptions"].(map[string]any)
		fields["LVReportTitle"] = title
		fields["LVPresenceGreaterThan"], fields["LVCoverGreaterThan"] = 37.5, -12.25
		got, err := decodeLongVegetationOptions(values)
		if err != nil || got.Title != title || got.PresenceGreaterThan != 37.5 || got.MeanCoverGreaterThan != -12.25 {
			t.Fatalf("literal Unicode/percent thresholds changed: %#v, %v", got, err)
		}
	}
	for _, number := range []any{-1, 101, 0.125, math.MaxFloat64, uint64(1 << 63)} {
		values := longVegetationOptionsFixture(t)
		fields := values["ReportOptions"].(map[string]any)
		fields["LVPresenceGreaterThan"], fields["LVCoverGreaterThan"] = number, number
		if _, err := decodeLongVegetationOptions(values); err != nil {
			t.Fatalf("guessed source threshold bound for %v: %v", number, err)
		}
	}
	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["LVPresenceGreaterThan"] = int(1<<53 + 1)
	if _, err := decodeLongVegetationOptions(values); err == nil || !strings.Contains(err.Error(), "exactly") {
		t.Fatalf("integer threshold rounded silently: %v", err)
	}
}

func TestDecodeLongVegetationOptionsYAMLAndNoMutation(t *testing.T) {
	for _, input := range []string{
		"LVConstantSppList: true\nDataQualityFilterEnforceLV: false\nLVPresenceGreaterThan: 2.5\nLVCoverGreaterThan: 7\n",
		"LVConstantSppList: -1\nDataQualityFilterEnforceLV: 0\nLVPresenceGreaterThan: 18446744073709551616\nLVCoverGreaterThan: 9223372036854775808\n",
	} {
		values := longVegetationOptionsFixture(t)
		var overrides map[string]any
		if err := yaml.Unmarshal([]byte(input), &overrides); err != nil {
			t.Fatal(err)
		}
		fields := values["ReportOptions"].(map[string]any)
		for key, value := range overrides {
			fields[key] = value
		}
		if _, err := decodeLongVegetationOptions(values); err != nil {
			t.Fatalf("valid YAML scalars: %v", err)
		}
	}
	for _, summary := range []any{-1, 0, nil, "publication-only"} {
		for _, invalid := range []bool{false, true} {
			values := longVegetationOptionsFixture(t)
			fields := values["ReportOptions"].(map[string]any)
			fields["LVReportSummary"] = summary
			for _, key := range []string{"LVQuickReport", "LVSpaceBetweenGroups", "LVUseSppCodesOnly"} {
				fields[key] = map[string]any{"preserved": []any{nil, "True", -1}}
			}
			if invalid {
				fields["LVAvgType"] = 11
			}
			before, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			_, err = decodeLongVegetationOptions(values)
			if (err != nil) != invalid {
				t.Fatalf("publication settings affected preview: %v", err)
			}
			after, marshalErr := yaml.Marshal(values)
			if marshalErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("decoder changed config on success/failure")
			}
		}
	}
	values := longVegetationOptionsFixture(t)
	fields := values["ReportOptions"].(map[string]any)
	for _, key := range []string{"LVReportSummary", "LVQuickReport", "LVSpaceBetweenGroups", "LVUseSppCodesOnly"} {
		delete(fields, key)
	}
	if _, err := decodeLongVegetationOptions(values); err != nil {
		t.Fatalf("publication-only settings required for preview: %v", err)
	}
}
