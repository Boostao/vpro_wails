package main

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationQualityNullOption(values configValues, key string) (bool, error) {
	switch key {
	case "DataQualityFilterSiteNullLV", "DataQualityFilterVegNullLV", "DataQualityFilterSoilNullLV":
	default:
		return false, fmt.Errorf("unsupported Long Vegetation quality Include Nulls preference %s", key)
	}
	fields, err := configSection(values, "ReportOptions")
	if err != nil {
		return false, err
	}
	if literal, legacy := fields[key].(string); legacy {
		switch literal {
		case "True":
			return true, nil
		case "False":
			return false, nil
		default:
			return false, fmt.Errorf("ReportOptions.%s legacy string must be exactly True or False", key)
		}
	}
	return longVegetationConfigBool(values, key)
}

func (s *desktopConfig) readLongVegetationQualityNullPreferences(ctx context.Context) (LongVegetationQualityNullPreferenceValues, error) {
	fail := func(err error) (LongVegetationQualityNullPreferenceValues, error) {
		return LongVegetationQualityNullPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation quality Include Nulls preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation quality Include Nulls preference configuration is unavailable"))
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return fail(err)
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return fail(err)
	}
	var result LongVegetationQualityNullPreferenceValues
	for _, field := range []struct {
		key   string
		value *bool
	}{
		{"DataQualityFilterSiteNullLV", &result.SiteIncludeNulls},
		{"DataQualityFilterVegNullLV", &result.VegIncludeNulls},
		{"DataQualityFilterSoilNullLV", &result.SoilIncludeNulls},
	} {
		*field.value, err = longVegetationQualityNullOption(values, field.key)
		if err != nil {
			return fail(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

func (s *desktopConfig) compareAndSetLongVegetationQualityNullPreferences(ctx context.Context,
	expected, proposed LongVegetationQualityNullPreferenceValues) (reportPreferenceUpdate, error) {
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation quality Include Nulls preferences", []reportPreferenceChange[bool]{
		{Key: "DataQualityFilterSiteNullLV", Expected: expected.SiteIncludeNulls, Proposed: proposed.SiteIncludeNulls},
		{Key: "DataQualityFilterVegNullLV", Expected: expected.VegIncludeNulls, Proposed: proposed.VegIncludeNulls},
		{Key: "DataQualityFilterSoilNullLV", Expected: expected.SoilIncludeNulls, Proposed: proposed.SoilIncludeNulls},
	}, longVegetationQualityNullOption)
}
