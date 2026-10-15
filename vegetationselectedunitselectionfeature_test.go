package main

import "testing"

func TestSelectedUnitSelectionAndConsumerFeaturesRemainIndependentDefaultOff(t *testing.T) {
	for _, name := range []string{longVegetationSelectedUnitsFeatureEnvironment, longVegetationSelectedUnitSelectionFeatureEnvironment} {
		for _, value := range []struct {
			value   string
			present bool
			enabled bool
			fail    bool
		}{
			{"", false, false, false}, {"false", true, false, false}, {"true", true, true, false},
			{"", true, false, true}, {"True", true, false, true}, {"1", true, false, true},
		} {
			enabled, err := siviFeature(name, func(requested string) (string, bool) {
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
	if longVegetationSelectedUnitsFeatureEnvironment == longVegetationSelectedUnitSelectionFeatureEnvironment ||
		longVegetationSelectedUnitsFeatureEnvironment == longVegetationSelectedFieldFeatureEnvironment ||
		longVegetationSelectedUnitSelectionFeatureEnvironment == longVegetationUnitPreferencesFeatureEnvironment {
		t.Fatal("selected unit selector/consumer borrowed species field or Unit Groups editor availability")
	}
}
