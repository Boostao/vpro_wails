package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
		cell             vprocore.ProjectMetadataCell
		master, personal bool
	}{
		{"null", vprocore.ProjectMetadataCell{Storage: "null"}, true, true},
		{"master-min", vprotest.MetadataInteger("-2147483648"), true, false},
		{"below-personal-min", vprotest.MetadataInteger("-32769"), true, false},
		{"personal-min", vprotest.MetadataInteger("-32768"), true, true},
		{"zero", vprotest.MetadataInteger("0"), true, true},
		{"personal-max", vprotest.MetadataInteger("32767"), true, true},
		{"above-personal-max", vprotest.MetadataInteger("32768"), true, false},
		{"master-max", vprotest.MetadataInteger("2147483647"), true, false},
		{"below-master-min", vprotest.MetadataInteger("-2147483649"), false, false},
		{"above-master-max", vprotest.MetadataInteger("2147483648"), false, false},
		{"signed64-min", vprotest.MetadataInteger("-9223372036854775808"), false, false},
		{"signed64-max", vprotest.MetadataInteger("9223372036854775807"), false, false},
		{"text", vprotest.MetadataText("3"), false, false},
		{"malformed-text", vprotest.MetadataText("\xff"), false, false},
		{"real", vprotest.SiviReal(3), false, false},
		{"malformed-integer", vprotest.MetadataInteger("not-integer"), false, false},
	} {
		for _, personal := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/personal-%t", test.name, personal), func(t *testing.T) {
				references := sftest.SiviSpeciesTestReferences(t)
				raw := references.Master
				expected := test.master
				if personal {
					raw, expected = references.Personal, test.personal
				}
				raw.Rows[0].Cells[2] = test.cell
				before := vprotest.CloneLifeformReferenceFixture(raw)
				projected, err := projectTwoPageVegetationSpeciesDefinitions(context.Background(), raw, personal)
				if (err == nil) != expected || !reflect.DeepEqual(raw, before) ||
					(expected && !reflect.DeepEqual(projected, before)) ||
					(!expected && !reflect.DeepEqual(projected, vprocore.ProjectMetadataTable{})) {
					t.Fatal("source-specific width policy changed physical input/output", raw, projected, err)
				}
				legacy, legacyErr := sourceform.ProjectSIVISpeciesDefinitions(context.Background(), raw, personal)
				if (legacyErr == nil) != test.personal ||
					(test.personal && !reflect.DeepEqual(legacy, before)) {
					t.Fatal("two-page LONG policy leaked into legacy SIVI projection", legacy, legacyErr)
				}
			})
		}
	}
	raw := sftest.SiviSpeciesTestReferences(t).Master
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := projectTwoPageVegetationSpeciesDefinitions(canceled, raw, false); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, vprocore.ProjectMetadataTable{}) {
		t.Fatal("cancelled projection returned partial definitions", result, err)
	}
	if result, err := projectTwoPageVegetationSpeciesDefinitions(nil, raw, false); err == nil ||
		!reflect.DeepEqual(result, vprocore.ProjectMetadataTable{}) {
		t.Fatal("nil-context projection returned definitions", result, err)
	}
	if result, err := sourceform.ProjectPhysicalSpeciesDefinitions(context.Background(), raw, false, nil); err == nil ||
		!reflect.DeepEqual(result, vprocore.ProjectMetadataTable{}) {
		t.Fatal("missing source policy accepted", result, err)
	}
}
