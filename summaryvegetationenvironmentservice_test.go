package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func summaryEnvironmentalCombinedFixture(t *testing.T, external bool) (*SummaryVegetationEnvironmentalService, ProjectState) {
	t.Helper()
	report, state := summaryVegetationServiceFixture(t, external)
	owner := report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"], `DROP TABLE IF EXISTS USysShortVegSummaryFieldList;`+
		strings.ReplaceAll(summaryFieldListTestSchema, "VPro64.", "")+`;
		INSERT INTO USysShortVegSummaryFieldList VALUES
		(0,'Env','n Plots',NULL,-1,'SVnPlots',NULL),
		(1,'Env','Nutrient','NutrientRegime',-1,'SVSnr',NULL),
		(-1,'Env','disabled',NULL,0,'unknown',NULL)`)
	mutateContextFixture(t, owner.attachments["project"], `DELETE FROM `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`;
		INSERT INTO `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`(PlotNumber,NutrientRegime) VALUES('P1','M'),('P2','N'),('P3',NULL)`)
	service, err := NewSummaryVegetationEnvironmentalService(report, func(key string) (string, bool) {
		if key == summaryVegetationEnvironmentalNumericFeatureEnvironment {
			return "", false
		}
		if key != summaryVegetationEnvironmentalFeatureEnvironment {
			t.Fatal("borrowed environment gate", key)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func TestSummaryEnvironmentalCombinedGatesAndOwnedNoWrites(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSummaryVegetationEnvironmentalService(nil, func(string) (string, bool) { return flag, flag != "" })
		if flag == "" || flag == "false" || flag == "true" {
			if err != nil || service.enabled != (flag == "true") {
				t.Fatal(flag, service, err)
			}
			if got, err := service.PreviewEnvironmental(t.Context(), "owned", 1, 6, 0); got != nil || err == nil {
				t.Fatal(got, err)
			}
		} else if err == nil {
			t.Fatal("nonliteral gate accepted", flag)
		}
	}
	for _, external := range []bool{false, true} {
		service, state := summaryEnvironmentalCombinedFixture(t, external)
		owner := service.report.contexts.projects.sqlite
		before := databaseBytes(t, owner.attachments)
		config, err := os.ReadFile(service.report.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
		if err != nil || result == nil || len(result.Rows) != 2 || len(result.Vegetation.Units) != 2 ||
			result.SourcePath != owner.attachments["VPro64"] || result.EnvSourcePath != owner.attachments["project"] {
			t.Fatal("combined source/provenance", result, err)
		}
		if *result.Rows[0].Values[0].Integer != "4" || *result.Rows[0].Values[1].Integer != "1" ||
			*result.Rows[1].Values[0].Text != "M(2)  N(1)  " || *result.Rows[1].Values[1].Text != "(Null 1)" {
			t.Fatal("same physical sources counts/joined fields", result.Rows)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SummaryVegetationEnvironmentalPreview
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(result, &decoded) {
			t.Fatal("strict combined wire", err)
		}
		*result.FieldList.Rows[0].Cells[2].Text = "caller"
		if *result.Rows[0].Field.Cells[2].Text != "n Plots" {
			t.Fatal("field witness aliases physical result")
		}
		*result.Rows[0].Values[0].Integer = "caller"
		again, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
		if err != nil || *again.Rows[0].Values[0].Integer != "4" {
			t.Fatal("result aliases next snapshot", again, err)
		}
		if !reflect.DeepEqual(databaseBytes(t, owner.attachments), before) {
			t.Fatal("combined reader changed databases/audits")
		}
		after, err := os.ReadFile(service.report.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("combined read changed YAML", err)
		}
		service.report.enabled = false
		if got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); got != nil || err == nil {
			t.Fatal("base gate bypassed", got, err)
		}
	}
}

func TestSummaryEnvironmentalCombinedSelectionAndPhysicalCorrection(t *testing.T) {
	service, state := summaryEnvironmentalCombinedFixture(t, false)
	owner := service.report.contexts.projects.sqlite
	verify := func(want string) {
		t.Helper()
		result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
		if want == "" && err != nil || want != "" && (result != nil || err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatal(want, result, err)
		}
	}
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE Label='disabled'`)
	verify("unavailable")
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE Label='disabled'; UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE FunctionName='SVSnr'`)
	mutateContextFixture(t, owner.attachments["project"], `ALTER TABLE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+` RENAME TO HiddenEnv`)
	verify("")
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE FunctionName='SVSnr'`)
	verify("original physical")
	mutateContextFixture(t, owner.attachments["project"], `ALTER TABLE HiddenEnv RENAME TO `+quoteHeaderIdentifier(state.ActiveProject+"_Env"))
	verify("")
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+` SET NutrientRegime=CAST(X'FF' AS TEXT) WHERE PlotNumber='P1'`)
	verify("malformed Unicode")
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+` SET NutrientRegime='M' WHERE PlotNumber='P1'`)
	verify("")
}

func TestSummaryEnvironmentalCombinedOwnershipCleanupCancellation(t *testing.T) {
	service, state := summaryEnvironmentalCombinedFixture(t, true)
	owner := service.report.contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.PreviewEnvironmental(ctx, state.ContextID, 1, 6, 0)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("lease cancellation", result, err)
	}
	original := owner.attachmentInfo["VPro64"]
	owner.attachmentInfo["VPro64"] = owner.attachmentInfo["project"]
	if result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); result != nil || err == nil {
		t.Fatal("ownership drift published", result, err)
	}
	owner.attachmentInfo["VPro64"] = original
	service.snapshot.commitRead = func(*sql.Tx) error { return errors.New("read commit failed") }
	if result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); result != nil || err == nil {
		t.Fatal("failed commit published", result, err)
	}
	service.snapshot.commitRead = nil
	service.snapshot.rollbackRead = func(*sql.Tx) error { return errors.New("read cleanup failed") }
	if result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); result != nil || err == nil {
		t.Fatal("failed cleanup published", result, err)
	}
	service.snapshot.rollbackRead = nil
	cancelled, stop := context.WithCancel(context.Background())
	service.snapshot.commitRead = func(*sql.Tx) error { stop(); return nil }
	if result, err := service.PreviewEnvironmental(cancelled, state.ContextID, 1, 6, 0); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("late cancel", result, err)
	}
	service.snapshot.commitRead = nil
	if _, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); err != nil {
		t.Fatal("pinned next read", err)
	}
	next, err := service.report.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); result != nil || err == nil {
		t.Fatal("stale context", result, err)
	}
	if _, err := service.PreviewEnvironmental(t.Context(), next.ContextID, 1, 6, 0); err != nil {
		t.Fatal("switched retry", err)
	}
}
