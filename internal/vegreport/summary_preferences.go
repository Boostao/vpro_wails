package vegreport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
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

func summaryVegetationPreferenceOption(values vprocore.ConfigValues, key string) (any, error) {
	switch key {
	case "SVGroupBy":
		return vprocore.ConfigInt(values, "ReportOptions", key, 1, 4)
	case "SVAvgType":
		return vprocore.ConfigInt(values, "ReportOptions", key, 1, 2)
	case "SVValue":
		return vprocore.ConfigInt(values, "ReportOptions", key, 1, 6)
	case "SVShowClass":
		value, err := vprocore.ConfigInt(values, "ReportOptions", key, 1, 20)
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

func ReadSummaryVegetationPreferences(s *vprocore.DesktopConfig, ctx context.Context) (summaryVegetationPreferenceValues, error) {
	fail := func(err error) (summaryVegetationPreferenceValues, error) {
		return summaryVegetationPreferenceValues{}, err
	}
	if ctx == nil || s == nil {
		return fail(errors.New("Summary Vegetation preferences require a context and configuration"))
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return fail(err)
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
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

func CompareAndSetSummaryVegetationPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed summaryVegetationPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	for _, value := range []summaryVegetationPreferenceValues{expected, proposed} {
		if err := validateSummaryVegetationPreferenceValues(value); err != nil {
			return vprocore.ReportPreferenceUpdate{}, err
		}
	}
	if expected != proposed {
		if (proposed.GroupBy == 2 || proposed.GroupBy == 3) && proposed.Display == 2 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation Strata/Lifeform Display2 has an original criteria type mismatch; saved selection unavailable")
		}
		if expected.ValueLimit != proposed.ValueLimit && proposed.ValueLimit != 0 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation supports the original display callback's limit clear only; arbitrary limit edits unavailable")
		}
		if expected.Display != proposed.Display && proposed.Display != 1 && proposed.Display != 3 && proposed.ValueLimit != 0 {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation Display change must clear SVShowValOpt outside modes1/3")
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Summary Vegetation preferences", []vprocore.ReportPreferenceChange[any]{
		{Key: "SVGroupBy", Expected: expected.GroupBy, Proposed: proposed.GroupBy},
		{Key: "SVAvgType", Expected: expected.Average, Proposed: proposed.Average},
		{Key: "SVValue", Expected: expected.Display, Proposed: proposed.Display},
		{Key: "SVShowClass", Expected: expected.ShowClass, Proposed: proposed.ShowClass},
		{Key: "SVShowValOpt", Expected: expected.ValueLimit, Proposed: proposed.ValueLimit},
	}, summaryVegetationPreferenceOption)
}

type summaryVegetationQualityPreferenceValues struct {
	Enforce          bool
	SiteMinimum      string
	VegMinimum       string
	SoilMinimum      string
	BECMinimum       string
	SiteIncludeNulls bool
	VegIncludeNulls  bool
	SoilIncludeNulls bool
	BECIncludeNulls  bool
}

func summaryVegetationQualityPreferenceOption(values vprocore.ConfigValues, key string) (any, error) {
	switch key {
	case "DataQualityFilterSite", "DataQualityFilterVeg", "DataQualityFilterSoil", "DataQualityFilterBECUseLV":
		value, err := vprocore.ConfigString(values, "ReportOptions", key)
		if err != nil {
			return nil, err
		}
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("ReportOptions.%s contains malformed Unicode", key)
		}
		return value, nil
	case "DataQualityFilterEnforce":
		return longVegetationConfigBool(values, key)
	case "DataQualityFilterSiteNull", "DataQualityFilterVegNull", "DataQualityFilterSoilNull", "DataQualityFilterBECUse":
		fields, err := vprocore.ConfigSection(values, "ReportOptions")
		if err != nil {
			return nil, err
		}
		if literal, ok := fields[key].(string); ok {
			switch literal {
			case "True":
				return true, nil
			case "False":
				return false, nil
			default:
				return nil, fmt.Errorf("ReportOptions.%s legacy string must be exactly True or False", key)
			}
		}
		if key == "DataQualityFilterBECUse" {
			if number, ok := fields[key].(int); ok && number == 1 {
				return true, nil
			}
		}
		return longVegetationConfigBool(values, key)
	default:
		return nil, fmt.Errorf("unsupported Summary Vegetation quality preference %s", key)
	}
}

