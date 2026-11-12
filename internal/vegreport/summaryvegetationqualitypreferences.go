package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type summaryVegetationQualityPreferenceValues struct {
	Enforce          bool
	SiteMinimum      string
	VegMinimum       string
	SoilMinimum      string
	BECMinimum       string
	SiteIncludeNulls bool
	VegIncludeNulls  bool
	SoilIncludeNulls bool
	BECIncludeNulls  bool
}

func summaryVegetationQualityPreferenceOption(values vprocore.ConfigValues, key string) (any, error) {
	switch key {
	case "DataQualityFilterSite", "DataQualityFilterVeg", "DataQualityFilterSoil", "DataQualityFilterBECUseLV":
		value, err := vprocore.ConfigString(values, "ReportOptions", key)
		if err != nil {
			return nil, err
		}
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("ReportOptions.%s contains malformed Unicode", key)
		}
		return value, nil
	case "DataQualityFilterEnforce":
		return longVegetationConfigBool(values, key)
	case "DataQualityFilterSiteNull", "DataQualityFilterVegNull", "DataQualityFilterSoilNull", "DataQualityFilterBECUse":
		fields, err := vprocore.ConfigSection(values, "ReportOptions")
		if err != nil {
			return nil, err
		}
		if literal, ok := fields[key].(string); ok {
			switch literal {
			case "True":
				return true, nil
			case "False":
				return false, nil
			default:
				return nil, fmt.Errorf("ReportOptions.%s legacy string must be exactly True or False", key)
			}
		}
		if key == "DataQualityFilterBECUse" {
			if number, ok := fields[key].(int); ok && number == 1 {
				return true, nil
			}
		}
		return longVegetationConfigBool(values, key)
	default:
		return nil, fmt.Errorf("unsupported Summary Vegetation quality preference %s", key)
	}
}

func summaryVegetationQualityPreferenceChanges(expected, proposed summaryVegetationQualityPreferenceValues) []vprocore.ReportPreferenceChange[any] {
	return []vprocore.ReportPreferenceChange[any]{
		{Key: "DataQualityFilterEnforce", Expected: expected.Enforce, Proposed: proposed.Enforce},
		{Key: "DataQualityFilterSite", Expected: expected.SiteMinimum, Proposed: proposed.SiteMinimum},
		{Key: "DataQualityFilterVeg", Expected: expected.VegMinimum, Proposed: proposed.VegMinimum},
		{Key: "DataQualityFilterSoil", Expected: expected.SoilMinimum, Proposed: proposed.SoilMinimum},
		{Key: "DataQualityFilterBECUseLV", Expected: expected.BECMinimum, Proposed: proposed.BECMinimum},
		{Key: "DataQualityFilterSiteNull", Expected: expected.SiteIncludeNulls, Proposed: proposed.SiteIncludeNulls},
		{Key: "DataQualityFilterVegNull", Expected: expected.VegIncludeNulls, Proposed: proposed.VegIncludeNulls},
		{Key: "DataQualityFilterSoilNull", Expected: expected.SoilIncludeNulls, Proposed: proposed.SoilIncludeNulls},
		{Key: "DataQualityFilterBECUse", Expected: expected.BECIncludeNulls, Proposed: proposed.BECIncludeNulls},
	}
}

func ReadSummaryVegetationQualityPreferences(s *vprocore.DesktopConfig, ctx context.Context) (summaryVegetationQualityPreferenceValues, error) {
	if ctx == nil || s == nil {
		return summaryVegetationQualityPreferenceValues{}, errors.New("Summary Vegetation quality preferences require a context and configuration")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return summaryVegetationQualityPreferenceValues{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return summaryVegetationQualityPreferenceValues{}, err
	}
	var result summaryVegetationQualityPreferenceValues
	for _, field := range []struct {
		key    string
		target *string
	}{
		{"DataQualityFilterSite", &result.SiteMinimum}, {"DataQualityFilterVeg", &result.VegMinimum},
		{"DataQualityFilterSoil", &result.SoilMinimum}, {"DataQualityFilterBECUseLV", &result.BECMinimum},
	} {
		value, err := summaryVegetationQualityPreferenceOption(values, field.key)
		if err != nil {
			return summaryVegetationQualityPreferenceValues{}, err
		}
		text, ok := value.(string)
		if !ok {
			return summaryVegetationQualityPreferenceValues{}, fmt.Errorf("Summary Vegetation %s lost text storage", field.key)
		}
		*field.target = text
	}
	for _, field := range []struct {
		key    string
		target *bool
	}{
		{"DataQualityFilterEnforce", &result.Enforce},
		{"DataQualityFilterSiteNull", &result.SiteIncludeNulls}, {"DataQualityFilterVegNull", &result.VegIncludeNulls},
		{"DataQualityFilterSoilNull", &result.SoilIncludeNulls}, {"DataQualityFilterBECUse", &result.BECIncludeNulls},
	} {
		value, err := summaryVegetationQualityPreferenceOption(values, field.key)
		if err != nil {
			return summaryVegetationQualityPreferenceValues{}, err
		}
		flag, ok := value.(bool)
		if !ok {
			return summaryVegetationQualityPreferenceValues{}, fmt.Errorf("Summary Vegetation %s lost boolean storage", field.key)
		}
		*field.target = flag
	}
	if err := ctx.Err(); err != nil {
		return summaryVegetationQualityPreferenceValues{}, err
	}
	return result, nil
}

func CompareAndSetSummaryVegetationQualityPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed summaryVegetationQualityPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if expected.Enforce != proposed.Enforce {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation quality enforcement invokes mutating source Form_Load; changes remain unavailable")
	}
	changes := summaryVegetationQualityPreferenceChanges(expected, proposed)
	for _, change := range changes {
		if text, ok := change.Proposed.(string); ok && change.Expected != change.Proposed &&
			(!utf8.ValidString(text) || strings.ContainsRune(text, 0)) {
			return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Summary Vegetation quality %s requires complete Unicode without NUL; no repair or normalization", change.Key)
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Summary Vegetation quality preferences", changes, summaryVegetationQualityPreferenceOption)
}
