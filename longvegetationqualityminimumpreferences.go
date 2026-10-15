package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func longVegetationQualityMinimumOption(values configValues, key string) (string, error) {
	switch key {
	case "DataQualityFilterSiteLV", "DataQualityFilterVegLV", "DataQualityFilterSoilLV":
		return configString(values, "ReportOptions", key)
	default:
		return "", fmt.Errorf("unsupported Long Vegetation quality minimum preference %s", key)
	}
}

func (s *desktopConfig) readLongVegetationQualityMinimumPreferences(ctx context.Context) (LongVegetationQualityMinimumPreferenceValues, error) {
	if ctx == nil {
		return LongVegetationQualityMinimumPreferenceValues{}, errors.New("Long Vegetation quality minimum preferences require a context")
	}
	if s == nil {
		return LongVegetationQualityMinimumPreferenceValues{}, errors.New("Long Vegetation quality minimum configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return LongVegetationQualityMinimumPreferenceValues{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
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

func (s *desktopConfig) compareAndSetLongVegetationQualityMinimumPreferences(ctx context.Context,
	expected, proposed LongVegetationQualityMinimumPreferenceValues) (reportPreferenceUpdate, error) {
	changes := []reportStringPreferenceChange{
		{Key: "DataQualityFilterSiteLV", Expected: expected.SiteMinimum, Proposed: proposed.SiteMinimum},
		{Key: "DataQualityFilterVegLV", Expected: expected.VegMinimum, Proposed: proposed.VegMinimum},
		{Key: "DataQualityFilterSoilLV", Expected: expected.SoilMinimum, Proposed: proposed.SoilMinimum},
	}
	for _, change := range changes {
		if change.Expected != change.Proposed && (!utf8.ValidString(change.Proposed) || strings.ContainsRune(change.Proposed, 0)) {
			return reportPreferenceUpdate{}, fmt.Errorf("Long Vegetation quality minimum %s requires complete Unicode without NUL; no repair or normalization", change.Key)
		}
	}
	return s.compareAndSetReportStrings(ctx, "Long Vegetation quality minimum preferences", changes)
}
