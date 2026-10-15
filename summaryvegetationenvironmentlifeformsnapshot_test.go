package main

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
	"time"
)

func summaryEnvironmentalLifeformServiceFixture(t *testing.T, external bool) (*SummaryVegetationEnvironmentalLifeformService, ProjectState) {
	t.Helper()
	environment, state := summaryEnvironmentalNumericServiceFixture(t, external)
	owner := environment.report.contexts.projects.sqlite
	veg := quoteHeaderIdentifier(state.ActiveProject + "_Veg")
	mutateContextFixture(t, owner.attachments["project"], "DELETE FROM "+veg)
	mutateContextFixture(t, owner.attachments["VLists"], "DELETE FROM USysAllSpecs")
	for form := 1; form <= 13; form++ {
		argument := strconv.Itoa(form)
		species := "LF" + argument
		mutateContextFixture(t, owner.attachments["project"], "INSERT INTO "+veg+
			"(PlotNumber,Species,Cover1) VALUES('P1','"+species+"',1.25),('P2','"+species+"',3.75)")
		mutateContextFixture(t, owner.attachments["VLists"],
			"INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype) VALUES('"+
				species+"','Lifeform "+argument+"',"+argument+",'"+species+"','U')")
		mutateContextFixture(t, owner.attachments["VPro64"],
			"INSERT INTO USysShortVegSummaryFieldList VALUES("+
				argument+",'Calc','Lifeform "+argument+"',NULL,-1,'SVLifeformCover','"+argument+"')")
	}
	mutateContextFixture(t, owner.attachments["project"], "INSERT INTO "+veg+
		"(PlotNumber,Species,Cover1) VALUES('P3','LF1',0)")
	service, err := NewSummaryVegetationEnvironmentalLifeformService(environment, func(key string) (string, bool) {
		if key != summaryVegetationEnvironmentalLifeformFeatureEnvironment {
			t.Fatal("lifeform constructor borrowed another gate", key)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func TestSummaryEnvironmentalLifeformSnapshotOwnedWireAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryEnvironmentalLifeformServiceFixture(t, external)
		owner := service.environment.report.contexts.projects.sqlite
		before := databaseBytes(t, owner.attachments)
		config, err := os.ReadFile(service.environment.report.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		snapshots := 0
		service.environment.snapshot.commitRead = func(*sql.Tx) error { snapshots++; return nil }
		got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
		if err != nil || got == nil || len(got.Rows) != 30 || snapshots != 1 ||
			got.SourcePath != owner.attachments["VPro64"] || got.EnvSourcePath != owner.attachments["project"] ||
			got.MasterSourcePath != owner.attachments["VLists"] || got.Source != summaryEnvironmentalLifeformSource {
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
		if path := os.Getenv("VPRO_SUMMARY_ENVIRONMENT_LIFEFORM_PREVIEW_WIRE_PATH"); path != "" && !external {
			got.Vegetation.ContextID = "summary-lifeform-wire"
			got.Vegetation.ProjectPath = `C:\fixture\Sample.db`
			got.Vegetation.SUPath = `C:\fixture\Sample.db`
			got.SourcePath = `C:\fixture\VPro64.db`
			got.EnvSourcePath = `C:\fixture\Sample.db`
			got.MasterSourcePath = `C:\fixture\VLists.db`
			raw, err = json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			if os.Getenv("VPRO_UPDATE_SUMMARY_ENVIRONMENT_LIFEFORM_PREVIEW_WIRE") == "true" {
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			physical, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(raw, physical) {
				t.Fatal("strict-Go lifeform fixture differs", err)
			}
		}
		if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
			t.Fatal("lifeform snapshot changed original databases/audits")
		}
		after, err := os.ReadFile(service.environment.report.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("lifeform snapshot changed configuration", err)
		}
		if result, err := service.environment.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0); err == nil || result != nil {
			t.Fatal("numeric service implicitly enabled lifeform", result, err)
		}
	}
}

func TestSummaryEnvironmentalLifeformSnapshotNeedsOnlySelectedInputs(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, true)
	owner := service.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE FunctionName NOT IN ('SVnPlots','SVLifeformCover')")
	mutateContextFixture(t, owner.attachments["project"], "ALTER TABLE "+
		quoteHeaderIdentifier(state.ActiveProject+"_Env")+" RENAME TO HiddenEnv")
	got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || got == nil || len(got.Rows) != 14 {
		t.Fatal("lifeform-only selection unnecessarily required Env", got, err)
	}
	mutateContextFixture(t, owner.attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE FunctionName='SVSnr'")
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); got != nil || err == nil || !strings.Contains(err.Error(), "original physical") {
		t.Fatal("selected category no longer requires Env", got, err)
	}
	mutateContextFixture(t, owner.attachments["project"], "ALTER TABLE HiddenEnv RENAME TO "+
		quoteHeaderIdentifier(state.ActiveProject+"_Env"))
	mutateContextFixture(t, owner.attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE FunctionName='SVLifeformCover'")
	mutateContextFixture(t, owner.attachments["project"], "UPDATE "+
		quoteHeaderIdentifier(state.ActiveProject+"_Veg")+" SET Cover2=0.1 WHERE PlotNumber='P1' AND Species='LF1'")
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil || len(got.Rows) != 2 {
		t.Fatal("unselected lifeform unnecessarily invokes exact SINGLE QuickVeg guard", got, err)
	}
	mutateContextFixture(t, owner.attachments["VPro64"],
		"UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE FunctionName='SVLifeformCover'")
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil || !strings.Contains(err.Error(), "exactly assigned") {
		t.Fatal("selected lifeform bypassed exact SINGLE refusal", got, err)
	}
}

func TestSummaryEnvironmentalLifeformSnapshotGatesAndCleanup(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSummaryVegetationEnvironmentalLifeformService(nil, func(string) (string, bool) {
			return value, value != ""
		})
		if value == "" || value == "false" || value == "true" {
			if err != nil || service == nil || service.enabled != (value == "true") {
				t.Fatal(value, service, err)
			}
		} else if err == nil || service != nil {
			t.Fatal("nonliteral feature accepted", value, service, err)
		}
	}
	for mask := 0; mask < 16; mask++ {
		service, state := summaryEnvironmentalLifeformServiceFixture(t, false)
		service.enabled = mask&1 != 0
		service.environment.numericEnabled = mask&2 != 0
		service.environment.enabled = mask&4 != 0
		service.environment.report.enabled = mask&8 != 0
		owner := service.environment.report.contexts.projects.sqlite
		before := databaseBytes(t, owner.attachments)
		if mask != 15 {
			projects := service.environment.report.contexts.projects
			projects.operationMu.Lock()
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			got, err := service.PreviewEnvironmentalLifeform(ctx, state.ContextID, 1, 6, 0)
			cancel()
			projects.operationMu.Unlock()
			if got != nil || err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("disabled gate acquired context read lease", mask, got, err)
			}
		} else if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil {
			t.Fatal("all enabled gates refused", got, err)
		}
		if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
			t.Fatal("gate/snapshot changed data", mask)
		}
	}
	service, state := summaryEnvironmentalLifeformServiceFixture(t, true)
	if got, err := service.PreviewEnvironmentalLifeform(nil, state.ContextID, 1, 6, 0); err == nil || got != nil {
		t.Fatal("nil request context accepted", got, err)
	}
	service.environment.snapshot.commitRead = func(*sql.Tx) error { return errors.New("lifeform snapshot commit failed") }
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatal("commit cleanup produced success-shaped result", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	service.environment.snapshot.commitRead = func(*sql.Tx) error { cancel(); return nil }
	if got, err := service.PreviewEnvironmentalLifeform(ctx, state.ContextID, 1, 6, 0); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancellation at snapshot completion lost identity", got, err)
	}
	service.environment.snapshot.commitRead = nil
	if got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil {
		t.Fatal("owned snapshot failed cancellation/commit recovery", got, err)
	}
}
