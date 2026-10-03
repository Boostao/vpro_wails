package main

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

type longVegetationOptions struct {
	Title                string
	Average              string
	ConstantSpeciesList  bool
	PresenceGreaterThan  float64
	MeanCoverGreaterThan float64
	Order                string
	ShowEnglishName      bool
}

func decodeLongVegetationOptions(values configValues) (longVegetationOptions, error) {
	fail := func(err error) (longVegetationOptions, error) { return longVegetationOptions{}, err }
	var options longVegetationOptions
	var err error
	options.Title, err = configString(values, "ReportOptions", "LVReportTitle")
	if err != nil {
		return fail(err)
	}
	if strings.ContainsRune(options.Title, 0) {
		return fail(fmt.Errorf("ReportOptions.LVReportTitle must not contain NUL"))
	}
	for _, key := range []string{"LVGroupBy", "LVUnitGroups"} {
		if _, err := configInt(values, "ReportOptions", key, 1, 1); err != nil {
			return fail(fmt.Errorf("Long Vegetation preview supports only %s=1 (layers and selected SU): %w", key, err))
		}
	}
	enforce, err := longVegetationConfigBool(values, "DataQualityFilterEnforceLV")
	if err != nil {
		return fail(err)
	}
	if enforce {
		return fail(fmt.Errorf("ReportOptions.DataQualityFilterEnforceLV is unavailable in Long Vegetation preview"))
	}
	average, err := configInt(values, "ReportOptions", "LVAvgType", 10, 20)
	if err != nil {
		return fail(err)
	}
	// USysLongVegOptions offers only 10 (By n Plots) and 20 (Characteristic).
	// V7mdlReportsLongVeg uses Sum/NumPlots for 10, Avg(observations) for 20.
	switch average {
	case 10:
		options.Average = "all-plots"
	case 20:
		options.Average = "observations"
	default:
		return fail(fmt.Errorf("ReportOptions.LVAvgType must be 10 (all-plots) or 20 (observations)"))
	}
	order, err := configInt(values, "ReportOptions", "LVOrderBy", 10, 20)
	if err != nil {
		return fail(err)
	}
	switch order {
	case 10:
		options.Order = "species"
	case 20:
		options.Order = "presence"
	default:
		return fail(fmt.Errorf("ReportOptions.LVOrderBy must be 10 (species) or 20 (presence)"))
	}
	show, err := configInt(values, "ReportOptions", "LVShowEnglishName", 0, 1)
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation preview supports only LVShowEnglishName=0 or 1; code and selected-field modes are unavailable: %w", err))
	}
	options.ShowEnglishName = show == 1
	options.ConstantSpeciesList, err = longVegetationConfigBool(values, "LVConstantSppList")
	if err != nil {
		return fail(err)
	}
	options.PresenceGreaterThan, err = longVegetationConfigNumber(values, "LVPresenceGreaterThan")
	if err != nil {
		return fail(err)
	}
	options.MeanCoverGreaterThan, err = longVegetationConfigNumber(values, "LVCoverGreaterThan")
	if err != nil {
		return fail(err)
	}
	// Presence remains percent here; the source divides it by 100 in report SQL.
	// No threshold bounds are defined by the source controls or AfterUpdate paths.
	// LVReportSummary remains untouched and outside preview scope; no summary is
	// computed. Quick/layout preferences are publication-only and untouched.
	return options, nil
}

func longVegetationConfigBool(values configValues, key string) (bool, error) {
	fields, err := configSection(values, "ReportOptions")
	if err != nil {
		return false, err
	}
	switch value := fields[key].(type) {
	case bool:
		return value, nil
	case int:
		if value == -1 || value == 0 {
			return value == -1, nil
		}
	}
	return false, fmt.Errorf("ReportOptions.%s must be a YAML boolean or source integer -1/0, not NULL or an implicit default", key)
}

func longVegetationConfigNumber(values configValues, key string) (float64, error) {
	fields, err := configSection(values, "ReportOptions")
	if err != nil {
		return 0, err
	}
	var number float64
	var integer *big.Int
	switch value := fields[key].(type) {
	case int:
		integer = big.NewInt(int64(value))
	case uint64:
		// yaml.v3 uses uint64 for positive integers outside the native int range.
		integer = new(big.Int).SetUint64(value)
	case float64:
		number = value
	default:
		return 0, fmt.Errorf("ReportOptions.%s must be a finite YAML number, not NULL or an implicit default", key)
	}
	if integer != nil {
		var accuracy big.Accuracy
		number, accuracy = new(big.Float).SetInt(integer).Float64()
		if accuracy != big.Exact {
			return 0, fmt.Errorf("ReportOptions.%s integer cannot be represented exactly as a threshold", key)
		}
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("ReportOptions.%s must be a finite YAML number", key)
	}
	return number, nil
}
