package vprocore

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var SoilChildTextMaximum = map[string]map[string]int{
	"Humus": {
		"Horizon": 8, "HumusStructureDegree": 1, "HumusStructureKind": 5,
		"MycelAbundance": 1, "FecalAbundance": 1, "RootsAbundance": 6, "RootsSize": 6, "Comment": 0,
	},
	"Mineral": {
		"Horizon": 8, "Texture": 4, "Colour": 14, "PitDepthLimit": 1, "PercentCoarseFragsShape": 1,
		"RootsAbundance": 6, "RootsSize": 6, "MineralStructureClass": 3, "MineralStructureKind": 7, "Comments": 0,
	},
}

var SoilChildSingleColumns = map[string]map[string]bool{
	"Humus":   {"UpperDepth": true, "LowerDepth": true, "HumusFormpH": true},
	"Mineral": {"UpperDepth": true, "LowerDepth": true, "MineralFormpH": true},
}

var SoilChildIntegerColumns = map[string]map[string]bool{
	"Humus": {"vonPost": true},
	"Mineral": {
		"ASP": true, "PercentCoarseFragsGravel": true, "PercentCoarseFragsCobbles": true,
		"PercentCoarseFragsStones": true, "PercentCoarseFragsTotal": true,
	},
}

func ValidateChildPhysicalText(name string, value any, maximum int) error {
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

func SoilChildJSONTextNames(kind string, record any) map[string]string {
	names := make(map[string]string)
	value := reflect.ValueOf(record)
	for _, field := range ChildFields[kind] {
		if _, text := SoilChildTextMaximum[kind][field.Column]; text {
			names[strings.ToLower(ChildJSONKey(value, field))] = kind + "." + field.Column
		}
	}
	return names
}

func ValidateSoilChildField(kind, column string, value any) error {
	if maximum, text := SoilChildTextMaximum[kind][column]; text {
		return ValidateChildPhysicalText(kind+"."+column, value, maximum)
	}
	if SoilChildSingleColumns[kind][column] {
		if value == nil {
			return nil
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s.%s requires a nullable number", kind, column)
		}
		return ValidateSingleRangeChange(kind+"."+column, nil, &number)
	}
	if SoilChildIntegerColumns[kind][column] {
		if value == nil {
			return nil
		}
		number, ok := value.(int)
		if !ok {
			return fmt.Errorf("%s.%s requires a nullable integer", kind, column)
		}
		return ValidateOrdinaryInteger(kind+"."+column, &number)
	}
	return fmt.Errorf("unsupported %s value for %q", kind, column)
}

func validateChildField(kind, column string, value any) error {
	if kind == "Veg" {
		return ValidateVegetationField(column, value)
	}
	if kind == "Other" {
		return ValidateOtherField(column, value)
	}
	return ValidateSoilChildField(kind, column, value)
}

// ValidateSoilHorizonDepthSign enforces LMH25 depth sign conventions when Horizon,
// UpperDepth, and LowerDepth are all non-nil:
//   - Upland L/F/H (!OF/!OM/!OH) requires UpperDepth > LowerDepth
//   - Wetland Of/Om/Oh and mineral A/B/C/R/W require UpperDepth < LowerDepth
func ValidateSoilHorizonDepthSign(kind string, horizon *string, upperDepth, lowerDepth *float64) error {
	if horizon == nil || upperDepth == nil || lowerDepth == nil {
		return nil
	}
	hz := strings.ToUpper(strings.TrimSpace(*horizon))
	if hz == "" {
		return nil
	}
	switch kind {
	case "Humus":
		if strings.HasPrefix(hz, "OF") || strings.HasPrefix(hz, "OM") || strings.HasPrefix(hz, "OH") {
			if !(*upperDepth < *lowerDepth) {
				return fmt.Errorf("Humus wetland horizon %q requires UpperDepth < LowerDepth (got %v .. %v)", *horizon, *upperDepth, *lowerDepth)
			}
		} else if strings.HasPrefix(hz, "L") || strings.HasPrefix(hz, "F") || strings.HasPrefix(hz, "H") {
			if !(*upperDepth > *lowerDepth) {
				return fmt.Errorf("Humus upland horizon %q requires UpperDepth > LowerDepth (got %v .. %v)", *horizon, *upperDepth, *lowerDepth)
			}
		}
	case "Mineral":
		if strings.HasPrefix(hz, "A") || strings.HasPrefix(hz, "B") || strings.HasPrefix(hz, "C") || strings.HasPrefix(hz, "R") || strings.HasPrefix(hz, "W") {
			if !(*upperDepth < *lowerDepth) {
				return fmt.Errorf("Mineral horizon %q requires UpperDepth < LowerDepth (got %v .. %v)", *horizon, *upperDepth, *lowerDepth)
			}
		}
	}
	return nil
}
