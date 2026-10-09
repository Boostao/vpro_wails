package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
)

type VegetationAttributeUpdate HeightRecordUpdate

func (update *VegetationAttributeUpdate) UnmarshalJSON(data []byte) error {
	var decoded HeightRecordUpdate
	if err := decoded.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("vegetation attribute transport: %w", err)
	}
	*update = VegetationAttributeUpdate(decoded)
	return nil
}

var vegetationAttributeProperties = map[string]bool{
	"ll": true, "af": true, "dc": true, "ut": true, "vi": true, "pv": true, "pg": true, "ffa": true,
	"cultural1": true, "cultural2": true, "other1": true, "other2": true,
}

var vegetationAttributeGroups = []string{
	"ArborealLichenLoading", "DistributionCode", "UtilizationCode", "VigourCode",
	"phenologyCodeVeg", "PhenologyCodeGen", "FruitFlowerAbundance", "Cultural1", "VegOther1", "VegOther2",
}

func vegetationPatchInteger(property string, number *float64) (any, error) {
	if number == nil {
		return nil, nil
	}
	if math.IsNaN(*number) || math.IsInf(*number, 0) || math.Trunc(*number) != *number ||
		math.Abs(*number) > 9007199254740991 {
		return nil, fmt.Errorf("vegetation attribute %q requires an exact finite integer", property)
	}
	return int(*number), nil
}

func (s *PlotService) UpdateVegetationAttributes(plot string, updates []VegetationAttributeUpdate) error {
	patches := make([]childRecordPatch, 0, len(updates))
	for _, update := range updates {
		if len(update.Values) == 0 || len(update.Values) != len(update.Expected) {
			return errors.New("vegetation attributes require nonempty matching values/expected keys")
		}
		patch := childRecordPatch{kind: "Veg", id: update.ID, cells: map[string]childExpectedValue{}}
		for property, number := range update.Values {
			expected, present := update.Expected[property]
			if !vegetationAttributeProperties[property] || !present {
				return fmt.Errorf("vegetation attribute %q is unavailable or has no expected value", property)
			}
			before, err := vegetationPatchInteger(property, expected)
			if err != nil {
				return err
			}
			after, err := vegetationPatchInteger(property, number)
			if err != nil {
				return err
			}
			patch.cells[property] = childExpectedValue{before, after, reflect.Int}
		}
		patches = append(patches, patch)
	}
	return s.updateChildPatches(plot, patches)
}

func (s *ContextService) UpdateVegetationAttributes(contextID, plot string, updates []VegetationAttributeUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateVegetationAttributes(plot, updates) })
}

func (s *ContextService) ListVegetationAttributeSuggestions(ctx context.Context, contextID string) ([]SoilSuggestion, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]SoilSuggestion, error) {
		return listChildSuggestions(ctx, plots, vegetationAttributeGroups, "phenologyCodeVeg")
	})
}
