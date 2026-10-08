package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLongVegetationQualityServiceOwnedFanoutNullExclusionAndRetry(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			owner := service.projects.sqlite
			db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["project"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`INSERT INTO Sample_Env(PlotNumber) SELECT '108050x'
				WHERE NOT EXISTS(SELECT 1 FROM Sample_Env WHERE PlotNumber='108050x');
				INSERT INTO Sample_Admin(Plot) SELECT '108050x'
				WHERE NOT EXISTS(SELECT 1 FROM Sample_Admin WHERE Plot='108050x');
				UPDATE Sample_Admin SET SitePlotQuality='Good',VegPlotQuality='Good',SoilPlotQuality='Good' WHERE Plot='108050';
				UPDATE Sample_Admin SET SitePlotQuality='Fair',VegPlotQuality='Fair',SoilPlotQuality='Fair' WHERE Plot='108050x'`)
			if err != nil {
				t.Fatal(err)
			}
			lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer lists.Close()
			var count int
			if err := lists.QueryRow(`SELECT COUNT(*) FROM USysTableOfLists WHERE Item='Good' AND ListName='DataQuality'`).Scan(&count); err != nil || count != 1 {
				t.Fatal("original DataQuality fixture differs", count, err)
			}
			if _, err := lists.Exec(`INSERT INTO USysTableOfLists(Item,ListName,ItemOrder) VALUES('Good','DataQuality',3)`); err != nil {
				t.Fatal(err)
			}
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": -1}); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, owner.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			options, err := service.GetLongVegetationOptions(context.Background(), state.ContextID)
			if err != nil || options.Settings.Quality == nil || options.Settings.Quality.Site.Minimum != "Poor" ||
				!options.Settings.Quality.Veg.IncludeNull || !options.Settings.Quality.Soil.IncludeNull {
				t.Fatal("literal default-init/source flags not decoded", options, err)
			}
			preview, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || preview.Report.Quality == nil || len(preview.Report.Quality.Occurrences) != 9 ||
				!reflect.DeepEqual(preview.Report.Quality.ExcludedMembershipIDs, []string{"2"}) ||
				!reflect.DeepEqual(preview.Settings, options.Settings) || len(preview.Report.Units) != 2 {
				t.Fatal("owned quality provenance/fanout/scope differs", preview, err)
			}
			for _, unit := range preview.Report.Units {
				want := 8
				if unit.Code.Storage == "null" {
					want = 1
				}
				if unit.NumPlots != want || len(unit.MembershipIDs) != 1 {
					t.Fatal("qualified occurrence denominator lost original physical IDs", unit)
				}
			}
			service.longVegetationLifeformEnabled = true
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 3}); err != nil {
				t.Fatal(err)
			}
			lifeform, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || lifeform.Settings.Grouping != "lifeform" ||
				!reflect.DeepEqual(lifeform.Report.Quality, preview.Report.Quality) ||
				len(lifeform.Report.Units) != len(preview.Report.Units) {
				t.Fatal("Lifeform reader changed quality source indices or qualification", lifeform, err)
			}
			for i, unit := range lifeform.Report.Units {
				if unit.NumPlots != preview.Report.Units[i].NumPlots ||
					!reflect.DeepEqual(unit.MembershipIDs, preview.Report.Units[i].MembershipIDs) {
					t.Fatal("Lifeform conversion lost qualified weights or physical ownership", unit)
				}
				for _, row := range unit.Rows {
					if row.Layer.Storage != "integer" && row.Layer.Storage != "null" {
						t.Fatal("quality-qualified Lifeform report used Layer fallback", row)
					}
				}
			}
			service.longVegetationStrataEnabled = true
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 2}); err != nil {
				t.Fatal(err)
			}
			strata, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || strata.Settings.Grouping != "strata" ||
				!reflect.DeepEqual(strata.Report.Quality, preview.Report.Quality) ||
				len(strata.Report.Units) != len(preview.Report.Units) {
				t.Fatal("Strata reader changed source quality qualification", strata, err)
			}
			for i, unit := range strata.Report.Units {
				if unit.NumPlots != preview.Report.Units[i].NumPlots ||
					!reflect.DeepEqual(unit.MembershipIDs, preview.Report.Units[i].MembershipIDs) {
					t.Fatal("Strata lost quality weights/physical identities", unit)
				}
			}
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 1}); err != nil {
				t.Fatal(err)
			}
			again, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(preview, again) {
				t.Fatal("repeated qualified reads differ", err)
			}
			service.longVegetationCodeEnabled = true
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVShowEnglishName": 2}); err != nil {
				t.Fatal(err)
			}
			codes, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || !codes.Settings.ShowSpeciesCode || codes.Settings.ShowEnglishName ||
				!reflect.DeepEqual(codes.Report.Quality, preview.Report.Quality) ||
				len(codes.Report.Units) != len(preview.Report.Units) {
				t.Fatal("supplementary Code changed quality qualification", codes, err)
			}
			for i, unit := range codes.Report.Units {
				if unit.NumPlots != preview.Report.Units[i].NumPlots ||
					!reflect.DeepEqual(unit.MembershipIDs, preview.Report.Units[i].MembershipIDs) {
					t.Fatal("Code mode lost physical/qualified weights", unit)
				}
			}
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVShowEnglishName": 1}); err != nil {
				t.Fatal(err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("quality read rewrote persistent YAML", err)
			}
			for _, update := range []map[string]any{
				{"DataQualityFilterSiteLV": "Excellent"},
				{"DataQualityFilterSiteLV": "NA"},
			} {
				if err := service.projects.preferences.update("ReportOptions", update); err != nil {
					t.Fatal(err)
				}
				if value, err := service.PreviewLongVegetation(context.Background(), state.ContextID); err == nil ||
					!reflect.DeepEqual(value, LongVegetationPreview{}) {
					t.Fatal("empty/missing quality threshold returned partial successful report", value, err)
				}
				assertProfileSUFiles(t, service, before)
			}
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterSiteLV": "Poor"}); err != nil {
				t.Fatal(err)
			}
			retry, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(retry, preview) {
				t.Fatal("quality rejection lost owned snapshot/attachments", retry, err)
			}
			if _, err := db.Exec(`UPDATE Sample_Admin SET SitePlotQuality=NULL WHERE Plot='108050'`); err != nil {
				t.Fatal(err)
			}
			nullBefore := databaseBytes(t, owner.attachments)
			nullPreview, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || nullPreview.Report.Quality == nil || len(nullPreview.Report.Quality.Occurrences) != 5 {
				t.Fatal("NULL joined order did not obey include flag", nullPreview, err)
			}
			for _, occurrence := range nullPreview.Report.Quality.Occurrences {
				if *occurrence.PlotNumber.Text == "108050" && occurrence.ListRowIDs[0] != nil {
					t.Fatal("NULL quality invented physical reference ID", occurrence)
				}
			}
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterSiteNullLV": "False"}); err != nil {
				t.Fatal(err)
			}
			excluded, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
			if err != nil || len(excluded.Report.Units) != 1 || excluded.Report.Units[0].Code.Storage != "null" ||
				len(excluded.Report.Quality.Occurrences) != 1 {
				t.Fatal("explicit legacy False failed to exclude NULL order", excluded, err)
			}
			assertProfileSUFiles(t, service, nullBefore)
		})
	}
}

