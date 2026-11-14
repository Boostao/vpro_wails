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
	"strings"
	"testing"
	"time"
)

func summaryEnvironmentalCountOwnedFixture(t *testing.T, external bool) (*SummaryVegetationEnvironmentalCountService, vprocore.ProjectState) {
	t.Helper()
	contexts, state := vprotest.ReportServiceFixture(t, external)
	vprotest.MutateContextFixture(t, contexts.Projects.Sqlite.Attachments["su"], `DELETE FROM Report_SU;
		INSERT INTO Report_SU VALUES('P1','A'),('P1','A'),(NULL,'A'),('P2','a'),(NULL,'zero'),('',''),('P3',' A ')`)
	service, err := NewSummaryVegetationEnvironmentalCountService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func TestSummaryEnvironmentalCountServiceGateAndRequest(t *testing.T) {
	nilService, err := NewSummaryVegetationEnvironmentalCountService(nil)
	if err != nil || nilService == nil {
		t.Fatal(nilService, err)
	}
	unit := "A"
	if got, err := nilService.GetEnvironmentalPlotCount(t.Context(), "owned", &unit); got != nil || err == nil {
		t.Fatal("unavailable service returned data", got, err)
	}
	service, state := summaryEnvironmentalCountOwnedFixture(t, false)
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
		owner := service.contexts.Projects.Sqlite
		before := vprotest.DatabaseBytes(t, owner.Attachments)
		config, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ unit, want string }{{"A", "2"}, {"a", "1"}, {" A ", "1"}, {"zero", "0"}, {"", "1"}} {
			result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &tc.unit)
			if err != nil || result == nil || result.ContextID != state.ContextID || result.Project != state.ActiveProject ||
				result.SU != "Report" || result.ProjectPath != state.ProjectPath || result.SUPath != state.SUPath ||
				result.SourcePath != owner.Attachments["su"] || result.SourceTable != "Report_SU" || result.SiteUnit != tc.unit ||
				len(result.Physical.Rows) != 7 || !reflect.DeepEqual(result.Value, vprocore.ProjectMetadataCell{Storage: "integer", Integer: &tc.want}) {
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
		if !reflect.DeepEqual(vprotest.DatabaseBytes(t, owner.Attachments), before) {
			t.Fatal("read changed database/audit/history bytes")
		}
		after, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("read changed config", err)
		}
	}
}

func TestSummaryEnvironmentalCountServiceOwnershipCleanupCancellation(t *testing.T) {
	service, state := summaryEnvironmentalCountOwnedFixture(t, true)
	owner := service.contexts.Projects.Sqlite
	unit := "A"
	owner.Mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.GetEnvironmentalPlotCount(ctx, state.ContextID, &unit)
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("queued lease cancellation returned data", result, err)
	}
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("identity drift returned data", result, err)
	}
	owner.AttachmentInfo["su"] = original
	service.snapshot.CommitRead = func(*sql.Tx) error { return errors.New("read completion failed") }
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("commit error returned data", result, err)
	}
	service.snapshot.CommitRead = nil
	service.snapshot.RollbackRead = func(*sql.Tx) error { return errors.New("read cleanup failed") }
	if result, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); result != nil || err == nil {
		t.Fatal("rollback error returned data", result, err)
	}
	service.snapshot.RollbackRead = nil
	cancelled, stop := context.WithCancel(context.Background())
	service.snapshot.CommitRead = func(*sql.Tx) error { stop(); return nil }
	if result, err := service.GetEnvironmentalPlotCount(cancelled, state.ContextID, &unit); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("late cancellation returned data", result, err)
	}
	service.snapshot.CommitRead = nil
	if _, err := service.GetEnvironmentalPlotCount(t.Context(), state.ContextID, &unit); err != nil {
		t.Fatal("next pinned snapshot lost attachments", err)
	}
	next, err := service.contexts.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
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
	owner := service.contexts.Projects.Sqlite
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
	vprotest.MutateContextFixture(t, owner.Attachments["su"], `UPDATE Report_SU SET PlotNumber=CAST(X'FF' AS TEXT) WHERE rowid=1`)
	verify("malformed Unicode")
	vprotest.MutateContextFixture(t, owner.Attachments["su"], `UPDATE Report_SU SET PlotNumber='P1' WHERE rowid=1`)
	verify("")
	vprotest.MutateContextFixture(t, owner.Attachments["su"], `ALTER TABLE Report_SU RENAME TO Original_SU; CREATE VIEW Report_SU AS SELECT * FROM Original_SU`)
	verify("original physical")
	vprotest.MutateContextFixture(t, owner.Attachments["su"], `DROP VIEW Report_SU; ALTER TABLE Original_SU RENAME TO Report_SU`)
	verify("")
	vprotest.MutateContextFixture(t, owner.Attachments["su"], `ALTER TABLE Report_SU ADD COLUMN rowid TEXT`)
	verify("unshadowed physical")
	vprotest.MutateContextFixture(t, owner.Attachments["su"], `ALTER TABLE Report_SU DROP COLUMN rowid`)
	verify("")
	owner.Selection.SU = "None"
	verify("selected normal SU")
	owner.Selection.SU = "USysSuTableDynamic"
	verify("dynamic-break")
	owner.Selection.SU = "Report"
	verify("")
}
