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

func summaryEnvironmentalCountOwnedFixture(t *testing.T, external bool) (*SummaryVegetationEnvironmentalCountService, ProjectState) {
	t.Helper()
	contexts, state := reportServiceFixture(t, external)
	mutateContextFixture(t, contexts.projects.sqlite.attachments["su"], `DELETE FROM Report_SU;
		INSERT INTO Report_SU VALUES('P1','A'),('P1','A'),(NULL,'A'),('P2','a'),(NULL,'zero'),('',''),('P3',' A ')`)
	service, err := NewSummaryVegetationEnvironmentalCountService(contexts, func(key string) (string, bool) {
		if key != summaryVegetationEnvironmentalCountFeatureEnvironment {
			t.Fatal("borrowed workflow gate", key)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func TestSummaryEnvironmentalCountServiceGateAndRequest(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSummaryVegetationEnvironmentalCountService(nil, func(string) (string, bool) { return flag, flag != "" })
		if flag == "" || flag == "false" || flag == "true" {
			if err != nil || service.enabled != (flag == "true") {
				t.Fatal(flag, service, err)
			}
			unit := "A"
			if got, err := service.GetEnvironmentalPlotCount(t.Context(), "owned", &unit); got != nil || err == nil {
				t.Fatal("unavailable service returned data", got, err)
			}
		} else if err == nil {
			t.Fatal("nonliteral flag accepted", flag)
		}
	}
	service, state := summaryEnvironmentalCountOwnedFixture(t, false)
	unit := "A"
	for _, id := range []string{"", "stale", state.ContextID + "\x00", string([]byte{0xff})} {
		if got, err := service.GetEnvironmentalPlotCount(t.Context(), id, &unit); got != nil || err == nil {
			t.Fatal("invalid context accepted", got, err)
		}
	}
	if got, err := service.GetEnvironmentalPlotCount(nil, state.ContextID, &unit); got != nil || err == nil {
		t.Fatal(got, err)
	}
	if got, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, nil); got != nil || err == nil || !strings.Contains(err.Error(), "error94") {
		t.Fatal(got, err)
	}
}

func TestSummaryEnvironmentalCountServiceOwnedWireAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryEnvironmentalCountOwnedFixture(t, external)
		owner := service.contexts.projects.sqlite
		before := databaseBytes(t, owner.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ unit, want string }{{"A", "2"}, {"a", "1"}, {" A ", "1"}, {"zero", "0"}, {"", "1"}} {
			result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &tc.unit)
			if err != nil || result == nil || result.ContextID != state.ContextID || result.Project != state.ActiveProject ||
				result.SU != "Report" || result.ProjectPath != state.ProjectPath || result.SUPath != state.SUPath ||
				result.SourcePath != owner.attachments["su"] || result.SourceTable != "Report_SU" || result.SiteUnit != tc.unit ||
				len(result.Physical.Rows) != 7 || !reflect.DeepEqual(result.Value, ProjectMetadataCell{Storage: "integer", Integer: &tc.want}) {
				t.Fatal("owned provenance/count changed", external, tc, result, err)
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var reopened SummaryVegetationEnvironmentalCountReview
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&reopened); err != nil || !reflect.DeepEqual(result, &reopened) {
				t.Fatal("strict complete DTO wire changed", err)
			}
			*result.Value.Integer = "poison"
			*result.Physical.Rows[0].Cells[0].Text = "poison"
			again, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &tc.unit)
			if err != nil || *again.Value.Integer != tc.want || *again.Physical.Rows[0].Cells[0].Text != "P1" {
				t.Fatal("caller changed subsequent read", again, err)
			}
		}
		if !reflect.DeepEqual(databaseBytes(t, owner.attachments), before) {
			t.Fatal("read changed database/audit/history bytes")
		}
		after, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("read changed config", err)
		}
	}
}

func TestSummaryEnvironmentalCountServiceOwnershipCleanupCancellation(t *testing.T) {
	service, state := summaryEnvironmentalCountOwnedFixture(t, true)
	owner := service.contexts.projects.sqlite
	unit := "A"
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.GetEnvironmentalPlotCount(ctx, state.ContextID, &unit)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("queued lease cancellation returned data", result, err)
	}
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("identity drift returned data", result, err)
	}
	owner.attachmentInfo["su"] = original
	service.snapshot.commitRead = func(*sql.Tx) error { return errors.New("read completion failed") }
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("commit error returned data", result, err)
	}
	service.snapshot.commitRead = nil
	service.snapshot.rollbackRead = func(*sql.Tx) error { return errors.New("read cleanup failed") }
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("rollback error returned data", result, err)
	}
	service.snapshot.rollbackRead = nil
	cancelled, stop := context.WithCancel(context.Background())
	service.snapshot.commitRead = func(*sql.Tx) error { stop(); return nil }
	if result, err := service.GetEnvironmentalPlotCount(cancelled, state.ContextID, &unit); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("late cancellation returned data", result, err)
	}
	service.snapshot.commitRead = nil
	if _, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); err != nil {
		t.Fatal("next pinned snapshot lost attachments", err)
	}
	next, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("stale context returned data", result, err)
	}
	if _, err := service.GetEnvironmentalPlotCount(t.Context(), next.ContextID, &unit); err != nil {
		t.Fatal("switched retry failed", err)
	}
}

func TestSummaryEnvironmentalCountServicePhysicalRefusalCorrection(t *testing.T) {
	service, state := summaryEnvironmentalCountOwnedFixture(t, false)
	owner := service.contexts.projects.sqlite
	unit := "A"
	verify := func(want string) {
		t.Helper()
		result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit)
		if want == "" && err != nil || want != "" && (result != nil || err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatal("physical refusal/retry", want, result, err)
		}
	}
	missing := "missing"
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &missing); result != nil || err == nil || !strings.Contains(err.Error(), "error3021") {
		t.Fatal("missing group invented", result, err)
	}
	mutateContextFixture(t, owner.attachments["su"], `UPDATE Report_SU SET PlotNumber=CAST(X'FF' AS TEXT) WHERE rowid=1`)
	verify("malformed Unicode")
	mutateContextFixture(t, owner.attachments["su"], `UPDATE Report_SU SET PlotNumber='P1' WHERE rowid=1`)
	verify("")
	mutateContextFixture(t, owner.attachments["su"], `ALTER TABLE Report_SU RENAME TO Original_SU; CREATE VIEW Report_SU AS SELECT * FROM Original_SU`)
	verify("original physical")
	mutateContextFixture(t, owner.attachments["su"], `DROP VIEW Report_SU; ALTER TABLE Original_SU RENAME TO Report_SU`)
	verify("")
	mutateContextFixture(t, owner.attachments["su"], `ALTER TABLE Report_SU ADD COLUMN rowid TEXT`)
	verify("unshadowed physical")
	mutateContextFixture(t, owner.attachments["su"], `ALTER TABLE Report_SU DROP COLUMN rowid`)
	verify("")
	owner.selection.SU = "None"
	verify("selected normal SU")
	owner.selection.SU = "USysSuTableDynamic"
	verify("dynamic-break")
	owner.selection.SU = "Report"
	verify("")
}
