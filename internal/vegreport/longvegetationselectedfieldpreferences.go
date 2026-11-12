package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

func ReadLongVegetationSelectedFieldPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationSelectedFieldPreferenceValues, error) {
	values, err := ReadLongVegetationNamingPreferences(s, ctx)
	return LongVegetationSelectedFieldPreferenceValues(values), err
}

func CompareAndSetLongVegetationSelectedFieldPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationSelectedFieldPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if !longVegetationNamingMode(expected.Mode) || !longVegetationNamingMode(proposed.Mode) {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation selected field requires source mode 0, 1, 2 or 4")
	}
	if expected != proposed {
		if proposed.SelectedField != "" && proposed.Mode != 4 ||
			proposed.SelectedField == "" && proposed.Mode != expected.Mode && proposed.Mode != 4 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation nonempty selected field must set mode 4; clearing the field must retain the staged mode")
		}
		if expected.SelectedField != proposed.SelectedField &&
			(!utf8.ValidString(proposed.SelectedField) || strings.ContainsRune(proposed.SelectedField, 0)) {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation selected field requires complete Unicode without NUL; no repair or normalization")
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation selected field preferences", []vprocore.ReportPreferenceChange[any]{
		{Key: "LVShowEnglishName", Expected: expected.Mode, Proposed: proposed.Mode},
		{Key: "LVShowField", Expected: expected.SelectedField, Proposed: proposed.SelectedField},
	}, longVegetationNamingOption)
}
