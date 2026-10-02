package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

func contextServiceFixture(t *testing.T) (*ContextService, ProjectState) {
	t.Helper()
	projects, _, _, _ := sqliteServiceFixture(t)
	plots, err := newPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewContextService(projects, plots)
	if err != nil {
		t.Fatal(err)
	}
	state, err := projects.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func contextSelection(state ProjectState) ContextSelection {
	return ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: state.ActiveSU, SUPath: state.SUPath, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath}
}

func TestContextServiceRejectsStaleReadsEveryMutationAndLegacyBypass(t *testing.T) {
	service, old := contextServiceFixture(t)
	current, err := service.SwitchContext(old.ContextID, contextSelection(old))
	if err != nil || current.ContextID == old.ContextID {
		t.Fatal("context publication did not rotate identity")
	}
	before, err := os.ReadFile(current.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := old.ContextID
	cases := map[string]func() error{
		"create header":         func() error { return service.CreatePlot(stale, FS882Header{PlotNumber: "stale"}) },
		"update header":         func() error { return service.UpdatePlot(stale, FS882Header{PlotNumber: "stale"}) },
		"save veg":              func() error { return service.SaveVegRecord(stale, VegRecord{}) },
		"update veg":            func() error { return service.UpdateVegRecord(stale, VegRecord{}) },
		"delete veg":            func() error { return service.DeleteVegRecord(stale, "stale", 1) },
		"save humus":            func() error { return service.SaveHumusRecord(stale, HumusRecord{}) },
		"update humus":          func() error { return service.UpdateHumusRecord(stale, HumusRecord{}) },
		"delete humus":          func() error { return service.DeleteHumusRecord(stale, "stale", 1) },
		"save mineral":          func() error { return service.SaveMineralRecord(stale, MineralRecord{}) },
		"update mineral":        func() error { return service.UpdateMineralRecord(stale, MineralRecord{}) },
		"delete mineral":        func() error { return service.DeleteMineralRecord(stale, "stale", 1) },
		"save other":            func() error { return service.SaveOtherRecord(stale, OtherRecord{}) },
		"update other":          func() error { return service.UpdateOtherRecord(stale, OtherRecord{}) },
		"other drafts":          func() error { return service.UpdateOtherRecords(stale, "stale", nil) },
		"soil drafts":           func() error { return service.UpdateSoilRecords(stale, "stale", nil) },
		"vegetation attributes": func() error { return service.UpdateVegetationAttributes(stale, "stale", nil) },
		"collected drafts":      func() error { return service.UpdateCollectedRecords(stale, "stale", nil) },
		"species drafts":        func() error { return service.UpdateVegetationSpecies(stale, "stale", nil) },
		"delete other":          func() error { return service.DeleteOtherRecord(stale, "stale", 1) },
		"height":                func() error { return service.UpdateHeightRecords(stale, "stale", nil) },
		"audit flags":           func() error { return service.SetAuditRestoreSelection(stale, "stale", nil) },
		"restoration": func() error {
			_, err := service.RestoreSelectedAuditRecords(stale, "stale", nil, AuditRestoreRetain)
			return err
		},
		"header read":        func() error { _, err := service.GetPlot(context.Background(), stale, "stale"); return err },
		"capabilities":       func() error { _, err := service.GetHeaderCapabilities(context.Background(), stale); return err },
		"child capabilities": func() error { _, err := service.GetChildCapabilities(context.Background(), stale, "Veg"); return err },
		"veg read":           func() error { _, err := service.ListVegRecords(context.Background(), stale, "stale"); return err },
		"humus read":         func() error { _, err := service.ListHumusRecords(context.Background(), stale, "stale"); return err },
		"mineral read":       func() error { _, err := service.ListMineralRecords(context.Background(), stale, "stale"); return err },
		"other read":         func() error { _, err := service.ListOtherRecords(context.Background(), stale, "stale"); return err },
		"soil suggestions":   func() error { _, err := service.ListSoilSuggestions(context.Background(), stale); return err },
		"attribute suggestions": func() error {
			_, err := service.ListVegetationAttributeSuggestions(context.Background(), stale)
			return err
		},
		"species references": func() error {
			_, err := service.ListVegetationSpecies(context.Background(), stale, "SubVegAXL_BC")
			return err
		},
		"species aliases": func() error {
			_, err := service.ListVegetationSpeciesAliases(context.Background(), stale, VegetationSpeciesLookup{Code: "ACAROSPO"})
			return err
		},
		"species users": func() error {
			_, err := service.ListVegetationSpeciesUsers(context.Background(), stale, VegetationSpeciesLookup{Code: "RAW"})
			return err
		},
		"audit read":          func() error { _, err := service.ListAuditEntries(context.Background(), stale, "stale"); return err },
		"switch":              func() error { _, err := service.SwitchContext(stale, contextSelection(current)); return err },
		"legacy header":       func() error { return service.plots.UpdatePlot(FS882Header{PlotNumber: "stale"}) },
		"legacy child":        func() error { return service.plots.DeleteVegRecord("stale", 1) },
		"legacy height":       func() error { return service.plots.UpdateHeightRecords("stale", nil) },
		"legacy other drafts": func() error { return service.plots.UpdateOtherRecords("stale", nil) },
		"legacy soil drafts":  func() error { return service.plots.UpdateSoilRecords("stale", nil) },
		"legacy attributes":   func() error { return service.plots.UpdateVegetationAttributes("stale", nil) },
		"legacy collected":    func() error { return service.plots.UpdateCollectedRecords("stale", nil) },
		"legacy species":      func() error { return service.plots.UpdateVegetationSpecies("stale", nil) },
		"legacy audit flags":  func() error { return service.plots.SetAuditRestoreSelection("stale", nil) },
		"legacy restoration": func() error {
			_, err := service.plots.RestoreSelectedAuditRecords("stale", nil, AuditRestoreRetain)
			return err
		},
		"legacy switching": func() error { _, err := service.projects.SelectProject("Sample"); return err },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatal("unguarded context access was accepted")
			}
		})
	}
	after, err := os.ReadFile(current.ProjectPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected stale requests changed data or audits")
	}
}

