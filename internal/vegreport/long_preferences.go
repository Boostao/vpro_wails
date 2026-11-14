package vegreport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func ReadLongVegetationPreference(s *vprocore.DesktopConfig, ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("Long Vegetation preferences require a context")
	}
	if s == nil {
		return "", errors.New("Long Vegetation preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return "", err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return "", err
	}
	title, err := vprocore.ConfigString(values, "ReportOptions", "LVReportTitle")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return title, nil
}

func CompareAndSetLongVegetationPreference(s *vprocore.DesktopConfig, ctx context.Context, expected, proposed string) (vprocore.ConfigStringPreferenceUpdate, error) {
	if ctx == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Vegetation preference changes require a context")
	}
	if s == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Vegetation preference configuration is unavailable")
	}
	if expected != proposed && (!utf8.ValidString(proposed) || strings.ContainsRune(proposed, 0)) {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Vegetation title requires complete Unicode without NUL; no repair or normalization")
	}
	return s.CompareAndSetReportStrings(ctx, "Long Vegetation preferences", []vprocore.ReportStringPreferenceChange{
		{Key: "LVReportTitle", Expected: expected, Proposed: proposed},
	})
}

func longVegetationStatisticOption(values vprocore.ConfigValues, key string) (int, error) {
	switch key {
	case "LVAvgType", "LVOrderBy":
	default:
		return 0, fmt.Errorf("unsupported Long Vegetation statistic preference %s", key)
	}
	value, err := vprocore.ConfigInt(values, "ReportOptions", key, 10, 20)
	if err != nil {
		return 0, err
	}
	if value != 10 && value != 20 {
		return 0, fmt.Errorf("ReportOptions.%s requires source choice 10 or 20; no defaults applied", key)
	}
	return value, nil
}

func ReadLongVegetationStatisticsPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationStatisticsPreferenceValues, error) {
	fail := func(err error) (LongVegetationStatisticsPreferenceValues, error) {
		return LongVegetationStatisticsPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation statistics preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation statistics preference configuration is unavailable"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return fail(err)
	}
	average, err := longVegetationStatisticOption(values, "LVAvgType")
	if err != nil {
		return fail(err)
	}
	order, err := longVegetationStatisticOption(values, "LVOrderBy")
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return LongVegetationStatisticsPreferenceValues{AvgType: average, OrderBy: order}, nil
}

func CompareAndSetLongVegetationStatisticsPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationStatisticsPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if proposed.AvgType != 10 && proposed.AvgType != 20 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation Average requires By n Plots (10) or Characteristic (20)")
	}
	if proposed.OrderBy != 10 && proposed.OrderBy != 20 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation Order by requires Species (10) or Presence (20)")
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation statistics preferences", []vprocore.ReportPreferenceChange[int]{
		{Key: "LVAvgType", Expected: expected.AvgType, Proposed: proposed.AvgType},
		{Key: "LVOrderBy", Expected: expected.OrderBy, Proposed: proposed.OrderBy},
	}, longVegetationStatisticOption)
}

func longVegetationSwitchOption(values vprocore.ConfigValues, key string) (bool, error) {
	switch key {
	case "LVQuickReport", "LVSpaceBetweenGroups", "LVReportSummary", "LVConstantSppList":
		return longVegetationConfigBool(values, key)
	default:
		return false, fmt.Errorf("unsupported Long Vegetation switch preference %s", key)
	}
}

func ReadLongVegetationSwitchPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationSwitchPreferenceValues, error) {
	fail := func(err error) (LongVegetationSwitchPreferenceValues, error) {
		return LongVegetationSwitchPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation switch preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation switch preference configuration is unavailable"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return fail(err)
	}
	var result LongVegetationSwitchPreferenceValues
	for _, field := range []struct {
		key   string
		value *bool
	}{
		{"LVQuickReport", &result.QuickReport}, {"LVSpaceBetweenGroups", &result.SpaceBetweenGroups},
		{"LVReportSummary", &result.ReportSummary}, {"LVConstantSppList", &result.ConstantSpeciesList},
	} {
		*field.value, err = longVegetationSwitchOption(values, field.key)
		if err != nil {
			return fail(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

func CompareAndSetLongVegetationSwitchPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationSwitchPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation switch preferences", []vprocore.ReportPreferenceChange[bool]{
		{Key: "LVQuickReport", Expected: expected.QuickReport, Proposed: proposed.QuickReport},
		{Key: "LVSpaceBetweenGroups", Expected: expected.SpaceBetweenGroups, Proposed: proposed.SpaceBetweenGroups},
		{Key: "LVReportSummary", Expected: expected.ReportSummary, Proposed: proposed.ReportSummary},
		{Key: "LVConstantSppList", Expected: expected.ConstantSpeciesList, Proposed: proposed.ConstantSpeciesList},
	}, longVegetationSwitchOption)
}

func longVegetationThresholdOption(values vprocore.ConfigValues, key string) (float64, error) {
	switch key {
	case "LVPresenceGreaterThan", "LVCoverGreaterThan":
		return longVegetationConfigNumber(values, key)
	default:
		return 0, fmt.Errorf("unsupported Long Vegetation threshold preference %s", key)
	}
}

func ReadLongVegetationThresholdPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationThresholdPreferenceValues, error) {
	fail := func(err error) (LongVegetationThresholdPreferenceValues, error) {
		return LongVegetationThresholdPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation threshold preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation threshold preference configuration is unavailable"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return fail(err)
	}
	presence, err := longVegetationThresholdOption(values, "LVPresenceGreaterThan")
	if err != nil {
		return fail(err)
	}
	cover, err := longVegetationThresholdOption(values, "LVCoverGreaterThan")
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return LongVegetationThresholdPreferenceValues{PresenceGreaterThan: presence, MeanCoverGreaterThan: cover}, nil
}

func CompareAndSetLongVegetationThresholdPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationThresholdPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	for _, values := range []LongVegetationThresholdPreferenceValues{expected, proposed} {
		for _, field := range []struct {
			name  string
			value float64
		}{{"presence", values.PresenceGreaterThan}, {"mean cover", values.MeanCoverGreaterThan}} {
			if math.IsNaN(field.value) || math.IsInf(field.value, 0) {
				return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Long Vegetation %s threshold must be finite", field.name)
			}
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation threshold preferences", []vprocore.ReportPreferenceChange[float64]{
		{Key: "LVPresenceGreaterThan", Expected: expected.PresenceGreaterThan, Proposed: proposed.PresenceGreaterThan},
		{Key: "LVCoverGreaterThan", Expected: expected.MeanCoverGreaterThan, Proposed: proposed.MeanCoverGreaterThan},
	}, longVegetationThresholdOption)
}

func longVegetationGroupOption(values vprocore.ConfigValues, key string) (int, error) {
	if key != "LVGroupBy" {
		return 0, fmt.Errorf("unsupported Long Vegetation grouping preference %s", key)
	}
	return vprocore.ConfigInt(values, "ReportOptions", key, 1, 4)
}

func ReadLongVegetationGroupPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationGroupPreferenceValues, error) {
	fail := func(err error) (LongVegetationGroupPreferenceValues, error) {
		return LongVegetationGroupPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation grouping preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation grouping preference configuration is unavailable"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return fail(err)
	}
	group, err := longVegetationGroupOption(values, "LVGroupBy")
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return LongVegetationGroupPreferenceValues{GroupBy: group}, nil
}

func CompareAndSetLongVegetationGroupPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationGroupPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	for _, value := range []int{expected.GroupBy, proposed.GroupBy} {
		if value < 1 || value > 4 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation Group by requires Layer (1), Strata (2), Lifeform (3) or None (4)")
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation grouping preferences", []vprocore.ReportPreferenceChange[int]{
		{Key: "LVGroupBy", Expected: expected.GroupBy, Proposed: proposed.GroupBy},
	}, longVegetationGroupOption)
}

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

func longVegetationNamingMode(mode int) bool {
	return mode == 0 || mode == 1 || mode == 2 || mode == 4
}

func longVegetationNamingOption(values vprocore.ConfigValues, key string) (any, error) {
	switch key {
	case "LVShowEnglishName":
		mode, err := vprocore.ConfigInt(values, "ReportOptions", key, 0, 4)
		if err != nil {
			return nil, err
		}
		if !longVegetationNamingMode(mode) {
			return nil, errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
		}
		return mode, nil
	case "LVShowField":
		return vprocore.ConfigString(values, "ReportOptions", key)
	default:
		return nil, fmt.Errorf("unsupported Long Vegetation Show preference %s", key)
	}
}

func ReadLongVegetationNamingPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationNamingPreferenceValues, error) {
	if ctx == nil {
		return LongVegetationNamingPreferenceValues{}, errors.New("Long Vegetation Show preferences require a context")
	}
	if s == nil {
		return LongVegetationNamingPreferenceValues{}, errors.New("Long Vegetation Show configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	mode, err := vprocore.ConfigInt(values, "ReportOptions", "LVShowEnglishName", 0, 4)
	if err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	if !longVegetationNamingMode(mode) {
		return LongVegetationNamingPreferenceValues{}, errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
	}
	field, err := vprocore.ConfigString(values, "ReportOptions", "LVShowField")
	if err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationNamingPreferenceValues{}, err
	}
	return LongVegetationNamingPreferenceValues{Mode: mode, SelectedField: field}, nil
}

func CompareAndSetLongVegetationNamingPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationNamingPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if !longVegetationNamingMode(expected.Mode) || !longVegetationNamingMode(proposed.Mode) {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
	}
	if expected != proposed {
		if proposed.Mode == 4 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation selected-field mode remains unavailable; its historical no-op is preserved")
		}
		if expected.Mode != proposed.Mode && proposed.SelectedField != "" ||
			expected.SelectedField != proposed.SelectedField && proposed.SelectedField != "" {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation Show selection must clear LVShowField; arbitrary selected-field assignments are unavailable")
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation Show preferences", []vprocore.ReportPreferenceChange[any]{
		{Key: "LVShowEnglishName", Expected: expected.Mode, Proposed: proposed.Mode},
		{Key: "LVShowField", Expected: expected.SelectedField, Proposed: proposed.SelectedField},
	}, longVegetationNamingOption)
}

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

