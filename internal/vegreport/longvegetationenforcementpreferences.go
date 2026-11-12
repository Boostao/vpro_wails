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
