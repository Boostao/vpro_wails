package main

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationUnitOption(values configValues, key string) (int, error) {
	if key != "LVUnitGroups" {
		return 0, fmt.Errorf("unsupported Long Vegetation report unit preference %s", key)
	}
	return configInt(values, "ReportOptions", key, 1, 3)
}

func (s *desktopConfig) readLongVegetationUnitPreferences(ctx context.Context) (LongVegetationUnitPreferenceValues, error) {
	fail := func(err error) (LongVegetationUnitPreferenceValues, error) {
		return LongVegetationUnitPreferenceValues{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation report unit preferences require a context"))
	}
	if s == nil {
		return fail(errors.New("Long Vegetation report unit preference configuration is unavailable"))
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return fail(err)
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
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

func (s *desktopConfig) compareAndSetLongVegetationUnitPreferences(ctx context.Context,
	expected, proposed LongVegetationUnitPreferenceValues) (reportPreferenceUpdate, error) {
	return s.compareAndSetReviewedLongVegetationUnitPreferences(ctx, expected, proposed, nil)
}

func (s *desktopConfig) compareAndSetReviewedLongVegetationUnitPreferences(ctx context.Context,
	expected, proposed LongVegetationUnitPreferenceValues, approvalGuard func(configValues) error) (reportPreferenceUpdate, error) {
	for _, value := range []int{expected.UnitGroups, proposed.UnitGroups} {
		if value < 1 || value > 3 {
			return reportPreferenceUpdate{}, errors.New("Long Vegetation report units require Site unit table (1), Selected fields (2) or None (3)")
		}
	}
	if proposed.UnitGroups == 2 && expected.UnitGroups != 2 && approvalGuard == nil {
		return reportPreferenceUpdate{}, errors.New("Long Vegetation Selected fields report units remain unavailable; saved LVUnitGroups was not changed")
	}
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation report unit preferences", []reportPreferenceChange[int]{
		{Key: "LVUnitGroups", Expected: expected.UnitGroups, Proposed: proposed.UnitGroups},
	}, func(values configValues, key string) (int, error) {
		if approvalGuard != nil {
			if err := approvalGuard(values); err != nil {
				return 0, err
			}
		}
		return longVegetationUnitOption(values, key)
	})
}