func longVegetationEnforcementOption(values vprocore.ConfigValues, key string) (any, error) {
	switch key {
	case "DataQualityFilterEnforceLV":
		return longVegetationConfigBool(values, key)
	case "LVUnitGroups":
		return vprocore.ConfigInt(values, "ReportOptions", key, 1, 3)
	default:
		return nil, fmt.Errorf("unsupported Long Vegetation enforcement preference %s", key)
	}
}

func ReadLongVegetationEnforcementPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationEnforcementPreferenceValues, error) {
	if ctx == nil {
		return LongVegetationEnforcementPreferenceValues{}, errors.New("Long Vegetation enforcement preferences require a context")
	}
	if s == nil {
		return LongVegetationEnforcementPreferenceValues{}, errors.New("Long Vegetation enforcement preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	enforce, err := longVegetationConfigBool(values, "DataQualityFilterEnforceLV")
	if err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	unitGroups, err := vprocore.ConfigInt(values, "ReportOptions", "LVUnitGroups", 1, 3)
	if err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	return LongVegetationEnforcementPreferenceValues{enforce, unitGroups}, nil
}

func CompareAndSetLongVegetationEnforcementPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationEnforcementPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if expected.UnitGroups < 1 || expected.UnitGroups > 3 || proposed.UnitGroups < 1 || proposed.UnitGroups > 3 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation enforcement requires source unit-group 1, 2 or 3")
	}
	if expected != proposed && proposed.UnitGroups != 1 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation enforcement callback must reset unit groups to 1; selected-field/None unit-group changes are unavailable")
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation enforcement preferences", []vprocore.ReportPreferenceChange[any]{
		{Key: "DataQualityFilterEnforceLV", Expected: expected.Enforce, Proposed: proposed.Enforce},
		{Key: "LVUnitGroups", Expected: expected.UnitGroups, Proposed: proposed.UnitGroups},
	}, longVegetationEnforcementOption)
}

func longVegetationUnitOption(values vprocore.ConfigValues, key string) (int, error) {
	if key != "LVUnitGroups" {
		return 0, fmt.Errorf("unsupported Long Vegetation report unit preference %s", key)
	}
	return vprocore.ConfigInt(values, "ReportOptions", key, 1, 3)
}

func ReadLongVegetationUnitPreferences(s *vprocore.DesktopConfig, ctx context.Context) (LongVegetationUnitPreferenceValues, error) {
	fail := func(err error) (LongVegetationUnitPreferenceValues, error) {
		return LongVegetationUnitPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation report unit preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation report unit preference configuration is unavailable"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return fail(err)
	}
	group, err := longVegetationUnitOption(values, "LVUnitGroups")
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return LongVegetationUnitPreferenceValues{UnitGroups: group}, nil
}

func CompareAndSetLongVegetationUnitPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationUnitPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	return CompareAndSetReviewedLongVegetationUnitPreferences(s, ctx, expected, proposed, nil)
}

func CompareAndSetReviewedLongVegetationUnitPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed LongVegetationUnitPreferenceValues, approvalGuard func(vprocore.ConfigValues) error) (vprocore.ReportPreferenceUpdate, error) {
	for _, value := range []int{expected.UnitGroups, proposed.UnitGroups} {
		if value < 1 || value > 3 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation report units require Site unit table (1), Selected fields (2) or None (3)")
		}
	}
	if proposed.UnitGroups == 2 && expected.UnitGroups != 2 && approvalGuard == nil {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Long Vegetation Selected fields report units remain unavailable; saved LVUnitGroups was not changed")
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Long Vegetation report unit preferences", []vprocore.ReportPreferenceChange[int]{
		{Key: "LVUnitGroups", Expected: expected.UnitGroups, Proposed: proposed.UnitGroups},
	}, func(values vprocore.ConfigValues, key string) (int, error) {
		if approvalGuard != nil {
			if err := approvalGuard(values); err != nil {
				return 0, err
			}
		}
		return longVegetationUnitOption(values, key)
	})
}