func summaryVegetationQualityPreferenceChanges(expected, proposed summaryVegetationQualityPreferenceValues) []vprocore.ReportPreferenceChange[any] {
	return []vprocore.ReportPreferenceChange[any]{
		{Key: "DataQualityFilterEnforce", Expected: expected.Enforce, Proposed: proposed.Enforce},
		{Key: "DataQualityFilterSite", Expected: expected.SiteMinimum, Proposed: proposed.SiteMinimum},
		{Key: "DataQualityFilterVeg", Expected: expected.VegMinimum, Proposed: proposed.VegMinimum},
		{Key: "DataQualityFilterSoil", Expected: expected.SoilMinimum, Proposed: proposed.SoilMinimum},
		{Key: "DataQualityFilterBECUseLV", Expected: expected.BECMinimum, Proposed: proposed.BECMinimum},
		{Key: "DataQualityFilterSiteNull", Expected: expected.SiteIncludeNulls, Proposed: proposed.SiteIncludeNulls},
		{Key: "DataQualityFilterVegNull", Expected: expected.VegIncludeNulls, Proposed: proposed.VegIncludeNulls},
		{Key: "DataQualityFilterSoilNull", Expected: expected.SoilIncludeNulls, Proposed: proposed.SoilIncludeNulls},
		{Key: "DataQualityFilterBECUse", Expected: expected.BECIncludeNulls, Proposed: proposed.BECIncludeNulls},
	}
}

