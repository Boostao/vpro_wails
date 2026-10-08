package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSummaryLifeformPreviewGateStrictTransportAndReadOnlyOverride(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	if value, err := service.PreviewSiteUnitSummaryLifeforms(context.Background(), state.ContextID, SiteUnitSummaryRequest{1}); err == nil || !reflect.DeepEqual(value, SiteUnitSummaryPreview{}) {
		t.Fatal("default-off source preview returned output", value, err)
	}
	service.siteUnitSummaryLifeformsEnabled = true
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []int{1, 2} {
		got, err := service.PreviewSiteUnitSummaryLifeforms(context.Background(), state.ContextID, SiteUnitSummaryRequest{method})
		if err != nil || len(got.Report.Fields) != 49 || got.Report.Method != method ||
			got.Report.QuerySource != "selected-su-filtered-env-admin-quickveg-lifeform" {
			t.Fatal("complete source preview unavailable", got, err)
		}
		input, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
		if err != nil {
			t.Fatal(err)
		}
		for i, unit := range got.Report.Units {
			if len(unit.Values) != 49 || !reflect.DeepEqual(unit.Plots, input.Environment.Report.Units[i].Plots) ||
				!reflect.DeepEqual(unit.Values[:23], input.Environment.Report.Units[i].Values[:23]) ||
				!reflect.DeepEqual(unit.Values[37:], input.Environment.Report.Units[i].Values[27:]) {
				t.Fatal("unrelated source fields or physical join scope changed", unit)
			}
			for form := range siteUnitLifeformCaptions {
				if unit.Values[23+form] != input.Lifeform[i].Rows[form].Value ||
					got.Report.Fields[23+form].Label != siteUnitLifeformCaptions[form] {
					t.Fatal("source caption/value projection differs", form)
				}
			}
		}
	}
	assertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || string(after) != string(config) {
		t.Fatal("explicit lifeform preview changed saved report scope", err)
	}
	for _, raw := range []string{`{}`, `{"method":1,"method":2}`, `{"method":null}`, `{"method":1,"mode":2}`} {
		var request SiteUnitSummaryRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("malformed source request accepted", raw)
		}
	}
	if _, err := siviFeature(siteUnitSummaryLifeformFeatureEnvironment, func(string) (string, bool) { return "bad", true }); err == nil {
		t.Fatal("malformed runtime feature silently accepted")
	}
}

func TestSummaryLifeformPreviewRefusalsAndDetachedProjection(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	input, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := siteUnitSummaryLifeformPreview(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	got.Report.Fields[0].Label = "changed"
	got.Report.Units[0].Values[0] = "changed"
	if input.Environment.Report.Fields[0].Label == "changed" || input.Environment.Report.Units[0].Values[0] == "changed" {
		t.Fatal("lifeform projection borrowed unrelated report values")
	}
	input.Lifeform[0].Code = "foreign"
	if value, err := siteUnitSummaryLifeformPreview(context.Background(), input); err == nil || !reflect.DeepEqual(value, SiteUnitSummaryPreview{}) {
		t.Fatal("foreign scalar unit published partial report", value, err)
	}
	service.siteUnitSummaryLifeformsEnabled = true
	for _, method := range []int{0, 3} {
		if value, err := service.PreviewSiteUnitSummaryLifeforms(context.Background(), state.ContextID, SiteUnitSummaryRequest{method}); err == nil || !reflect.DeepEqual(value, SiteUnitSummaryPreview{}) {
			t.Fatal("invalid source method accepted", method, value, err)
		}
	}
	if _, err := service.PreviewSiteUnitSummaryLifeforms(nil, state.ContextID, SiteUnitSummaryRequest{1}); err == nil {
		t.Fatal("missing context accepted")
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewSiteUnitSummaryLifeforms(context.Background(), state.ContextID, SiteUnitSummaryRequest{1}); err == nil {
		t.Fatal("stale source context accepted")
	}
	if _, err := service.PreviewSiteUnitSummaryLifeforms(context.Background(), next.ContextID, SiteUnitSummaryRequest{1}); err != nil {
		t.Fatal("stale refusal leaked ownership lease", err)
	}
}
