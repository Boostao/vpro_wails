package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageSoilReferencesStrictGoClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageSoilReferenceWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	type owner struct {
		ContextID string `json:"contextId"`
		Project   string `json:"project"`
		Plot      string `json:"plot"`
		Form      string `json:"form"`
	}
	type fixture struct {
		Owner owner                 `json:"owner"`
		Wire  TwoPageSoilReferences `json:"wire"`
	}
	var fixtures []fixture
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 2 {
		t.Fatal("shared fixture must cover both actual facade forms")
	}
	for i, test := range []struct {
		form     string
		bindings [][2]string
		counts   []int
	}{
		{"SoilHumusXL", [][2]string{
			{"Horizon", "HumusHorizon"}, {"HumusStructureKind", "HumusStructureKind"},
			{"HumusStructureDegree", "HumusStructureDegree"}, {"vonPost", "vonPost"},
			{"MycelAbundance", "MycelAbundance"}, {"FecalAbundance", "MycelAbundance"},
		}, []int{20, 9, 4, 11, 5, 5}},
		{"SoilMineralXL", [][2]string{
			{"ASP", "MinSoilAspect"}, {"Texture", "SoilTexture"},
			{"MineralStructureClass", "MineralStructureClass"}, {"MineralStructureKind", "MineralStructureKind"},
		}, []int{15, 30, 6, 10}},
	} {
		got := fixtures[i]
		if got.Owner != (owner{"soil-reference-wire-context", "Sample", "108050", test.form}) ||
			got.Wire.ContextID != got.Owner.ContextID || got.Wire.Project != got.Owner.Project ||
			got.Wire.Plot != got.Owner.Plot || got.Wire.Form != test.form || len(got.Wire.Fields) != len(test.bindings) {
			t.Fatal("actual Go/client wire owner or form differs", got.Owner)
		}
		for j, binding := range test.bindings {
			field := got.Wire.Fields[j]
			if field.Column != binding[0] || field.ListName != binding[1] || len(field.Definitions.Rows) != test.counts[j] ||
				field.Definitions.Columns[0].Name != "ListName" || field.Definitions.Columns[0].DeclaredType != "TEXT" ||
				field.Definitions.Columns[7] != (ProjectMetadataColumn{"Validate", "BOOLEAN"}) {
				t.Fatal("production wire mapping/schema/counts differ", binding)
			}
			for _, row := range field.Definitions.Rows {
				if len(row.Cells) != 10 {
					t.Fatal("production wire omitted complete metadata cells")
				}
				for _, cell := range row.Cells {
					if _, err := metadataCellValue(cell); err != nil {
						t.Fatal("production wire repaired or lost original storage", err)
					}
				}
			}
		}
	}
	encoded, err := json.Marshal(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	var reopened []fixture
	decoder = json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reopened); err != nil || !reflect.DeepEqual(reopened, fixtures) {
		t.Fatal("strict Go/reference client roundtrip differs", err)
	}
}
