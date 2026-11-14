package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryEnvironmentalLifeformServiceFixture(t *testing.T, external bool) (*SummaryVegetationEnvironmentalLifeformService, vprocore.ProjectState) {
	t.Helper()
	environment, state := summaryEnvironmentalNumericServiceFixture(t, external)
	owner := environment.report.contexts.Projects.Sqlite
	veg := vprocore.QuoteHeaderIdentifier(state.ActiveProject + "_Veg")
	vprotest.MutateContextFixture(t, owner.Attachments["project"], "DELETE FROM "+veg)
	vprotest.MutateContextFixture(t, owner.Attachments["VLists"], "DELETE FROM USysAllSpecs")
	for form := 1; form <= 13; form++ {
		argument := strconv.Itoa(form)
		species := "LF" + argument
		vprotest.MutateContextFixture(t, owner.Attachments["project"], "INSERT INTO "+veg+
			"(PlotNumber,Species,Cover1) VALUES('P1','"+species+"',1.25),('P2','"+species+"',3.75)")
		vprotest.MutateContextFixture(t, owner.Attachments["VLists"],
			"INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype) VALUES('"+
				species+"','Lifeform "+argument+"',"+argument+",'"+species+"','U')")
		vprotest.MutateContextFixture(t, owner.Attachments["VPro64"],
			"INSERT INTO USysShortVegSummaryFieldList VALUES("+
				argument+",'Calc','Lifeform "+argument+"',NULL,-1,'SVLifeformCover','"+argument+"')")
	}
	vprotest.MutateContextFixture(t, owner.Attachments["project"], "INSERT INTO "+veg+
		"(PlotNumber,Species,Cover1) VALUES('P3','LF1',0)")
	service, err := NewSummaryVegetationEnvironmentalLifeformService(environment)
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func TestSummaryEnvironmentalLifeformSnapshotOwnedWireAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryEnvironmentalLifeformServiceFixture(t, external)
		owner := service.environment.report.contexts.Projects.Sqlite
		before := vprotest.DatabaseBytes(t, owner.Attachments)
		config, err := os.ReadFile(service.environment.report.contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		snapshots := 0
		service.environment.snapshot.CommitRead = func(*sql.Tx) error { snapshots++; return nil }
		got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
		if err != nil || got == nil || len(got.Rows) != 30 || snapshots != 1 ||
			got.SourcePath != owner.Attachments["VPro64"] || got.EnvSourcePath != owner.Attachments["project"] ||
			got.MasterSourcePath != owner.Attachments["VLists"] || got.Source != summaryEnvironmentalLifeformSource {
			t.Fatal("one owned mixed snapshot/provenance", got, err, snapshots)
		}
		count := 0
		for _, row := range got.Rows {
			if row.Field.Cells[5].Text == nil || *row.Field.Cells[5].Text != "SVLifeformCover" {
				continue
			}
			count++
			argument := *row.Field.Cells[6].Text
			for i, text := range []string{"3.8 - 4.4 - 5.0", ""} {
				if i == 1 && argument == "1" {
					text = "0.0 - 0.0 - 0.0"
				}
				value := row.Values[i]
				if value.Kind != "cell" || value.Cell == nil || value.Cell.Storage != "text" ||
					value.Cell.Text == nil || *value.Cell.Text != text {
					t.Fatal("source QuickVeg double membership composition/value", argument, i, value, text)
				}
			}
		}
		if count != 13 {
			t.Fatal("all13 original argument rows required", count)
		}
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SummaryVegetationEnvironmentalLifeformPreview
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(got, &decoded) {
			t.Fatal("strict lifeform wire round trip", err)
		}
		if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, owner.Attachments)) {
			t.Fatal("lifeform snapshot changed original databases/audits")
		}
		after, err := os.ReadFile(service.environment.report.contexts.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("lifeform snapshot changed configuration", err)
		}
	}
}

func TestSummaryEnvironmentalLifeformSnapshotNeedsOnlySelectedInputs(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, true)
	owner := service.environment.report.contexts.Projects.Sqlite
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE FunctionName NOT IN ('SVnPlots','SVLifeformCover')")
	vprotest.MutateContextFixture(t, owner.Attachments["project"], "ALTER TABLE "+
		vprocore.QuoteHeaderIdentifier(state.ActiveProject+"_Env")+" RENAME TO HiddenEnv")
	got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || got == nil || len(got.Rows) != 14 {
		t.Fatal("lifeform-only selection unnecessarily required Env", got, err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE FunctionName='SVSnr'")
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); got != nil || err == nil || !strings.Contains(err.Error(), "original physical") {
		t.Fatal("selected category no longer requires Env", got, err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["project"], "ALTER TABLE HiddenEnv RENAME TO "+
		vprocore.QuoteHeaderIdentifier(state.ActiveProject+"_Env"))
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE FunctionName='SVLifeformCover'")
	vprotest.MutateContextFixture(t, owner.Attachments["project"], "UPDATE "+
		vprocore.QuoteHeaderIdentifier(state.ActiveProject+"_Veg")+" SET Cover2=0.1 WHERE PlotNumber='P1' AND Species='LF1'")
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil || len(got.Rows) != 2 {
		t.Fatal("unselected lifeform unnecessarily invokes exact SINGLE QuickVeg guard", got, err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE FunctionName='SVLifeformCover'")
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil || !strings.Contains(err.Error(), "exactly assigned") {
		t.Fatal("selected lifeform bypassed exact SINGLE refusal", got, err)
	}
}

func TestSummaryEnvironmentalLifeformSnapshotGatesAndCleanup(t *testing.T) {
	nilService, err := NewSummaryVegetationEnvironmentalLifeformService(nil)
	if err != nil || nilService == nil {
		t.Fatal(nilService, err)
	}
	if got, err := nilService.PreviewEnvironmentalLifeform(t.Context(), "owned", 1, 6, 0); got != nil || err == nil {
		t.Fatal("unavailable lifeform service succeeded", got, err)
	}
	service, state := summaryEnvironmentalLifeformServiceFixture(t, true)
	if got, err := service.PreviewEnvironmentalLifeform(nil, state.ContextID, 1, 6, 0); err == nil || got != nil {
		t.Fatal("nil request context accepted", got, err)
	}
	service.environment.snapshot.CommitRead = func(*sql.Tx) error { return errors.New("lifeform snapshot commit failed") }
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatal("commit cleanup produced success-shaped result", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	service.environment.snapshot.CommitRead = func(*sql.Tx) error { cancel(); return nil }
	if got, err := service.PreviewEnvironmentalLifeform(ctx, state.ContextID, 1, 6, 0); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancellation at snapshot completion lost identity", got, err)
	}
	service.environment.snapshot.CommitRead = nil
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil {
		t.Fatal("owned snapshot failed cancellation/commit recovery", got, err)
	}
}
