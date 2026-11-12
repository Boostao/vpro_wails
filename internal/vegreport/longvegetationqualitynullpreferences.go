package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationQualityNullOption(values vprocore.ConfigValues, key string) (bool, error) {
	switch key {
	case "DataQualityFilterSiteNullLV", "DataQualityFilterVegNullLV", "DataQualityFilterSoilNullLV":
	default:
		return false, fmt.Errorf("unsupported Long Vegetation quality Include Nulls preference %s", key)
	}
	fields, err := vprocore.ConfigSection(values, "ReportOptions")
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

func ReadLongVegetationQualityNullPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationQualityNullPreferenceValues, error) {
	fail := func(err error) (LongVegetationQualityNullPreferenceValues, error) {
		return LongVegetationQualityNullPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation quality Include Nulls preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation quality Include Nulls preference configuration is unavailable"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
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

func CompareAndSetLongVegetationQualityNullPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationQualityNullPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation quality Include Nulls preferences", []vprocore.ReportPreferenceChange[bool]{
		{Key: "DataQualityFilterSiteNullLV", Expected: expected.SiteIncludeNulls, Proposed: proposed.SiteIncludeNulls},
		{Key: "DataQualityFilterVegNullLV", Expected: expected.VegIncludeNulls, Proposed: proposed.VegIncludeNulls},
		{Key: "DataQualityFilterSoilNullLV", Expected: expected.SoilIncludeNulls, Proposed: proposed.SoilIncludeNulls},
	}, longVegetationQualityNullOption)
}
