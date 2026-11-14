package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	service, _ := vprotest.ReportServiceFixture(t, false)
	options.NoneGrouping = true
	if err := CheckLongVegetationGrouping(service, options); err == nil {
		t.Fatal("conflicting grouping flags accepted")
	}
}

func TestLongVegetationLifeformOwnedReaderDispatchIdentityAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := vprotest.ReportServiceFixture(t, external)
		if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LVGroupBy": 3}); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		config, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		options, err := GetLongVegetationOptions(service, context.Background(), state.ContextID)
		if err != nil || options.Settings.Grouping != "lifeform" {
			t.Fatal("owned Lifeform settings unavailable", options, err)
		}
		result, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
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
		if result, err := PreviewLongVegetation(service, context.Background(), "stale"); err == nil || !reflect.DeepEqual(result, LongVegetationPreview{}) {
			t.Fatal("stale Lifeform request published data", result, err)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if result, err := PreviewLongVegetation(service, cancelled, state.ContextID); err == nil || !reflect.DeepEqual(result, LongVegetationPreview{}) {
			t.Fatal("cancelled Lifeform request published data", result, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("Lifeform reader wrote preferences", err)
		}
	}
}

func TestLongVegetationLifeformRequiresOriginalPersonalTableWithoutLayerFallback(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LVGroupBy": 3}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(service.Projects.SupportPaths["VUser"])
	if err != nil {
		t.Fatal(err)
	}
	personalPath := filepath.Join(t.TempDir(), "personal-missing.db")
	if err := os.WriteFile(personalPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	vprotest.MutateContextFixture(t, personalPath, `ALTER TABLE USysUserSpp RENAME TO MissingPersonal`)
	service.Projects.SupportPaths["VUser"] = personalPath
	if _, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state)); err == nil ||
		!strings.Contains(err.Error(), "VUser.USysUserSpp") {
		t.Fatal("context accepted missing original personal source", err)
	}
	current, err := service.Projects.GetState(context.Background())
	if err != nil || current.ContextID != state.ContextID {
		t.Fatal("rejected personal context destroyed the existing owner", current, err)
	}
	if result, err := PreviewLongVegetation(service, context.Background(), state.ContextID); err != nil ||
		result.Settings.Grouping != "lifeform" || result.ContextID != state.ContextID {
		t.Fatal("old owned Lifeform reader was replaced or fell back after rejected context", result, err)
	}
}
