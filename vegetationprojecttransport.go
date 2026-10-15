package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"unicode/utf8"
)

func longVegetationProjectTransport(ctx context.Context, source vegetationLayerReport, sourceKind string) (LongVegetationReport, error) {
	if ctx == nil {
		return LongVegetationReport{}, errors.New("project report transport requires a context")
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationReport{}, err
	}
	if sourceKind != vegetationProjectFieldsScope && sourceKind != vegetationProjectAllScope {
		return LongVegetationReport{}, errors.New("project report transport requires its explicit original source kind")
	}
	if source.Project == "" || source.SU == "" || !utf8.ValidString(source.Project+source.SU) ||
		strings.ContainsRune(source.Project+source.SU, 0) ||
		source.ProjectUnits == nil || len(source.ProjectUnits) == 0 || len(source.ProjectUnits) > 1048572 ||
		source.SelectedUnits != nil || source.NoneUnits != nil ||
		sourceKind == vegetationProjectAllScope && source.SU != "None" ||
		source.Quality != nil && (sourceKind == vegetationProjectAllScope || source.SU == "None") {
		return LongVegetationReport{}, errors.New("project report transport requires exclusive original project provenance and actual quality authority")
	}
	inputs, err := validateProjectUnitInputs(ctx, source.ProjectUnits)
	if err != nil {
		return LongVegetationReport{}, err
	}
	if (inputs[0].AdminRowID != nil) != (sourceKind == vegetationProjectFieldsScope) {
		return LongVegetationReport{}, errors.New("project report source kind does not match original tuple provenance")
	}
	seen := make([]bool, len(inputs))
	unitCodes := map[string]bool{}
	for _, unit := range source.Units {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		if unit.NumPlots <= 0 || len(unit.MembershipIDs) != 0 || len(unit.ProjectUnitIndexes) != unit.NumPlots {
			return LongVegetationReport{}, errors.New("project unit denominator requires original tuple indexes, not physical SU membership IDs")
		}
		if _, err := vegetationReportIdentity(unit.Code); err != nil {
			return LongVegetationReport{}, err
		}
		key := vegetationTextKey(unit.Code)
		if unitCodes[key] {
			return LongVegetationReport{}, errors.New("project report repeats an original unit label")
		}
		unitCodes[key] = true
		for _, index := range unit.ProjectUnitIndexes {
			if err := ctx.Err(); err != nil {
				return LongVegetationReport{}, err
			}
			if index < 0 || index >= len(inputs) || seen[index] || !reflect.DeepEqual(inputs[index].SiteUnit, unit.Code) {
				return LongVegetationReport{}, errors.New("project unit index has missing, repeated or mismatched original tuple ownership")
			}
			seen[index] = true
		}
	}
	excluded := map[string]int{}
	for _, diagnostic := range source.Diagnostics {
		if diagnostic.Code != "excluded_unit_without_vegetation" {
			continue
		}
		if diagnostic.Count <= 0 || excluded[diagnostic.Identity] != 0 || unitCodes[diagnostic.Identity] {
			return LongVegetationReport{}, errors.New("project report has inconsistent excluded-unit diagnostics")
		}
		excluded[diagnostic.Identity] = diagnostic.Count
	}
	omitted := map[string]int{}
	excludedIndexes := []int{}
	for i, owned := range seen {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		if !owned {
			key := vegetationTextKey(inputs[i].SiteUnit)
			omitted[key]++
			excludedIndexes = append(excludedIndexes, i)
		}
	}
	if !reflect.DeepEqual(excluded, omitted) {
		return LongVegetationReport{}, errors.New("project report omitted original tuple ownership without exact excluded-unit evidence")
	}
	result, err := cloneLongVegetationTransport(ctx, source)
	if err != nil {
		return LongVegetationReport{}, err
	}
	result.SourceKind = sourceKind
	if len(excludedIndexes) > 0 {
		result.ExcludedProjectUnitIndexes = excludedIndexes
	}
	result.ProjectUnits = make([]LongVegetationProjectUnit, len(inputs))
	for i, row := range inputs {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		result.ProjectUnits[i] = LongVegetationProjectUnit{row.EnvRowID, row.AdminRowID, row.PlotNumber, row.SiteUnit}
	}
	return result, nil
}
