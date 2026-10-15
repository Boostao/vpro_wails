package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSummaryVegetationClassClientWireFixture(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, false)
	cases := []SummaryVegetationPreview{}
	for _, display := range []int{4, 5} {
		for _, show := range []int{1, 10, 20} {
			for _, average := range []int{1, 2} {
				result, err := service.PreviewClass(context.Background(), state.ContextID, average, display, show)
				if err != nil {
					t.Fatal(err)
				}
				result.ContextID, result.Project, result.ProjectPath, result.SU, result.SUPath =
					"owned", "Sample", `C:\fixture\project # 'é'.db`, "Report", `C:\fixture\project # 'é'.db`
				cases = append(cases, result)
			}
		}
	}
	encoded, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("frontend", "src", "summaryVegetationClassWire.json")
	if os.Getenv("VPRO_UPDATE_SUMMARY_VEGETATION_WIRE") == "true" {
		if err := os.WriteFile(fixture, append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var expected []SummaryVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil || !reflect.DeepEqual(cases, expected) || len(cases) != 12 {
		t.Fatal("class wire differs from production DTO/physical read", err)
	}
}
