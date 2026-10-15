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

func TestSummaryVegetationGroupedModesClientWireFixture(t *testing.T) {
	service, state := summaryVegetationGroupedServiceFixture(t, false)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	cases := []SummaryVegetationPreview{}
	for _, grouping := range []int{12, 13, 14} {
		for _, display := range []int{1, 2, 3, 4, 5} {
			shows := []int{0}
			if display == 4 || display == 5 {
				shows = []int{1, 10, 20}
			}
			for _, show := range shows {
				for _, average := range []int{1, 2} {
					result, err := service.PreviewGroupedDisplay(context.Background(), state.ContextID, average, grouping, display, show)
					if display == 2 && grouping != 14 {
						if err == nil || !reflect.DeepEqual(result, SummaryVegetationPreview{}) {
							t.Fatal("source failure was silently enabled", result, err)
						}
						continue
					}
					if err != nil || result.OrderField.DAOType != 10 || result.OrderField.Size != 0 {
						t.Fatal(grouping, display, show, average, result, err)
					}
					result.ContextID, result.Project, result.ProjectPath, result.SU, result.SUPath =
						"owned", "Sample", `C:\fixture\project.db`, "Report", `C:\fixture\project.db`
					cases = append(cases, result)
				}
			}
		}
	}
	assertProfileSUFiles(t, service.contexts, before)
	encoded, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("frontend", "src", "summaryVegetationGroupedModesWire.json")
	if os.Getenv("VPRO_UPDATE_SUMMARY_VEGETATION_GROUPED_MODES_WIRE") == "true" {
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
	if err := decoder.Decode(&expected); err != nil || !reflect.DeepEqual(cases, expected) || len(cases) != 50 {
		t.Fatal("actual Go additional grouped wire differs", len(cases), err)
	}
	for _, options := range [][4]int{{1, 11, 1, 0}, {1, 12, 4, 0}, {1, 14, 4, 2}, {1, 14, 1, 10}} {
		if _, err := service.PreviewGroupedDisplay(context.Background(), state.ContextID, options[0], options[1], options[2], options[3]); err == nil {
			t.Fatal("invalid grouped options accepted", options)
		}
	}
	if _, err := service.PreviewGroupedDisplay(context.Background(), "foreign", 1, 14, 2, 0); err == nil {
		t.Fatal("foreign grouped mode owner accepted")
	}
	service.enabled = false
	if _, err := service.PreviewGroupedDisplay(context.Background(), state.ContextID, 1, 14, 2, 0); err == nil {
		t.Fatal("additional grouped mode gate bypassed")
	}
}
