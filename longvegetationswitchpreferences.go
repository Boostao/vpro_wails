package main

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationSwitchOption(values configValues, key string) (bool, error) {
	switch key {
	case "LVQuickReport", "LVSpaceBetweenGroups", "LVReportSummary", "LVConstantSppList":
		return longVegetationConfigBool(values, key)
	default:
		return false, fmt.Errorf("unsupported Long Vegetation switch preference %s", key)
	}
}

func (s *desktopConfig) readLongVegetationSwitchPreferences(ctx context.Context) (LongVegetationSwitchPreferenceValues, error) {
	fail := func(err error) (LongVegetationSwitchPreferenceValues, error) {
		return LongVegetationSwitchPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation switch preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation switch preference configuration is unavailable"))
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return fail(err)
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
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

func (s *desktopConfig) compareAndSetLongVegetationSwitchPreferences(ctx context.Context,
	expected, proposed LongVegetationSwitchPreferenceValues) (reportPreferenceUpdate, error) {
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation switch preferences", []reportPreferenceChange[bool]{
		{Key: "LVQuickReport", Expected: expected.QuickReport, Proposed: proposed.QuickReport},
		{Key: "LVSpaceBetweenGroups", Expected: expected.SpaceBetweenGroups, Proposed: proposed.SpaceBetweenGroups},
		{Key: "LVReportSummary", Expected: expected.ReportSummary, Proposed: proposed.ReportSummary},
		{Key: "LVConstantSppList", Expected: expected.ConstantSpeciesList, Proposed: proposed.ConstantSpeciesList},
	}, longVegetationSwitchOption)
}
