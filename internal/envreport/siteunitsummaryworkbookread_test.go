package envreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
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

func TestSiteUnitSummaryWorkbookReadOwnedDetachedAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := vprotest.ReportServiceFixture(t, external)
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		config, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			input, err := ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{})
			if err != nil || len(input.Tables) != 4 {
				t.Fatal("complete owned input unavailable", err)
			}
			service.SiteUnitSummaryEnabled = true
			expected, err := PreviewSiteUnitSummary(service, context.Background(), state.ContextID, SiteUnitSummaryRequest{method})
			if err != nil || !reflect.DeepEqual(expected, input.Preview) {
				t.Fatal("shared source capture changed existing preview", err)
			}
			service.SiteUnitSummaryEnabled = false
			raw, err := json.Marshal(input.Tables)
			if err != nil {
				t.Fatal(err)
			}
			input.Tables[0].Rows[0].Cells[0] = vprotest.MetadataText("detached")
			input.Preview.Report.Units[0].Values[0] = "detached"
			input.Preview.Report.Fields[0].Label = "detached"
			again, err := ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{})
			if err != nil || !reflect.DeepEqual(again.Preview, expected) {
				t.Fatal("input was not detached or incorrectly required preview gate", err)
			}
			actual, err := json.Marshal(again.Tables)
			if err != nil || !bytes.Equal(raw, actual) {
				t.Fatal("raw source evidence was not detached", err)
			}
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("private source capture wrote configuration", err)
		}
	}
}

func TestSiteUnitSummaryWorkbookReadCancellationCleanupOwnershipAndRetry(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	owner, zero := service.Projects.Sqlite, siteUnitSummaryWorkbookInput{}
	owner.Mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	input, err := ReadSiteUnitSummaryWorkbookInput(service, ctx, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(input, zero) {
		t.Fatal("cancelled acquisition produced source", err)
	}
	sentinel := errors.New("summary workbook completion/cleanup failure")
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return sentinel }},
		{RollbackRead: func(*sql.Tx) error { return sentinel }},
	} {
		input, err = ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, 1, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(input, zero) {
			t.Fatal("failed completion/cleanup retained source", err)
		}
	}
	late, cancelLate := context.WithCancel(context.Background())
	input, err = ReadSiteUnitSummaryWorkbookInput(service, late, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{
		CommitRead: func(*sql.Tx) error { cancelLate(); return nil },
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(input, zero) {
		t.Fatal("late cancellation retained source", err)
	}
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	input, err = ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	owner.AttachmentInfo["su"] = original
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("foreign source attachment accepted", err)
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	input, err = ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("stale context retained source", err)
	}
	if _, err := ReadSiteUnitSummaryWorkbookInput(service, context.Background(), next.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err != nil {
		t.Fatal("failed read leaked lease", err)
	}
}

func TestSiteUnitSummaryWorkbookReadRefusesInvalidAndPartialSources(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	zero := siteUnitSummaryWorkbookInput{}
	for _, method := range []int{0, 3} {
		if input, err := ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, method, vprocore.PublicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(input, zero) {
			t.Fatal("invalid method accepted", method, err)
		}
	}
	if _, err := ReadSiteUnitSummaryWorkbookInput(service, nil, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil {
		t.Fatal("nil context accepted")
	}
	for _, unavailable := range []*vprocore.ContextService{nil, {}} {
		if _, err := ReadSiteUnitSummaryWorkbookInput(unavailable, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil {
			t.Fatal("unavailable service accepted")
		}
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.Sqlite.Attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`ALTER TABLE MasterSiteUnitList RENAME COLUMN SiteSeries TO UnavailableField`)
	if err = errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	input, err := ReadSiteUnitSummaryWorkbookInput(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("incomplete source produced partial workbook input", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	input, err = ReadSiteUnitSummaryWorkbookInput(service, context.Background(), next.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !strings.Contains(err.Error(), "selected normal SU") || !reflect.DeepEqual(input, zero) {
		t.Fatal("None became whole-project scope", err)
	}
}
