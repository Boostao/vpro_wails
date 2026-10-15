package main

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

func (s *desktopConfig) readLongVegetationSelectedFieldPreferences(ctx context.Context) (LongVegetationSelectedFieldPreferenceValues, error) {
	values, err := s.readLongVegetationNamingPreferences(ctx)
	return LongVegetationSelectedFieldPreferenceValues(values), err
}

func (s *desktopConfig) compareAndSetLongVegetationSelectedFieldPreferences(ctx context.Context,
	expected, proposed LongVegetationSelectedFieldPreferenceValues) (reportPreferenceUpdate, error) {
	if !longVegetationNamingMode(expected.Mode) || !longVegetationNamingMode(proposed.Mode) {
		return reportPreferenceUpdate{}, errors.New("Long Vegetation selected field requires source mode 0, 1, 2 or 4")
	}
	if expected != proposed {
		if proposed.SelectedField != "" && proposed.Mode != 4 ||
			proposed.SelectedField == "" && proposed.Mode != expected.Mode && proposed.Mode != 4 {
			return reportPreferenceUpdate{}, errors.New("Long Vegetation nonempty selected field must set mode 4; clearing the field must retain the staged mode")
		}
		if expected.SelectedField != proposed.SelectedField &&
			(!utf8.ValidString(proposed.SelectedField) || strings.ContainsRune(proposed.SelectedField, 0)) {
			return reportPreferenceUpdate{}, errors.New("Long Vegetation selected field requires complete Unicode without NUL; no repair or normalization")
		}
	}
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation selected field preferences", []reportPreferenceChange[any]{
		{Key: "LVShowEnglishName", Expected: expected.Mode, Proposed: proposed.Mode},
		{Key: "LVShowField", Expected: expected.SelectedField, Proposed: proposed.SelectedField},
	}, longVegetationNamingOption)
}
