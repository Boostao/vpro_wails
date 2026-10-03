package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

func TestScopedPlotLookupExactLiteralScopeAndNoWrites(t *testing.T) {
	service, state := contextServiceFixture(t)
	ctx := context.Background()
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const literal = " A' # "
	if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES(?); INSERT INTO Sample_Admin(Plot) VALUES(?);
		CREATE TABLE Find_SU(PlotNumber TEXT,SiteUnit TEXT); INSERT INTO Find_SU VALUES(?,NULL)`, literal, literal, literal); err != nil {
		t.Fatal(err)
	}
	for _, number := range []string{"É😀", "historical long plot identifier"} {
		if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES(?); INSERT INTO Sample_Admin(Plot) VALUES(?)`, number, number); err != nil {
			t.Fatal(err)
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, number := range []string{literal, "108050", "108050x", "É😀", "historical long plot identifier"} {
		result, err := service.LookupScopedPlot(ctx, state.ContextID, ScopedPlotLookup{number})
		if err != nil || result.ContextID != state.ContextID || result.Plot.PlotNumber != number {
			t.Fatal("exact literal lookup failed", number, result, err)
		}
	}
	for _, number := range []string{"A' #", "108050X", "", "not present", "\x00", string([]byte{0xff})} {
		if _, err := service.LookupScopedPlot(ctx, state.ContextID, ScopedPlotLookup{number}); err == nil {
			t.Fatal("missing/normalized/malformed lookup accepted", number)
		}
	}
	assertProfileSUFiles(t, service, before)
	next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Find", SUPath: state.ProjectPath, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.LookupScopedPlot(ctx, state.ContextID, ScopedPlotLookup{literal}); err == nil {
		t.Fatal("stale context found a plot")
	}
	if _, err := service.LookupScopedPlot(ctx, next.ContextID, ScopedPlotLookup{"108050"}); err == nil {
		t.Fatal("lookup escaped selected SU")
	}
	if result, err := service.LookupScopedPlot(ctx, next.ContextID, ScopedPlotLookup{literal}); err != nil || result.Plot.PlotNumber != literal {
		t.Fatal("selected SU literal lookup failed", result, err)
	}
	assertProfileSUFiles(t, service, before)
	current, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || bytes.Equal(config, current) {
		t.Fatal("explicit scope switch did not persist separately", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.LookupScopedPlot(ctx, next.ContextID, ScopedPlotLookup{literal}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled lookup lost cancellation", err)
	}
}

func TestScopedPlotLookupCancellationWhileCoordinatorIsBusy(t *testing.T) {
	service, state := contextServiceFixture(t)
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := service.LookupScopedPlot(ctx, state.ContextID, ScopedPlotLookup{"108050"})
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("queued lookup ignored cancellation or lost its cause", err)
	}
	if _, err := service.LookupScopedPlot(context.Background(), state.ContextID, ScopedPlotLookup{"108050"}); err != nil {
		t.Fatal("cancelled lookup leaked an ownership lease", err)
	}
}

func TestScopedPlotLookupRejectsAmbiguityAndIncompleteTransport(t *testing.T) {
	service, state := contextServiceFixture(t)
	for _, raw := range []string{`{}`, `{"plotNumber":null}`, `{"plotNumber":"\ud800"}`,
		`{"plotNumber":"108050","allProjects":true}`} {
		var request ScopedPlotLookup
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("partial/repaired/unsupported find transport accepted", raw)
		}
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DROP INDEX uidx_Sample_Admin_PlotNumber`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Admin(Plot) VALUES('108050')`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if _, err := service.LookupScopedPlot(context.Background(), state.ContextID, ScopedPlotLookup{"108050"}); err == nil {
		t.Fatal("identical Admin duplicates were collapsed into an apparently unique editor target")
	}
	assertProfileSUFiles(t, service, before)
}
