package main

import (
	"context"
	"errors"
	"fmt"
)

func longVegetationEnforcementOption(values configValues, key string) (any, error) {
	switch key {
	case "DataQualityFilterEnforceLV":
		return longVegetationConfigBool(values, key)
	case "LVUnitGroups":
		return configInt(values, "ReportOptions", key, 1, 3)
	default:
		return nil, fmt.Errorf("unsupported Long Vegetation enforcement preference %s", key)
	}
}

func (s *desktopConfig) readLongVegetationEnforcementPreferences(ctx context.Context) (LongVegetationEnforcementPreferenceValues, error) {
	if ctx == nil {
		return LongVegetationEnforcementPreferenceValues{}, errors.New("Long Vegetation enforcement preferences require a context")
	}
	if s == nil {
		return LongVegetationEnforcementPreferenceValues{}, errors.New("Long Vegetation enforcement preference configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	enforce, err := longVegetationConfigBool(values, "DataQualityFilterEnforceLV")
	if err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	unitGroups, err := configInt(values, "ReportOptions", "LVUnitGroups", 1, 3)
	if err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationEnforcementPreferenceValues{}, err
	}
	return LongVegetationEnforcementPreferenceValues{enforce, unitGroups}, nil
}

func (s *desktopConfig) compareAndSetLongVegetationEnforcementPreferences(ctx context.Context,
	expected, proposed LongVegetationEnforcementPreferenceValues) (reportPreferenceUpdate, error) {
	if expected.UnitGroups < 1 || expected.UnitGroups > 3 || proposed.UnitGroups < 1 || proposed.UnitGroups > 3 {
		return reportPreferenceUpdate{}, errors.New("Long Vegetation enforcement requires source unit-group 1, 2 or 3")
	}
	if expected != proposed && proposed.UnitGroups != 1 {
		return reportPreferenceUpdate{}, errors.New("Long Vegetation enforcement callback must reset unit groups to 1; selected-field/None unit-group changes are unavailable")
	}
	return compareAndSetReportPreferences(s, ctx, "Long Vegetation enforcement preferences", []reportPreferenceChange[any]{
		{Key: "DataQualityFilterEnforceLV", Expected: expected.Enforce, Proposed: proposed.Enforce},
		{Key: "LVUnitGroups", Expected: expected.UnitGroups, Proposed: proposed.UnitGroups},
	}, longVegetationEnforcementOption)
}
