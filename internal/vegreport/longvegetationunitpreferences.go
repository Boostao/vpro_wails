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
