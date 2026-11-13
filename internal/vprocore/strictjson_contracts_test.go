package vprocore_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

func TestVProCoreStrictJSONDuplicateKeysAndTrailingTokensWhilePreservingCalibratedContracts(t *testing.T) {
	// 1. ScopedPlotLookup & PlotProfileSource (DecodeProfileLifecycleJSON callers)
	for _, payload := range []string{
		`{"plotNumber":"00337","plotNumber":"108050"}`,
		`{"plotNumber":"00337","PlotNumber":"108050"}`,
		`{"plotNumber":"00337"}{}`,
		`{"plotNumber":"00337"} 123`,
	} {
		keep := vprocore.ScopedPlotLookup{PlotNumber: "untouched"}
		before := keep
		if err := json.Unmarshal([]byte(payload), &keep); err == nil {
			t.Fatalf("ScopedPlotLookup accepted duplicate/trailing JSON: %s", payload)
		}
		if !reflect.DeepEqual(keep, before) {
			t.Fatalf("ScopedPlotLookup mutated target on error: %s", payload)
		}
	}

	for _, payload := range []string{
		`{"name":"Sample","path":"/tmp/a.db","Name":"Other"}`,
		`{"name":"Sample","path":"/tmp/a.db","pa\u0074h":"/tmp/b.db"}`,
		`{"name":"Sample","path":"/tmp/a.db"}{}`,
	} {
		var src vprocore.PlotProfileSource
		if err := json.Unmarshal([]byte(payload), &src); err == nil {
			t.Fatalf("PlotProfileSource accepted duplicate/trailing JSON: %s", payload)
		}
	}

	// 2. ProjectMetadataEdit
	validMeta := `{"plotNumber":"00337","projectId":null,"id":1,"columns":[],"original":{"rowId":"1","cells":[]},"changes":[],"standardPopulation":"all"}`
	var meta vprocore.ProjectMetadataEdit
	if err := json.Unmarshal([]byte(validMeta), &meta); err != nil {
		t.Fatalf("valid ProjectMetadataEdit rejected: %v", err)
	}
	for _, badMeta := range []string{
		strings.Replace(validMeta, `"plotNumber":"00337"`, `"plotNumber":"00337","plotNumber":"108050"`, 1),
		strings.Replace(validMeta, `"plotNumber":"00337"`, `"plotNumber":"00337","PlotNumber":"108050"`, 1),
		strings.Replace(validMeta, `"standardPopulation":"all"`, `"standardPopulation":"all","ſtandardPopulation":"none"`, 1),
		strings.Replace(validMeta, `"rowId":"1"`, `"rowId":"1","ROWID":"2"`, 1),
		validMeta + `{}`,
	} {
		if err := json.Unmarshal([]byte(badMeta), &vprocore.ProjectMetadataEdit{}); err == nil {
			t.Fatalf("ProjectMetadataEdit accepted duplicate/trailing JSON: %s", badMeta)
		}
	}

	// 3. Child records: reject exact + EqualFold duplicate keys + trailing tokens,
	// while preserving single Unicode-folded keys ("rootſSize":null) and unknown legacy fields!
	for _, badVeg := range []string{
		`{"species":"A","species":"B"}`,
		`{"species":"A","SPECIES":"B"}`,
		`{"species":"A","ſpecies":"B"}`,
		`{"species":"A"}{}`,
	} {
		if err := json.Unmarshal([]byte(badVeg), &vprocore.VegRecord{}); err == nil {
			t.Fatalf("VegRecord accepted duplicate/trailing JSON: %s", badVeg)
		}
	}
	for _, badHumus := range []string{
		`{"rootsSize":"1","rootſSize":"2"}`,
		`{"horizon":"L","HORIZON":"F"}`,
		`{"horizon":"L"} trailing`,
	} {
		if err := json.Unmarshal([]byte(badHumus), &vprocore.HumusRecord{}); err == nil {
			t.Fatalf("HumusRecord accepted duplicate/trailing JSON: %s", badHumus)
		}
	}
	for _, badMineral := range []string{
		`{"rootsSize":"1","rootſSize":"2"}`,
		`{"texture":"SL","Texture":"LS"}`,
		`{"texture":"SL"}{}`,
	} {
		if err := json.Unmarshal([]byte(badMineral), &vprocore.MineralRecord{}); err == nil {
			t.Fatalf("MineralRecord accepted duplicate/trailing JSON: %s", badMineral)
		}
	}
	for _, badOther := range []string{
		`{"dataName":"A","DataName":"B"}`,
		`{"dataName":"A"}{}`,
	} {
		if err := json.Unmarshal([]byte(badOther), &vprocore.OtherRecord{}); err == nil {
			t.Fatalf("OtherRecord accepted duplicate/trailing JSON: %s", badOther)
		}
	}

	// Preserved child contracts: single Unicode folded key + unknown legacy property tolerance
	var humus vprocore.HumusRecord
	if err := json.Unmarshal([]byte(`{"rootſSize":null,"legacyUnknownProp":"tolerated"}`), &humus); err != nil {
		t.Fatalf("HumusRecord rejected single folded key or unknown legacy prop: %v", err)
	}
	if !vprocore.ChildPresence(humus)["rootssize"] {
		t.Fatal("HumusRecord lost childPresent for single folded key rootſSize")
	}

	// 4. FS882Header: reject duplicate keys while preserving fieldNotes \ud800 and single case-folded keys
	for _, badHeader := range []string{
		`{"plotNumber":"00337","plotNumber":"108050"}`,
		`{"plotNumber":"00337","PlotNumber":"108050"}`,
		`{"sitePlotQuality":"Poor","SitePlotQuality":"Good"}`,
		`{"sitePlotQuality":"Poor","ſitePlotQuality":"Good"}`,
	} {
		h := vprocore.FS882Header{PlotNumber: "unchanged"}
		before := h
		if err := json.Unmarshal([]byte(badHeader), &h); err == nil {
			t.Fatalf("FS882Header accepted duplicate key: %s", badHeader)
		}
		if !reflect.DeepEqual(h, before) {
			t.Fatalf("FS882Header mutated destination on error: %s", badHeader)
		}
	}
	var h vprocore.FS882Header
	if err := json.Unmarshal([]byte(`{"fieldNotes":"\ud800"}`), &h); err != nil || h.FieldNotes == nil || *h.FieldNotes != "\uFFFD" {
		t.Fatalf("FS882Header broke calibrated fieldNotes per-slice contract: err=%v, val=%v", err, h.FieldNotes)
	}
	if err := json.Unmarshal([]byte(`{"sItEpLoTqUaLiTy":"Good"}`), &h); err != nil || h.SitePlotQuality == nil || *h.SitePlotQuality != "Good" {
		t.Fatalf("FS882Header broke single case-folded key contract: err=%v", err)
	}
	// Pre-populated pointer on FS882Header must NOT be overwritten in place on later field type error!
	keepQual := "Good"
	preH := vprocore.FS882Header{PlotNumber: "unchanged", SitePlotQuality: &keepQual}
	if err := json.Unmarshal([]byte(`{"sitePlotQuality":"Poor","elevation":"not-a-number"}`), &preH); err == nil {
		t.Fatal("FS882Header accepted invalid elevation type")
	}
	if preH.SitePlotQuality == nil || *preH.SitePlotQuality != "Good" || keepQual != "Good" {
		t.Fatalf("FS882Header mutated pre-populated pointer on type error: %q / %q", *preH.SitePlotQuality, keepQual)
	}
}

