package main

import (
	"context"
	"errors"
	"fmt"
	"math"
)

type summaryVegetationPreferenceValues struct {
	GroupBy    int
	Average    int
	Display    int
	ShowClass  int
	ValueLimit float64
}

func validateSummaryVegetationPreferenceValues(value summaryVegetationPreferenceValues) error {
	if value.GroupBy < 1 || value.GroupBy > 4 || value.Average < 1 || value.Average > 2 ||
		value.Display < 1 || value.Display > 6 || value.ShowClass != 1 && value.ShowClass != 10 && value.ShowClass != 20 ||
		math.IsNaN(value.ValueLimit) || math.IsInf(value.ValueLimit, 0) {
		return errors.New("Summary Vegetation preferences require original Group by, Calculate, Display, Show and finite historical limit values")
	}
	return nil
}

func summaryVegetationPreferenceOption(values configValues, key string) (any, error) {
	switch key {
	case "SVGroupBy":
		return configInt(values, "ReportOptions", key, 1, 4)
	case "SVAvgType":
		return configInt(values, "ReportOptions", key, 1, 2)
	case "SVValue":
		return configInt(values, "ReportOptions", key, 1, 6)
	case "SVShowClass":
		value, err := configInt(values, "ReportOptions", key, 1, 20)
		if err != nil {
			return nil, err
		}
		if value != 1 && value != 10 && value != 20 {
			return nil, errors.New("Summary Vegetation Show requires original1,10 or20")
		}
		return value, nil
	case "SVShowValOpt":
		return longVegetationConfigNumber(values, key)
	default:
		return nil, fmt.Errorf("unsupported Summary Vegetation preference %s", key)
	}
}

func (s *desktopConfig) readSummaryVegetationPreferences(ctx context.Context) (summaryVegetationPreferenceValues, error) {
	fail := func(err error) (summaryVegetationPreferenceValues, error) {
		return summaryVegetationPreferenceValues{}, err
	}
	if ctx == nil || s == nil {
		return fail(errors.New("Summary Vegetation preferences require a context and configuration"))
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return fail(err)
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return fail(err)
	}
	result := summaryVegetationPreferenceValues{}
	for _, field := range []struct {
		key    string
		target *int
	}{
		{"SVGroupBy", &result.GroupBy}, {"SVAvgType", &result.Average},
		{"SVValue", &result.Display}, {"SVShowClass", &result.ShowClass},
	} {
		value, err := summaryVegetationPreferenceOption(values, field.key)
		if err != nil {
			return fail(err)
		}
		number, ok := value.(int)
		if !ok {
			return fail(fmt.Errorf("Summary Vegetation %s lost integer storage", field.key))
		}
		*field.target = number
	}
	value, err := summaryVegetationPreferenceOption(values, "SVShowValOpt")
	if err != nil {
		return fail(err)
	}
	number, ok := value.(float64)
	if !ok {
		return fail(errors.New("Summary Vegetation SVShowValOpt lost numeric storage"))
	}
	result.ValueLimit = number
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

func (s *desktopConfig) compareAndSetSummaryVegetationPreferences(ctx context.Context,
	expected, proposed summaryVegetationPreferenceValues) (reportPreferenceUpdate, error) {
	for _, value := range []summaryVegetationPreferenceValues{expected, proposed} {
		if err := validateSummaryVegetationPreferenceValues(value); err != nil {
			return reportPreferenceUpdate{}, err
		}
	}
	if expected != proposed {
		if (proposed.GroupBy == 2 || proposed.GroupBy == 3) && proposed.Display == 2 {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation Strata/Lifeform Display2 has an original criteria type mismatch; saved selection unavailable")
		}
		if expected.ValueLimit != proposed.ValueLimit && proposed.ValueLimit != 0 {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation supports the original display callback's limit clear only; arbitrary limit edits unavailable")
		}
		if expected.Display != proposed.Display && proposed.Display != 1 && proposed.Display != 3 && proposed.ValueLimit != 0 {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation Display change must clear SVShowValOpt outside modes1/3")
		}
	}
	return compareAndSetReportPreferences(s, ctx, "Summary Vegetation preferences", []reportPreferenceChange[any]{
		{Key: "SVGroupBy", Expected: expected.GroupBy, Proposed: proposed.GroupBy},
		{Key: "SVAvgType", Expected: expected.Average, Proposed: proposed.Average},
		{Key: "SVValue", Expected: expected.Display, Proposed: proposed.Display},
		{Key: "SVShowClass", Expected: expected.ShowClass, Proposed: proposed.ShowClass},
		{Key: "SVShowValOpt", Expected: expected.ValueLimit, Proposed: proposed.ValueLimit},
	}, summaryVegetationPreferenceOption)
}