func TestContextServiceOperationLeaseBlocksSwitchUntilOriginalContextCompletes(t *testing.T) {
	service, before := contextServiceFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := withContextPlot(service, before.ContextID, func(scoped *PlotService) (struct{}, error) {
			close(entered)
			<-release
			page, err := scoped.projects.ListPlots(context.Background(), 0, 1)
			if err != nil {
				return struct{}{}, err
			}
			_, err = scoped.GetPlot(page.Plots[0].PlotNumber)
			return struct{}{}, err
		})
		finished <- err
	}()
	<-entered
	if service.projects.operationMu.TryLock() {
		service.projects.operationMu.Unlock()
		close(release)
		t.Fatal("in-flight operation did not retain its context lease")
	}
	switched := make(chan error, 1)
	go func() {
		_, err := service.SwitchContext(before.ContextID, contextSelection(before))
		switched <- err
	}()
	close(release)
	for _, result := range []chan error{finished, switched} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("context operation/switch deadlocked")
		}
	}
	after, err := service.projects.GetState(context.Background())
	if err != nil || after.ContextID == before.ContextID {
		t.Fatal("switch was not published after the operation completed")
	}
}

func TestContextServiceClosedOwnerRejectsCurrentToken(t *testing.T) {
	service, state := contextServiceFixture(t)
	if err := service.projects.closeSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPlot(context.Background(), state.ContextID, "stale"); err == nil {
		t.Fatal("closed context accepted a formerly current token")
	}
	if _, err := service.SwitchContext(state.ContextID, contextSelection(state)); err == nil {
		t.Fatal("closed owner was implicitly resurrected")
	}
}
