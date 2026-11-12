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
