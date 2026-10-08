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

func TestLifeformWorkbookReadCombinedOwnedSnapshotZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state := reportServiceFixture(t, external)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		input, err := contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{})
		if err != nil {
			t.Fatal(err)
		}
		lifeform, err := enabledLifeformService(t, contexts).Preview(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		attributes, err := enabledAttributeSummary(t, contexts).Preview(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(input.Lifeform, lifeform) || !reflect.DeepEqual(input.Attributes, attributes) ||
			!reflect.DeepEqual(input.Lifeform.Report.Memberships, input.Attributes.Report.Memberships) || len(input.Tables) != 5 {
			t.Fatal("combined captured input differs from accepted source planners")
		}
		originalTables, err := json.Marshal(input.Tables)
		if err != nil {
			t.Fatal(err)
		}
		input.Lifeform.Report.Catalogue[0].Label = metadataText("detached")
		input.Attributes.Report.Definitions[0].Categories[0] = "detached"
		input.Tables[0].Rows[0].Cells[0] = metadataText("detached")
		again, err := contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{})
		if err != nil || !reflect.DeepEqual(again.Lifeform, lifeform) || !reflect.DeepEqual(again.Attributes, attributes) {
			t.Fatal("combined input not detached", err)
		}
		againTables, err := json.Marshal(again.Tables)
		if err != nil || !bytes.Equal(originalTables, againTables) {
			t.Fatal("raw approval evidence did not detach", err)
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("combined read changed configuration", err)
		}
	}
}

func TestLifeformWorkbookReadPreparesCompleteOwnedSource(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state := reportServiceFixture(t, external)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		input, err := contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{})
		if err != nil {
			t.Fatal(err)
		}
		workbook, err := prepareLifeformSummaryWorkbook(context.Background(), input.Lifeform, input.Attributes,
			lifeformWorkbookLayout{Details: [6]bool{true, true, true, true, true, true}})
		if err != nil || len(workbook.Sheets) != len(input.Lifeform.Report.Units) {
			t.Fatal("owned source cannot produce complete workbook", err)
		}
		book := openLifeformWorkbook(t, workbook)
		for i, sheet := range workbook.Sheets {
			if !reflect.DeepEqual(sheet.Unit, input.Lifeform.Report.Units[i].Code) {
				t.Fatal("owned worksheet mapping changed unit identity", sheet)
			}
			lifeformWorkbookCell(t, book, sheet.Name, "A1", "Project: "+state.ActiveProject)
			lifeformWorkbookCell(t, book, sheet.Name, "A2", "Site Unit Table: "+state.ActiveSU)
			for j, row := range input.Attributes.Report.Units[i].Rows {
				value := ""
				if row.Count != nil {
					value = strconv.Itoa(*row.Count)
				}
				lifeformWorkbookCell(t, book, sheet.Name, "B"+strconv.Itoa(j+10), value)
				lifeformWorkbookCell(t, book, sheet.Name, "C"+strconv.Itoa(j+10), strconv.Itoa(row.PlotOccurrences))
			}
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestLifeformWorkbookReadCancellationOwnershipCompletionAndRetry(t *testing.T) {
	contexts, state := reportServiceFixture(t, true)
	owner := contexts.projects.sqlite
	zero := lifeformWorkbookInput{}
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	input, err := contexts.readLifeformWorkbookInput(ctx, state.ContextID, publicationReadSnapshotHooks{})
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(input, zero) {
		t.Fatal("cancelled combined read published", input, err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("completion failed") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("cleanup failed") }},
	} {
		input, err := contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, hooks)
		if err == nil || !reflect.DeepEqual(input, zero) {
			t.Fatal("failed completion or cleanup published partial input", input, err)
		}
	}
	late, cancelLate := context.WithCancel(context.Background())
	input, err = contexts.readLifeformWorkbookInput(late, state.ContextID, publicationReadSnapshotHooks{
		commitRead: func(*sql.Tx) error { cancelLate(); return nil },
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(input, zero) {
		t.Fatal("late cancellation published combined input", input, err)
	}
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	input, err = contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(input, zero) {
		t.Fatal("foreign file ownership published combined input", input, err)
	}
	owner.attachmentInfo["su"] = original
	next, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if input, err = contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{}); err == nil ||
		!reflect.DeepEqual(input, zero) {
		t.Fatal("stale context published combined input", input, err)
	}
	if _, err := contexts.readLifeformWorkbookInput(context.Background(), next.ContextID, publicationReadSnapshotHooks{}); err != nil {
		t.Fatal("combined read refusal leaked lease", err)
	}
	if _, err := contexts.readLifeformWorkbookInput(nil, next.ContextID, publicationReadSnapshotHooks{}); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestLifeformWorkbookReadNoPartialSourceAndScopeRefusal(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	owner := contexts.projects.sqlite
	mutate := func(role, statement string) {
		t.Helper()
		db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(statement)
		if err = errors.Join(err, db.Close()); err != nil {
			t.Fatal(err)
		}
	}
	mutate("VPro64", `DROP TABLE LayerCode`)
	if _, err := contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{}); err != nil {
		t.Fatal("combined source incorrectly requires Long Vegetation layers", err)
	}
	mutate("VLists", `ALTER TABLE USysSppAttributes RENAME COLUMN Wetland_Ind TO UnavailableField`)
	before := databaseBytes(t, owner.attachments)
	input, err := contexts.readLifeformWorkbookInput(context.Background(), state.ContextID, publicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(input, lifeformWorkbookInput{}) {
		t.Fatal("valid Lifeform half masked unavailable attribute half", input, err)
	}
	assertProfileSUFiles(t, contexts, before)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	input, err = contexts.readLifeformWorkbookInput(context.Background(), next.ContextID, publicationReadSnapshotHooks{})
	if err == nil || !strings.Contains(err.Error(), "selected normal SU") || !reflect.DeepEqual(input, lifeformWorkbookInput{}) {
		t.Fatal("missing SU became all-project or partial report", input, err)
	}
}
