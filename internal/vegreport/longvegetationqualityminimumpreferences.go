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

func longVegetationQualityMinimumOption(values vprocore.ConfigValues, key string) (string, error) {
	switch key {
	case "DataQualityFilterSiteLV", "DataQualityFilterVegLV", "DataQualityFilterSoilLV":
		return vprocore.ConfigString(values, "ReportOptions", key)
	default:
		return "", fmt.Errorf("unsupported Long Vegetation quality minimum preference %s", key)
	}
}

func ReadLongVegetationQualityMinimumPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationQualityMinimumPreferenceValues, error) {
	if ctx == nil {
		return LongVegetationQualityMinimumPreferenceValues{}, errors.New("Long Vegetation quality minimum preferences require a context")
	}
	if s == nil {
		return LongVegetationQualityMinimumPreferenceValues{}, errors.New("Long Vegetation quality minimum configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return LongVegetationQualityMinimumPreferenceValues{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return LongVegetationQualityMinimumPreferenceValues{}, err
	}
	var result LongVegetationQualityMinimumPreferenceValues
	for _, field := range []struct {
		key   string
		value *string
	}{
		{"DataQualityFilterSiteLV", &result.SiteMinimum},
		{"DataQualityFilterVegLV", &result.VegMinimum},
		{"DataQualityFilterSoilLV", &result.SoilMinimum},
	} {
		*field.value, err = longVegetationQualityMinimumOption(values, field.key)
		if err != nil {
			return LongVegetationQualityMinimumPreferenceValues{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationQualityMinimumPreferenceValues{}, err
	}
	return result, nil
}

func CompareAndSetLongVegetationQualityMinimumPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationQualityMinimumPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	changes := []vprocore.ReportStringPreferenceChange{
		{Key: "DataQualityFilterSiteLV", Expected: expected.SiteMinimum, Proposed: proposed.SiteMinimum},
		{Key: "DataQualityFilterVegLV", Expected: expected.VegMinimum, Proposed: proposed.VegMinimum},
		{Key: "DataQualityFilterSoilLV", Expected: expected.SoilMinimum, Proposed: proposed.SoilMinimum},
	}
	for _, change := range changes {
		if change.Expected != change.Proposed && (!utf8.ValidString(change.Proposed) || strings.ContainsRune(change.Proposed, 0)) {
			return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Long Vegetation quality minimum %s requires complete Unicode without NUL; no repair or normalization", change.Key)
		}
	}
	return s.CompareAndSetReportStrings(ctx, "Long Vegetation quality minimum preferences", changes)
}
