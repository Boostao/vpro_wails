package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type summaryVegetationPresentationPreferenceValues struct {
	Title           string
	ShowCommon      bool
	ShowGuide       bool
	HierarchyHeader bool
	IncludeEnv      bool
	CreateSummary   bool
	CombineVariants bool
	Constancy       bool
	GroupBy         int
	Display         int
	ValueLimit      float64
}

func summaryVegetationPresentationPreferenceOption(values configValues, key string) (any, error) {
	switch key {
	case "SVReportTitle":
		return configString(values, "ReportOptions", key)
	case "SVShowEnglishName", "SVShowGuideName", "SVShowHierarchyHeader", "SVIncludeEnvFields",
		"SVReportSummary", "SVSubSppAsSpp", "SVConstancyFormat":
		return longVegetationConfigBool(values, key)
	case "SVGroupBy", "SVValue", "SVShowValOpt":
		return summaryVegetationPreferenceOption(values, key)
	default:
		return nil, fmt.Errorf("unsupported Summary Vegetation presentation preference %s", key)
	}
}

func summaryVegetationPresentationPreferenceChanges(expected, proposed summaryVegetationPresentationPreferenceValues) []reportPreferenceChange[any] {
	return []reportPreferenceChange[any]{
		{Key: "SVReportTitle", Expected: expected.Title, Proposed: proposed.Title},
		{Key: "SVShowEnglishName", Expected: expected.ShowCommon, Proposed: proposed.ShowCommon},
		{Key: "SVShowGuideName", Expected: expected.ShowGuide, Proposed: proposed.ShowGuide},
		{Key: "SVShowHierarchyHeader", Expected: expected.HierarchyHeader, Proposed: proposed.HierarchyHeader},
		{Key: "SVIncludeEnvFields", Expected: expected.IncludeEnv, Proposed: proposed.IncludeEnv},
		{Key: "SVReportSummary", Expected: expected.CreateSummary, Proposed: proposed.CreateSummary},
		{Key: "SVSubSppAsSpp", Expected: expected.CombineVariants, Proposed: proposed.CombineVariants},
		{Key: "SVConstancyFormat", Expected: expected.Constancy, Proposed: proposed.Constancy},
		{Key: "SVGroupBy", Expected: expected.GroupBy, Proposed: proposed.GroupBy},
		{Key: "SVValue", Expected: expected.Display, Proposed: proposed.Display},
		{Key: "SVShowValOpt", Expected: expected.ValueLimit, Proposed: proposed.ValueLimit},
	}
}

func (s *desktopConfig) readSummaryVegetationPresentationPreferences(ctx context.Context) (summaryVegetationPresentationPreferenceValues, error) {
	if ctx == nil || s == nil {
		return summaryVegetationPresentationPreferenceValues{}, errors.New("Summary Vegetation presentation preferences require a context and configuration")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	var result summaryVegetationPresentationPreferenceValues
	result.Title, err = configString(values, "ReportOptions", "SVReportTitle")
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	for _, field := range []struct {
		key    string
		target *bool
	}{
		{"SVShowEnglishName", &result.ShowCommon}, {"SVShowGuideName", &result.ShowGuide},
		{"SVShowHierarchyHeader", &result.HierarchyHeader}, {"SVIncludeEnvFields", &result.IncludeEnv},
		{"SVReportSummary", &result.CreateSummary}, {"SVSubSppAsSpp", &result.CombineVariants},
		{"SVConstancyFormat", &result.Constancy},
	} {
		value, err := longVegetationConfigBool(values, field.key)
		if err != nil {
			return summaryVegetationPresentationPreferenceValues{}, err
		}
		*field.target = value
	}
	result.GroupBy, err = configInt(values, "ReportOptions", "SVGroupBy", 1, 4)
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	result.Display, err = configInt(values, "ReportOptions", "SVValue", 1, 6)
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	result.ValueLimit, err = longVegetationConfigNumber(values, "SVShowValOpt")
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	if err := ctx.Err(); err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	return result, nil
}

func (s *desktopConfig) compareAndSetSummaryVegetationPresentationPreferences(ctx context.Context,
	expected, proposed summaryVegetationPresentationPreferenceValues, constancyApplied bool) (reportPreferenceUpdate, error) {
	for _, value := range []summaryVegetationPresentationPreferenceValues{expected, proposed} {
		if value.GroupBy < 1 || value.GroupBy > 4 || value.Display < 1 || value.Display > 6 ||
			math.IsNaN(value.ValueLimit) || math.IsInf(value.ValueLimit, 0) {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation presentation requires original Group by1..4, Display1..6 and a finite historical limit")
		}
	}
	if expected.Title != proposed.Title && (!utf8.ValidString(proposed.Title) || strings.ContainsRune(proposed.Title, 0)) {
		return reportPreferenceUpdate{}, errors.New("Summary Vegetation title requires complete Unicode without NUL; no repair or normalization")
	}
	if expected.ValueLimit != proposed.ValueLimit {
		return reportPreferenceUpdate{}, errors.New("Summary Vegetation constancy callback selects Display1 without clearing the historical limit; limit edits unavailable")
	}
	if constancyApplied {
		if proposed.GroupBy != 3 || proposed.Display != 1 || proposed.Constancy && proposed.ShowCommon {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation constancy-on callback requires Group by3 and Display1; common names stay off while constancy is selected")
		}
	} else {
		if !expected.Constancy && proposed.Constancy {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation constancy-on requires its coupled callback plan")
		}
		if expected.GroupBy != proposed.GroupBy || expected.Display != proposed.Display {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation presentation cannot independently edit shared Group by or Display")
		}
		if proposed.Constancy && expected.ShowCommon != proposed.ShowCommon {
			return reportPreferenceUpdate{}, errors.New("Summary Vegetation common-name control is disabled while constancy is selected")
		}
	}
	return compareAndSetReportPreferences(s, ctx, "Summary Vegetation presentation preferences",
		summaryVegetationPresentationPreferenceChanges(expected, proposed), summaryVegetationPresentationPreferenceOption)
}
