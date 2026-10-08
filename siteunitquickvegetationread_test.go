package main

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
		service, state := reportServiceFixture(t, external)
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			input, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
			if err != nil || len(input.Tables) != 7 || len(input.Quick.Entries) == 0 || len(input.References.Table.Rows) == 0 {
				t.Fatal("complete owned preparation unavailable", input, err)
			}
			if len(input.Lifeform) != len(input.Environment.Report.Units) || len(input.Lifeform[0].Rows) != 14 {
				t.Fatal("owned source did not include complete fourteen-lifeform scalar preparation")
			}
			environment, err := service.readSiteUnitSummaryWorkbookInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
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
			input.Tables[0].Rows[0].Cells[0] = metadataText("changed")
			input.Environment.Report.Units[0].Values[0] = "changed"
			again, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(again)
			if err != nil || !bytes.Equal(raw, actual) {
				t.Fatal("snapshot/preparation was borrowed or nondeterministic", err)
			}
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("private source capture wrote configuration", err)
		}
	}
}

func TestSummaryQuickVegetationOwnedCancellationCleanupOwnershipAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	owner, zero := service.projects.sqlite, siteUnitQuickVegetationInput{}
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	input, err := service.readSiteUnitQuickVegetationInput(ctx, state.ContextID, 1, publicationReadSnapshotHooks{})
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(input, zero) {
		t.Fatal("cancelled acquisition retained partial source", err)
	}
	sentinel := errors.New("QuickVeg completion/cleanup failure")
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return sentinel }},
		{rollbackRead: func(*sql.Tx) error { return sentinel }},
	} {
		input, err = service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(input, zero) {
			t.Fatal("failed completion/cleanup retained source", err)
		}
	}
	late, cancelLate := context.WithCancel(context.Background())
	input, err = service.readSiteUnitQuickVegetationInput(late, state.ContextID, 1, publicationReadSnapshotHooks{
		commitRead: func(*sql.Tx) error { cancelLate(); return nil },
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(input, zero) {
		t.Fatal("late cancellation retained source", err)
	}
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	input, err = service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	owner.attachmentInfo["su"] = original
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("foreign attachment accepted", err)
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if input, err = service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("stale context accepted", err)
	}
	if _, err := service.readSiteUnitQuickVegetationInput(context.Background(), next.ContextID, 1, publicationReadSnapshotHooks{}); err != nil {
		t.Fatal("failed read leaked lease", err)
	}
}

func TestSummaryQuickVegetationOwnedRefusesIncompleteReferenceHalf(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	zero := siteUnitQuickVegetationInput{}
	for _, method := range []int{0, 3} {
		if got, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(got, zero) {
			t.Fatal("invalid method accepted", method, err)
		}
	}
	if got, err := service.readSiteUnitQuickVegetationInput(nil, state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(got, zero) {
		t.Fatal("missing context accepted", err)
	}
	owner := service.projects.sqlite
	mutateContextFixture(t, owner.attachments["VUser"], `ALTER TABLE USysUserSpp RENAME COLUMN LifeForm TO MissingLifeform`)
	if got, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil || !reflect.DeepEqual(got, zero) {
		t.Fatal("reference failure published successful QuickVeg/environment half", err)
	}
	mutateContextFixture(t, owner.attachments["VUser"], `ALTER TABLE USysUserSpp RENAME COLUMN MissingLifeform TO LifeForm`)
	if _, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err != nil {
		t.Fatal("corrected reference retry failed", err)
	}
}
