package main

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"testing"
)

func TestProjectMetadataConfirmedStandardProposalsRemainExplicitAssignments(t *testing.T) {
	raw, err := os.ReadFile("resources/project-metadata-standard.json")
	if err != nil {
		t.Fatal(err)
	}
	var defaults map[string]string
	if err := json.Unmarshal(raw, &defaults); err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 24 || defaults["CoverA1Description"] != "Dominate trees" ||
		defaults["GeoRefMethod"] != "GPS +/- 10m" || defaults["CoordinateSystem"] != "dd.mm.ss.s" {
		t.Fatal("source-standard literal contract changed")
	}
	source, err := os.ReadFile("testdata/projectmetadata/standard-population.vba")
	if err != nil {
		t.Fatal(err)
	}
	sourceDefaults := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^\s*Me\.(\w+) = "([^"]*)"\r?$`).FindAllStringSubmatch(string(source), -1) {
		sourceDefaults[match[1]] = match[2]
	}
	if !reflect.DeepEqual(defaults, sourceDefaults) {
		t.Fatal("desktop defaults differ from the source prompt's literal assignments")
	}
	for _, standard := range []string{"DEIF 2024", "DTE 2024", "LMH25"} {
		t.Run(standard, func(t *testing.T) {
			service, state, request := metadataEditFixture(t)
			before := metadataFileBytes(t, service)
			request.StandardPopulation = "populate"
			request.Changes = []ProjectMetadataChange{{"EcosysCollectionStandard", metadataText(standard)}}
			for name, value := range defaults {
				policy, present := projectMetadataFields[name]
				if !present || policy.kind != "text" || policy.limitToList {
					t.Fatal("source population lost its ordinary text domain:", name)
				}
				request.Changes = append(request.Changes, ProjectMetadataChange{name, metadataText(value)})
			}
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
				t.Fatal(err)
			}
			review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
			if err != nil {
				t.Fatal(err)
			}
			for name, value := range defaults {
				actual := metadataTestCell(t, review.ProjectRecords, "-200", name)
				if actual.Text == nil || *actual.Text != value {
					t.Fatal("source proposal changed:", name, actual)
				}
			}
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			var audits int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE "Table"='_Metadata' AND ID=-200`).Scan(&audits); err != nil || audits != 28 {
				t.Fatal("25 explicit fields and3 stamps must share28 audits:", audits, err)
			}
			assertMetadataFileBytes(t, service, before, "project")
		})
	}
}

func TestProjectMetadataPopulationDecisionCannotInventDefaultsOrRecognizedStandard(t *testing.T) {
	for _, standard := range []string{"", "OTHER", " LMH25", "LMH25 "} {
		t.Run(standard, func(t *testing.T) {
			service, state, request := metadataEditFixture(t)
			before := metadataFileBytes(t, service)
			request.StandardPopulation = "populate"
			request.Changes = append(request.Changes, ProjectMetadataChange{"EcosysCollectionStandard", metadataText(standard)})
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("population accepted without a changed recognized source standard")
			}
			assertMetadataFileBytes(t, service, before, "")
		})
	}
	service, state, request := metadataEditFixture(t)
	request.StandardPopulation = "populate"
	request.Changes = []ProjectMetadataChange{{"EcosysCollectionStandard", metadataText("DEIF 2024")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	if metadataTestCell(t, review.ProjectRecords, "-200", "CoordinatingAgency").Storage != "null" {
		t.Fatal("backend silently invented source defaults absent from explicit assignments")
	}
}
