package main

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationNamingMode(mode int) bool {
	return mode == 0 || mode == 1 || mode == 2 || mode == 4
}

func longVegetationNamingOption(values configValues, key string) (any, error) {
	switch key {
	case "LVShowEnglishName":
		mode, err := configInt(values, "ReportOptions", key, 0, 4)
		if err != nil {
			return nil, err
		}
		if !longVegetationNamingMode(mode) {
			return nil, errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
		}
		return mode, nil
	case "LVShowField":
		return configString(values, "ReportOptions", key)
	default:
		return nil, fmt.Errorf("unsupported Long Vegetation Show preference %s", key)
	}
}

func (s *desktopConfig) readLongVegetationNamingPreferences(ctx context.Context) (LongVegetationNamingPreferenceValues, error) {
	if ctx == nil {
		return LongVegetationNamingPreferenceValues{}, errors.New("Long Vegetation Show preferences require a context")
	}
	if s == nil {
		return LongVegetationNamingPreferenceValues{}, errors.New("Long Vegetation Show configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	mode, err := configInt(values, "ReportOptions", "LVShowEnglishName", 0, 4)
	if err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	if !longVegetationNamingMode(mode) {
		return LongVegetationNamingPreferenceValues{}, errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
	}
	field, err := configString(values, "ReportOptions", "LVShowField")
	if err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	return LongVegetationNamingPreferenceValues{Mode: mode, SelectedField: field}, nil
}

func (s *desktopConfig) compareAndSetLongVegetationNamingPreferences(ctx context.Context,
	expected, proposed LongVegetationNamingPreferenceValues) (reportPreferenceUpdate, error) {
	if !longVegetationNamingMode(expected.Mode) || !longVegetationNamingMode(proposed.Mode) {
		return reportPreferenceUpdate{}, errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
	}
	if expected != proposed {
		if proposed.Mode == 4 {
			return reportPreferenceUpdate{}, errors.New("Long Vegetation selected-field mode remains unavailable; its historical no-op is preserved")
		}
		if expected.Mode != proposed.Mode && proposed.SelectedField != "" ||
			expected.SelectedField != proposed.SelectedField && proposed.SelectedField != "" {
			return reportPreferenceUpdate{}, errors.New("Long Vegetation Show selection must clear LVShowField; arbitrary selected-field assignments are unavailable")
		}
	}
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation Show preferences", []reportPreferenceChange[any]{
		{Key: "LVShowEnglishName", Expected: expected.Mode, Proposed: proposed.Mode},
		{Key: "LVShowField", Expected: expected.SelectedField, Proposed: proposed.SelectedField},
	}, longVegetationNamingOption)
}
