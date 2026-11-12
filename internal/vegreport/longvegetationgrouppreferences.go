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
