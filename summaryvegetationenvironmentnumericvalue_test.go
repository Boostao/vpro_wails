package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestSummaryEnvironmentalNumericValueSourceWire(t *testing.T) {
	type wireCase struct {
		Function string                              `json:"function"`
		Argument ProjectMetadataCell                 `json:"argument"`
		Unit     string                              `json:"unit"`
		Value    SummaryVegetationEnvironmentalValue `json:"value"`
	}
	wireCases := []wireCase{}
	type observation struct {
		Function string   `json:"function"`
		Suffix   string   `json:"suffix"`
		Unit     *string  `json:"unit"`
		Actual   []string `json:"actual"`
	}
	type physicalFixture struct {
		source   string
		env      ProjectMetadataTable
		su       ProjectMetadataTable
		function string
		count    int
	}
	depthEnv, depthSU := summaryEnvironmentalDepthPhysicalFixture(t)
	coverEnv, coverSU := summaryEnvironmentalCoverPhysicalFixture(t)
	slopeEnv, slopeSU := summaryEnvironmentalSlopePhysicalFixture(t)
	ageEnv, ageSU := summaryEnvironmentalAgePhysicalFixture(t)
	for _, fixture := range []physicalFixture{
		{summaryEnvironmentalDepthSourceFixture, depthEnv, depthSU, "", 22},
		{summaryEnvironmentalCoverSourceFixture, coverEnv, coverSU, "", 110},
		{summaryEnvironmentalSlopeSourceFixture, slopeEnv, slopeSU, "SVSlope", 9},
		{summaryEnvironmentalAgeSourceFixture, ageEnv, ageSU, "", 18},
	} {
		var source struct {
			Observations []observation `json:"observations"`
		}
		if err := json.Unmarshal([]byte(fixture.source), &source); err != nil || len(source.Observations) != fixture.count {
			t.Fatal("complete source observations required", err)
		}
		for _, item := range source.Observations {
			function := item.Function
			if fixture.function != "" {
				function = fixture.function
			}
			argument := ProjectMetadataCell{Storage: "null"}
			if item.Suffix != "" {
				argument = ProjectMetadataCell{Storage: "text", Text: &item.Suffix}
			}
			got, err := prepareSummaryVegetationEnvironmentalNumericValue(t.Context(), function, argument, item.Unit, fixture.env, fixture.su)
			if item.Unit == nil {
				if err == nil || !reflect.DeepEqual(item.Actual[:2], []string{"error", "94"}) {
					t.Fatal(got, err, item)
				}
				continue
			}
			var want SummaryVegetationEnvironmentalValue
			switch item.Actual[1] {
			case "Empty":
				want = SummaryVegetationEnvironmentalValue{Kind: "empty"}
			case "String":
				text := item.Actual[2]
				want = SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "text", Text: &text}}
			default:
				t.Fatal("unknown source kind", item)
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal(got, err, want, item)
			}
			wireCases = append(wireCases, wireCase{function, argument, *item.Unit, got})
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &wire); err != nil || len(wire) != 2 {
				t.Fatal("wire shape", string(encoded), err)
			}
			if want.Kind == "empty" && string(encoded) != `{"kind":"empty","cell":null}` {
				t.Fatal("Empty must not become metadata NULL or String empty", string(encoded))
			}
			if want.Kind == "cell" {
				var cell map[string]json.RawMessage
				if err := json.Unmarshal(wire["cell"], &cell); err != nil || len(cell) != 5 ||
					string(cell["storage"]) != `"text"` || string(cell["integer"]) != "null" ||
					string(cell["real"]) != "null" || string(cell["blobHex"]) != "null" {
					t.Fatal("complete metadata cell wire required", string(encoded), err)
				}
				var text string
				if err := json.Unmarshal(cell["text"], &text); err != nil || text != item.Actual[2] {
					t.Fatal("source text bytes changed", string(encoded), err)
				}
			}
		}
	}
	if len(wireCases) != 144 {
		t.Fatal("all144 successful original cases required", len(wireCases))
	}
	if path := os.Getenv("VPRO_SUMMARY_ENVIRONMENT_NUMERIC_WIRE_PATH"); path != "" {
		encoded, err := json.MarshalIndent(wireCases, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, '\n')
		if os.Getenv("VPRO_UPDATE_SUMMARY_ENVIRONMENT_NUMERIC_WIRE") == "true" {
			if err := os.WriteFile(path, encoded, 0600); err != nil {
				t.Fatal(err)
			}
		}
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, encoded) {
			t.Fatal("strict-Go numeric fixture differs", err)
		}
		var decoded []wireCase
		decoder := json.NewDecoder(bytes.NewReader(actual))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(decoded, wireCases) {
			t.Fatal("strict-Go numeric roundtrip differs", err)
		}
	}
}

func TestSummaryEnvironmentalNumericValueArgumentsAndContext(t *testing.T) {
	text := func(value string) ProjectMetadataCell { return ProjectMetadataCell{Storage: "text", Text: &value} }
	for _, function := range []string{"SVSeepage", "SVRootingDepth", "SVStandAge", "SVElevation", "SVSlope"} {
		if _, err := summaryEnvironmentalNumericArgument(function, ProjectMetadataCell{Storage: "null"}); err != nil {
			t.Fatal(function, err)
		}
		for _, argument := range []ProjectMetadataCell{text(""), text("1"), metadataInteger("1"),
			{Storage: "null", Text: text("").Text}} {
			if _, err := summaryEnvironmentalNumericArgument(function, argument); err == nil {
				t.Fatal(function, argument)
			}
		}
	}
	for _, function := range []string{"SVSubstrate", "SVStrataCover"} {
		for _, argument := range []ProjectMetadataCell{text(""), text("Tree "), text("tree"), text("1"), text("DecWood "),
			{Storage: "null"}, metadataInteger("1"), {Storage: "text", Text: text("Tree").Text, Integer: text("1").Text},
			text(string([]byte{0xff}))} {
			if _, err := summaryEnvironmentalNumericArgument(function, argument); err == nil {
				t.Fatal(function, argument)
			}
		}
	}
	for _, function := range []string{"SVHumusThickness", "SVLifeFormCover", "SVSubstrate ", "svSlope", "SVnPlots"} {
		if _, err := summaryEnvironmentalNumericArgument(function, ProjectMetadataCell{Storage: "null"}); err == nil {
			t.Fatal("unsupported function", function)
		}
	}
	env, su := summaryEnvironmentalDepthPhysicalFixture(t)
	unit := "mixed"
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := prepareSummaryVegetationEnvironmentalNumericValue(ctx, "SVSeepage", ProjectMetadataCell{Storage: "null"}, &unit, env, su)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, SummaryVegetationEnvironmentalValue{}) {
		t.Fatal(got, err)
	}
	if _, err := prepareSummaryVegetationEnvironmentalNumericValue(nil, "SVSeepage", ProjectMetadataCell{Storage: "null"}, &unit, env, su); err == nil {
		t.Fatal("nil context accepted")
	}
}
