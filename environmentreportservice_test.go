package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func reportServiceFixture(t *testing.T, external bool) (*ContextService, ProjectState) {
	t.Helper()
	service, state := contextServiceFixture(t)
	path := state.ProjectPath
	if external {
		path = filepath.Join(t.TempDir(), "external.db")
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Report_SU(PlotNumber TEXT,SiteUnit TEXT);
		INSERT INTO Report_SU VALUES('108050','  Unit ''quoted''  '),('missing','Orphan'),('108050x',NULL)`)
	err = errors.Join(err, db.Close())
	if err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "Report", path
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	return service, next
}

func TestLongEnvironmentPreviewOwnedPhysicalScopeAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			options, err := service.GetLongEnvironmentOptions(context.Background(), state.ContextID)
			if err != nil || options.ContextID != state.ContextID || options.Title != "Long Evironment Report" {
				t.Fatal("original YAML default unavailable", options, err)
			}
			preview, err := service.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{"  Custom title  "})
			if err != nil || preview.ContextID != state.ContextID || preview.ProjectPath != state.ProjectPath || preview.SUPath != state.SUPath {
				t.Fatal("owned readonly preview failed", preview, err)
			}
			if len(preview.Report.Fields) != 72 || len(preview.Report.Units) != 2 || preview.Report.Title != "  Custom title  " ||
				preview.Report.Units[0].Code != "  Unit 'quoted'  " || len(preview.Report.Units[0].Plots) != 1 ||
				preview.Report.Units[0].Plots[0].PlotNumber != "108050" || preview.Report.Units[1].Plots[0].Status != "missing_env_and_admin" {
				t.Fatal("selected physical SU/orphan scope differs", preview)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("preview/options changed configuration", err)
			}
		})
	}
}

func TestLongEnvironmentPreviewCancellationStalenessAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	preview, err := service.PreviewLongEnvironment(ctx, state.ContextID, LongEnvironmentRequest{"Title"})
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(preview, LongEnvironmentPreview{}) {
		t.Fatal("cancelled read publishes output or loses cancellation", preview, err)
	}
	if _, err := service.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err != nil {
		t.Fatal("cancelled report leaked lease", err)
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err == nil {
		t.Fatal("stale preview published")
	}
	if _, err := service.GetLongEnvironmentOptions(context.Background(), state.ContextID); err == nil {
		t.Fatal("stale options published")
	}
	if _, err := service.PreviewLongEnvironment(context.Background(), next.ContextID, LongEnvironmentRequest{"Title"}); err != nil {
		t.Fatal(err)
	}
}

func TestLongEnvironmentPreviewStrictTransportAndConfiguredTitle(t *testing.T) {
	for _, raw := range []string{`{}`, `{"title":null}`, `{"title":"\ud800"}`, `{"title":"ok","quality":"Poor"}`} {
		var request LongEnvironmentRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("incomplete/repaired/unsupported transport accepted", raw)
		}
	}
	service, state := reportServiceFixture(t, false)
	for _, title := range []string{"\x00", string([]byte{0xff})} {
		if _, err := service.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{title}); err == nil {
			t.Fatal("invalid title repaired", title)
		}
	}
	if err := service.projects.preferences.update("ReportOptions", map[string]any{"LEReportTitle": "  Remembered title  "}); err != nil {
		t.Fatal(err)
	}
	options, err := service.GetLongEnvironmentOptions(context.Background(), state.ContextID)
	if err != nil || options.Title != "  Remembered title  " {
		t.Fatal("existing YAML title discarded", options, err)
	}
	if err := service.projects.preferences.update("ReportOptions", map[string]any{"LEReportTitle": 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetLongEnvironmentOptions(context.Background(), state.ContextID); err == nil {
		t.Fatal("invalid configured title silently defaulted")
	}
}

func TestLongEnvironmentPreviewRequiresSelectedAndUnchangedOwnedFiles(t *testing.T) {
	service, state := contextServiceFixture(t)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewLongEnvironment(context.Background(), next.ContextID, LongEnvironmentRequest{"Title"}); err == nil {
		t.Fatal("no SU silently became all project plots")
	}
	service, state = reportServiceFixture(t, true)
	owner := service.projects.sqlite
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	before := databaseBytes(t, owner.attachments)
	if preview, err := service.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err == nil ||
		!reflect.DeepEqual(preview, LongEnvironmentPreview{}) {
		t.Fatal("changed file ownership published report", preview, err)
	}
	owner.attachmentInfo["su"] = original
	assertProfileSUFiles(t, service, before)
	if _, err := service.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err != nil {
		t.Fatal("rejected ownership read leaked lease", err)
	}
}
