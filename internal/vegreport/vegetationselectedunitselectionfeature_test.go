package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "testing"

func TestSelectedUnitSelectionAndConsumerFeaturesRemainIndependentDefaultOff(t *testing.T) {
	for _, name := range []string{LongVegetationSelectedUnitsFeatureEnvironment, LongVegetationSelectedUnitSelectionFeatureEnvironment} {
		for _, value := range []struct {
			value   string
			present bool
			enabled bool
			fail    bool
		}{
			{"", false, true, false}, {"false", true, false, false}, {"true", true, true, false},
			{"", true, false, true}, {"True", true, false, true}, {"1", true, false, true},
		} {
			enabled, err := vprocore.SiviFeature(name, func(requested string) (string, bool) {
				if requested != name {
					t.Fatal("selected-unit feature borrowed another workflow's gate", requested, name)
				}
				return value.value, value.present
			})
			if enabled != value.enabled || (err != nil) != value.fail {
				t.Fatal("feature silently defaulted malformed gate", name, value, enabled, err)
			}
		}
	}
	if LongVegetationSelectedUnitsFeatureEnvironment == LongVegetationSelectedUnitSelectionFeatureEnvironment ||
		LongVegetationSelectedUnitsFeatureEnvironment == LongVegetationSelectedFieldFeatureEnvironment ||
		LongVegetationSelectedUnitSelectionFeatureEnvironment == longVegetationUnitPreferencesFeatureEnvironment {
		t.Fatal("selected unit selector/consumer borrowed species field or Unit Groups editor availability")
	}
}
