package main

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var soilChildTextMaximum = map[string]map[string]int{
	"Humus": {
		"Horizon": 8, "HumusStructureDegree": 1, "HumusStructureKind": 5,
		"MycelAbundance": 1, "FecalAbundance": 1, "RootsAbundance": 6, "RootsSize": 6, "Comment": 0,
	},
	"Mineral": {
		"Horizon": 8, "Texture": 4, "Colour": 14, "PitDepthLimit": 1, "PercentCoarseFragsShape": 1,
		"RootsAbundance": 6, "RootsSize": 6, "MineralStructureClass": 3, "MineralStructureKind": 7, "Comments": 0,
	},
}

var soilChildSingleColumns = map[string]map[string]bool{
	"Humus":   {"UpperDepth": true, "LowerDepth": true, "HumusFormpH": true},
	"Mineral": {"UpperDepth": true, "LowerDepth": true, "MineralFormpH": true},
}

var soilChildIntegerColumns = map[string]map[string]bool{
	"Humus": {"vonPost": true},
	"Mineral": {
		"ASP": true, "PercentCoarseFragsGravel": true, "PercentCoarseFragsCobbles": true,
		"PercentCoarseFragsStones": true, "PercentCoarseFragsTotal": true,
	},
}

func validateChildPhysicalText(name string, value any, maximum int) error {
	if value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok || !utf8.ValidString(text) {
		return fmt.Errorf("%s requires nullable valid Unicode text", name)
	}
	if maximum > 0 && len(utf16.Encode([]rune(text))) > maximum {
		return fmt.Errorf("%s exceeds %d UTF-16 units", name, maximum)
	}
	return nil
}

func soilChildJSONTextNames(kind string, record any) map[string]string {
	names := make(map[string]string)
	value := reflect.ValueOf(record)
	for _, field := range childFields[kind] {
		if _, text := soilChildTextMaximum[kind][field.column]; text {
			names[strings.ToLower(childJSONKey(value, field))] = kind + "." + field.column
		}
	}
	return names
}

func validateSoilChildField(kind, column string, value any) error {
	if maximum, text := soilChildTextMaximum[kind][column]; text {
		return validateChildPhysicalText(kind+"."+column, value, maximum)
	}
	if soilChildSingleColumns[kind][column] {
		if value == nil {
			return nil
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s.%s requires a nullable number", kind, column)
		}
		return validateSingleRangeChange(kind+"."+column, nil, &number)
	}
	if soilChildIntegerColumns[kind][column] {
		if value == nil {
			return nil
		}
		number, ok := value.(int)
		if !ok {
			return fmt.Errorf("%s.%s requires a nullable integer", kind, column)
		}
		return validateOrdinaryInteger(kind+"."+column, &number)
	}
	return fmt.Errorf("unsupported %s value for %q", kind, column)
}

func validateChildField(kind, column string, value any) error {
	if kind == "Veg" {
		return validateVegetationField(column, value)
	}
	if kind == "Other" {
		return validateOtherField(column, value)
	}
	return validateSoilChildField(kind, column, value)
}
