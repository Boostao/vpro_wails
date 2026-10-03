package main

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
			service, state := reportServiceFixture(t, external)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			report, err := service.previewLongVegetationLayers(context.Background(), state.ContextID)
			if err != nil || report.Project != state.ActiveProject || report.SU != state.ActiveSU ||
				report.Title != "Long Vegetation Report" || len(report.Units) != 2 {
				t.Fatal("owned selected-SU preview failed", report, err)
			}
			for _, unit := range report.Units {
				if vegetationTextKey(unit.Code) == "text:Orphan" {
					t.Fatal("orphan-only named unit invented source report")
				}
				if unit.NumPlots != 1 {
					t.Fatal("physical report denominator changed", unit)
				}
			}
			again, err := service.previewLongVegetationLayers(context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(report, again) {
				t.Fatal("repeat reads change result", err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("readonly preview changed configuration", err)
			}
		})
	}
}

func TestLongVegetationOwnedLayerPreviewCancellationStaleNoneAndInvalidOptions(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	originalPaths := map[string]string{}
	for role, path := range owner.attachments {
		originalPaths[role] = path
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if report, err := service.previewLongVegetationLayers(ctx, state.ContextID); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal("cancelled request produced report", report, err)
	}
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err := service.previewLongVegetationLayers(ctx, state.ContextID)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiting coordinator lease ignored cancellation", err)
	}
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.previewLongVegetationLayers(context.Background(), state.ContextID); err == nil {
		t.Fatal("stale context accepted")
	}
	if _, err := service.previewLongVegetationLayers(context.Background(), next.ContextID); err == nil ||
		!strings.Contains(err.Error(), "selected SU") {
		t.Fatal("None silently expanded to project scope", err)
	}
	for role, original := range before {
		observed, err := os.ReadFile(originalPaths[role])
		if err != nil || !bytes.Equal(original, observed) {
			t.Fatal("cancellation/switch changed original bytes", role, err)
		}
	}
	service, state = reportServiceFixture(t, true)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := bytes.Replace(config, []byte("LVGroupBy: 1"), []byte("LVGroupBy: 3"), 1)
	if bytes.Equal(config, invalid) {
		t.Fatal("invalid option fixture did not change intended setting")
	}
	if err := os.WriteFile(service.projects.preferences.path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if report, err := service.previewLongVegetationLayers(context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal("inactive grouping silently ignored", report, err)
	}
}
