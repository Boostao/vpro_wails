package envreport

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
	"testing"
	"time"
)

func TestSummaryQuickVegetationOwnedCaptureDetachedAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := vprotest.ReportServiceFixture(t, external)
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		config, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			input, err := ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{})
			if err != nil || len(input.Tables) != 7 || len(input.Quick.Entries) == 0 || len(input.References.Table.Rows) == 0 {
				t.Fatal("complete owned preparation unavailable", input, err)
			}
			if len(input.Lifeform) != len(input.Environment.Report.Units) || len(input.Lifeform[0].Rows) != 14 {
				t.Fatal("owned source did not include complete fourteen-lifeform scalar preparation")
			}
			environment, err := ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{})
			if err != nil || !reflect.DeepEqual(input.Environment, environment.Preview) ||
				!reflect.DeepEqual(input.Tables[:4], environment.Tables) {
				t.Fatal("accepted environment source changed", err)
			}
			raw, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			*input.Quick.Entries[0].Species.Text = "changed"
			*input.References.Table.Rows[0].Cells[0].Text = "changed"
			input.Tables[0].Rows[0].Cells[0] = vprotest.MetadataText("changed")
			input.Environment.Report.Units[0].Values[0] = "changed"
			again, err := ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{})
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(again)
			if err != nil || !bytes.Equal(raw, actual) {
				t.Fatal("snapshot/preparation was borrowed or nondeterministic", err)
			}
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("private source capture wrote configuration", err)
		}
	}
}

func TestSummaryQuickVegetationOwnedCancellationCleanupOwnershipAndRetry(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	owner, zero := service.Projects.Sqlite, siteUnitQuickVegetationInput{}
	owner.Mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	input, err := ReadSiteUnitQuickVegetationInput(service, ctx, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(input, zero) {
		t.Fatal("cancelled acquisition retained partial source", err)
	}
	sentinel := errors.New("QuickVeg completion/cleanup failure")
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return sentinel }},
		{RollbackRead: func(*sql.Tx) error { return sentinel }},
	} {
		input, err = ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, 1, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(input, zero) {
			t.Fatal("failed completion/cleanup retained source", err)
		}
	}
	late, cancelLate := context.WithCancel(context.Background())
	input, err = ReadSiteUnitQuickVegetationInput(service, late, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{
		CommitRead: func(*sql.Tx) error { cancelLate(); return nil },
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(input, zero) {
		t.Fatal("late cancellation retained source", err)
	}
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	input, err = ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	owner.AttachmentInfo["su"] = original
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("foreign attachment accepted", err)
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if input, err = ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("stale context accepted", err)
	}
	if _, err := ReadSiteUnitQuickVegetationInput(service, context.Background(), next.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err != nil {
		t.Fatal("failed read leaked lease", err)
	}
}

func TestSummaryQuickVegetationOwnedRefusesIncompleteReferenceHalf(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	zero := siteUnitQuickVegetationInput{}
	for _, method := range []int{0, 3} {
		if got, err := ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(got, zero) {
			t.Fatal("invalid method accepted", method, err)
		}
	}
	if got, err := ReadSiteUnitQuickVegetationInput(service, nil, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(got, zero) {
		t.Fatal("missing context accepted", err)
	}
	owner := service.Projects.Sqlite
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `ALTER TABLE USysUserSpp RENAME COLUMN LifeForm TO MissingLifeform`)
	if got, err := ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(got, zero) {
		t.Fatal("reference failure published successful QuickVeg/environment half", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `ALTER TABLE USysUserSpp RENAME COLUMN MissingLifeform TO LifeForm`)
	if _, err := ReadSiteUnitQuickVegetationInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err != nil {
		t.Fatal("corrected reference retry failed", err)
	}
}
