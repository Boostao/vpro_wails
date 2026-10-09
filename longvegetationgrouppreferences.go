package main

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationGroupOption(values configValues, key string) (int, error) {
	if key != "LVGroupBy" {
		return 0, fmt.Errorf("unsupported Long Vegetation grouping preference %s", key)
	}
	return configInt(values, "ReportOptions", key, 1, 4)
}

func (s *desktopConfig) readLongVegetationGroupPreferences(ctx context.Context) (LongVegetationGroupPreferenceValues, error) {
	fail := func(err error) (LongVegetationGroupPreferenceValues, error) {
		return LongVegetationGroupPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation grouping preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation grouping preference configuration is unavailable"))
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return fail(err)
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
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

func (s *desktopConfig) compareAndSetLongVegetationGroupPreferences(ctx context.Context,
	expected, proposed LongVegetationGroupPreferenceValues) (reportPreferenceUpdate, error) {
	for _, value := range []int{expected.GroupBy, proposed.GroupBy} {
		if value < 1 || value > 4 {
			return reportPreferenceUpdate{}, errors.New("Long Vegetation Group by requires Layer (1), Strata (2), Lifeform (3) or None (4)")
		}
	}
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation grouping preferences", []reportPreferenceChange[int]{
		{Key: "LVGroupBy", Expected: expected.GroupBy, Proposed: proposed.GroupBy},
	}, longVegetationGroupOption)
}
