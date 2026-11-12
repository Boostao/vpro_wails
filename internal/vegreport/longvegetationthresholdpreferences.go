package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"math"
)

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