func ReadSummaryVegetationQualityPreferences(s *vprocore.DesktopConfig, ctx context.Context) (summaryVegetationQualityPreferenceValues, error) {
	if ctx == nil || s == nil {
		return summaryVegetationQualityPreferenceValues{}, errors.New("Summary Vegetation quality preferences require a context and configuration")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return summaryVegetationQualityPreferenceValues{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return summaryVegetationQualityPreferenceValues{}, err
	}
	var result summaryVegetationQualityPreferenceValues
	for _, field := range []struct {
		key    string
		target *string
	}{
		{"DataQualityFilterSite", &result.SiteMinimum}, {"DataQualityFilterVeg", &result.VegMinimum},
		{"DataQualityFilterSoil", &result.SoilMinimum}, {"DataQualityFilterBECUseLV", &result.BECMinimum},
	} {
		value, err := summaryVegetationQualityPreferenceOption(values, field.key)
		if err != nil {
			return summaryVegetationQualityPreferenceValues{}, err
		}
		text, ok := value.(string)
		if !ok {
			return summaryVegetationQualityPreferenceValues{}, fmt.Errorf("Summary Vegetation %s lost text storage", field.key)
		}
		*field.target = text
	}
	for _, field := range []struct {
		key    string
		target *bool
	}{
		{"DataQualityFilterEnforce", &result.Enforce},
		{"DataQualityFilterSiteNull", &result.SiteIncludeNulls}, {"DataQualityFilterVegNull", &result.VegIncludeNulls},
		{"DataQualityFilterSoilNull", &result.SoilIncludeNulls}, {"DataQualityFilterBECUse", &result.BECIncludeNulls},
	} {
		value, err := summaryVegetationQualityPreferenceOption(values, field.key)
		if err != nil {
			return summaryVegetationQualityPreferenceValues{}, err
		}
		flag, ok := value.(bool)
		if !ok {
			return summaryVegetationQualityPreferenceValues{}, fmt.Errorf("Summary Vegetation %s lost boolean storage", field.key)
		}
		*field.target = flag
	}
	if err := ctx.Err(); err != nil {
		return summaryVegetationQualityPreferenceValues{}, err
	}
	return result, nil
}

func CompareAndSetSummaryVegetationQualityPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed summaryVegetationQualityPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if expected.Enforce != proposed.Enforce {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation quality enforcement invokes mutating source Form_Load; changes remain unavailable")
	}
	changes := summaryVegetationQualityPreferenceChanges(expected, proposed)
	for _, change := range changes {
		if text, ok := change.Proposed.(string); ok && change.Expected != change.Proposed &&
			(!utf8.ValidString(text) || strings.ContainsRune(text, 0)) {
			return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Summary Vegetation quality %s requires complete Unicode without NUL; no repair or normalization", change.Key)
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Summary Vegetation quality preferences", changes, summaryVegetationQualityPreferenceOption)
}

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

func summaryVegetationPresentationPreferenceOption(values vprocore.ConfigValues, key string) (any, error) {
	switch key {
	case "SVReportTitle":
		return vprocore.ConfigString(values, "ReportOptions", key)
	case "SVShowEnglishName", "SVShowGuideName", "SVShowHierarchyHeader", "SVIncludeEnvFields",
		"SVReportSummary", "SVSubSppAsSpp", "SVConstancyFormat":
		return longVegetationConfigBool(values, key)
	case "SVGroupBy", "SVValue", "SVShowValOpt":
		return summaryVegetationPreferenceOption(values, key)
	default:
		return nil, fmt.Errorf("unsupported Summary Vegetation presentation preference %s", key)
	}
}

func summaryVegetationPresentationPreferenceChanges(expected, proposed summaryVegetationPresentationPreferenceValues) []vprocore.ReportPreferenceChange[any] {
	return []vprocore.ReportPreferenceChange[any]{
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

func ReadSummaryVegetationPresentationPreferences(s *vprocore.DesktopConfig, ctx context.Context) (summaryVegetationPresentationPreferenceValues, error) {
	if ctx == nil || s == nil {
		return summaryVegetationPresentationPreferenceValues{}, errors.New("Summary Vegetation presentation preferences require a context and configuration")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	var result summaryVegetationPresentationPreferenceValues
	result.Title, err = vprocore.ConfigString(values, "ReportOptions", "SVReportTitle")
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
	result.GroupBy, err = vprocore.ConfigInt(values, "ReportOptions", "SVGroupBy", 1, 4)
	if err != nil {
		return summaryVegetationPresentationPreferenceValues{}, err
	}
	result.Display, err = vprocore.ConfigInt(values, "ReportOptions", "SVValue", 1, 6)
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

func CompareAndSetSummaryVegetationPresentationPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed summaryVegetationPresentationPreferenceValues, constancyApplied bool) (vprocore.ReportPreferenceUpdate, error) {
	for _, value := range []summaryVegetationPresentationPreferenceValues{expected, proposed} {
		if value.GroupBy < 1 || value.GroupBy > 4 || value.Display < 1 || value.Display > 6 ||
			math.IsNaN(value.ValueLimit) || math.IsInf(value.ValueLimit, 0) {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation presentation requires original Group by1..4, Display1..6 and a finite historical limit")
		}
	}
	if expected.Title != proposed.Title && (!utf8.ValidString(proposed.Title) || strings.ContainsRune(proposed.Title, 0)) {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation title requires complete Unicode without NUL; no repair or normalization")
	}
	if expected.ValueLimit != proposed.ValueLimit {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation constancy callback selects Display1 without clearing the historical limit; limit edits unavailable")
	}
	if constancyApplied {
		if proposed.GroupBy != 3 || proposed.Display != 1 || proposed.Constancy && proposed.ShowCommon {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation constancy-on callback requires Group by3 and Display1; common names stay off while constancy is selected")
		}
	} else {
		if !expected.Constancy && proposed.Constancy {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation constancy-on requires its coupled callback plan")
		}
		if expected.GroupBy != proposed.GroupBy || expected.Display != proposed.Display {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation presentation cannot independently edit shared Group by or Display")
		}
		if proposed.Constancy && expected.ShowCommon != proposed.ShowCommon {
			return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation common-name control is disabled while constancy is selected")
		}
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Summary Vegetation presentation preferences",
		summaryVegetationPresentationPreferenceChanges(expected, proposed), summaryVegetationPresentationPreferenceOption)
}
