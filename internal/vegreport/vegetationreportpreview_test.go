package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLongVegetationOwnedLayerPreviewExternalScopeAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := vprotest.ReportServiceFixture(t, external)
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			report, err := PreviewLongVegetationLayers(service, context.Background(), state.ContextID)
			if err != nil || report.Project != state.ActiveProject || report.SU != state.ActiveSU ||
				report.Title != "Long Vegetation Report" || len(report.Units) != 2 {
				t.Fatal("owned selected-SU preview failed", report, err)
			}
			for _, unit := range report.Units {
				if reportcore.VegetationTextKey(unit.Code) == "text:Orphan" {
					t.Fatal("orphan-only named unit invented source report")
				}
				if unit.NumPlots != 1 {
					t.Fatal("physical report denominator changed", unit)
				}
			}
			again, err := PreviewLongVegetationLayers(service, context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(report, again) {
				t.Fatal("repeat reads change result", err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("readonly preview changed configuration", err)
			}
		})
	}
}

func TestLongVegetationOwnedLayerPreviewCancellationStaleNoneAndInvalidOptions(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	owner := service.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	originalPaths := map[string]string{}
	for role, path := range owner.Attachments {
		originalPaths[role] = path
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if report, err := PreviewLongVegetationLayers(service, ctx, state.ContextID); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal("cancelled request produced report", report, err)
	}
	owner.Mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err := PreviewLongVegetationLayers(service, ctx, state.ContextID)
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiting coordinator lease ignored cancellation", err)
	}
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewLongVegetationLayers(service, context.Background(), state.ContextID); err == nil {
		t.Fatal("stale context accepted")
	}
	if _, err := PreviewLongVegetationLayers(service, context.Background(), next.ContextID); err == nil ||
		!strings.Contains(err.Error(), "selected SU") {
		t.Fatal("None silently expanded to project scope", err)
	}
	for role, original := range before {
		observed, err := os.ReadFile(originalPaths[role])
		if err != nil || !bytes.Equal(original, observed) {
			t.Fatal("cancellation/switch changed original bytes", role, err)
		}
	}
	service, state = vprotest.ReportServiceFixture(t, true)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := bytes.Replace(config, []byte("LVGroupBy: 1"), []byte("LVGroupBy: 3"), 1)
	if bytes.Equal(config, invalid) {
		t.Fatal("invalid option fixture did not change intended setting")
	}
	if err := os.WriteFile(service.Projects.Preferences.Path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if report, err := PreviewLongVegetationLayers(service, context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal("inactive grouping silently ignored", report, err)
	}
}
