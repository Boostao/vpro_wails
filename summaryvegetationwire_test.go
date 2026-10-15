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

func TestSummaryVegetationClientWireFixture(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, false)
	cases := []SummaryVegetationPreview{}
	for _, display := range []int{1, 2, 3, 6} {
		for _, average := range []int{1, 2} {
			result, err := service.Preview(context.Background(), state.ContextID, average, display)
			if err != nil {
				t.Fatal(err)
			}
			result.ContextID, result.Project, result.ProjectPath, result.SU, result.SUPath =
				"owned", "Sample", `C:\fixture\project # 'é'.db`, "Report", `C:\fixture\project # 'é'.db`
			cases = append(cases, result)
		}
	}
	encoded, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("frontend", "src", "summaryVegetationWire.json")
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
	if err := decoder.Decode(&expected); err != nil || !reflect.DeepEqual(cases, expected) {
		t.Fatal("client fixture differs from actual production DTO and physical read", err)
	}
}
