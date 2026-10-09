package main

import (
	"context"
	"errors"
	"fmt"
	"math"
)

func longVegetationThresholdOption(values configValues, key string) (float64, error) {
	switch key {
	case "LVPresenceGreaterThan", "LVCoverGreaterThan":
		return longVegetationConfigNumber(values, key)
	default:
		return 0, fmt.Errorf("unsupported Long Vegetation threshold preference %s", key)
	}
}

func (s *desktopConfig) readLongVegetationThresholdPreferences(ctx context.Context) (LongVegetationThresholdPreferenceValues, error) {
	fail := func(err error) (LongVegetationThresholdPreferenceValues, error) {
		return LongVegetationThresholdPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation threshold preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation threshold preference configuration is unavailable"))
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return fail(err)
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
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

func (s *desktopConfig) compareAndSetLongVegetationThresholdPreferences(ctx context.Context,
	expected, proposed LongVegetationThresholdPreferenceValues) (reportPreferenceUpdate, error) {
	for _, values := range []LongVegetationThresholdPreferenceValues{expected, proposed} {
		for _, field := range []struct {
			name  string
			value float64
		}{{"presence", values.PresenceGreaterThan}, {"mean cover", values.MeanCoverGreaterThan}} {
			if math.IsNaN(field.value) || math.IsInf(field.value, 0) {
				return reportPreferenceUpdate{}, fmt.Errorf("Long Vegetation %s threshold must be finite", field.name)
			}
		}
	}
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation threshold preferences", []reportPreferenceChange[float64]{
		{Key: "LVPresenceGreaterThan", Expected: expected.PresenceGreaterThan, Proposed: proposed.PresenceGreaterThan},
		{Key: "LVCoverGreaterThan", Expected: expected.MeanCoverGreaterThan, Proposed: proposed.MeanCoverGreaterThan},
	}, longVegetationThresholdOption)
}
