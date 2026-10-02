package main

import (
	"fmt"
	"math"
)

var vegetationTextMaximum = map[string]int{"Species": 8, "Layer": 2, "Collected": 1}

var vegetationSingleColumns = map[string]bool{
	"Cover1": true, "Cover2": true, "Cover3": true, "TotalA": true,
	"Cover4": true, "Cover5": true, "Cover5a": true, "Cover5b": true, "Cover5c": true, "TotalB": true,
	"Cover6": true, "Cover7": true, "Cover8": true, "Cover9": true, "Cover10": true,
	"Height1": true, "Height2": true, "Height3": true, "Height4": true, "Height5": true, "Height6": true,
}

var vegetationIntegerColumns = map[string]bool{
	"AF": true, "DC": true, "UT": true, "VI": true, "PG": true, "FFA": true,
	"Cultural1": true, "Cultural2": true, "Other1": true, "Other2": true,
}

var vegetationTextJSONNames = map[string]string{
	"species": "Veg.Species", "layer": "Veg.Layer", "collected": "Veg.Collected",
}

func validateVegetationField(column string, value any) error {
	if maximum, text := vegetationTextMaximum[column]; text {
		if column == "Species" && value == nil {
			return fmt.Errorf("Veg.Species requires non-NULL text")
		}
		return validateChildPhysicalText("Veg."+column, value, maximum)
	}
	if vegetationSingleColumns[column] {
		if value == nil {
			return nil
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("Veg.%s requires a nullable number", column)
		}
		return validateSingleRangeChange("Veg."+column, nil, &number)
	}
	if vegetationIntegerColumns[column] || column == "LL" || column == "PV" {
		if value == nil {
			return nil
		}
		number, ok := value.(int)
		if !ok {
			return fmt.Errorf("Veg.%s requires a nullable integer", column)
		}
		if vegetationIntegerColumns[column] {
			return validateOrdinaryInteger("Veg."+column, &number)
		}
		if number < math.MinInt32 || number > math.MaxInt32 {
			return fmt.Errorf("Veg.%s requires an Access Long (-2147483648 to 2147483647)", column)
		}
		return nil
	}
	return fmt.Errorf("unsupported Veg value for %q", column)
}
