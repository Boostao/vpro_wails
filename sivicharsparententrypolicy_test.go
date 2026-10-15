package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSIVICHARSEntryClientPolicies(t *testing.T) {
	policies, err := siviCHARSParentEntryPolicies()
	if err != nil || len(policies) != 68 {
		t.Fatal("client policy cohort changed", err)
	}
	byColumn := map[string]SIVICHARSParentEntryPolicy{}
	writable := 0
	for _, policy := range policies {
		if _, duplicate := byColumn[policy.Column]; duplicate || policy.ControlID == "" || policy.Binding == "" {
			t.Fatal("ambiguous source policy", policy)
		}
		byColumn[policy.Column] = policy
		if policy.Writable {
			writable++
		}
	}
	if writable != 67 || byColumn["SubZone"].Writable || byColumn["SubZone"].Diagnostic == "" ||
		byColumn["SV_AhorizonType"].AllowEmpty || !byColumn["SnowCoverregime"].AllowEmpty {
		t.Fatal("source availability/empty distinctions changed", byColumn)
	}
	for _, column := range []string{"PlotNumber", "ProjectID", "PlotType", "SpeciesListComplete", "SV_RootZoneTexture", "SeepageDepth"} {
		if _, exists := byColumn[column]; exists {
			t.Fatal("client inherited a foreign or readonly policy", column)
		}
	}
	for _, expected := range []struct {
		column, kind string
		maximum      int
	}{
		{"RootZoneParticleSize", "text", 6}, {"SV_WaterTableCM", "single", 0},
		{"Latitude", "double", 0}, {"Longitude", "double", 0},
		{"SnowCoverregime", "text", 1}, {"SV_AhorizonType", "text", 5},
	} {
		actual := byColumn[expected.column]
		if actual.Kind != expected.kind || actual.Maximum != expected.maximum || actual.Lower != nil || actual.Upper != nil {
			t.Fatal("source storage domain changed", expected, actual)
		}
	}
	if year := byColumn["StartDate"]; year.Kind != "integer" || year.Owner != "Admin" ||
		year.Lower == nil || *year.Lower != 1900 || year.Upper == nil || *year.Upper != 2500 {
		t.Fatal("source year range changed", year)
	}
	if aspect := byColumn["Aspect"]; aspect.Lower == nil || *aspect.Lower != -32768 ||
		aspect.Upper == nil || *aspect.Upper != 32767 {
		t.Fatal("source INTEGER range changed", aspect)
	}
	policies[0].Column = "caller edit"
	again, err := siviCHARSParentEntryPolicies()
	if err != nil || again[0].Column == policies[0].Column {
		t.Fatal("policy snapshot aliased a caller", err)
	}
}

func TestSIVICHARSEntryClientGoWire(t *testing.T) {
	policies, err := siviCHARSParentEntryPolicies()
	if err != nil {
		t.Fatal(err)
	}
	wire := struct {
		Original *SIVIParentProjection
		Policies []SIVICHARSParentEntryPolicy
		Request  SIVICHARSParentEntryWrite
		Fields   []SIVIParentSharedReference
	}{Original: charsEntryOriginal(t), Policies: policies}
	_, _, review := charsEntryServiceFixture(t)
	wire.Fields = review.Fields
	wire.Request = SIVICHARSParentEntryWrite{Original: wire.Original, Edits: []SIVIParentCellEdit{
		charsEntryEdit(t, wire.Original, "RootZoneParticleSize", metadataText(" a  ")),
		charsEntryEdit(t, wire.Original, "SV_WaterTableCM", siviReal(0.10000000149011612)),
		charsEntryEdit(t, wire.Original, "StartDate", metadataInteger("1900")),
		charsEntryEdit(t, wire.Original, "SV_FloodPlain", metadataInteger("-1")),
	}}
	requestData, err := json.Marshal(wire.Request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SIVICHARSParentEntryWrite
	if err := json.Unmarshal(requestData, &decoded); err != nil {
		t.Fatal("actual client request shape fails strict Go decoding", err)
	}
	if plan, err := planSIVICHARSParentEntry(context.Background(), decoded.Original, decoded.Edits); err != nil || len(plan) != 4 {
		t.Fatal("actual client request did not prepare its complete source plan", plan, err)
	}
	data, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("VPRO_CHARS_POLICY_WIRE_FIXTURE"); output != "" {
		if err := os.WriteFile(output, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "chars-sivi-entry-client-wire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("portable CHARS client fixture no longer matches the actual Go wire")
	}
}
