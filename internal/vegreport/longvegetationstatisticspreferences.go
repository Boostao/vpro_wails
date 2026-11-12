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