func TestLongVegetationQualityTransportClonesEveryProvenanceBoundary(t *testing.T) {
	su, env, admin, lists := vegetationQualityFixture()
	selection, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Poor", 7))
	if err != nil {
		t.Fatal(err)
	}
	source := vegetationLayerReport{Project: "P", SU: "S", Quality: &selection}
	value, err := longVegetationTransport(context.Background(), source)
	if err != nil || value.Quality == nil {
		t.Fatal(err)
	}
	*value.Quality.Occurrences[0].PlotNumber.Text = "Changed"
	*value.Quality.Occurrences[0].SiteUnit.Text = "Other"
	*value.Quality.Occurrences[0].ListRowIDs[0] = "Changed"
	*value.Quality.Occurrences[0].Values[0].Text = "Altered"
	*value.Quality.References[0].Item.Text = "Mutated"
	value.Quality.ThresholdRowIDs[0][0] = "Other"
	if *selection.Occurrences[0].Membership.PlotNumber.Text != "P" ||
		*selection.Occurrences[0].Membership.SiteUnit.Text != "  Unit  " ||
		*selection.Occurrences[0].ListRowIDs[0] != "3" || selection.ThresholdRowIDs[0][0] != "1" ||
		*selection.Occurrences[0].Values[0].Text != "Good" || *selection.References[0].Item.Text != "Poor" {
		t.Fatal("public provenance aliases owned metadata", selection)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := longVegetationTransport(ctx, source); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(value, LongVegetationReport{}) {
		t.Fatal("cancelled quality transport exposed partial report", value, err)
	}
	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["DataQualityFilterEnforceLV"] = true
	options, err := decodeLongVegetationOptions(values)
	if err != nil || options.Quality == nil {
		t.Fatal(err)
	}
	settings := vegetationSettings(options)
	settings.Quality.Site.Minimum = "Other"
	if (*options.Quality)[0].Minimum != "Poor" {
		t.Fatal("settings alias raw quality criteria")
	}
	values["ReportOptions"].(map[string]any)["DataQualityFilterSiteNullLV"] = nil
	if _, err := decodeLongVegetationOptions(values); err == nil || !strings.Contains(err.Error(), "DataQualityFilterSiteNullLV") {
		t.Fatal("active invalid NULL toggle silently defaulted", err)
	}
}
