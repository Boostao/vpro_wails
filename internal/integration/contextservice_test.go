package integration

import (
	"github.com/boostao/vpro-wails/internal/classification"
	"github.com/boostao/vpro-wails/internal/plotentry"
	"github.com/boostao/vpro-wails/internal/validation"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestContextServiceRejectsStaleReadsEveryMutationAndLegacyBypass(t *testing.T) {
	service, old := vprotest.ContextServiceFixture(t)
	current, err := service.SwitchContext(old.ContextID, vprotest.ContextSelection(old))
	if err != nil || current.ContextID == old.ContextID {
		t.Fatal("context publication did not rotate identity")
	}
	before, err := os.ReadFile(current.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := old.ContextID
	cases := map[string]func() error{
		"create header":         func() error { return service.CreatePlot(stale, vprocore.FS882Header{PlotNumber: "stale"}) },
		"update header":         func() error { return service.UpdatePlot(stale, vprocore.FS882Header{PlotNumber: "stale"}) },
		"save veg":              func() error { return service.SaveVegRecord(stale, vprocore.VegRecord{}) },
		"update veg":            func() error { return service.UpdateVegRecord(stale, vprocore.VegRecord{}) },
		"delete veg":            func() error { return service.DeleteVegRecord(stale, "stale", 1) },
		"save humus":            func() error { return service.SaveHumusRecord(stale, vprocore.HumusRecord{}) },
		"update humus":          func() error { return service.UpdateHumusRecord(stale, vprocore.HumusRecord{}) },
		"delete humus":          func() error { return service.DeleteHumusRecord(stale, "stale", 1) },
		"save mineral":          func() error { return service.SaveMineralRecord(stale, vprocore.MineralRecord{}) },
		"update mineral":        func() error { return service.UpdateMineralRecord(stale, vprocore.MineralRecord{}) },
		"delete mineral":        func() error { return service.DeleteMineralRecord(stale, "stale", 1) },
		"save other":            func() error { return service.SaveOtherRecord(stale, vprocore.OtherRecord{}) },
		"update other":          func() error { return service.UpdateOtherRecord(stale, vprocore.OtherRecord{}) },
		"other drafts":          func() error { return service.UpdateOtherRecords(stale, "stale", nil) },
		"soil drafts":           func() error { return service.UpdateSoilRecords(stale, "stale", nil) },
		"vegetation attributes": func() error { return plotentry.CtxUpdateVegetationAttributes(service, stale, "stale", nil) },
		"collected drafts":      func() error { return service.UpdateCollectedRecords(stale, "stale", nil) },
		"species drafts":        func() error { return service.UpdateVegetationSpecies(stale, "stale", nil) },
		"delete other":          func() error { return service.DeleteOtherRecord(stale, "stale", 1) },
		"height":                func() error { return service.UpdateHeightRecords(stale, "stale", nil) },
		"audit flags":           func() error { return service.SetAuditRestoreSelection(stale, "stale", nil) },
		"restoration": func() error {
			_, err := service.RestoreSelectedAuditRecords(stale, "stale", nil, vprocore.AuditRestoreRetain)
			return err
		},
		"header read":        func() error { _, err := service.GetPlot(context.Background(), stale, "stale"); return err },
		"capabilities":       func() error { _, err := service.GetHeaderCapabilities(context.Background(), stale); return err },
		"child capabilities": func() error { _, err := service.GetChildCapabilities(context.Background(), stale, "Veg"); return err },
		"veg read":           func() error { _, err := service.ListVegRecords(context.Background(), stale, "stale"); return err },
		"humus read":         func() error { _, err := service.ListHumusRecords(context.Background(), stale, "stale"); return err },
		"mineral read":       func() error { _, err := service.ListMineralRecords(context.Background(), stale, "stale"); return err },
		"other read":         func() error { _, err := service.ListOtherRecords(context.Background(), stale, "stale"); return err },
		"soil suggestions": func() error {
			_, err := plotentry.ListSoilSuggestions(service, context.Background(), stale)
			return err
		},
		"attribute suggestions": func() error {
			_, err := plotentry.ListVegetationAttributeSuggestions(service, context.Background(), stale)
			return err
		},
		"species references": func() error {
			_, err := service.ListVegetationSpecies(context.Background(), stale, "SubVegAXL_BC")
			return err
		},
		"species aliases": func() error {
			_, err := service.ListVegetationSpeciesAliases(context.Background(), stale, vprocore.VegetationSpeciesLookup{Code: "ACAROSPO"})
			return err
		},
		"species users": func() error {
			_, err := service.ListVegetationSpeciesUsers(context.Background(), stale, vprocore.VegetationSpeciesLookup{Code: "RAW"})
			return err
		},
		"species check review": func() error {
			_, err := validation.ReviewSpeciesCodes(service, context.Background(), stale)
			return err
		},
		"species check target": func() error {
			_, err := validation.LookupSpeciesCodeCheckTarget(service, context.Background(), stale, vprocore.VegetationSpeciesLookup{Code: "ABIE_RK"})
			return err
		},
		"species check save": func() error { return validation.SaveSpeciesCodeCheck(service, stale, nil) },
		"project metadata": func() error {
			_, err := service.ReviewProjectMetadata(context.Background(), stale, "stale")
			return err
		},
		"project metadata save": func() error {
			return service.SaveProjectMetadata(context.Background(), stale, vprocore.ProjectMetadataEdit{})
		},
		"project metadata fields": func() error {
			_, err := classification.ListProjectMetadataFields(service, context.Background(), stale)
			return err
		},
		"blank metadata creation": func() error {
			_, err := classification.CreateBlankProjectMetadata(service, context.Background(), stale, classification.ProjectMetadataCreate{})
			return err
		},
		"audit read":          func() error { _, err := service.ListAuditEntries(context.Background(), stale, "stale"); return err },
		"switch":              func() error { _, err := service.SwitchContext(stale, vprotest.ContextSelection(current)); return err },
		"legacy header":       func() error { return service.Plots.UpdatePlot(vprocore.FS882Header{PlotNumber: "stale"}) },
		"legacy child":        func() error { return service.Plots.DeleteVegRecord("stale", 1) },
		"legacy height":       func() error { return service.Plots.UpdateHeightRecords("stale", nil) },
		"legacy other drafts": func() error { return service.Plots.UpdateOtherRecords("stale", nil) },
		"legacy soil drafts":  func() error { return service.Plots.UpdateSoilRecords("stale", nil) },
		"legacy attributes":   func() error { return plotentry.UpdateVegetationAttributes(service.Plots, "stale", nil) },
		"legacy collected":    func() error { return service.Plots.UpdateCollectedRecords("stale", nil) },
		"legacy species":      func() error { return service.Plots.UpdateVegetationSpecies("stale", nil) },
		"legacy numbers":      func() error { return service.Plots.UpdateVegetationNumbers("stale", nil) },
		"scoped numbers":      func() error { return service.UpdateVegetationNumbers(stale, "stale", nil) },
		"scoped deletion": func() error {
			return plotentry.CtxDeleteReviewedVegetation(service, stale, "stale", plotentry.VegetationDeletionRequest{})
		},
		"scoped creation": func() error {
			_, err := service.CreateSourceVegetation(stale, "stale", vprocore.VegetationCreationRequest{})
			return err
		},
		"personal definition": func() error {
			_, err := plotentry.CtxCreatePersonalSpeciesDefinition(service, context.Background(), stale, plotentry.PersonalSpeciesDefinitionRequest{})
			return err
		},
		"deletion review": func() error {
			_, err := plotentry.ReviewVegetationDeletion(service, context.Background(), stale, "stale", "SubVegAXL_BC", 0)
			return err
		},
		"legacy deletion": func() error {
			return plotentry.DeleteReviewedVegetation(service.Plots, "stale", plotentry.VegetationDeletionRequest{Expected: strings.Repeat("0", 64)})
		},
		"legacy audit flags": func() error { return service.Plots.SetAuditRestoreSelection("stale", nil) },
		"legacy restoration": func() error {
			_, err := service.Plots.RestoreSelectedAuditRecords("stale", nil, vprocore.AuditRestoreRetain)
			return err
		},
		"legacy switching": func() error { _, err := service.Projects.SelectProject("Sample"); return err },
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
	service, before := vprotest.ContextServiceFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := vprocore.WithContextPlot(service, before.ContextID, func(scoped *vprocore.PlotService) (struct{}, error) {
			close(entered)
			<-release
			page, err := scoped.Projects.ListPlots(context.Background(), 0, 1)
			if err != nil {
				return struct{}{}, err
			}
			_, err = scoped.GetPlot(page.Plots[0].PlotNumber)
			return struct{}{}, err
		})
		finished <- err
	}()
	<-entered
	if service.Projects.OperationMu.TryLock() {
		service.Projects.OperationMu.Unlock()
		close(release)
		t.Fatal("in-flight operation did not retain its context lease")
	}
	switched := make(chan error, 1)
	go func() {
		_, err := service.SwitchContext(before.ContextID, vprotest.ContextSelection(before))
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
	after, err := service.Projects.GetState(context.Background())
	if err != nil || after.ContextID == before.ContextID {
		t.Fatal("switch was not published after the operation completed")
	}
}

func TestContextServiceClosedOwnerRejectsCurrentToken(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	if err := service.Projects.CloseSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPlot(context.Background(), state.ContextID, "stale"); err == nil {
		t.Fatal("closed context accepted a formerly current token")
	}
	if _, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state)); err == nil {
		t.Fatal("closed owner was implicitly resurrected")
	}
}
