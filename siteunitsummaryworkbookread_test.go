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

func TestSiteUnitSummaryWorkbookReadOwnedDetachedAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := reportServiceFixture(t, external)
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			input, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
			if err != nil || len(input.Tables) != 4 {
				t.Fatal("complete owned input unavailable", err)
			}
			service.siteUnitSummaryEnabled = true
			expected, err := service.PreviewSiteUnitSummary(context.Background(), state.ContextID, SiteUnitSummaryRequest{method})
			if err != nil || !reflect.DeepEqual(expected, input.Preview) {
				t.Fatal("shared source capture changed existing preview", err)
			}
			service.siteUnitSummaryEnabled = false
			raw, err := json.Marshal(input.Tables)
			if err != nil {
				t.Fatal(err)
			}
			input.Tables[0].Rows[0].Cells[0] = metadataText("detached")
			input.Preview.Report.Units[0].Values[0] = "detached"
			input.Preview.Report.Fields[0].Label = "detached"
			again, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
			if err != nil || !reflect.DeepEqual(again.Preview, expected) {
				t.Fatal("input was not detached or incorrectly required preview gate", err)
			}
			actual, err := json.Marshal(again.Tables)
			if err != nil || !bytes.Equal(raw, actual) {
				t.Fatal("raw source evidence was not detached", err)
			}
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("private source capture wrote configuration", err)
		}
	}
}

func TestSiteUnitSummaryWorkbookReadCancellationCleanupOwnershipAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	owner, zero := service.projects.sqlite, siteUnitSummaryWorkbookInput{}
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	input, err := service.readSiteUnitSummaryWorkbookInput(ctx, state.ContextID, 1, publicationReadSnapshotHooks{})
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(input, zero) {
		t.Fatal("cancelled acquisition produced source", err)
	}
	sentinel := errors.New("summary workbook completion/cleanup failure")
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return sentinel }},
		{rollbackRead: func(*sql.Tx) error { return sentinel }},
	} {
		input, err = service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, 1, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(input, zero) {
			t.Fatal("failed completion/cleanup retained source", err)
		}
	}
	late, cancelLate := context.WithCancel(context.Background())
	input, err = service.readSiteUnitSummaryWorkbookInput(late, state.ContextID, 1, publicationReadSnapshotHooks{
		commitRead: func(*sql.Tx) error { cancelLate(); return nil },
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(input, zero) {
		t.Fatal("late cancellation retained source", err)
	}
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	input, err = service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	owner.attachmentInfo["su"] = original
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("foreign source attachment accepted", err)
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	input, err = service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("stale context retained source", err)
	}
	if _, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), next.ContextID, 1, publicationReadSnapshotHooks{}); err != nil {
		t.Fatal("failed read leaked lease", err)
	}
}

func TestSiteUnitSummaryWorkbookReadRefusesInvalidAndPartialSources(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	zero := siteUnitSummaryWorkbookInput{}
	for _, method := range []int{0, 3} {
		if input, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(input, zero) {
			t.Fatal("invalid method accepted", method, err)
		}
	}
	if _, err := service.readSiteUnitSummaryWorkbookInput(nil, state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil {
		t.Fatal("nil context accepted")
	}
	for _, unavailable := range []*ContextService{nil, {}} {
		if _, err := unavailable.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil {
			t.Fatal("unavailable service accepted")
		}
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`ALTER TABLE MasterSiteUnitList RENAME COLUMN SiteSeries TO UnavailableField`)
	if err = errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	input, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("incomplete source produced partial workbook input", err)
	}
	assertProfileSUFiles(t, service, before)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	input, err = service.readSiteUnitSummaryWorkbookInput(context.Background(), next.ContextID, 1, publicationReadSnapshotHooks{})
	if err == nil || !strings.Contains(err.Error(), "selected normal SU") || !reflect.DeepEqual(input, zero) {
		t.Fatal("None became whole-project scope", err)
	}
}
