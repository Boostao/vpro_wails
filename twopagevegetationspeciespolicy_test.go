package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestTwoPageVegetationSpeciesDefinitionSourceRangePolicy(t *testing.T) {
	for _, test := range []struct {
		name             string
		cell             ProjectMetadataCell
		master, personal bool
	}{
		{"null", ProjectMetadataCell{Storage: "null"}, true, true},
		{"master-min", metadataInteger("-2147483648"), true, false},
		{"below-personal-min", metadataInteger("-32769"), true, false},
		{"personal-min", metadataInteger("-32768"), true, true},
		{"zero", metadataInteger("0"), true, true},
		{"personal-max", metadataInteger("32767"), true, true},
		{"above-personal-max", metadataInteger("32768"), true, false},
		{"master-max", metadataInteger("2147483647"), true, false},
		{"below-master-min", metadataInteger("-2147483649"), false, false},
		{"above-master-max", metadataInteger("2147483648"), false, false},
		{"signed64-min", metadataInteger("-9223372036854775808"), false, false},
		{"signed64-max", metadataInteger("9223372036854775807"), false, false},
		{"text", metadataText("3"), false, false},
		{"malformed-text", metadataText("\xff"), false, false},
		{"real", siviReal(3), false, false},
		{"malformed-integer", metadataInteger("not-integer"), false, false},
	} {
		for _, personal := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/personal-%t", test.name, personal), func(t *testing.T) {
				references := siviSpeciesTestReferences(t)
				raw := references.Master
				expected := test.master
				if personal {
					raw, expected = references.Personal, test.personal
				}
				raw.Rows[0].Cells[2] = test.cell
				before := cloneLifeformReferenceFixture(raw)
				projected, err := projectTwoPageVegetationSpeciesDefinitions(context.Background(), raw, personal)
				if (err == nil) != expected || !reflect.DeepEqual(raw, before) ||
					(expected && !reflect.DeepEqual(projected, before)) ||
					(!expected && !reflect.DeepEqual(projected, ProjectMetadataTable{})) {
					t.Fatal("source-specific width policy changed physical input/output", raw, projected, err)
				}
				legacy, legacyErr := projectSIVISpeciesDefinitions(context.Background(), raw, personal)
				if (legacyErr == nil) != test.personal ||
					(test.personal && !reflect.DeepEqual(legacy, before)) {
					t.Fatal("two-page LONG policy leaked into legacy SIVI projection", legacy, legacyErr)
				}
			})
		}
	}
	raw := siviSpeciesTestReferences(t).Master
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := projectTwoPageVegetationSpeciesDefinitions(canceled, raw, false); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, ProjectMetadataTable{}) {
		t.Fatal("cancelled projection returned partial definitions", result, err)
	}
	if result, err := projectTwoPageVegetationSpeciesDefinitions(nil, raw, false); err == nil ||
		!reflect.DeepEqual(result, ProjectMetadataTable{}) {
		t.Fatal("nil-context projection returned definitions", result, err)
	}
	if result, err := projectPhysicalSpeciesDefinitions(context.Background(), raw, false, nil); err == nil ||
		!reflect.DeepEqual(result, ProjectMetadataTable{}) {
		t.Fatal("missing source policy accepted", result, err)
	}
}
