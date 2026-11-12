package plotentry

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
)

type VegetationAttributeUpdate vprocore.HeightRecordUpdate

func (update *VegetationAttributeUpdate) UnmarshalJSON(data []byte) error {
	var decoded vprocore.HeightRecordUpdate
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

func UpdateVegetationAttributes(s *vprocore.PlotService, plot string, updates []VegetationAttributeUpdate) error {
	patches := make([]vprocore.ChildRecordPatch, 0, len(updates))
	for _, update := range updates {
		if len(update.Values) == 0 || len(update.Values) != len(update.Expected) {
			return errors.New("vegetation attributes require nonempty matching values/expected keys")
		}
		patch := vprocore.ChildRecordPatch{Kind: "Veg", Id: update.ID, Cells: map[string]vprocore.ChildExpectedValue{}}
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
			patch.Cells[property] = vprocore.ChildExpectedValue{Expected: before, Value: after, Scalar: reflect.Int}
		}
		patches = append(patches, patch)
	}
	return s.UpdateChildPatches(plot, patches)
}

func CtxUpdateVegetationAttributes(s *vprocore.ContextService, contextID, plot string, updates []VegetationAttributeUpdate) error {
	return s.Edit(contextID, func(plots *vprocore.PlotService) error { return UpdateVegetationAttributes(plots, plot, updates) })
}

func ListVegetationAttributeSuggestions(s *vprocore.ContextService, ctx context.Context, contextID string) ([]SoilSuggestion, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) ([]SoilSuggestion, error) {
		return listChildSuggestions(ctx, plots, vegetationAttributeGroups, "phenologyCodeVeg")
	})
}
