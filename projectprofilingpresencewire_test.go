package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectProfileSpeciesPresenceSharedWire(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiling-presence-wire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Request  json.RawMessage `json:"request"`
		Response json.RawMessage `json:"response"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		t.Fatal(err)
	}
	request, err := decodeSpeciesPresenceRequest(wire.Request)
	if err != nil {
		t.Fatal(err)
	}
	if request.Subvarieties || request.ProjectLump != nil || len(request.OriginalRules.Columns) != 9 ||
		len(request.OriginalRules.Rows) != 1 || request.OriginalRules.Columns[1].DeclaredType != "TEXT" {
		t.Fatal("shared wire changed the complete nullable saved witness", request)
	}
	var result ProjectProfileSpeciesPresencePreview
	decoder = json.NewDecoder(bytes.NewReader(wire.Response))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.ContextID != "owned-context" || result.Mode != profileSpeciesPresenceMode ||
		result.Project != "Sample" || result.Table != "Sample_Profile" || result.SU != "None" ||
		result.TotalPlots != 52 || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("shared wire changed independent readonly result expectations", result)
	}
	for _, pair := range []struct {
		input  json.RawMessage
		output any
	}{{wire.Request, request}, {wire.Response, result}} {
		encoded, err := json.Marshal(pair.output)
		if err != nil {
			t.Fatal(err)
		}
		var original, reopened any
		if err := json.Unmarshal(pair.input, &original); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &reopened); err != nil || !reflect.DeepEqual(original, reopened) {
			t.Fatal("shared client/strict-Go wire lost fields or NULL values", original, reopened, err)
		}
	}
}
