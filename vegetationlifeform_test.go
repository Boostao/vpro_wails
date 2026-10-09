package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLongVegetationLifeformDecodeAndDefaultIndependentGate(t *testing.T) {
	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["LVGroupBy"] = 3
	options, err := decodeLongVegetationOptions(values)
	if err != nil || !options.LifeformGrouping || options.NoneGrouping || vegetationSettings(options).Grouping != "lifeform" {
		t.Fatal("source Lifeform grouping lost explicit identity", options, err)
	}
	service, state := reportServiceFixture(t, false)
	if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 3}); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, noneEnabled := range []bool{false, true} {
		service.longVegetationNoneEnabled = noneEnabled
		if result, err := service.GetLongVegetationOptions(context.Background(), state.ContextID); err == nil || !reflect.DeepEqual(result, LongVegetationOptions{}) {
			t.Fatal("None enabled Lifeform options", result, err)
		}
		if result, err := service.PreviewLongVegetation(context.Background(), state.ContextID); err == nil || !reflect.DeepEqual(result, LongVegetationPreview{}) {
			t.Fatal("default Lifeform preview published data", result, err)
		}
		if result, err := service.previewLongVegetationLayers(context.Background(), state.ContextID); err == nil || !reflect.DeepEqual(result, vegetationLayerReport{}) {
			t.Fatal("private context path bypassed Lifeform gate", result, err)
		}
	}
	assertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, after) {
		t.Fatal("Lifeform default refusal changed retained settings", err)
	}
	if enabled, err := siviFeature(longVegetationLifeformFeatureEnvironment, func(string) (string, bool) { return "", false }); err != nil || enabled {
		t.Fatal("Lifeform default gate enabled", err)
	}
	if _, err := siviFeature(longVegetationLifeformFeatureEnvironment, func(string) (string, bool) { return "TRUE", true }); err == nil {
		t.Fatal("malformed Lifeform gate accepted")
	}
	options.NoneGrouping = true
	if err := service.checkLongVegetationGrouping(options); err == nil {
		t.Fatal("conflicting grouping flags accepted")
	}
}

func TestLongVegetationLifeformOwnedReaderDispatchIdentityAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := reportServiceFixture(t, external)
		if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 3}); err != nil {
			t.Fatal(err)
		}
		service.longVegetationLifeformEnabled = true
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		options, err := service.GetLongVegetationOptions(context.Background(), state.ContextID)
		if err != nil || options.Settings.Grouping != "lifeform" {
			t.Fatal("owned Lifeform settings unavailable", options, err)
		}
		result, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
		if err != nil || result.ContextID != state.ContextID || result.Settings.Grouping != "lifeform" ||
			result.ProjectPath != options.ProjectPath || result.SUPath != options.SUPath || len(result.Report.Units) == 0 {
			t.Fatal("Lifeform reader lost full owned scope", result, err)
		}
		rows := 0
		for _, unit := range result.Report.Units {
			for _, row := range unit.Rows {
				rows++
				if row.Layer.Storage != "integer" && row.Layer.Storage != "null" {
					t.Fatal("Lifeform dispatch fell through to source Layer labels", row)
				}
			}
		}
		if rows == 0 {
			t.Fatal("Lifeform reader manufactured an empty success")
		}
		if result, err := service.PreviewLongVegetation(context.Background(), "stale"); err == nil || !reflect.DeepEqual(result, LongVegetationPreview{}) {
			t.Fatal("stale Lifeform request published data", result, err)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if result, err := service.PreviewLongVegetation(cancelled, state.ContextID); err == nil || !reflect.DeepEqual(result, LongVegetationPreview{}) {
			t.Fatal("cancelled Lifeform request published data", result, err)
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("Lifeform reader wrote preferences", err)
		}
	}
}

func TestLongVegetationLifeformRequiresOriginalPersonalTableWithoutLayerFallback(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": 3}); err != nil {
		t.Fatal(err)
	}
	service.longVegetationLifeformEnabled = true
	original, err := os.ReadFile(service.projects.supportPaths["VUser"])
	if err != nil {
		t.Fatal(err)
	}
	personalPath := filepath.Join(t.TempDir(), "personal-missing.db")
	if err := os.WriteFile(personalPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, personalPath, `ALTER TABLE USysUserSpp RENAME TO MissingPersonal`)
	service.projects.supportPaths["VUser"] = personalPath
	if _, err := service.SwitchContext(state.ContextID, contextSelection(state)); err == nil ||
		!strings.Contains(err.Error(), "VUser.USysUserSpp") {
		t.Fatal("context accepted missing original personal source", err)
	}
	current, err := service.projects.GetState(context.Background())
	if err != nil || current.ContextID != state.ContextID {
		t.Fatal("rejected personal context destroyed the existing owner", current, err)
	}
	if result, err := service.PreviewLongVegetation(context.Background(), state.ContextID); err != nil ||
		result.Settings.Grouping != "lifeform" || result.ContextID != state.ContextID {
		t.Fatal("old owned Lifeform reader was replaced or fell back after rejected context", result, err)
	}
}
